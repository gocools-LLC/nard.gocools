# Nard Architecture

## Purpose

Core protocol and runtime platform for decentralized and edge-native workloads.

## High-Level Model

GoCools platform layers:

```text
nard.gocools
   -> arch.gocools
      -> flow.gocools
```

This repository focuses on **Nard** and integrates with the other layers through APIs and shared stack metadata.

## Core Capabilities

- distributed service discovery
- decentralized naming and routing
- peer-to-peer networking
- edge runtime hosting with IPv6-native patterns

## Guardrails

All managed cloud resources must include:

```text
gocools:stack-id
gocools:environment
gocools:owner
```

Destructive actions require stack validation and environment-aware protections.

## Core Protocol RFC

Nard core protocol draft:

- [RFC-0002 Core Protocol](rfc/rfc-0002-core-protocol.md)

## Discovery Service

Nard includes a peer discovery service with bootstrap, TTL eviction, and convergence behavior.

- [discovery.md](discovery.md)

## Naming Registry

Nard includes a decentralized naming registry prototype with deterministic conflict resolution and TTL registrations.

- [naming.md](naming.md)

## P2P Transport

Nard includes a QUIC baseline transport abstraction with lifecycle hooks and metrics.

- [p2p-transport.md](p2p-transport.md)

## Edge Agent

Nard includes an edge runtime node agent skeleton with lifecycle states, capability reporting, workload hooks, and heartbeat emission.

- [edge-agent.md](edge-agent.md)

## Service Routing

Nard includes a heartbeat-based routing subsystem with unhealthy peer eviction, round-robin selection, and explainable routing decisions.

- [routing.md](routing.md)

## IPv6 Validation

Nard includes IPv6-first and dual-stack connectivity tests plus CI coverage and environment caveats.

- [ipv6-testing.md](ipv6-testing.md)

## Identity and Rotation

Nard includes a node identity model with signed documents, key lifecycle rotation, trust bootstrap expectations, and compromised-key revocation flow.

- [identity.md](identity.md)

## CLI Lifecycle Management

Nard includes CLI lifecycle commands for start/join/status with structured output and deterministic exit codes.

- [cli.md](cli.md)

## Devnet and Release

Nard includes a v0.1.0 devnet topology spec, contributor smoke checks, release gates, and rollback notes.

- [devnet.md](devnet.md)
