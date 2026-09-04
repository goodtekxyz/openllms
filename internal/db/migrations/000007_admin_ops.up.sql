-- +migrate Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at timestamptz NULL;

CREATE TABLE IF NOT EXISTS admin_notify_log (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind       text NOT NULL,
    subject    text NOT NULL,
    body       text NOT NULL,
    status     text NOT NULL DEFAULT 'pending',
    detail     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS admin_notify_log_created_idx ON admin_notify_log (created_at DESC);
