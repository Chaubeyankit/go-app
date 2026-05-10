-- Fix api_keys key_prefix column size
-- Backend generates 16 character prefix (sk_live_ + 8 hex chars) but column was varchar(12)

ALTER TABLE api_keys
    ALTER COLUMN key_prefix TYPE VARCHAR(16);
