-- +migrate Up
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS plan text NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS trial_started_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS trial_ends_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS trial_used boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS current_period_start timestamptz NULL,
    ADD COLUMN IF NOT EXISTS current_period_end timestamptz NULL,
    ADD COLUMN IF NOT EXISTS cancel_at_period_end boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS billing_provider text NULL,
    ADD COLUMN IF NOT EXISTS provider_customer_id text NULL,
    ADD COLUMN IF NOT EXISTS provider_subscription_id text NULL,
    ADD COLUMN IF NOT EXISTS billing_status text NOT NULL DEFAULT 'none';

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS trial_used boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS billing_orders (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider          text NOT NULL,
    provider_order_id text NOT NULL DEFAULT '',
    transaction_id    text NOT NULL DEFAULT '',
    plan              text NOT NULL,
    amount_usd        numeric(10,2) NOT NULL DEFAULT 0,
    currency          text NOT NULL DEFAULT 'USD',
    period_start      timestamptz NULL,
    period_end        timestamptz NULL,
    status            text NOT NULL DEFAULT 'pending',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS billing_orders_provider_tx_uidx
    ON billing_orders (provider, transaction_id)
    WHERE transaction_id <> '';

CREATE INDEX IF NOT EXISTS billing_orders_project_idx ON billing_orders (project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS billing_events (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider     text NOT NULL,
    event_id     text NOT NULL DEFAULT '',
    event_type   text NOT NULL,
    payload      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, event_id)
);

CREATE TABLE IF NOT EXISTS billing_renewal_jobs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    due_at     timestamptz NOT NULL,
    link_url   text NOT NULL DEFAULT '',
    link_id    text NOT NULL DEFAULT '',
    state      text NOT NULL DEFAULT 'scheduled',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS billing_renewal_due_idx ON billing_renewal_jobs (due_at) WHERE state = 'scheduled';
