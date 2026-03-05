# Nard

Core protocol and runtime platform for decentralized and edge-native workloads.

## Features
- distributed service discovery
- decentralized naming and routing
- peer-to-peer networking
- edge runtime hosting with IPv6-native patterns

## Quick Start

```bash
go run ./cmd/nard
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
- [Roadmap](docs/roadmap.md)
- [RFC-0001](docs/rfc/rfc-0001-platform.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
