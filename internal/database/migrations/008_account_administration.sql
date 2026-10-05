ALTER TABLE users ADD COLUMN access_version INTEGER NOT NULL DEFAULT 1 CHECK (access_version > 0);
ALTER TABLE users ADD COLUMN suspended_at INTEGER CHECK (suspended_at IS NULL OR suspended_at > 0);

ALTER TABLE role_grants RENAME TO previous_role_grants;
CREATE TABLE role_grants (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    role TEXT NOT NULL CHECK (role IN ('ADMINISTRATOR', 'COMMITTEE', 'TREASURER', 'AUDITOR')),
    valid_from INTEGER NOT NULL,
    valid_until INTEGER NOT NULL CHECK (valid_until > valid_from),
    revoked_at INTEGER,
    granted_by TEXT REFERENCES users(id),
    revoked_by TEXT REFERENCES users(id)
) STRICT;
INSERT INTO role_grants(id,user_id,role,valid_from,valid_until,revoked_at,granted_by)
    SELECT id,user_id,role,valid_from,valid_until,revoked_at,granted_by FROM previous_role_grants;
DROP TABLE previous_role_grants;
CREATE INDEX role_grants_user ON role_grants(user_id);
CREATE INDEX account_access_audit ON audit_events(json_extract(before_json,'$.user_id'),id DESC);
