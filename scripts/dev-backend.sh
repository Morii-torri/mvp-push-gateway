#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v go >/dev/null 2>&1; then
  echo "go is required but was not found in PATH" >&2
  exit 1
fi

cd "$ROOT_DIR/backend"
exec go run ./cmd/server
