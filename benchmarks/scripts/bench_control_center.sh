#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
BENCH_SCENARIOS="${BENCH_SCENARIOS:-control_center}"
VUS="${VUS:-30}"
DURATION="${DURATION:-90s}"
GATEWAY_TOKEN="${GATEWAY_TOKEN:-}"

BASE_URL="$BASE_URL" \
BENCH_SCENARIOS="$BENCH_SCENARIOS" \
VUS="$VUS" \
DURATION="$DURATION" \
GATEWAY_TOKEN="$GATEWAY_TOKEN" \
"$ROOT_DIR/benchmarks/scripts/bench_baseline.sh"
