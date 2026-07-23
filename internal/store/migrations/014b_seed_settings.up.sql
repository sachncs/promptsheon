-- 014b: seed the otl.* defaults so the first-boot GET has rows.
--
-- The settings layer's "get" path returns rows (or the hardcoded
-- default if no row exists). On a fresh boot, before the operator
-- has run any `settings set` command, the table is empty; the
-- resolver falls back to the env defaults for every key.
--
-- These seed rows exist only so the GET endpoint has a stable
-- shape on day-zero; the resolver still reads env vars first
-- (env is the floor; DB is the ceiling; the seed is a no-op
-- when env is set).

INSERT OR IGNORE INTO system_config (key, value, updated_by) VALUES
    ('otl.endpoint',     '""',                  'system'),
    ('otl.insecure',     'false',               'system'),
    ('otl.sample_ratio', '1.0',                 'system'),
    ('llm.openai.api_key_ref',     '""',           'system'),
    ('llm.anthropic.api_key_ref',  '""',           'system');
