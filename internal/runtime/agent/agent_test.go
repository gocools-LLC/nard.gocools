package agent

import (
	"context"
	"testing"
	"time"
)

func TestAgentStartStopLifecycle(t *testing.T) {
	instance := NewAgent(Config{
		NodeID:            "node-a",
		HeartbeatInterval: 10 * time.Millisecond,
		Capabilities: Capability{
			CPUCores: 4,
			MemoryMB: 8192,
		},
	})

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if state := instance.State(); state != StateRunning {
		t.Fatalf("expected running state, got %s", state)
	}

	if err := instance.Stop(context.Background()); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if state := instance.State(); state != StateStopped {
		t.Fatalf("expected stopped state, got %s", state)
	}
}

func TestWorkloadRegistrationHook(t *testing.T) {
	registeredCh := make(chan Workload, 1)
	instance := NewAgent(Config{
		NodeID: "node-a",
		Hooks: Hooks{
			OnWorkloadRegistered: func(workload Workload) {
				registeredCh <- workload
			},
		},
	})

	workload, err := instance.RegisterWorkload("svc-a", "ghcr.io/gocools/svc-a:latest", map[string]string{"team": "platform"})
	if err != nil {
		t.Fatalf("register workload failed: %v", err)
	}
	if workload.ID != "svc-a" {
		t.Fatalf("expected workload id svc-a, got %q", workload.ID)
	}

	select {
	case hookWorkload := <-registeredCh:
		if hookWorkload.ID != "svc-a" {
			t.Fatalf("unexpected hook workload: %+v", hookWorkload)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for workload hook callback")
	}
}

func TestCapabilityReport(t *testing.T) {
	instance := NewAgent(Config{
		NodeID: "node-a",
		Capabilities: Capability{
			CPUCores: 8,
			MemoryMB: 16384,
			Labels: map[string]string{
				"zone": "edge-1",
			},
		},
	})

	report := instance.CapabilityReport()
	if report.NodeID != "node-a" {
		t.Fatalf("expected node-a in capability report, got %q", report.NodeID)
	}
	if report.CPUCores != 8 || report.MemoryMB != 16384 {
		t.Fatalf("unexpected capability report: %+v", report)
	}
}

func TestHeartbeatEmitted(t *testing.T) {
	instance := NewAgent(Config{
		NodeID:            "node-a",
		HeartbeatInterval: 20 * time.Millisecond,
	})
	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer instance.Stop(context.Background())

	select {
	case heartbeat := <-instance.Heartbeats():
		if heartbeat.NodeID != "node-a" || heartbeat.State != StateRunning {
			t.Fatalf("unexpected heartbeat: %+v", heartbeat)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for heartbeat")
	}
}
