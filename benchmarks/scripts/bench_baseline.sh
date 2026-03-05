#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_ROOT="$ROOT_DIR/benchmarks/results"
TS="$(date +%Y%m%d_%H%M%S)"
OUT_DIR="$OUT_ROOT/$TS"

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
BENCH_SCENARIOS="${BENCH_SCENARIOS:-health,tools_invoke_web_fetch}"
VUS="${VUS:-20}"
DURATION="${DURATION:-60s}"
GATEWAY_TOKEN="${GATEWAY_TOKEN:-}"
WEB_FETCH_TARGET="${WEB_FETCH_TARGET:-https://example.com}"
MAX_CHARS="${MAX_CHARS:-2000}"

mkdir -p "$OUT_DIR"

if ! command -v k6 >/dev/null 2>&1; then
  echo "k6 is not installed. Install: https://grafana.com/docs/k6/latest/set-up/install-k6/" >&2
  exit 1
fi

if ! curl -fsS "$BASE_URL/health" >/dev/null; then
  echo "gateway is not reachable at $BASE_URL" >&2
  exit 1
fi

echo "Starting baseline run"
echo "  output: $OUT_DIR"
echo "  base:   $BASE_URL"
echo "  vus:    $VUS"
echo "  dur:    $DURATION"

echo "timestamp=$TS" > "$OUT_DIR/metadata.env"
echo "base_url=$BASE_URL" >> "$OUT_DIR/metadata.env"
echo "vus=$VUS" >> "$OUT_DIR/metadata.env"
echo "duration=$DURATION" >> "$OUT_DIR/metadata.env"
echo "scenarios=$BENCH_SCENARIOS" >> "$OUT_DIR/metadata.env"

IFS=',' read -r -a SCENARIOS <<< "$BENCH_SCENARIOS"
for scenario in "${SCENARIOS[@]}"; do
  script="$ROOT_DIR/benchmarks/k6/${scenario}.js"
  if [[ ! -f "$script" ]]; then
    echo "missing scenario: $script" >&2
    exit 1
  fi

  echo "Running scenario: $scenario"
  BASE_URL="$BASE_URL" \
  GATEWAY_TOKEN="$GATEWAY_TOKEN" \
  WEB_FETCH_TARGET="$WEB_FETCH_TARGET" \
  MAX_CHARS="$MAX_CHARS" \
  VUS="$VUS" \
  DURATION="$DURATION" \
  k6 run "$script" \
    --summary-export "$OUT_DIR/${scenario}.summary.json" \
    --out "json=$OUT_DIR/${scenario}.raw.json" \
    > "$OUT_DIR/${scenario}.stdout.log"

done

echo "Baseline complete: $OUT_DIR"
