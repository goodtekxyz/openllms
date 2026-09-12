-- +migrate Up
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS quota_5h_remaining_pct double precision NULL,
    ADD COLUMN IF NOT EXISTS quota_5h_reset_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS quota_7d_remaining_pct double precision NULL,
    ADD COLUMN IF NOT EXISTS quota_7d_reset_at timestamptz NULL;
