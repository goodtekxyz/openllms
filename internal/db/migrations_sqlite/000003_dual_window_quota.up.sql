-- SQLite: dual-window quota columns (Phase B).
ALTER TABLE accounts ADD COLUMN quota_5h_remaining_pct REAL NULL;
ALTER TABLE accounts ADD COLUMN quota_5h_reset_at TEXT NULL;
ALTER TABLE accounts ADD COLUMN quota_7d_remaining_pct REAL NULL;
ALTER TABLE accounts ADD COLUMN quota_7d_reset_at TEXT NULL;
