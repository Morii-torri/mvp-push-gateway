-- +goose Up

ALTER TABLE delivery_channels
    ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE delivery_channels
    DROP COLUMN IF EXISTS description;
