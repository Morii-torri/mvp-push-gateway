#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
timeout_seconds="${MGP_DB_READY_TIMEOUT_SECONDS:-60}"

if [[ -z "${MGP_POSTGRES_DSN:-}" ]]; then
  echo "MGP_POSTGRES_DSN is required" >&2
  exit 2
fi

echo "waiting for postgres"
start_time="$SECONDS"
until psql "$MGP_POSTGRES_DSN" -v ON_ERROR_STOP=1 -c 'SELECT 1' >/dev/null 2>&1; do
  if (( SECONDS - start_time >= timeout_seconds )); then
    echo "postgres did not become ready within ${timeout_seconds}s" >&2
    exit 1
  fi
  sleep 2
done

if [[ "${MGP_SKIP_MIGRATIONS:-false}" != "true" ]]; then
  "$ROOT_DIR/scripts/apply-migrations.sh"
fi

exec "$ROOT_DIR/mgp-server"
