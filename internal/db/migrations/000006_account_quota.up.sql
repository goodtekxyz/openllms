-- +migrate Up
ALTER TABLE accounts
    ADD COLUMN quota_remaining_pct double precision NULL,
    ADD COLUMN quota_reset_at timestamptz NULL,
    ADD COLUMN quota_updated_at timestamptz NULL;
