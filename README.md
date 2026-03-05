# Nard

Core protocol and runtime platform for decentralized and edge-native workloads.

## Features
- distributed service discovery
- decentralized naming and routing
- peer-to-peer networking
- edge runtime hosting with IPv6-native patterns
- heartbeat-driven service routing and failover
- IPv6-first and dual-stack connectivity test coverage
- node identity documents with key rotation and revocation handling
- node lifecycle CLI for start/join/status flows
- devnet smoke-test and release gating checklist

## Quick Start

```bash
go run ./cmd/nard
curl -s localhost:8082/healthz
curl -s localhost:8082/api/v1/node/capabilities
curl -s localhost:8082/api/v1/node/state
```

## Repository Layout

- `cmd/nard`: CLI entrypoint.
- `internal/`: internal application logic.
- `pkg/`: reusable packages.
- `docs/`: architecture, roadmap, and RFCs.

## Security Model

- AWS STS AssumeRole (no permanent access keys)
- least-privilege IAM roles
- tag-based ownership and safe destructive operations

## Required Resource Tags

```text
gocools:stack-id
gocools:environment
gocools:owner
```

## Documentation

- [Architecture](docs/architecture.md)
- [Peer Discovery](docs/discovery.md)
- [Naming Registry](docs/naming.md)
- [P2P Transport](docs/p2p-transport.md)
- [Edge Agent](docs/edge-agent.md)
- [Routing](docs/routing.md)
- [IPv6 Testing](docs/ipv6-testing.md)
- [Identity and Key Rotation](docs/identity.md)
- [CLI](docs/cli.md)
- [Devnet](docs/devnet.md)
- [Roadmap](docs/roadmap.md)
- [RFC-0001](docs/rfc/rfc-0001-platform.md)
- [RFC-0002 Core Protocol](docs/rfc/rfc-0002-core-protocol.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
