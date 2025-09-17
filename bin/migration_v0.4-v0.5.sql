-- Migration from v0.4 to v0.5: Add time limit fields to tokens table
-- This migration adds support for time-based token restrictions

-- MySQL migration
-- ALTER TABLE tokens ADD COLUMN time_limit_enabled BOOLEAN DEFAULT FALSE AFTER `group`;
-- ALTER TABLE tokens ADD COLUMN time_limit_config VARCHAR(2048) DEFAULT '' AFTER time_limit_enabled;

-- PostgreSQL migration
-- ALTER TABLE tokens ADD COLUMN time_limit_enabled BOOLEAN DEFAULT FALSE;
-- ALTER TABLE tokens ADD COLUMN time_limit_config VARCHAR(2048) DEFAULT '';

-- SQLite migration (if needed)
-- ALTER TABLE tokens ADD COLUMN time_limit_enabled BOOLEAN DEFAULT 0;
-- ALTER TABLE tokens ADD COLUMN time_limit_config TEXT DEFAULT '';

-- Note: GORM AutoMigrate will handle adding these columns automatically
-- This file serves as documentation for the database schema changes
