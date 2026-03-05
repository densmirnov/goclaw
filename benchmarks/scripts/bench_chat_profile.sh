#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_ROOT="$ROOT_DIR/benchmarks/results"
TS="$(date +%Y%m%d_%H%M%S)"
OUT_DIR="$OUT_ROOT/$TS"
PPROF_DIR="$OUT_DIR/pprof"

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
PPROF_URL="${PPROF_URL:-http://127.0.0.1:6060}"
VUS="${VUS:-4}"
DURATION="${DURATION:-45s}"
PROFILE_SECONDS="${PROFILE_SECONDS:-30}"
GATEWAY_TOKEN="${GATEWAY_TOKEN:-}"
BENCH_USER_ID="${BENCH_USER_ID:-bench-user}"
BENCH_AGENT_ID="${BENCH_AGENT_ID:-default}"
BENCH_MODEL="${BENCH_MODEL:-goclaw:${BENCH_AGENT_ID}}"
BENCH_MESSAGE="${BENCH_MESSAGE:-Напиши одно короткое предложение без вызова инструментов.}"

mkdir -p "$OUT_DIR" "$PPROF_DIR"

if ! command -v k6 >/dev/null 2>&1; then
  echo "k6 is not installed" >&2
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "go is not installed" >&2
  exit 1
fi
if ! curl -fsS "$BASE_URL/health" >/dev/null; then
  echo "gateway is not reachable at $BASE_URL" >&2
  exit 1
fi
if ! curl -fsS "$PPROF_URL/debug/pprof/" >/dev/null; then
  echo "pprof endpoint is not reachable at $PPROF_URL" >&2
  exit 1
fi

echo "timestamp=$TS" > "$OUT_DIR/metadata.env"
echo "base_url=$BASE_URL" >> "$OUT_DIR/metadata.env"
echo "pprof_url=$PPROF_URL" >> "$OUT_DIR/metadata.env"
echo "vus=$VUS" >> "$OUT_DIR/metadata.env"
echo "duration=$DURATION" >> "$OUT_DIR/metadata.env"
echo "profile_seconds=$PROFILE_SECONDS" >> "$OUT_DIR/metadata.env"
echo "scenario=chat_completions" >> "$OUT_DIR/metadata.env"

echo "Running chat_completions load + pprof capture"
(
  BASE_URL="$BASE_URL" \
  GATEWAY_TOKEN="$GATEWAY_TOKEN" \
  BENCH_USER_ID="$BENCH_USER_ID" \
  BENCH_AGENT_ID="$BENCH_AGENT_ID" \
  BENCH_MODEL="$BENCH_MODEL" \
  BENCH_MESSAGE="$BENCH_MESSAGE" \
  VUS="$VUS" \
  DURATION="$DURATION" \
  k6 run "$ROOT_DIR/benchmarks/k6/chat_completions.js" \
    --summary-export "$OUT_DIR/chat_completions.summary.json" \
    --out "json=$OUT_DIR/chat_completions.raw.json" \
    > "$OUT_DIR/chat_completions.stdout.log"
) &
K6_PID=$!

sleep 3

go tool pprof -proto "$PPROF_URL/debug/pprof/profile?seconds=$PROFILE_SECONDS" > "$PPROF_DIR/cpu.pb.gz"
go tool pprof -proto "$PPROF_URL/debug/pprof/heap" > "$PPROF_DIR/heap.pb.gz"
go tool pprof -proto "$PPROF_URL/debug/pprof/allocs" > "$PPROF_DIR/allocs.pb.gz"
go tool pprof -proto "$PPROF_URL/debug/pprof/goroutine" > "$PPROF_DIR/goroutine.pb.gz"

wait "$K6_PID"

echo "Chat profile complete: $OUT_DIR"
