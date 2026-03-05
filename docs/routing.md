# Service Routing and Heartbeat Protocol

Implementation: `internal/routing`

## Heartbeat Protocol

Nodes advertise liveness and service ownership using heartbeat messages.

Message fields:

- `node_id`
- `address`
- `services`
- `sequence`
- `timestamp`

Helpers:

- `EncodeHeartbeat(msg)`
- `DecodeHeartbeat(payload)`
- `ObserveHeartbeatPayload(payload)`

## Routing Behavior

Routing is health-aware and service-scoped.

- only peers with non-expired heartbeats are considered healthy
- unhealthy peers are evicted by `SweepUnhealthy()`
- `Route(service, avoidNodeID)` returns an explainable decision
- round-robin selection is used for healthy peers
- failover increments metrics when the previous route becomes unhealthy

## Explainable Decisions

Route decisions include:

- selected node
- candidate list
- strategy (`round_robin_failover`)
- reason string

This makes routing choices auditable in tests and future API surfaces.
