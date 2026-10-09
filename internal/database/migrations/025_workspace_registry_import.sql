CREATE TABLE workspace_setup (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    society_key TEXT NOT NULL UNIQUE,
    society_name TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('FICTIONAL_REHEARSAL','LOCAL_WORKSPACE')),
    bootstrap_id TEXT NOT NULL UNIQUE,
    input_digest TEXT NOT NULL,
    administrator_id TEXT NOT NULL REFERENCES users(id),
    bootstrap_password_hash TEXT NOT NULL,
    mfa_key_fingerprint TEXT NOT NULL,
    message_key_fingerprint TEXT NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER workspace_setup_no_update BEFORE UPDATE ON workspace_setup BEGIN
    SELECT RAISE(ABORT, 'workspace setup is immutable');
END;
CREATE TRIGGER workspace_setup_no_delete BEFORE DELETE ON workspace_setup BEGIN
    SELECT RAISE(ABORT, 'workspace setup is immutable');
END;

CREATE TABLE registry_imports (
    id TEXT PRIMARY KEY,
    operation_key TEXT NOT NULL UNIQUE,
    request_digest TEXT NOT NULL,
    source_key TEXT NOT NULL UNIQUE,
    input_digest TEXT NOT NULL,
    base_digest TEXT NOT NULL,
    effective_date TEXT NOT NULL,
    counts_json TEXT NOT NULL CHECK (json_valid(counts_json)),
    actor_user_id TEXT NOT NULL REFERENCES users(id),
    verification_note TEXT NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER registry_imports_no_update BEFORE UPDATE ON registry_imports BEGIN
    SELECT RAISE(ABORT, 'registry import provenance is immutable');
END;
CREATE TRIGGER registry_imports_no_delete BEFORE DELETE ON registry_imports BEGIN
    SELECT RAISE(ABORT, 'registry import provenance is immutable');
END;

CREATE TABLE registry_import_entities (
    import_id TEXT NOT NULL REFERENCES registry_imports(id),
    entity_kind TEXT NOT NULL CHECK (entity_kind IN ('BUILDING','HOME','PERSON','RELATIONSHIP')),
    source_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    PRIMARY KEY (import_id,entity_kind,source_id),
    UNIQUE (entity_kind,entity_id)
) STRICT;
CREATE TRIGGER registry_import_entities_no_update BEFORE UPDATE ON registry_import_entities BEGIN
    SELECT RAISE(ABORT, 'registry source identities are immutable');
END;
CREATE TRIGGER registry_import_entities_no_delete BEFORE DELETE ON registry_import_entities BEGIN
    SELECT RAISE(ABORT, 'registry source identities are immutable');
END;
