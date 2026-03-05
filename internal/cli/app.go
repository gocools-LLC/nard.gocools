package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gocools-LLC/nard.gocools/internal/apiserver"
	"github.com/gocools-LLC/nard.gocools/internal/runtime/agent"
)

const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2
)

type App struct {
	stdout     io.Writer
	stderr     io.Writer
	version    string
	httpClient *http.Client
	now        func() time.Time
}

func NewApp(version string, stdout io.Writer, stderr io.Writer) *App {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	return &App{
		stdout:  stdout,
		stderr:  stderr,
		version: version,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		now: time.Now,
	}
}

func (a *App) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return a.runNode(ctx, []string{"start"})
	}

	switch args[0] {
	case "-h", "--help", "help":
		a.printRootHelp()
		return exitOK
	case "node":
		return a.runNode(ctx, args[1:])
	default:
		fmt.Fprintf(a.stderr, "unknown command %q\n\n", args[0])
		a.printRootHelp()
		return exitUsage
	}
}

func (a *App) printRootHelp() {
	fmt.Fprintln(a.stdout, "Nard CLI")
	fmt.Fprintln(a.stdout, "")
	fmt.Fprintln(a.stdout, "Usage:")
	fmt.Fprintln(a.stdout, "  nard node <start|join|status> [flags]")
	fmt.Fprintln(a.stdout, "")
	fmt.Fprintln(a.stdout, "Commands:")
	fmt.Fprintln(a.stdout, "  node start    Start a local Nard node API/runtime")
	fmt.Fprintln(a.stdout, "  node join     Join a seed node after health validation")
	fmt.Fprintln(a.stdout, "  node status   Inspect node health, state, and capabilities")
}

func (a *App) runNode(ctx context.Context, args []string) int {
	if len(args) == 0 {
		a.printNodeHelp()
		return exitUsage
	}

	switch args[0] {
	case "-h", "--help", "help":
		a.printNodeHelp()
		return exitOK
	case "start":
		return a.runNodeStart(ctx, args[1:])
	case "join":
		return a.runNodeJoin(ctx, args[1:])
	case "status":
		return a.runNodeStatus(ctx, args[1:])
	default:
		fmt.Fprintf(a.stderr, "unknown node subcommand %q\n\n", args[0])
		a.printNodeHelp()
		return exitUsage
	}
}

func (a *App) printNodeHelp() {
	fmt.Fprintln(a.stdout, "Usage:")
	fmt.Fprintln(a.stdout, "  nard node start  [--addr <addr>] [--node-id <id>] [--profile <name>] [--output <json|text>] [--check]")
	fmt.Fprintln(a.stdout, "  nard node join   --seed <url> [--node-id <id>] [--profile <name>] [--output <json|text>] [--timeout <duration>]")
	fmt.Fprintln(a.stdout, "  nard node status [--endpoint <url>] [--output <json|text>] [--timeout <duration>] [--retries <n>] [--retry-backoff <duration>]")
	fmt.Fprintln(a.stdout, "")
	fmt.Fprintln(a.stdout, "Examples:")
	fmt.Fprintln(a.stdout, "  nard node start --profile dev")
	fmt.Fprintln(a.stdout, "  nard node join --seed http://127.0.0.1:8082 --node-id node-b")
	fmt.Fprintln(a.stdout, "  nard node status --endpoint http://127.0.0.1:8082")
}

