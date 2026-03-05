package agent

import (
	"context"
	"errors"
	"sync"
	"time"
)

type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
)

type Capability struct {
	NodeID    string            `json:"node_id"`
	CPUCores  int               `json:"cpu_cores"`
	MemoryMB  int               `json:"memory_mb"`
	Labels    map[string]string `json:"labels,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type Workload struct {
	ID         string            `json:"id"`
	Image      string            `json:"image,omitempty"`
	Registered time.Time         `json:"registered"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type Heartbeat struct {
	NodeID    string    `json:"node_id"`
	State     State     `json:"state"`
	Timestamp time.Time `json:"timestamp"`
}

type Hooks struct {
	OnWorkloadRegistered func(workload Workload)
}

type Config struct {
	NodeID            string
	Capabilities      Capability
	HeartbeatInterval time.Duration
	Hooks             Hooks
}

type Agent struct {
	mu                sync.RWMutex
	nodeID            string
	state             State
	capabilities      Capability
	workloads         map[string]Workload
	heartbeatInterval time.Duration
	heartbeatCh       chan Heartbeat
	stopCh            chan struct{}
	closedCh          chan struct{}
	hooks             Hooks
	now               func() time.Time
}

func NewAgent(cfg Config) *Agent {
	interval := cfg.HeartbeatInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	nodeID := cfg.NodeID
	if nodeID == "" {
		nodeID = "node-local"
	}

	capability := cfg.Capabilities
	capability.NodeID = nodeID
	capability.UpdatedAt = time.Now().UTC()

	return &Agent{
		nodeID:            nodeID,
		state:             StateStopped,
		capabilities:      capability,
		workloads:         map[string]Workload{},
		heartbeatInterval: interval,
		heartbeatCh:       make(chan Heartbeat, 32),
		stopCh:            make(chan struct{}),
		closedCh:          make(chan struct{}),
		hooks:             cfg.Hooks,
		now:               time.Now,
	}
}

func (a *Agent) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.state == StateRunning || a.state == StateStarting {
		a.mu.Unlock()
		return nil
	}
	a.state = StateStarting
	a.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	a.mu.Lock()
	a.state = StateRunning
	a.mu.Unlock()

	go a.heartbeatLoop()
	return nil
}

func (a *Agent) Stop(ctx context.Context) error {
	a.mu.Lock()
	if a.state == StateStopped {
		a.mu.Unlock()
		return nil
	}
	a.state = StateStopping
	close(a.stopCh)
	a.mu.Unlock()

	select {
	case <-a.closedCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	a.mu.Lock()
	a.state = StateStopped
	a.mu.Unlock()
	return nil
}

func (a *Agent) RegisterWorkload(id string, image string, metadata map[string]string) (Workload, error) {
	if id == "" {
		return Workload{}, errors.New("workload id is required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	workload := Workload{
		ID:         id,
		Image:      image,
		Registered: a.now().UTC(),
		Metadata:   cloneMap(metadata),
	}
	a.workloads[id] = workload

	if a.hooks.OnWorkloadRegistered != nil {
		a.hooks.OnWorkloadRegistered(workload)
	}
	return workload, nil
}

func (a *Agent) CapabilityReport() Capability {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := a.capabilities
	report.UpdatedAt = a.now().UTC()
	report.Labels = cloneMap(report.Labels)
	return report
}

func (a *Agent) State() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

func (a *Agent) Heartbeats() <-chan Heartbeat {
	return a.heartbeatCh
}

func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(a.heartbeatInterval)
	defer ticker.Stop()
	defer close(a.closedCh)

	for {
		select {
		case <-ticker.C:
			a.mu.RLock()
			state := a.state
			nodeID := a.nodeID
			a.mu.RUnlock()

			if state != StateRunning {
				continue
			}
			select {
			case a.heartbeatCh <- Heartbeat{
				NodeID:    nodeID,
				State:     state,
				Timestamp: a.now().UTC(),
			}:
			default:
			}
		case <-a.stopCh:
			return
		}
	}
}

func cloneMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return map[string]string{}
	}

	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
