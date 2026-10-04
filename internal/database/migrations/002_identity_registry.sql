ALTER TABLE flats ADD COLUMN version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0);

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    resident_id TEXT REFERENCES residents(id),
    login TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'DISABLED')),
    auth_version INTEGER NOT NULL DEFAULT 1 CHECK (auth_version > 0),
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE role_grants (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    role TEXT NOT NULL CHECK (role IN ('ADMINISTRATOR', 'COMMITTEE', 'TREASURER')),
    valid_from INTEGER NOT NULL,
    valid_until INTEGER NOT NULL CHECK (valid_until > valid_from),
    revoked_at INTEGER,
    granted_by TEXT REFERENCES users(id)
) STRICT;
CREATE INDEX role_grants_user ON role_grants(user_id);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    auth_version INTEGER NOT NULL,
    csrf_token TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
) STRICT;
CREATE INDEX sessions_user ON sessions(user_id);
CREATE INDEX sessions_expiry ON sessions(expires_at);

CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY,
    actor_user_id TEXT NOT NULL REFERENCES users(id),
    action TEXT NOT NULL,
    flat_id TEXT REFERENCES flats(id),
    occurred_at INTEGER NOT NULL,
    reason TEXT NOT NULL,
    before_json TEXT NOT NULL CHECK (json_valid(before_json)),
    after_json TEXT NOT NULL CHECK (json_valid(after_json))
) STRICT;
CREATE INDEX audit_flat ON audit_events(flat_id, id DESC);
CREATE TRIGGER audit_no_update BEFORE UPDATE ON audit_events BEGIN
    SELECT RAISE(ABORT, 'audit events are immutable');
END;
CREATE TRIGGER audit_no_delete BEFORE DELETE ON audit_events BEGIN
    SELECT RAISE(ABORT, 'audit events are immutable');
END;
