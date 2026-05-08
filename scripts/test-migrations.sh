#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/lib/load-env.sh"
load_project_env "$ROOT_DIR"

if [[ -z "${MGP_TEST_DATABASE_URL:-}" ]]; then
  echo "MGP_TEST_DATABASE_URL is required, for example postgres://user:pass@127.0.0.1:5432/mgp_test?sslmode=disable" >&2
  exit 2
fi

cd "$ROOT_DIR/backend"
go test ./internal/db -run TestMigrationsApplyToPostgres -count=1
