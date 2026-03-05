package routing

import (
	"errors"
	"testing"
	"time"
)

func TestHeartbeatEncodeDecode(t *testing.T) {
	ts := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	msg := HeartbeatMessage{
		NodeID:    "node-a",
		Address:   "[2001:db8::1]:9000",
		Services:  []string{"checkout", "checkout", "search"},
		Sequence:  9,
		Timestamp: ts,
	}

	payload, err := EncodeHeartbeat(msg)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := DecodeHeartbeat(payload)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.NodeID != "node-a" || decoded.Address != "[2001:db8::1]:9000" {
		t.Fatalf("unexpected decoded identity: %+v", decoded)
	}
	if len(decoded.Services) != 2 || decoded.Services[0] != "checkout" || decoded.Services[1] != "search" {
		t.Fatalf("unexpected decoded services: %+v", decoded.Services)
	}
	if decoded.Sequence != 9 {
		t.Fatalf("unexpected sequence: %d", decoded.Sequence)
	}
}

func TestUnhealthyPeersRemovedFromRouting(t *testing.T) {
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	service := NewService(Config{HeartbeatTTL: 30 * time.Second})
	service.now = func() time.Time { return base }

	if err := service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    "node-a",
		Address:   "10.0.0.1:9000",
		Services:  []string{"api"},
		Sequence:  1,
		Timestamp: base,
	}); err != nil {
		t.Fatalf("observe node-a failed: %v", err)
	}
	if err := service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    "node-b",
		Address:   "10.0.0.2:9000",
		Services:  []string{"api"},
		Sequence:  1,
		Timestamp: base,
	}); err != nil {
		t.Fatalf("observe node-b failed: %v", err)
	}

	base = base.Add(20 * time.Second)
	if err := service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    "node-b",
		Address:   "10.0.0.2:9000",
		Services:  []string{"api"},
		Sequence:  2,
		Timestamp: base,
	}); err != nil {
		t.Fatalf("refresh node-b failed: %v", err)
	}

	base = base.Add(15 * time.Second)
	evicted := service.SweepUnhealthy()
	if len(evicted) != 1 || evicted[0].NodeID != "node-a" {
		t.Fatalf("expected node-a eviction, got %+v", evicted)
	}

	routes := service.Routes("api")
	if len(routes) != 1 || routes[0].NodeID != "node-b" {
		t.Fatalf("expected only node-b route, got %+v", routes)
	}
}

func TestFailoverUnderNodeDrop(t *testing.T) {
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	service := NewService(Config{HeartbeatTTL: 10 * time.Second})
	service.now = func() time.Time { return base }

	_ = service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    "node-a",
		Address:   "10.0.0.1:9000",
		Services:  []string{"checkout"},
		Sequence:  1,
		Timestamp: base,
	})
	_ = service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    "node-b",
		Address:   "10.0.0.2:9000",
		Services:  []string{"checkout"},
		Sequence:  1,
		Timestamp: base,
	})

	first, err := service.Route("checkout", "")
	if err != nil {
		t.Fatalf("first route failed: %v", err)
	}
	if first.Selected == nil {
		t.Fatal("first route did not select a node")
	}

	primary := first.Selected.NodeID
	secondary := "node-b"
	if primary == "node-b" {
		secondary = "node-a"
	}

	base = base.Add(6 * time.Second)
	_ = service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    secondary,
		Address:   "10.0.0.2:9000",
		Services:  []string{"checkout"},
		Sequence:  2,
		Timestamp: base,
	})

	base = base.Add(6 * time.Second)
	_ = service.SweepUnhealthy()

	second, err := service.Route("checkout", "")
	if err != nil {
		t.Fatalf("second route failed: %v", err)
	}
	if second.Selected == nil || second.Selected.NodeID != secondary {
		t.Fatalf("expected failover to %s, got %+v", secondary, second.Selected)
	}

	metrics := service.Metrics()
	if metrics.Failovers == 0 {
		t.Fatalf("expected failover metric to increment, got %+v", metrics)
	}
}

func TestRoutingDecisionIsExplainable(t *testing.T) {
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	service := NewService(Config{HeartbeatTTL: 20 * time.Second})
	service.now = func() time.Time { return base }

	_ = service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    "node-a",
		Address:   "10.0.0.1:9000",
		Services:  []string{"search"},
		Sequence:  1,
		Timestamp: base,
	})
	_ = service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    "node-b",
		Address:   "10.0.0.2:9000",
		Services:  []string{"search"},
		Sequence:  1,
		Timestamp: base,
	})

	decision, err := service.Route("search", "node-a")
	if err != nil {
		t.Fatalf("route failed: %v", err)
	}
	if decision.Selected == nil || decision.Selected.NodeID != "node-b" {
		t.Fatalf("expected node-b selection, got %+v", decision.Selected)
	}
	if decision.Strategy == "" || decision.Reason == "" {
		t.Fatalf("decision should include strategy and reason: %+v", decision)
	}
	if len(decision.Candidates) != 2 {
		t.Fatalf("expected full candidate list in decision, got %+v", decision.Candidates)
	}
}

func TestNoHealthyRouteReturnsErrNoHealthyPeers(t *testing.T) {
	service := NewService(Config{HeartbeatTTL: 10 * time.Second})
	decision, err := service.Route("payments", "")
	if err == nil {
		t.Fatal("expected error when no healthy peers exist")
	}
	if !errors.Is(err, ErrNoHealthyPeers) {
		t.Fatalf("expected ErrNoHealthyPeers, got %v", err)
	}
	if decision.Reason == "" {
		t.Fatalf("expected explainable miss reason, got %+v", decision)
	}
}
