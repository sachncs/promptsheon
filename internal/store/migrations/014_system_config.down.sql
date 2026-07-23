-- Roll back 014_system_config.
DROP INDEX IF EXISTS system_config_updated_at_idx;
DROP TABLE IF EXISTS system_config;
