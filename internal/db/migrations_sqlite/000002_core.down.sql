-- +migrate Down
DROP TABLE IF EXISTS admin_notify_log;
DROP TABLE IF EXISTS rate_limit_buckets;
DROP TABLE IF EXISTS usage_events;
DROP TABLE IF EXISTS route_accounts;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS routes;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS users;
