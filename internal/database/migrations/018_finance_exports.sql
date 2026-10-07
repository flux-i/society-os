CREATE TABLE finance_exports (
    id TEXT PRIMARY KEY,
    actor_id TEXT NOT NULL REFERENCES users(id),
    filter_json TEXT NOT NULL CHECK(json_valid(filter_json)),
    authority TEXT NOT NULL CHECK(authority IN ('TREASURY','PERSONAL')),
    homes_json TEXT NOT NULL CHECK(json_valid(homes_json) AND json_array_length(homes_json)>0),
    scope_label TEXT NOT NULL,
    summary_json TEXT NOT NULL CHECK(json_valid(summary_json)),
    content_hash TEXT NOT NULL CHECK(length(content_hash)=64),
    sha256 TEXT NOT NULL CHECK(length(sha256)=64),
    row_count INTEGER NOT NULL CHECK(row_count BETWEEN 1 AND 10000),
    byte_length INTEGER NOT NULL CHECK(byte_length BETWEEN 1 AND 5242880),
    csv_bytes BLOB NOT NULL CHECK(length(csv_bytes)=byte_length),
    generated_at TEXT NOT NULL
) STRICT;
CREATE INDEX finance_export_actor ON finance_exports(actor_id,generated_at DESC,id);
CREATE TRIGGER finance_export_update BEFORE UPDATE ON finance_exports
BEGIN SELECT RAISE(ABORT,'reviewed export snapshots are immutable'); END;
CREATE TRIGGER finance_export_delete BEFORE DELETE ON finance_exports
BEGIN SELECT RAISE(ABORT,'export snapshots require an adopted retention policy'); END;
