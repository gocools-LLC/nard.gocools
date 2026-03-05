package routing

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

var ErrNoHealthyPeers = errors.New("no healthy peers for service")

const defaultClockSkewTolerance = 5 * time.Second

type HeartbeatMessage struct {
	NodeID    string    `json:"node_id"`
	Address   string    `json:"address"`
	Services  []string  `json:"services"`
	Sequence  uint64    `json:"sequence"`
	Timestamp time.Time `json:"timestamp"`
}

func (m HeartbeatMessage) Validate() error {
	if strings.TrimSpace(m.NodeID) == "" {
		return errors.New("node_id is required")
	}
	if strings.TrimSpace(m.Address) == "" {
		return errors.New("address is required")
	}
	if len(normalizeServices(m.Services)) == 0 {
		return errors.New("at least one service is required")
	}
	return nil
}

func EncodeHeartbeat(msg HeartbeatMessage) ([]byte, error) {
	msg.Services = normalizeServices(msg.Services)
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(msg)
}

func DecodeHeartbeat(payload []byte) (HeartbeatMessage, error) {
	var msg HeartbeatMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return HeartbeatMessage{}, err
	}
	msg.Services = normalizeServices(msg.Services)
	if err := msg.Validate(); err != nil {
		return HeartbeatMessage{}, err
	}
	return msg, nil
}

type Config struct {
	HeartbeatTTL       time.Duration
	ClockSkewTolerance time.Duration
}

