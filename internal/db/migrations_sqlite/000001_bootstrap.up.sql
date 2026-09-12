-- +migrate Up
CREATE TABLE IF NOT EXISTS schema_meta (
    key        text PRIMARY KEY,
    value      text NOT NULL,
    updated_at text NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO schema_meta (key, value)
VALUES ('app', 'llms'), ('bootstrap_task', 'OSS-SQLite')
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = datetime('now');
