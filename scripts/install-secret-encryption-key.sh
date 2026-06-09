#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${1:-$ROOT_DIR/.env}"

generate_key() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32
    return
  fi
  if command -v python3 >/dev/null 2>&1; then
    python3 - <<'PY'
import base64
import os

print(base64.b64encode(os.urandom(32)).decode("ascii"))
PY
    return
  fi
  echo "openssl or python3 is required to generate MGP_SECRET_ENCRYPTION_KEY" >&2
  exit 1
}

if [[ ! -f "$ENV_FILE" ]]; then
  touch "$ENV_FILE"
fi

if grep -Eq '^MGP_SECRET_ENCRYPTION_KEY=.+$' "$ENV_FILE"; then
  echo "secret encryption key already configured"
  exit 0
fi

KEY="$(generate_key)"
KEY_ID="primary-$(date -u +%Y%m%d%H%M%S)"

MGP_ENV_FILE="$ENV_FILE" \
MGP_GENERATED_SECRET_KEY="$KEY" \
MGP_GENERATED_SECRET_KEY_ID="$KEY_ID" \
python3 - <<'PY'
import os
from pathlib import Path

env_file = Path(os.environ["MGP_ENV_FILE"])
key = os.environ["MGP_GENERATED_SECRET_KEY"]
key_id = os.environ["MGP_GENERATED_SECRET_KEY_ID"]
lines = env_file.read_text(encoding="utf-8").splitlines()

def upsert(name: str, value: str) -> None:
    prefix = f"{name}="
    for index, line in enumerate(lines):
        if line.startswith(prefix):
            if not line[len(prefix):].strip():
                lines[index] = f"{name}={value}"
            return
    lines.append(f"{name}={value}")

upsert("MGP_SECRET_ENCRYPTION_KEY_ID", key_id)
upsert("MGP_SECRET_ENCRYPTION_KEY", key)
env_file.write_text("\n".join(lines) + "\n", encoding="utf-8")
PY

echo "secret encryption key installed in $ENV_FILE"
