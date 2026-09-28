-- +migrate Up
-- Free plan: migrate legacy trial rows; clear soft-cap on paid plans.
UPDATE projects
SET plan = 'free',
    billing_status = 'active',
    trial_ends_at = NULL,
    current_period_end = NULL,
    soft_cap_tokens = 5000000
WHERE plan = 'trial';

UPDATE projects
SET soft_cap_tokens = 0
WHERE plan IN ('starter', 'pro');
