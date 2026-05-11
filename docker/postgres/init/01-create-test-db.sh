#!/bin/sh
set -eu

if [ -z "${MGP_TEST_POSTGRES_DB:-}" ]; then
  exit 0
fi

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
  SELECT 'CREATE DATABASE ${MGP_TEST_POSTGRES_DB} OWNER ${POSTGRES_USER}'
  WHERE NOT EXISTS (
    SELECT FROM pg_database WHERE datname = '${MGP_TEST_POSTGRES_DB}'
  )\gexec
EOSQL
