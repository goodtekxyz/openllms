-- +migrate Up
CREATE TABLE accounts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    vendor          text NOT NULL,
    name            text NOT NULL,
    auth_type       text NOT NULL DEFAULT 'api_key',
    infisical_path  text NOT NULL,
    base_url        text NOT NULL DEFAULT '',
    health          text NOT NULL DEFAULT 'ok',
    cooldown_until  timestamptz NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, vendor, name)
);

CREATE TABLE route_accounts (
    route_id    uuid NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    account_id  uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    position    int NOT NULL DEFAULT 0,
    weight      int NOT NULL DEFAULT 1,
    PRIMARY KEY (route_id, account_id)
);

CREATE TABLE usage_events (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    route_id     uuid REFERENCES routes(id) ON DELETE SET NULL,
    account_id   uuid REFERENCES accounts(id) ON DELETE SET NULL,
    api_key_id   uuid REFERENCES api_keys(id) ON DELETE SET NULL,
    model        text NOT NULL DEFAULT '',
    status_code  int NOT NULL DEFAULT 0,
    latency_ms   int NOT NULL DEFAULT 0,
    tokens_in    int NOT NULL DEFAULT 0,
    tokens_out   int NOT NULL DEFAULT 0,
    error        text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX usage_events_project_created_idx ON usage_events (project_id, created_at DESC);
