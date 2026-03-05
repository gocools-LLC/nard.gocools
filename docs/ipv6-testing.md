# IPv6 Testing and Diagnostics

Nard includes an IPv6-first connectivity test suite in:

- `internal/p2p/transport/ipv6_test.go`
- `internal/discovery/peers/ipv6_test.go`
- `internal/routing/ipv6_test.go`

## CI Target

GitHub Actions job:

- `.github/workflows/ci.yml` -> `ipv6-tests`

The job runs:

```bash
go test ./... -run 'IPv6|DualStack' -count=1 -v
```

## Environment Caveats

- Some CI runners and local containers disable `tcp6` loopback.
- Transport IPv6 tests detect this with `net.Listen("tcp6", "[::1]:0")`.
- When unavailable, IPv6 transport tests skip with diagnostic output.

Diagnostic fields logged on skip:

- `goos`
- `goarch`
- `GITHUB_ACTIONS`

## Local Usage

Run full tests:

```bash
go test ./...
```

Run IPv6-focused tests only:

```bash
make test-ipv6
```
