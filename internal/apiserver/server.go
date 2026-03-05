package apiserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gocools-LLC/nard.gocools/internal/runtime/agent"
)

type NodeAgent interface {
	CapabilityReport() agent.Capability
	State() agent.State
}

type Config struct {
	Addr    string
	Version string
	Logger  *slog.Logger
	Agent   NodeAgent
}

type statusResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(cfg Config) *http.Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	addr := cfg.Addr
	if addr == "" {
		addr = ":8082"
	}

	version := cfg.Version
	if version == "" {
		version = "dev"
	}

	nodeAgent := cfg.Agent
	if nodeAgent == nil {
		nodeAgent = agent.NewAgent(agent.Config{
			NodeID: "node-local",
			Capabilities: agent.Capability{
				CPUCores: 2,
				MemoryMB: 2048,
				Labels: map[string]string{
					"role": "edge",
				},
			},
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", statusHandler(version, "ok"))
	mux.HandleFunc("/readyz", statusHandler(version, "ready"))
	mux.HandleFunc("/api/v1/node/capabilities", capabilitiesHandler(nodeAgent))
	mux.HandleFunc("/api/v1/node/state", nodeStateHandler(nodeAgent))

	return &http.Server{
		Addr:              addr,
		Handler:           requestLogMiddleware(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func statusHandler(version string, status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		writeJSON(w, http.StatusOK, statusResponse{
			Service: "nard",
			Status:  status,
			Version: version,
		})
	}
}

func capabilitiesHandler(nodeAgent NodeAgent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, nodeAgent.CapabilityReport())
	}
}

func nodeStateHandler(nodeAgent NodeAgent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"state": string(nodeAgent.State()),
		})
	}
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func requestLogMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		status := rec.statusCode
		if status == 0 {
			status = http.StatusOK
		}

		logger.Info(
			"http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.ResponseWriter.Write(p)
}
