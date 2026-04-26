DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS oauth_identities;
ALTER TABLE users
    DROP COLUMN IF EXISTS mfa_enabled,
    DROP COLUMN IF EXISTS mfa_secret_enc,
    DROP COLUMN IF EXISTS mfa_enabled_at;