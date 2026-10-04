ALTER TABLE users ADD COLUMN verified_at INTEGER;
ALTER TABLE users ADD COLUMN password_changed_at INTEGER;
ALTER TABLE users ADD COLUMN is_demo INTEGER NOT NULL DEFAULT 0 CHECK (is_demo IN (0, 1));
ALTER TABLE users ADD COLUMN mfa_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN mfa_window_start INTEGER NOT NULL DEFAULT 0;
UPDATE users SET verified_at = created_at, password_changed_at = created_at, is_demo = 1
WHERE id IN ('demo-user-admin', 'demo-user-committee', 'demo-user-owner', 'demo-user-tenant');

ALTER TABLE sessions ADD COLUMN mfa_verified_at INTEGER;
ALTER TABLE sessions ADD COLUMN reauthenticated_at INTEGER;
UPDATE sessions SET reauthenticated_at = created_at;
-- Existing privileged password-only sessions must not keep administrative access.
DELETE FROM sessions;

CREATE TABLE mfa_factors (
    user_id TEXT PRIMARY KEY REFERENCES users(id),
    secret_ciphertext BLOB NOT NULL,
    confirmed_at INTEGER NOT NULL,
    last_step INTEGER NOT NULL DEFAULT -1
) STRICT;

CREATE TABLE mfa_pending (
    session_hash TEXT PRIMARY KEY REFERENCES sessions(token_hash) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id),
    secret_ciphertext BLOB NOT NULL,
    expires_at INTEGER NOT NULL
) STRICT;

CREATE TABLE mfa_recovery_codes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    code_hash TEXT NOT NULL UNIQUE,
    consumed_at INTEGER,
    expires_at INTEGER,
    session_hash TEXT REFERENCES sessions(token_hash) ON DELETE CASCADE
) STRICT;
CREATE INDEX recovery_codes_user ON mfa_recovery_codes(user_id);

CREATE TABLE account_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    purpose TEXT NOT NULL CHECK (purpose IN ('INVITE', 'PASSWORD_RESET')),
    token_hash TEXT NOT NULL UNIQUE,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER,
    revoked_at INTEGER,
    auth_version INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    verification_note TEXT NOT NULL
) STRICT;
CREATE INDEX account_tokens_user ON account_tokens(user_id, purpose);
CREATE UNIQUE INDEX user_resident_unique ON users(resident_id) WHERE resident_id IS NOT NULL;
