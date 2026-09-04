-- +migrate Down
DROP TABLE IF EXISTS billing_renewal_jobs;
DROP TABLE IF EXISTS billing_events;
DROP TABLE IF EXISTS billing_orders;
ALTER TABLE users DROP COLUMN IF EXISTS trial_used;
ALTER TABLE projects
    DROP COLUMN IF EXISTS plan,
    DROP COLUMN IF EXISTS trial_started_at,
    DROP COLUMN IF EXISTS trial_ends_at,
    DROP COLUMN IF EXISTS trial_used,
    DROP COLUMN IF EXISTS current_period_start,
    DROP COLUMN IF EXISTS current_period_end,
    DROP COLUMN IF EXISTS cancel_at_period_end,
    DROP COLUMN IF EXISTS billing_provider,
    DROP COLUMN IF EXISTS provider_customer_id,
    DROP COLUMN IF EXISTS provider_subscription_id,
    DROP COLUMN IF EXISTS billing_status;
