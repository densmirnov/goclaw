#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_ROOT="$ROOT_DIR/benchmarks/results"
TS="$(date +%Y%m%d_%H%M%S)"
OUT_DIR="$OUT_ROOT/$TS/pprof"

PPROF_URL="${PPROF_URL:-http://127.0.0.1:6060}"
PROFILE_SECONDS="${PROFILE_SECONDS:-30}"

mkdir -p "$OUT_DIR"

if ! command -v go >/dev/null 2>&1; then
  echo "go is not installed" >&2
  exit 1
fi

if ! curl -fsS "$PPROF_URL/debug/pprof/" >/dev/null; then
  echo "pprof endpoint is not reachable at $PPROF_URL" >&2
  echo "Start gateway with GOCLAW_PPROF_ADDR=127.0.0.1:6060" >&2
  exit 1
fi

echo "Capturing pprof profiles into $OUT_DIR"

go tool pprof -proto "$PPROF_URL/debug/pprof/profile?seconds=$PROFILE_SECONDS" > "$OUT_DIR/cpu.pb.gz"
go tool pprof -proto "$PPROF_URL/debug/pprof/heap" > "$OUT_DIR/heap.pb.gz"
go tool pprof -proto "$PPROF_URL/debug/pprof/allocs" > "$OUT_DIR/allocs.pb.gz"
go tool pprof -proto "$PPROF_URL/debug/pprof/goroutine" > "$OUT_DIR/goroutine.pb.gz"

echo "pprof capture complete: $OUT_DIR"
