-- 026_create_siwe_nonces.sql
-- SIWE nonces live in Postgres so /auth/nonce and /auth/verify can be served
-- by different instances. One active nonce per wallet (re-issue overwrites).

CREATE TABLE IF NOT EXISTS siwe_nonces (
    address VARCHAR(42) PRIMARY KEY,
    nonce VARCHAR(64) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_siwe_nonces_expires_at ON siwe_nonces(expires_at);