func (a *App) runNodeStart(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("node start", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	addr := fs.String("addr", envOrDefault("NARD_HTTP_ADDR", ":8082"), "HTTP bind address")
	nodeID := fs.String("node-id", envOrDefault("NARD_NODE_ID", "node-local"), "Node identifier")
	profile := fs.String("profile", envOrDefault("NARD_PROFILE", "default"), "Runtime profile")
	output := fs.String("output", "json", "Output format: json|text")
	check := fs.Bool("check", false, "Start then immediately shutdown after startup checks")

	fs.Usage = func() {
		fmt.Fprintln(a.stderr, "Usage: nard node start [--addr <addr>] [--node-id <id>] [--profile <name>] [--output <json|text>] [--check]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(a.stderr, "node start does not accept positional args")
		fs.Usage()
		return exitUsage
	}
	if !isValidOutputMode(*output) {
		fmt.Fprintln(a.stderr, "invalid output mode")
		fs.Usage()
		return exitUsage
	}

	result, err := a.startNode(ctx, startConfig{
		Addr:    *addr,
		NodeID:  *nodeID,
		Profile: *profile,
		Check:   *check,
	})
	if err != nil {
		fmt.Fprintf(a.stderr, "node start failed: %v\n", err)
		return exitRuntime
	}

	a.writeOutput(*output, result)
	return exitOK
}

func (a *App) runNodeJoin(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("node join", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	seed := fs.String("seed", "", "Seed node endpoint (http://host:port)")
	nodeID := fs.String("node-id", envOrDefault("NARD_NODE_ID", "node-local"), "Node identifier")
	profile := fs.String("profile", envOrDefault("NARD_PROFILE", "default"), "Runtime profile")
	output := fs.String("output", "json", "Output format: json|text")
	timeout := fs.Duration("timeout", 3*time.Second, "Health probe timeout")

	fs.Usage = func() {
		fmt.Fprintln(a.stderr, "Usage: nard node join --seed <url> [--node-id <id>] [--profile <name>] [--output <json|text>] [--timeout <duration>]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(a.stderr, "node join does not accept positional args")
		fs.Usage()
		return exitUsage
	}
	if strings.TrimSpace(*seed) == "" {
		fmt.Fprintln(a.stderr, "--seed is required")
		fs.Usage()
		return exitUsage
	}
	if !isValidOutputMode(*output) {
		fmt.Fprintln(a.stderr, "invalid output mode")
		fs.Usage()
		return exitUsage
	}

	endpoint, err := normalizeEndpoint(*seed)
	if err != nil {
		fmt.Fprintf(a.stderr, "invalid seed endpoint: %v\n", err)
		return exitUsage
	}

	healthURL := endpoint + "/healthz"
	probeCtx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, healthURL, nil)
	if err != nil {
		fmt.Fprintf(a.stderr, "join request build failed: %v\n", err)
		return exitRuntime
	}

	response, err := a.httpClient.Do(request)
	if err != nil {
		fmt.Fprintf(a.stderr, "seed health probe failed: %v\n", err)
		return exitRuntime
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		fmt.Fprintf(a.stderr, "seed health probe returned status %d\n", response.StatusCode)
		return exitRuntime
	}

	a.writeOutput(*output, map[string]any{
		"action":     "join",
		"status":     "joined",
		"node_id":    *nodeID,
		"seed":       endpoint,
		"profile":    *profile,
		"checked_at": a.now().UTC().Format(time.RFC3339),
	})
	return exitOK
}

func (a *App) runNodeStatus(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("node status", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	endpoint := fs.String("endpoint", envOrDefault("NARD_ENDPOINT", "http://127.0.0.1:8082"), "Node API endpoint")
	output := fs.String("output", "json", "Output format: json|text")
	timeout := fs.Duration("timeout", 5*time.Second, "Per-attempt request timeout")
	retries := fs.Int("retries", 2, "Retry attempts on transient failures")
	retryBackoff := fs.Duration("retry-backoff", 300*time.Millisecond, "Backoff between retry attempts")

	fs.Usage = func() {
		fmt.Fprintln(a.stderr, "Usage: nard node status [--endpoint <url>] [--output <json|text>] [--timeout <duration>] [--retries <n>] [--retry-backoff <duration>]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(a.stderr, "node status does not accept positional args")
		fs.Usage()
		return exitUsage
	}
	if !isValidOutputMode(*output) {
		fmt.Fprintln(a.stderr, "invalid output mode")
		fs.Usage()
		return exitUsage
	}
	if *retries < 0 {
		fmt.Fprintln(a.stderr, "retries must be >= 0")
		fs.Usage()
		return exitUsage
	}
	if *retryBackoff < 0 {
		fmt.Fprintln(a.stderr, "retry-backoff must be >= 0")
		fs.Usage()
		return exitUsage
	}

	normalizedEndpoint, err := normalizeEndpoint(*endpoint)
	if err != nil {
		fmt.Fprintf(a.stderr, "invalid endpoint: %v\n", err)
		return exitUsage
	}

	health, err := a.fetchJSONWithRetry(ctx, normalizedEndpoint+"/healthz", *timeout, *retries, *retryBackoff)
	if err != nil {
		fmt.Fprintf(a.stderr, "status health request failed: endpoint=%s attempts=%d timeout=%s error=%v\n", normalizedEndpoint+"/healthz", *retries+1, timeout.String(), err)
		return exitRuntime
	}

	statePayload, err := a.fetchJSONWithRetry(ctx, normalizedEndpoint+"/api/v1/node/state", *timeout, *retries, *retryBackoff)
	if err != nil {
		fmt.Fprintf(a.stderr, "status state request failed: endpoint=%s attempts=%d timeout=%s error=%v\n", normalizedEndpoint+"/api/v1/node/state", *retries+1, timeout.String(), err)
		return exitRuntime
	}

	capabilityPayload, err := a.fetchJSONWithRetry(ctx, normalizedEndpoint+"/api/v1/node/capabilities", *timeout, *retries, *retryBackoff)
	if err != nil {
		fmt.Fprintf(a.stderr, "status capability request failed: endpoint=%s attempts=%d timeout=%s error=%v\n", normalizedEndpoint+"/api/v1/node/capabilities", *retries+1, timeout.String(), err)
		return exitRuntime
	}

	stateValue := ""
	if rawState, ok := statePayload["state"]; ok {
		stateValue = fmt.Sprint(rawState)
	}

	result := map[string]any{
		"action":       "status",
		"endpoint":     normalizedEndpoint,
		"checked_at":   a.now().UTC().Format(time.RFC3339),
		"health":       health,
		"state":        stateValue,
		"capabilities": capabilityPayload,
	}
	a.writeOutput(*output, result)
	return exitOK
}

type startConfig struct {
	Addr    string
	NodeID  string
	Profile string
	Check   bool
}

func (a *App) startNode(ctx context.Context, cfg startConfig) (map[string]any, error) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))

	nodeAgent := agent.NewAgent(agent.Config{
		NodeID:            cfg.NodeID,
		HeartbeatInterval: 1 * time.Second,
		Capabilities: agent.Capability{
			CPUCores: 2,
			MemoryMB: 2048,
			Labels: map[string]string{
				"profile": cfg.Profile,
				"role":    "edge",
			},
		},
	})
	if err := nodeAgent.Start(ctx); err != nil {
		return nil, err
	}
	defer nodeAgent.Stop(context.Background())

	server := apiserver.New(apiserver.Config{
		Addr:    cfg.Addr,
		Version: a.version,
		Logger:  logger,
		Agent:   nodeAgent,
	})

	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	if cfg.Check {
		select {
		case err := <-serverErrCh:
			return nil, err
		case <-time.After(200 * time.Millisecond):
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return nil, err
		}
		return map[string]any{
			"action":  "start",
			"status":  "checked",
			"addr":    cfg.Addr,
			"node_id": cfg.NodeID,
			"profile": cfg.Profile,
			"version": a.version,
		}, nil
	}

	signalCtx, stopSignals := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	select {
	case err := <-serverErrCh:
		return nil, err
	case <-signalCtx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return nil, err
	}

	return map[string]any{
		"action":  "start",
		"status":  "stopped",
		"addr":    cfg.Addr,
		"node_id": cfg.NodeID,
		"profile": cfg.Profile,
		"version": a.version,
	}, nil
}

func (a *App) fetchJSON(ctx context.Context, endpoint string, timeout time.Duration) (map[string]any, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	response, err := a.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, statusError{Code: response.StatusCode}
	}

	payload := map[string]any{}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (a *App) fetchJSONWithRetry(ctx context.Context, endpoint string, timeout time.Duration, retries int, retryBackoff time.Duration) (map[string]any, error) {
	attempts := retries + 1
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		payload, err := a.fetchJSON(ctx, endpoint, timeout)
		if err == nil {
			return payload, nil
		}
		lastErr = err

		if attempt == attempts || !isRetryableFetchError(err) {
			break
		}
		if err := waitRetry(ctx, retryBackoff); err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", attempts, lastErr)
}

type statusError struct {
	Code int
}

func (e statusError) Error() string {
	return fmt.Sprintf("status %d", e.Code)
}

func isRetryableFetchError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var responseErr statusError
	if errors.As(err, &responseErr) {
		return responseErr.Code == http.StatusTooManyRequests || responseErr.Code == http.StatusRequestTimeout || responseErr.Code >= http.StatusInternalServerError
	}

	return false
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *App) writeOutput(mode string, payload any) {
	switch mode {
	case "text":
		switch value := payload.(type) {
		case map[string]any:
			for key, item := range value {
				fmt.Fprintf(a.stdout, "%s: %v\n", key, item)
			}
		default:
			fmt.Fprintln(a.stdout, payload)
		}
	default:
		encoder := json.NewEncoder(a.stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(payload)
	}
}

func isValidOutputMode(mode string) bool {
	return mode == "json" || mode == "text"
}

func normalizeEndpoint(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("endpoint is empty")
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("scheme must be http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", errors.New("host is required")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
