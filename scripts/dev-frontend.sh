#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v npm >/dev/null 2>&1; then
  echo "npm is required but was not found in PATH" >&2
  exit 1
fi

cd "$ROOT_DIR/frontend"
if [ ! -d node_modules ]; then
  npm install
fi

exec npm run dev
