#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
status=0
tmp_file="$(mktemp)"
trap 'rm -f "$tmp_file"' EXIT

find "$ROOT_DIR/scripts" "$ROOT_DIR/docker" -type f -name '*.sh' | sort > "$tmp_file"

while IFS= read -r file; do
  sh -n "$file" || status=$?
done < "$tmp_file"

if [[ "$status" -eq 0 ]]; then
  echo "shell-check-ok"
fi

exit "$status"
