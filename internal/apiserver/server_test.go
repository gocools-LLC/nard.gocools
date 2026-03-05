package apiserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gocools-LLC/nard.gocools/internal/runtime/agent"
)

func TestCapabilitiesEndpoint(t *testing.T) {
	nodeAgent := agent.NewAgent(agent.Config{
		NodeID: "node-test",
		Capabilities: agent.Capability{
			CPUCores: 4,
			MemoryMB: 8192,
		},
	})

	handler := New(Config{
		Version: "test-version",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Agent:   nodeAgent,
	}).Handler

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/capabilities", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	var payload agent.Capability
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode capability payload: %v", err)
	}
	if payload.NodeID != "node-test" {
		t.Fatalf("expected node-test, got %q", payload.NodeID)
	}
	if payload.CPUCores != 4 || payload.MemoryMB != 8192 {
		t.Fatalf("unexpected capability payload: %+v", payload)
	}
}

func TestNodeStateEndpoint(t *testing.T) {
	nodeAgent := agent.NewAgent(agent.Config{
		NodeID:            "node-test",
		HeartbeatInterval: 10 * time.Millisecond,
	})
	if err := nodeAgent.Start(context.Background()); err != nil {
		t.Fatalf("agent start failed: %v", err)
	}
	defer nodeAgent.Stop(context.Background())

	handler := New(Config{
		Version: "test-version",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Agent:   nodeAgent,
	}).Handler

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/state", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode state payload: %v", err)
	}
	if payload["state"] != string(agent.StateRunning) {
		t.Fatalf("expected state %q, got %q", agent.StateRunning, payload["state"])
	}
}
