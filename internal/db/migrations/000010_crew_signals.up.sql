-- +migrate Up
CREATE TABLE IF NOT EXISTS crew_signals (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    login      text NOT NULL DEFAULT '',
    note       text NOT NULL DEFAULT '',
    signal_day date NOT NULL DEFAULT ((CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::date),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, signal_day)
);

CREATE INDEX IF NOT EXISTS crew_signals_created_at_idx ON crew_signals (created_at DESC);
