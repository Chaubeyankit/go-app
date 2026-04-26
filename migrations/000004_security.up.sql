-- OAuth identities — a user can have multiple providers linked
CREATE TABLE oauth_identities (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     VARCHAR(50)  NOT NULL,   -- "google" | "github"
    provider_id  VARCHAR(255) NOT NULL,   -- provider's user ID
    email        VARCHAR(255) NOT NULL,
    name         VARCHAR(255),
    avatar_url   VARCHAR(500),
    access_token TEXT,                    -- encrypted at rest in production
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_oauth_provider UNIQUE (provider, provider_id)
);

CREATE INDEX idx_oauth_identities_user_id  ON oauth_identities(user_id);
CREATE INDEX idx_oauth_identities_provider ON oauth_identities(provider, provider_id);

-- MFA secrets — one row per user, nullable means MFA not yet enabled
ALTER TABLE users
    ADD COLUMN mfa_enabled     BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN mfa_secret_enc  TEXT,       -- AES-encrypted TOTP secret
    ADD COLUMN mfa_enabled_at  TIMESTAMPTZ;

-- API keys — named, scoped, revocable
CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         VARCHAR(100) NOT NULL,
    key_hash     VARCHAR(64)  NOT NULL,   -- SHA-256 of the raw key
    key_prefix   VARCHAR(12)  NOT NULL,   -- first 8 chars for display: "sk_live_ab12"
    scopes       TEXT[]       NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,            -- NULL = never expires
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_api_keys_hash    ON api_keys(key_hash);
CREATE INDEX        idx_api_keys_user_id ON api_keys(user_id);