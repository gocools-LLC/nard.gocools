package registry

import (
	"testing"
	"time"
)

func TestRegisterAndResolve(t *testing.T) {
	service := NewService(30 * time.Second)
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }

	_, err := service.Register(RegisterRequest{
		Name:        "api.service",
		Target:      "10.0.0.10:9000",
		OwnerNodeID: "node-b",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	record, exists := service.Resolve("api.service")
	if !exists {
		t.Fatal("expected record to resolve")
	}
	if record.Target != "10.0.0.10:9000" {
		t.Fatalf("unexpected target: %q", record.Target)
	}
}

func TestConflictResolutionDeterministic(t *testing.T) {
	service := NewService(30 * time.Second)
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }

	_, _ = service.Register(RegisterRequest{
		Name:        "api.service",
		Target:      "10.0.0.10:9000",
		OwnerNodeID: "node-z",
	})
	base = base.Add(1 * time.Second)
	_, _ = service.Register(RegisterRequest{
		Name:        "api.service",
		Target:      "10.0.0.11:9000",
		OwnerNodeID: "node-a",
	})

	record, exists := service.Resolve("api.service")
	if !exists {
		t.Fatal("expected record to resolve")
	}
	if record.OwnerNodeID != "node-a" {
		t.Fatalf("expected node-a to win deterministic conflict, got %q", record.OwnerNodeID)
	}
}

func TestRegistrationTTLExpiry(t *testing.T) {
	service := NewService(5 * time.Second)
	base := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }

	_, _ = service.Register(RegisterRequest{
		Name:        "db.service",
		Target:      "10.0.0.20:5432",
		OwnerNodeID: "node-a",
		TTL:         2 * time.Second,
	})

	base = base.Add(3 * time.Second)
	_, exists := service.Resolve("db.service")
	if exists {
		t.Fatal("expected record to expire by TTL")
	}
}
