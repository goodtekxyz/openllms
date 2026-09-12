-- +migrate Down
DROP TABLE IF EXISTS admin_notify_log;
ALTER TABLE users DROP COLUMN IF EXISTS last_login_at;
