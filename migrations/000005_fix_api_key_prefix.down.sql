-- Rollback: This would truncate existing prefixes, so we'll warn instead
-- DO NOT RUN THIS MIGRATION DOWN IF YOU HAVE EXISTING API KEYS

-- Uncomment below to rollback (will truncate prefixes to 12 chars):
-- ALTER TABLE api_keys
--     ALTER COLUMN key_prefix TYPE VARCHAR(12);
