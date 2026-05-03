-- Idempotent seed data for staging PR environments.
-- Safe to run multiple times — all inserts use ON CONFLICT DO NOTHING.

INSERT INTO schema_info (key, value)
VALUES ('seeded', 'true')
ON CONFLICT (key) DO NOTHING;
