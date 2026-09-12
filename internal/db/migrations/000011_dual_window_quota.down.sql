-- +migrate Down
ALTER TABLE accounts
    DROP COLUMN IF EXISTS quota_5h_remaining_pct,
    DROP COLUMN IF EXISTS quota_5h_reset_at,
    DROP COLUMN IF EXISTS quota_7d_remaining_pct,
    DROP COLUMN IF EXISTS quota_7d_reset_at;
