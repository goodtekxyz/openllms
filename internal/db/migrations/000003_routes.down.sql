-- +migrate Down
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_route_id_fkey;
DROP TABLE IF EXISTS routes;
