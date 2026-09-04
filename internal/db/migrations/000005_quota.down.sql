-- +migrate Down
DROP TABLE IF EXISTS rate_limit_buckets;
ALTER TABLE projects DROP COLUMN IF EXISTS soft_cap_usd;
ALTER TABLE projects DROP COLUMN IF EXISTS soft_cap_tokens;
