# P2P Transport (QUIC Baseline)

Nard QUIC transport implementation: `internal/p2p/transport`

## Transport Abstraction

Implemented capabilities:

- start listener (`Start`)
- dial/send payloads (`Send`)
- close peer connection (`ClosePeer`)
- close transport (`Close`)

## Lifecycle Hooks

Supported hooks:

- `OnConnect`
- `OnDisconnect`
- `OnReconnect`
- `OnMessage`

## Metrics

Exported counters:

- `dial_attempts`
- `connections`
- `reconnects`
- `disconnects`
- `bytes_sent`
- `bytes_received`

## Baseline Validation

Tests validate:

- multi-node connectivity
- reconnect behavior after peer close
- metrics updates for sent/received bytes

