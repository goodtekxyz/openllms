-- +migrate Down
ALTER TABLE accounts
    DROP COLUMN IF EXISTS quota_remaining_pct,
    DROP COLUMN IF EXISTS quota_reset_at,
    DROP COLUMN IF EXISTS quota_updated_at;
