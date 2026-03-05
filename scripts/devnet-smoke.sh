#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
BIN_PATH="${TMP_DIR}/nard"
SEED_ADDR="${SEED_ADDR:-127.0.0.1:18082}"
SEED_ENDPOINT="http://${SEED_ADDR}"

cleanup() {
  if [[ -n "${SEED_PID:-}" ]]; then
    kill -TERM "${SEED_PID}" >/dev/null 2>&1 || true
    wait "${SEED_PID}" >/dev/null 2>&1 || true
  fi
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

cd "${ROOT_DIR}"

echo "Building nard binary for smoke test..."
go build -o "${BIN_PATH}" ./cmd/nard

echo "Starting devnet seed node on ${SEED_ADDR}..."
"${BIN_PATH}" node start \
  --addr "${SEED_ADDR}" \
  --node-id devnet-seed \
  --profile devnet \
  --output json >"${TMP_DIR}/seed.log" 2>&1 &
SEED_PID=$!

echo "Waiting for seed node readiness..."
READY=0
for _ in $(seq 1 30); do
  if "${BIN_PATH}" node status --endpoint "${SEED_ENDPOINT}" --output json >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 1
done

if [[ "${READY}" -ne 1 ]]; then
  echo "Seed node failed readiness checks."
  echo "Seed logs:"
  cat "${TMP_DIR}/seed.log"
  exit 1
fi

echo "Running mesh smoke checks (join + status)..."
"${BIN_PATH}" node join --seed "${SEED_ENDPOINT}" --node-id devnet-peer-1 --profile devnet --output json >/dev/null
"${BIN_PATH}" node join --seed "${SEED_ENDPOINT}" --node-id devnet-peer-2 --profile devnet --output json >/dev/null
"${BIN_PATH}" node status --endpoint "${SEED_ENDPOINT}" --output json >/dev/null

echo "Devnet smoke checks passed."
