#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="${BASE:-}"
NEW_DIR="${NEW:-}"

if [[ -z "$BASE_DIR" || -z "$NEW_DIR" ]]; then
  echo "Usage: BASE=<path> NEW=<path> $0" >&2
  exit 1
fi

if [[ ! -d "$BASE_DIR" || ! -d "$NEW_DIR" ]]; then
  echo "Both BASE and NEW must be existing directories" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for compare script" >&2
  exit 1
fi

echo "Comparing runs"
echo "  base: $BASE_DIR"
echo "  new:  $NEW_DIR"

echo ""
printf "%-32s %-12s %-12s %-12s %-12s %-12s\n" "scenario" "p50(ms)" "p95(ms)" "req/s" "fail(%)" "p95 delta"

for new_summary in "$NEW_DIR"/*.summary.json; do
  [[ -f "$new_summary" ]] || continue
  scenario="$(basename "$new_summary" .summary.json)"
  base_summary="$BASE_DIR/$scenario.summary.json"

  if [[ ! -f "$base_summary" ]]; then
    printf "%-32s %-12s %-12s %-12s %-12s %-12s\n" "$scenario" "-" "-" "-" "-" "missing"
    continue
  fi

  b_p50=$(jq -r '.metrics.http_req_duration["p(50)"] // .metrics.http_req_duration.med // 0' "$base_summary")
  b_p95=$(jq -r '.metrics.http_req_duration["p(95)"] // 0' "$base_summary")
  b_rps=$(jq -r '.metrics.http_reqs.rate // 0' "$base_summary")
  b_fail=$(jq -r '.metrics.http_req_failed.value // 0' "$base_summary")

  n_p50=$(jq -r '.metrics.http_req_duration["p(50)"] // .metrics.http_req_duration.med // 0' "$new_summary")
  n_p95=$(jq -r '.metrics.http_req_duration["p(95)"] // 0' "$new_summary")
  n_rps=$(jq -r '.metrics.http_reqs.rate // 0' "$new_summary")
  n_fail=$(jq -r '.metrics.http_req_failed.value // 0' "$new_summary")

  delta=$(awk -v b="$b_p95" -v n="$n_p95" 'BEGIN { if (b == 0) { print "n/a" } else { printf "%+.1f%%", ((n-b)/b)*100 } }')

  printf "%-32s %-12.2f %-12.2f %-12.2f %-12.2f %-12s\n" \
    "$scenario" "$n_p50" "$n_p95" "$n_rps" "$(awk -v f="$n_fail" 'BEGIN {print f*100}')" "$delta"
done
