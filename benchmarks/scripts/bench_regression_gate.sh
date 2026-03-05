#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="${BASE:-}"
NEW_DIR="${NEW:-}"
P95_DEGRADE_MAX_PCT="${P95_DEGRADE_MAX_PCT:-15}"
FAIL_RATE_DELTA_MAX="${FAIL_RATE_DELTA_MAX:-0.01}"
ALLOC_DEGRADE_MAX_PCT="${ALLOC_DEGRADE_MAX_PCT:-20}"

if [[ -z "$BASE_DIR" || -z "$NEW_DIR" ]]; then
  echo "Usage: BASE=<path> NEW=<path> $0" >&2
  exit 1
fi

if [[ ! -d "$BASE_DIR" || ! -d "$NEW_DIR" ]]; then
  echo "Both BASE and NEW must be existing directories" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for regression gate" >&2
  exit 1
fi

echo "Regression gate"
echo "  base: $BASE_DIR"
echo "  new:  $NEW_DIR"
echo "  limits: p95 +${P95_DEGRADE_MAX_PCT}% | fail_rate +${FAIL_RATE_DELTA_MAX} abs | alloc +${ALLOC_DEGRADE_MAX_PCT}%"

failures=0

for new_summary in "$NEW_DIR"/*.summary.json; do
  [[ -f "$new_summary" ]] || continue
  scenario="$(basename "$new_summary" .summary.json)"
  base_summary="$BASE_DIR/$scenario.summary.json"
  if [[ ! -f "$base_summary" ]]; then
    echo "[WARN] missing baseline for scenario: $scenario (skip)"
    continue
  fi

  b_p95=$(jq -r '.metrics.http_req_duration["p(95)"] // 0' "$base_summary")
  n_p95=$(jq -r '.metrics.http_req_duration["p(95)"] // 0' "$new_summary")
  b_fail=$(jq -r '.metrics.http_req_failed.value // 0' "$base_summary")
  n_fail=$(jq -r '.metrics.http_req_failed.value // 0' "$new_summary")

  if awk -v b="$b_p95" -v n="$n_p95" -v pct="$P95_DEGRADE_MAX_PCT" 'BEGIN { exit !(b > 0 && n > b*(1+pct/100)) }'; then
    echo "[FAIL] $scenario p95 degraded: base=$b_p95 new=$n_p95"
    failures=$((failures + 1))
  else
    echo "[OK]   $scenario p95: base=$b_p95 new=$n_p95"
  fi

  if awk -v b="$b_fail" -v n="$n_fail" -v d="$FAIL_RATE_DELTA_MAX" 'BEGIN { exit !(n > b + d) }'; then
    echo "[FAIL] $scenario fail_rate degraded: base=$b_fail new=$n_fail"
    failures=$((failures + 1))
  else
    echo "[OK]   $scenario fail_rate: base=$b_fail new=$n_fail"
  fi
done

base_alloc="$BASE_DIR/pprof/allocs.pb.gz"
new_alloc="$NEW_DIR/pprof/allocs.pb.gz"
if [[ -f "$base_alloc" && -f "$new_alloc" ]]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "[WARN] go not found: skip alloc profile regression check"
  else
    parse_alloc_bytes() {
      local f="$1"
      # "Showing nodes accounting for 123.45MB, 100% of 123.45MB total"
      go tool pprof -top -sample_index=alloc_space "$f" 2>/dev/null | \
        awk '/Showing nodes accounting for/ {print $8; exit}' | \
        awk '
          function to_bytes(v,   num, unit) {
            num=v; gsub(/[A-Za-z]+/, "", num); num+=0;
            unit=v; gsub(/[0-9.]/, "", unit);
            if (unit=="B") return num;
            if (unit=="kB" || unit=="KB") return num*1000;
            if (unit=="MB") return num*1000*1000;
            if (unit=="GB") return num*1000*1000*1000;
            return num;
          }
          { print to_bytes($0) }
        '
    }

    b_alloc=$(parse_alloc_bytes "$base_alloc")
    n_alloc=$(parse_alloc_bytes "$new_alloc")
    if [[ -n "$b_alloc" && -n "$n_alloc" ]]; then
      if awk -v b="$b_alloc" -v n="$n_alloc" -v pct="$ALLOC_DEGRADE_MAX_PCT" 'BEGIN { exit !(b > 0 && n > b*(1+pct/100)) }'; then
        echo "[FAIL] alloc_space degraded: base=${b_alloc}B new=${n_alloc}B"
        failures=$((failures + 1))
      else
        echo "[OK]   alloc_space: base=${b_alloc}B new=${n_alloc}B"
      fi
    else
      echo "[WARN] could not parse alloc profile totals; skip alloc gate"
    fi
  fi
else
  echo "[WARN] alloc profiles are missing (expected pprof/allocs.pb.gz); skip alloc gate"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "Regression gate FAILED: $failures violation(s)"
  exit 1
fi

echo "Regression gate PASSED"
