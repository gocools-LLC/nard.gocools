package registry

import (
	"errors"
	"strings"
	"sync"
	"time"
)

type Record struct {
	Name         string        `json:"name"`
	Target       string        `json:"target"`
	OwnerNodeID  string        `json:"owner_node_id"`
	RegisteredAt time.Time     `json:"registered_at"`
	ExpiresAt    time.Time     `json:"expires_at"`
	TTL          time.Duration `json:"ttl"`
}

type RegisterRequest struct {
	Name        string
	Target      string
	OwnerNodeID string
	TTL         time.Duration
}

type Service struct {
	mu         sync.RWMutex
	records    map[string]Record
	defaultTTL time.Duration
	now        func() time.Time
}

func NewService(defaultTTL time.Duration) *Service {
	if defaultTTL <= 0 {
		defaultTTL = 30 * time.Second
	}

	return &Service{
		records:    map[string]Record{},
		defaultTTL: defaultTTL,
		now:        time.Now,
	}
}

func (s *Service) Register(request RegisterRequest) (Record, error) {
	name := strings.TrimSpace(strings.ToLower(request.Name))
	if name == "" {
		return Record{}, errors.New("name is required")
	}
	if strings.TrimSpace(request.Target) == "" {
		return Record{}, errors.New("target is required")
	}
	if strings.TrimSpace(request.OwnerNodeID) == "" {
		return Record{}, errors.New("owner_node_id is required")
	}

	ttl := request.TTL
	if ttl <= 0 {
		ttl = s.defaultTTL
	}

	now := s.now().UTC()
	candidate := Record{
		Name:         name,
		Target:       request.Target,
		OwnerNodeID:  request.OwnerNodeID,
		RegisteredAt: now,
		ExpiresAt:    now.Add(ttl),
		TTL:          ttl,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.records[name]
	if !exists || isExpired(existing, now) || shouldReplace(existing, candidate) {
		s.records[name] = candidate
		return candidate, nil
	}

	return existing, nil
}

func (s *Service) Resolve(name string) (Record, bool) {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if normalized == "" {
		return Record{}, false
	}

	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.records[normalized]
	if !exists {
		return Record{}, false
	}
	if isExpired(record, now) {
		delete(s.records, normalized)
		return Record{}, false
	}

	return record, true
}

func (s *Service) Snapshot() []Record {
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]Record, 0, len(s.records))
	for name, record := range s.records {
		if isExpired(record, now) {
			delete(s.records, name)
			continue
		}
		records = append(records, record)
	}
	return records
}

func shouldReplace(existing Record, candidate Record) bool {
	if candidate.OwnerNodeID < existing.OwnerNodeID {
		return true
	}
	if candidate.OwnerNodeID > existing.OwnerNodeID {
		return false
	}

	// Same owner: latest registration wins.
	return candidate.RegisteredAt.After(existing.RegisteredAt)
}

func isExpired(record Record, now time.Time) bool {
	return !record.ExpiresAt.After(now)
}
