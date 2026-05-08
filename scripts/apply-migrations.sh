#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/lib/load-env.sh"
load_project_env "$ROOT_DIR"

if [[ -z "${MGP_POSTGRES_DSN:-}" ]]; then
  echo "MGP_POSTGRES_DSN is required" >&2
  exit 2
fi

if command -v psql >/dev/null 2>&1; then
  PSQL_BIN="psql"
elif [[ -x "/opt/homebrew/opt/postgresql@16/bin/psql" ]]; then
  PSQL_BIN="/opt/homebrew/opt/postgresql@16/bin/psql"
else
  echo "psql is required but was not found in PATH" >&2
  exit 1
fi

"$PSQL_BIN" "$MGP_POSTGRES_DSN" -v ON_ERROR_STOP=1 -c "
  CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    filename text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
  );
"

for migration in "$ROOT_DIR"/backend/migrations/*.sql; do
  filename="$(basename "$migration")"
  version="${filename%%_*}"
  applied="$("$PSQL_BIN" "$MGP_POSTGRES_DSN" -Atc "SELECT 1 FROM schema_migrations WHERE version = '$version'")"
  if [[ "$applied" == "1" ]]; then
    echo "migration $filename already applied"
    continue
  fi

  temp_file="$(mktemp)"
  {
    echo "BEGIN;"
    awk '
      /^-- \+goose Up/ { in_up = 1; next }
      /^-- \+goose Down/ { exit }
      in_up == 1 { print }
    ' "$migration"
    echo "INSERT INTO schema_migrations (version, filename) VALUES ('$version', '$filename');"
    echo "COMMIT;"
  } > "$temp_file"

  "$PSQL_BIN" "$MGP_POSTGRES_DSN" -v ON_ERROR_STOP=1 -f "$temp_file"
  rm -f "$temp_file"
  echo "applied migration $filename"
done
