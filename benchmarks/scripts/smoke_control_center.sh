#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:18081}"
TOKEN="${GATEWAY_TOKEN:-}"
USER_ID="${GOCLAW_USER_ID:-smoke-admin}"

hdr=(-H "Accept: application/json" -H "X-GoClaw-User-Id: ${USER_ID}")
if [[ -n "${TOKEN}" ]]; then
  hdr+=(-H "Authorization: Bearer ${TOKEN}")
fi

echo "smoke: GET ${BASE_URL}/v1/admin/control-center/overview"
code=$(curl -sS -o /tmp/goclaw_smoke_control_center.json -w "%{http_code}" "${hdr[@]}" "${BASE_URL}/v1/admin/control-center/overview")

if [[ "${code}" != "200" ]]; then
  echo "smoke failed: http ${code}"
  cat /tmp/goclaw_smoke_control_center.json || true
  exit 1
fi

jq -e '.agents and .recent_actions and .errors' /tmp/goclaw_smoke_control_center.json >/dev/null
echo "smoke passed"
