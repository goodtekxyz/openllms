-- +migrate Up
ALTER TABLE projects
    ADD COLUMN soft_cap_usd numeric(12,4) NULL,
    ADD COLUMN soft_cap_tokens bigint NULL;

CREATE TABLE rate_limit_buckets (
    api_key_id uuid PRIMARY KEY REFERENCES api_keys(id) ON DELETE CASCADE,
    window_start timestamptz NOT NULL,
    count int NOT NULL DEFAULT 0
);
