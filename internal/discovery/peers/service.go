package peers

import (
	"slices"
	"sync"
	"time"
)

type Peer struct {
	NodeID    string    `json:"node_id"`
	Address   string    `json:"address"`
	LastSeen  time.Time `json:"last_seen"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Config struct {
	Bootstrap []Peer
	TTL       time.Duration
}

type Metrics struct {
	BootstrapPeers int `json:"bootstrap_peers"`
	KnownPeers     int `json:"known_peers"`
	AddedPeers     int `json:"added_peers"`
	UpdatedPeers   int `json:"updated_peers"`
	EvictedPeers   int `json:"evicted_peers"`
}

type Service struct {
	mu      sync.RWMutex
	peers   map[string]Peer
	ttl     time.Duration
	metrics Metrics
	now     func() time.Time
}

func NewService(cfg Config) *Service {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}

	service := &Service{
		peers: map[string]Peer{},
		ttl:   ttl,
		now:   time.Now,
	}

	for _, peer := range cfg.Bootstrap {
		service.upsertLocked(peer.NodeID, peer.Address, true)
	}
	service.metrics.BootstrapPeers = len(cfg.Bootstrap)
	service.metrics.KnownPeers = len(service.peers)

	return service
}

func (s *Service) Upsert(peer Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upsertLocked(peer.NodeID, peer.Address, false)
}

func (s *Service) Merge(peers []Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, peer := range peers {
		s.upsertLocked(peer.NodeID, peer.Address, false)
	}
}

func (s *Service) Discover(limit int) []Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]Peer, 0, len(s.peers))
	for _, peer := range s.peers {
		list = append(list, peer)
	}

	slices.SortFunc(list, func(a, b Peer) int {
		if a.NodeID < b.NodeID {
			return -1
		}
		if a.NodeID > b.NodeID {
			return 1
		}
		return 0
	})

	if limit <= 0 || limit >= len(list) {
		return list
	}
	return list[:limit]
}

func (s *Service) EvictExpired() []Peer {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	evicted := make([]Peer, 0)

	for nodeID, peer := range s.peers {
		if peer.ExpiresAt.After(now) {
			continue
		}

		evicted = append(evicted, peer)
		delete(s.peers, nodeID)
		s.metrics.EvictedPeers++
	}

	slices.SortFunc(evicted, func(a, b Peer) int {
		if a.NodeID < b.NodeID {
			return -1
		}
		if a.NodeID > b.NodeID {
			return 1
		}
		return 0
	})
	s.metrics.KnownPeers = len(s.peers)

	return evicted
}

func (s *Service) Metrics() Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

func (s *Service) upsertLocked(nodeID string, address string, bootstrap bool) {
	if nodeID == "" || address == "" {
		return
	}

	now := s.now().UTC()
	peer := Peer{
		NodeID:    nodeID,
		Address:   address,
		LastSeen:  now,
		ExpiresAt: now.Add(s.ttl),
	}

	if _, exists := s.peers[nodeID]; exists {
		s.peers[nodeID] = peer
		if !bootstrap {
			s.metrics.UpdatedPeers++
		}
	} else {
		s.peers[nodeID] = peer
		if !bootstrap {
			s.metrics.AddedPeers++
		}
	}

	s.metrics.KnownPeers = len(s.peers)
}
