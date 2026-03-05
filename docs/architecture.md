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
