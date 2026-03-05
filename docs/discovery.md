# Peer Discovery Service

Nard peer discovery implementation: `internal/discovery/peers`

## Capabilities

- bootstrap peer list support
- peer table upsert/merge maintenance
- TTL-based peer eviction
- discovery metrics export

## Peer Lifecycle

1. bootstrap peers are loaded at startup
2. peers are updated with `LastSeen` and `ExpiresAt`
3. `EvictExpired` removes stale peers using TTL rules

## Exported Metrics

- `bootstrap_peers`
- `known_peers`
- `added_peers`
- `updated_peers`
- `evicted_peers`

## Convergence

Multi-node convergence is validated in tests by exchanging discovered peer lists and merging updates.

