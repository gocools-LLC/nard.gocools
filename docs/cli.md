# Node Lifecycle CLI

Implementation: `internal/cli`, entrypoint `cmd/nard/main.go`

## Commands

```bash
nard node start  [--addr <addr>] [--node-id <id>] [--profile <name>] [--output json|text] [--check]
nard node join   --seed <url> [--node-id <id>] [--profile <name>] [--output json|text] [--timeout <duration>]
nard node status [--endpoint <url>] [--output json|text] [--timeout <duration>]
```

## Output

Default output mode is structured JSON (`--output json`).

Text output mode is available with `--output text`.

## Exit Codes

- `0`: success
- `1`: runtime/connection/server error
- `2`: usage/argument/flag error

## Notes

- `node join` validates seed liveness via `GET /healthz`.
- `node status` reads `healthz`, `node/state`, and `node/capabilities`.
- `node start --check` is used for local/CI startup validation.
