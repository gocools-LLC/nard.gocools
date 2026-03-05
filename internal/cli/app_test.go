package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNodeHelpListsLifecycleCommands(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp("test", stdout, stderr)

	code := app.Run(context.Background(), []string{"node", "--help"})
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d stderr=%s", code, stderr.String())
	}

	help := stdout.String()
	for _, expected := range []string{"start", "join", "status"} {
		if !strings.Contains(help, expected) {
			t.Fatalf("expected help to include %q, got:\n%s", expected, help)
		}
	}
}

func TestExitCodesArePredictable(t *testing.T) {
	t.Run("usage errors return exitUsage", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		app := NewApp("test", stdout, stderr)

		code := app.Run(context.Background(), []string{"node", "unknown"})
		if code != exitUsage {
			t.Fatalf("expected exitUsage, got %d", code)
		}
	})

	t.Run("runtime failures return exitRuntime", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		app := NewApp("test", stdout, stderr)

		code := app.Run(context.Background(), []string{
			"node",
			"status",
			"--endpoint",
			"http://127.0.0.1:1",
			"--timeout",
			"200ms",
		})
		if code != exitRuntime {
			t.Fatalf("expected exitRuntime, got %d", code)
		}
	})
}

func TestNodeJoinHappyPath(t *testing.T) {
	seed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"nard","status":"ok","version":"test"}`))
	}))
	defer seed.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp("test", stdout, stderr)

	code := app.Run(context.Background(), []string{
		"node",
		"join",
		"--seed",
		seed.URL,
		"--node-id",
		"node-x",
		"--profile",
		"dev",
		"--output",
		"json",
	})
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d stderr=%s", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse join output: %v", err)
	}
	if payload["status"] != "joined" {
		t.Fatalf("expected joined status, got %+v", payload)
	}
	if payload["seed"] != seed.URL {
		t.Fatalf("expected seed %q, got %+v", seed.URL, payload)
	}
}

func TestNodeStatusHappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`{"service":"nard","status":"ok","version":"test"}`))
		case "/api/v1/node/state":
			_, _ = w.Write([]byte(`{"state":"running"}`))
		case "/api/v1/node/capabilities":
			_, _ = w.Write([]byte(`{"node_id":"node-a","cpu_cores":4,"memory_mb":4096}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp("test", stdout, stderr)
	app.httpClient = server.Client()
	app.now = func() time.Time {
		return time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	}

	code := app.Run(context.Background(), []string{
		"node",
		"status",
		"--endpoint",
		server.URL,
		"--output",
		"json",
	})
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d stderr=%s", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse status output: %v", err)
	}
	if payload["state"] != "running" {
		t.Fatalf("expected running state, got %+v", payload)
	}
	if payload["endpoint"] != server.URL {
		t.Fatalf("expected endpoint %q, got %+v", server.URL, payload)
	}
}

func TestNodeStartCheckHappyPath(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp("test", stdout, stderr)

	code := app.Run(context.Background(), []string{
		"node",
		"start",
		"--addr",
		"127.0.0.1:0",
		"--node-id",
		"node-check",
		"--profile",
		"dev",
		"--check",
		"--output",
		"json",
	})
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d stderr=%s", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse start output: %v", err)
	}
	if payload["status"] != "checked" {
		t.Fatalf("expected checked status, got %+v", payload)
	}
}
