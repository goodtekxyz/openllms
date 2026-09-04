-- +migrate Up
CREATE TABLE routes (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    slug          text NOT NULL,
    strategy      text NOT NULL DEFAULT 'sequential',
    config        jsonb NOT NULL DEFAULT '{}'::jsonb,
    default_model text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, slug)
);

CREATE INDEX routes_slug_idx ON routes (slug);

ALTER TABLE api_keys
    ADD CONSTRAINT api_keys_route_id_fkey
    FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE SET NULL;
