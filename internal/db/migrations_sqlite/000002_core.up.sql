-- +migrate Up
-- OSS core schema aligned with Postgres 000002–000007 (no Cloud billing).
-- TEXT UUIDs; application supplies primary keys.

CREATE TABLE users (
    id            text PRIMARY KEY NOT NULL,
    github_id     text UNIQUE,
    login         text NOT NULL DEFAULT '',
    created_at    text NOT NULL DEFAULT (datetime('now')),
    last_login_at text NULL
);

CREATE TABLE projects (
    id              text PRIMARY KEY NOT NULL,
    owner_id        text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            text NOT NULL DEFAULT 'default',
    created_at      text NOT NULL DEFAULT (datetime('now')),
    soft_cap_usd    real NULL,
    soft_cap_tokens integer NULL
);

CREATE TABLE api_keys (
    id           text PRIMARY KEY NOT NULL,
    project_id   text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name         text NOT NULL DEFAULT 'default',
    key_prefix   text NOT NULL,
    key_hash     text NOT NULL UNIQUE,
    route_id     text NULL,
    revoked_at   text NULL,
    created_at   text NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX api_keys_prefix_idx ON api_keys (key_prefix);
CREATE INDEX api_keys_project_idx ON api_keys (project_id);

CREATE TABLE routes (
    id            text PRIMARY KEY NOT NULL,
    project_id    text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    slug          text NOT NULL,
    strategy      text NOT NULL DEFAULT 'sequential',
    config        text NOT NULL DEFAULT '{}',
    default_model text NOT NULL DEFAULT '',
    created_at    text NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, slug)
);

CREATE INDEX routes_slug_idx ON routes (slug);
CREATE INDEX api_keys_route_id_idx ON api_keys (route_id);

CREATE TABLE accounts (
    id                   text PRIMARY KEY NOT NULL,
    project_id           text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    vendor               text NOT NULL,
    name                 text NOT NULL,
    auth_type            text NOT NULL DEFAULT 'api_key',
    infisical_path       text NOT NULL,
    base_url             text NOT NULL DEFAULT '',
    health               text NOT NULL DEFAULT 'ok',
    cooldown_until       text NULL,
    created_at           text NOT NULL DEFAULT (datetime('now')),
    quota_remaining_pct  real NULL,
    quota_reset_at       text NULL,
    quota_updated_at     text NULL,
    UNIQUE (project_id, vendor, name)
);

CREATE TABLE route_accounts (
    route_id   text NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    account_id text NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    position   integer NOT NULL DEFAULT 0,
    weight     integer NOT NULL DEFAULT 1,
    PRIMARY KEY (route_id, account_id)
);

CREATE TABLE usage_events (
    id          text PRIMARY KEY NOT NULL,
    project_id  text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    route_id    text REFERENCES routes(id) ON DELETE SET NULL,
    account_id  text REFERENCES accounts(id) ON DELETE SET NULL,
    api_key_id  text REFERENCES api_keys(id) ON DELETE SET NULL,
    model       text NOT NULL DEFAULT '',
    status_code integer NOT NULL DEFAULT 0,
    latency_ms  integer NOT NULL DEFAULT 0,
    tokens_in   integer NOT NULL DEFAULT 0,
    tokens_out  integer NOT NULL DEFAULT 0,
    error       text NOT NULL DEFAULT '',
    created_at  text NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX usage_events_project_created_idx ON usage_events (project_id, created_at DESC);

CREATE TABLE rate_limit_buckets (
    api_key_id   text PRIMARY KEY REFERENCES api_keys(id) ON DELETE CASCADE,
    window_start text NOT NULL,
    count        integer NOT NULL DEFAULT 0
);

CREATE TABLE admin_notify_log (
    id         text PRIMARY KEY NOT NULL,
    kind       text NOT NULL,
    subject    text NOT NULL,
    body       text NOT NULL,
    status     text NOT NULL DEFAULT 'pending',
    detail     text NOT NULL DEFAULT '',
    created_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX admin_notify_log_created_idx ON admin_notify_log (created_at DESC);
