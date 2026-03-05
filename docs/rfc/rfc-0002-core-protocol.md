# RFC-0002: Nard Core Protocol

- Status: Draft
- Project: nard.gocools
- Last Updated: 2026-03-05

## 1. Abstract

This RFC defines the baseline Nard wire protocol for:

- node identity
- peer discovery
- service naming
- routing updates

The protocol is transport-agnostic and intended to run over secure P2P links.

## 2. Goals

- deterministic handshake and session negotiation
- explicit protocol versioning and compatibility behavior
- compact message envelope for discovery/routing primitives
- safe extension model for future message types

## 3. Protocol Versioning

Current protocol identifier:

`nard.core/v0alpha1`

### Compatibility Strategy

- minor additive changes are backward compatible in the same `v0alpha1` line
- field removal/rename requires protocol version increment
- unknown optional fields must be ignored by receivers

## 4. Message Envelope

All protocol messages use the envelope:

```json
{
  "version": "nard.core/v0alpha1",
  "type": "discover.request",
  "node_id": "node-abc",
  "timestamp": "2026-03-05T00:00:00Z",
  "request_id": "req-123",
  "payload": {}
}
```

Required fields:

- `version`
- `type`
- `node_id`
- `timestamp`
- `request_id`
- `payload`

## 5. Core Message Types

### 5.1 Handshake

- `handshake.hello`
- `handshake.ack`

Purpose:

- protocol version negotiation
- node identity proof exchange

### 5.2 Discovery

- `discover.request`
- `discover.response`

Purpose:

- request peer candidates
- return peer descriptors and expiry metadata

### 5.3 Naming

- `name.register`
- `name.resolve.request`
- `name.resolve.response`

Purpose:

- map logical service names to routable endpoints

### 5.4 Routing

- `route.update`
- `route.withdraw`

Purpose:

- propagate reachability and route changes

## 6. Handshake State Machine

States:

1. `INIT`: no session established
2. `HELLO_SENT`: hello sent, awaiting ack
3. `ESTABLISHED`: session active
4. `TERMINATED`: session closed

Transitions:

- `INIT -> HELLO_SENT` on outbound hello
- `HELLO_SENT -> ESTABLISHED` on valid ack
- any state -> `TERMINATED` on protocol violation or transport close

## 7. Error Model

Protocol errors should include:

- `code` (machine-readable)
- `message` (human-readable)
- `request_id` (correlation)

Example codes:

- `ERR_UNSUPPORTED_VERSION`
- `ERR_INVALID_ENVELOPE`
- `ERR_AUTH_FAILED`
- `ERR_ROUTE_REJECTED`

## 8. Security Baseline

- node identity is mandatory in handshake
- message timestamps are required for replay-window enforcement
- transport-level encryption is required
- reject messages with unknown mandatory fields

## 9. Open Questions

1. identity key format (ed25519 vs secp256k1)
2. gossip fanout and convergence targets
3. route conflict resolution policy
4. TTL semantics for name registration
5. anti-entropy sync interval for route tables

