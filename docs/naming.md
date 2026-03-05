# Decentralized Naming Registry

Nard naming prototype implementation: `internal/naming/registry`

## API Surface

- `Register(name, target, owner_node_id, ttl)`
- `Resolve(name)`

## Conflict Resolution

Conflict resolution is deterministic:

1. lower `owner_node_id` wins between competing registrations
2. for same owner, latest registration timestamp wins

## TTL Semantics

- every registration has `ExpiresAt`
- `Resolve` and `Snapshot` evict expired entries lazily

## Lookup Latency Targets

Prototype targets for local node operation:

- p50 lookup latency: `< 5ms`
- p95 lookup latency: `< 20ms`

These targets apply to in-memory resolver path and will be re-evaluated for distributed deployments.

