#!/bin/sh
set -eu

: "${POSTGRES_DB:=mvp_push_gateway_dev}"
: "${POSTGRES_USER:=mvp_push_gateway}"

if [ -z "${POSTGRES_PASSWORD:-}" ]; then
  echo "POSTGRES_PASSWORD is required" >&2
  exit 2
fi

export POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD
export PGDATA="${PGDATA:-/var/lib/postgresql/data}"

mkdir -p "$PGDATA" /run/nginx

postgres_pid=""
backend_pid=""
nginx_pid=""

shutdown() {
  for pid in "$nginx_pid" "$backend_pid" "$postgres_pid"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
}

trap shutdown INT TERM

docker-entrypoint.sh postgres &
postgres_pid="$!"

echo "waiting for bundled postgres"
until pg_isready -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; do
  if ! kill -0 "$postgres_pid" 2>/dev/null; then
    echo "postgres exited before becoming ready" >&2
    exit 1
  fi
  sleep 1
done

if [ -z "${MGP_POSTGRES_DSN:-}" ]; then
  export MGP_POSTGRES_DSN="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable"
fi

/app/mgp-migrate

/app/mgp-server &
backend_pid="$!"

nginx -g "daemon off;" &
nginx_pid="$!"

while true; do
  for pid in "$postgres_pid" "$backend_pid" "$nginx_pid"; do
    if ! kill -0 "$pid" 2>/dev/null; then
      wait "$pid" || exit "$?"
      exit 1
    fi
  done
  sleep 2
done
