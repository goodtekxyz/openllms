-- +migrate Down
-- Best-effort reverse: restore prior paid soft-caps; leave free rows as free.
UPDATE projects
SET soft_cap_tokens = 10000000
WHERE plan = 'starter' AND soft_cap_tokens = 0;

UPDATE projects
SET soft_cap_tokens = 50000000
WHERE plan = 'pro' AND soft_cap_tokens = 0;
