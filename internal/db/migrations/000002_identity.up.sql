-- +migrate Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id  text UNIQUE,
    login      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       text NOT NULL DEFAULT 'default',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name         text NOT NULL DEFAULT 'default',
    key_prefix   text NOT NULL,
    key_hash     text NOT NULL UNIQUE,
    route_id     uuid NULL,
    revoked_at   timestamptz NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX api_keys_prefix_idx ON api_keys (key_prefix);
CREATE INDEX api_keys_project_idx ON api_keys (project_id);
