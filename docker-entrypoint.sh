#!/bin/sh
set -e

case "${1:-serve}" in
  serve)
    # Managed mode: auto-upgrade (schema migrations + data hooks) before starting.
    if [ "$GOCLAW_MODE" = "managed" ] && [ -n "$GOCLAW_POSTGRES_DSN" ]; then
      # Wait briefly for Postgres to accept connections to avoid startup races.
      # We probe through `upgrade --status` because it exercises the same DB path
      # as migrations + runtime startup.
      i=0
      until /app/goclaw upgrade --status >/dev/null 2>&1; do
        i=$((i + 1))
        if [ "$i" -ge 30 ]; then
          echo "Managed mode: Postgres did not become ready within 60s; continuing with best effort."
          break
        fi
        echo "Managed mode: waiting for Postgres... ($i/30)"
        sleep 2
      done

      echo "Managed mode: running upgrade..."
      /app/goclaw upgrade || \
        echo "Upgrade warning (may already be up-to-date)"
    fi
    exec /app/goclaw
    ;;
  upgrade)
    shift
    exec /app/goclaw upgrade "$@"
    ;;
  migrate)
    shift
    exec /app/goclaw migrate "$@"
    ;;
  onboard)
    exec /app/goclaw onboard
    ;;
  version)
    exec /app/goclaw version
    ;;
  *)
    exec /app/goclaw "$@"
    ;;
esac
