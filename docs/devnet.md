# Devnet v0.1.0 Plan

## Topology

Minimal public devnet topology for `v0.1.0`:

- `seed-1` (bootstrap + health endpoint)
- `peer-1` (joins seed and validates routing/discovery path)
- `peer-2` (joins seed and validates multi-peer behavior)

Recommended local addresses:

- `seed-1`: `127.0.0.1:18082`
- `peer-*`: ephemeral local clients using `nard node join`

## Contributor Smoke Tests

Run local smoke tests:

```bash
make smoke-devnet
```

Equivalent script:

```bash
./scripts/devnet-smoke.sh
```

Smoke flow:

1. build `nard` binary
2. start local seed node
3. poll readiness using `nard node status`
4. run `nard node join` from two peer identities
5. run final status check
6. teardown seed process

## Release Gating Checklist (v0.1.0)

- [ ] `go test ./...` passes
- [ ] `make test-ipv6` passes (or skips with documented environment caveat)
- [ ] `make smoke-devnet` passes
- [ ] CLI help for `nard node start|join|status` is verified
- [ ] identity/rotation docs reviewed (`docs/identity.md`)
- [ ] release artifacts generated from tagged commit (`v0.1.0`)

## Rollback and Incident Notes

If devnet release validation fails:

1. stop new seed rollout
2. keep previous known-good tag active
3. publish issue summary with failure mode and impacted checks
4. revoke compromised node keys if applicable (see `docs/identity.md`)
5. patch and re-run all gates before re-tagging

During active incident response:

- capture smoke script logs from failing run
- capture node status JSON (`nard node status --output json`)
- document timeline and mitigation in a tracking issue
