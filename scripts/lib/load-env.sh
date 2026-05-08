#!/usr/bin/env bash

load_project_env() {
  local root_dir="$1"
  local env_file="$root_dir/.env"

  if [[ -f "$env_file" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$env_file"
    set +a
  fi
}
