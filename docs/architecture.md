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

- distributed service discovery\n- decentralized naming and routing\n- peer-to-peer networking\n- edge runtime hosting with IPv6-native patterns

## Guardrails

All managed cloud resources must include:

```text
gocools:stack-id
gocools:environment
gocools:owner
```

Destructive actions require stack validation and environment-aware protections.