type Candidate struct {
	NodeID        string    `json:"node_id"`
	Address       string    `json:"address"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

type Decision struct {
	Service    string      `json:"service"`
	Selected   *Candidate  `json:"selected,omitempty"`
	Candidates []Candidate `json:"candidates"`
	Strategy   string      `json:"strategy"`
	Reason     string      `json:"reason"`
}

type Metrics struct {
	HeartbeatsReceived int `json:"heartbeats_received"`
	KnownNodes         int `json:"known_nodes"`
	NodesEvicted       int `json:"nodes_evicted"`
	RouteLookups       int `json:"route_lookups"`
	RouteMisses        int `json:"route_misses"`
	Failovers          int `json:"failovers"`
}

type nodeState struct {
	NodeID        string
	Address       string
	Services      []string
	Sequence      uint64
	LastHeartbeat time.Time
	ExpiresAt     time.Time
}

type Service struct {
	mu           sync.RWMutex
	nodes        map[string]nodeState
	serviceIndex map[string]map[string]struct{}
	rrCursor     map[string]int
	lastRoute    map[string]string
	ttl          time.Duration
	clockSkew    time.Duration
	metrics      Metrics
	now          func() time.Time
}

func NewService(cfg Config) *Service {
	ttl := cfg.HeartbeatTTL
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	clockSkewTolerance := cfg.ClockSkewTolerance
	if clockSkewTolerance <= 0 {
		clockSkewTolerance = defaultClockSkewTolerance
	}

	return &Service{
		nodes:        map[string]nodeState{},
		serviceIndex: map[string]map[string]struct{}{},
		rrCursor:     map[string]int{},
		lastRoute:    map[string]string{},
		ttl:          ttl,
		clockSkew:    clockSkewTolerance,
		now:          time.Now,
	}
}

func (s *Service) ObserveHeartbeat(msg HeartbeatMessage) error {
	msg.Services = normalizeServices(msg.Services)
	if err := msg.Validate(); err != nil {
		return err
	}

	now := s.now().UTC()
	lastHeartbeat := normalizeHeartbeatTimestamp(msg.Timestamp.UTC(), now, s.clockSkew)

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.nodes[msg.NodeID]
	if exists && msg.Sequence > 0 && msg.Sequence < existing.Sequence {
		return nil
	}

	if exists {
		s.removeNodeFromServiceIndexLocked(existing)
	}

	updated := nodeState{
		NodeID:        msg.NodeID,
		Address:       msg.Address,
		Services:      msg.Services,
		Sequence:      msg.Sequence,
		LastHeartbeat: lastHeartbeat,
		ExpiresAt:     maxTime(lastHeartbeat, now).Add(s.ttl),
	}
	s.nodes[msg.NodeID] = updated
	s.addNodeToServiceIndexLocked(updated)

	s.metrics.HeartbeatsReceived++
	s.metrics.KnownNodes = len(s.nodes)
	return nil
}

func (s *Service) ObserveHeartbeatPayload(payload []byte) error {
	msg, err := DecodeHeartbeat(payload)
	if err != nil {
		return err
	}
	return s.ObserveHeartbeat(msg)
}

func (s *Service) SweepUnhealthy() []Candidate {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	evicted := make([]Candidate, 0)

	for nodeID, state := range s.nodes {
		if state.ExpiresAt.After(now) {
			continue
		}

		evicted = append(evicted, candidateFromState(state))
		s.removeNodeFromServiceIndexLocked(state)
		delete(s.nodes, nodeID)
	}

	slices.SortFunc(evicted, compareCandidatesByNodeID)
	s.metrics.NodesEvicted += len(evicted)
	s.metrics.KnownNodes = len(s.nodes)
	return evicted
}

func (s *Service) Routes(service string) []Candidate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.routesLocked(service, s.now().UTC())
}

func (s *Service) Route(service string, avoidNodeID string) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metrics.RouteLookups++
	trimmedService := strings.TrimSpace(service)
	decision := Decision{
		Service:  trimmedService,
		Strategy: "round_robin_failover",
	}
	if trimmedService == "" {
		s.metrics.RouteMisses++
		decision.Reason = "service name is required"
		return decision, errors.New("service name is required")
	}

	now := s.now().UTC()
	candidates := s.routesLocked(trimmedService, now)
	decision.Candidates = candidates
	if len(candidates) == 0 {
		s.metrics.RouteMisses++
		decision.Reason = "no healthy peers available for service"
		return decision, fmt.Errorf("%w: %s", ErrNoHealthyPeers, trimmedService)
	}

	pool := candidates
	if avoidNodeID != "" {
		filtered := make([]Candidate, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.NodeID == avoidNodeID {
				continue
			}
			filtered = append(filtered, candidate)
		}
		if len(filtered) > 0 {
			pool = filtered
			decision.Reason = "excluded avoid node from healthy candidates"
		}
	}

	cursor := s.rrCursor[trimmedService]
	selected := pool[cursor%len(pool)]
	s.rrCursor[trimmedService] = (cursor + 1) % len(pool)
	decision.Selected = &selected

	previous := s.lastRoute[trimmedService]
	if previous != "" && previous != selected.NodeID && !containsNode(candidates, previous) {
		s.metrics.Failovers++
		if decision.Reason == "" {
			decision.Reason = "previous route became unhealthy; failover selected"
		} else {
			decision.Reason += "; previous route became unhealthy"
		}
	}

	if decision.Reason == "" {
		switch len(pool) {
		case 1:
			decision.Reason = "single healthy peer available"
		default:
			decision.Reason = "round-robin across healthy peers"
		}
	}

	s.lastRoute[trimmedService] = selected.NodeID
	return decision, nil
}

func (s *Service) Metrics() Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

func (s *Service) routesLocked(service string, now time.Time) []Candidate {
	nodes, exists := s.serviceIndex[service]
	if !exists || len(nodes) == 0 {
		return nil
	}

	candidates := make([]Candidate, 0, len(nodes))
	for nodeID := range nodes {
		state, ok := s.nodes[nodeID]
		if !ok || !state.ExpiresAt.After(now) {
			continue
		}
		candidates = append(candidates, candidateFromState(state))
	}

	slices.SortFunc(candidates, compareCandidatesByNodeID)
	return candidates
}

func (s *Service) addNodeToServiceIndexLocked(state nodeState) {
	for _, service := range state.Services {
		if _, exists := s.serviceIndex[service]; !exists {
			s.serviceIndex[service] = map[string]struct{}{}
		}
		s.serviceIndex[service][state.NodeID] = struct{}{}
	}
}

func (s *Service) removeNodeFromServiceIndexLocked(state nodeState) {
	for _, service := range state.Services {
		nodes, exists := s.serviceIndex[service]
		if !exists {
			continue
		}
		delete(nodes, state.NodeID)
		if len(nodes) == 0 {
			delete(s.serviceIndex, service)
		}
	}
}

func candidateFromState(state nodeState) Candidate {
	return Candidate{
		NodeID:        state.NodeID,
		Address:       state.Address,
		LastHeartbeat: state.LastHeartbeat,
	}
}

func compareCandidatesByNodeID(a, b Candidate) int {
	if a.NodeID < b.NodeID {
		return -1
	}
	if a.NodeID > b.NodeID {
		return 1
	}
	return 0
}

func containsNode(candidates []Candidate, nodeID string) bool {
	for _, candidate := range candidates {
		if candidate.NodeID == nodeID {
			return true
		}
	}
	return false
}

func normalizeServices(services []string) []string {
	if len(services) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(services))
	for _, service := range services {
		trimmed := strings.TrimSpace(service)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}

	if len(set) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(set))
	for service := range set {
		normalized = append(normalized, service)
	}
	slices.Sort(normalized)
	return normalized
}

func normalizeHeartbeatTimestamp(timestamp time.Time, observedAt time.Time, clockSkewTolerance time.Duration) time.Time {
	if timestamp.IsZero() {
		return observedAt
	}
	if timestamp.Before(observedAt) {
		return observedAt
	}

	maxAllowed := observedAt.Add(clockSkewTolerance)
	if timestamp.After(maxAllowed) {
		return maxAllowed
	}
	return timestamp
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
