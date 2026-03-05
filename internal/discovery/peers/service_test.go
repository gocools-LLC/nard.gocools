package peers

import (
	"testing"
	"time"
)

func TestBootstrapPeersLoaded(t *testing.T) {
	service := NewService(Config{
		Bootstrap: []Peer{
			{NodeID: "node-a", Address: "10.0.0.1:9000"},
			{NodeID: "node-b", Address: "10.0.0.2:9000"},
		},
		TTL: 5 * time.Minute,
	})

	discovered := service.Discover(0)
	if len(discovered) != 2 {
		t.Fatalf("expected 2 bootstrap peers, got %d", len(discovered))
	}

	metrics := service.Metrics()
	if metrics.BootstrapPeers != 2 || metrics.KnownPeers != 2 {
		t.Fatalf("unexpected bootstrap metrics: %+v", metrics)
	}
}

func TestPeerTableMaintenanceAndMetrics(t *testing.T) {
	service := NewService(Config{TTL: 5 * time.Minute})
	current := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return current }

	service.Upsert(Peer{NodeID: "node-a", Address: "10.0.0.1:9000"})
	current = current.Add(1 * time.Minute)
	service.Upsert(Peer{NodeID: "node-a", Address: "10.0.0.1:9000"})
	service.Upsert(Peer{NodeID: "node-b", Address: "10.0.0.2:9000"})

	metrics := service.Metrics()
	if metrics.AddedPeers != 2 || metrics.UpdatedPeers != 1 || metrics.KnownPeers != 2 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
}

func TestTTLEviction(t *testing.T) {
	service := NewService(Config{TTL: 2 * time.Minute})
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }

	service.Upsert(Peer{NodeID: "node-a", Address: "10.0.0.1:9000"})
	service.Upsert(Peer{NodeID: "node-b", Address: "10.0.0.2:9000"})

	base = base.Add(3 * time.Minute)
	evicted := service.EvictExpired()
	if len(evicted) != 2 {
		t.Fatalf("expected 2 evicted peers, got %d", len(evicted))
	}

	metrics := service.Metrics()
	if metrics.EvictedPeers != 2 || metrics.KnownPeers != 0 {
		t.Fatalf("unexpected eviction metrics: %+v", metrics)
	}
}

func TestMultiNodeConvergence(t *testing.T) {
	nodeA := NewService(Config{
		Bootstrap: []Peer{
			{NodeID: "node-b", Address: "10.0.0.2:9000"},
		},
		TTL: 5 * time.Minute,
	})
	nodeB := NewService(Config{
		Bootstrap: []Peer{
			{NodeID: "node-a", Address: "10.0.0.1:9000"},
		},
		TTL: 5 * time.Minute,
	})

	nodeA.Upsert(Peer{NodeID: "node-a", Address: "10.0.0.1:9000"})
	nodeB.Upsert(Peer{NodeID: "node-b", Address: "10.0.0.2:9000"})

	nodeA.Merge(nodeB.Discover(0))
	nodeB.Merge(nodeA.Discover(0))

	peersA := nodeA.Discover(0)
	peersB := nodeB.Discover(0)
	if len(peersA) != 2 || len(peersB) != 2 {
		t.Fatalf("expected convergence to 2 peers; A=%d B=%d", len(peersA), len(peersB))
	}
}
