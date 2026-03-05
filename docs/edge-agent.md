# Edge Runtime Node Agent

Nard edge agent implementation: `internal/runtime/agent`

## Lifecycle State Machine

States:

- `stopped`
- `starting`
- `running`
- `stopping`

## Capabilities

Agent reports node capabilities:

- `node_id`
- `cpu_cores`
- `memory_mb`
- `labels`

Capability API:

- `GET /api/v1/node/capabilities`

## Workload Registration

Agent supports workload registration hooks for runtime integration:

- `RegisterWorkload(id, image, metadata)`
- `OnWorkloadRegistered` callback

## Heartbeats

Agent emits periodic heartbeat events while running:

- `NodeID`
- `State`
- `Timestamp`

Node state API:

- `GET /api/v1/node/state`

