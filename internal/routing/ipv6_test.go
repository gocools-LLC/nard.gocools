package routing

import (
	"testing"
	"time"
)

func TestRoutingDualStackCandidates(t *testing.T) {
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	service := NewService(Config{HeartbeatTTL: 30 * time.Second})
	service.now = func() time.Time { return base }

	_ = service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    "node-v4",
		Address:   "10.0.0.20:9000",
		Services:  []string{"api"},
		Sequence:  1,
		Timestamp: base,
	})
	_ = service.ObserveHeartbeat(HeartbeatMessage{
		NodeID:    "node-v6",
		Address:   "[2001:db8::20]:9000",
		Services:  []string{"api"},
		Sequence:  1,
		Timestamp: base,
	})

	decision, err := service.Route("api", "")
	if err != nil {
		t.Fatalf("route failed: %v", err)
	}
	if decision.Selected == nil {
		t.Fatalf("expected selected candidate, got %+v", decision)
	}
	if len(decision.Candidates) != 2 {
		t.Fatalf("expected 2 dual-stack candidates, got %+v", decision.Candidates)
	}
}
