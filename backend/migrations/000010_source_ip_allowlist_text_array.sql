-- +goose Up

ALTER TABLE inbound_sources
    ALTER COLUMN ip_allowlist DROP DEFAULT,
    ALTER COLUMN ip_allowlist TYPE text[] USING COALESCE(ip_allowlist::text[], ARRAY[]::text[]),
    ALTER COLUMN ip_allowlist SET DEFAULT ARRAY[]::text[];

-- +goose Down

CREATE OR REPLACE FUNCTION mgp_ip_allowlist_to_cidr(values text[])
RETURNS cidr[]
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT COALESCE(array_agg(value::cidr), ARRAY[]::cidr[])
    FROM unnest(values) AS value
    WHERE position('-' in value) = 0
$$;

ALTER TABLE inbound_sources
    ALTER COLUMN ip_allowlist DROP DEFAULT,
    ALTER COLUMN ip_allowlist TYPE cidr[] USING mgp_ip_allowlist_to_cidr(ip_allowlist),
    ALTER COLUMN ip_allowlist SET DEFAULT ARRAY[]::cidr[];

DROP FUNCTION mgp_ip_allowlist_to_cidr(text[]);
