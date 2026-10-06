CREATE TABLE statement_groups (
 id TEXT PRIMARY KEY,
 current_file_id TEXT REFERENCES statement_files(id) DEFERRABLE INITIALLY DEFERRED,
 current_publication_id TEXT REFERENCES statement_publications(id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TABLE statement_files (
 id TEXT PRIMARY KEY,
 group_id TEXT NOT NULL REFERENCES statement_groups(id),
 replaces_id TEXT REFERENCES statement_files(id),
 title TEXT NOT NULL CHECK(length(title) BETWEEN 5 AND 120),
 kind TEXT NOT NULL CHECK(kind IN('INCOME','BALANCE','BUDGET','AUDIT')),
 period_start TEXT NOT NULL,
 period_end TEXT NOT NULL CHECK(period_end>=period_start),
 prepared_by TEXT NOT NULL CHECK(length(prepared_by) BETWEEN 2 AND 120),
 source TEXT NOT NULL CHECK(length(source) BETWEEN 5 AND 300),
 filename TEXT NOT NULL CHECK(length(filename) BETWEEN 1 AND 150),
 size_bytes INTEGER NOT NULL CHECK(size_bytes BETWEEN 1 AND 10485760),
 sha256 TEXT NOT NULL CHECK(length(sha256)=64),
 content_type TEXT NOT NULL DEFAULT '' CHECK(content_type IN('','application/pdf','text/csv; charset=utf-8','application/vnd.openxmlformats-officedocument.spreadsheetml.sheet')),
 uploaded_by TEXT NOT NULL REFERENCES users(id),
 created_at INTEGER NOT NULL,
 expires_at INTEGER NOT NULL,
 uploaded_at INTEGER NOT NULL DEFAULT 0,
 available_at INTEGER NOT NULL DEFAULT 0,
 validation TEXT NOT NULL DEFAULT 'PENDING' CHECK(validation IN('PENDING','VALIDATING','AVAILABLE','REJECTED','ABANDONED')),
 validation_code TEXT NOT NULL DEFAULT '',
 lease_until INTEGER NOT NULL DEFAULT 0,
 lease_token TEXT NOT NULL DEFAULT '',
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3),
 state TEXT NOT NULL DEFAULT 'PENDING' CHECK(state IN('PENDING','APPROVED','DECLINED','WITHDRAWN')),
 reviewed_by TEXT REFERENCES users(id),
 reviewed_at INTEGER NOT NULL DEFAULT 0,
 revision INTEGER NOT NULL DEFAULT 0 CHECK(revision>=0),
 version INTEGER NOT NULL DEFAULT 1 CHECK(version>=1),
 CHECK(validation<>'AVAILABLE' OR (content_type<>'' AND available_at>0)),
 CHECK(state<>'APPROVED' OR (validation='AVAILABLE' AND revision>0 AND reviewed_by IS NOT NULL AND reviewed_by<>uploaded_by AND reviewed_at>0))
) STRICT;
CREATE UNIQUE INDEX statement_one_pending_file ON statement_files(group_id) WHERE state='PENDING' AND validation IN('PENDING','VALIDATING','AVAILABLE');
CREATE INDEX statement_files_order ON statement_files(group_id,created_at,id);
CREATE TABLE statement_objects (
 file_id TEXT PRIMARY KEY REFERENCES statement_files(id),
 original_bytes BLOB NOT NULL CHECK(length(original_bytes) BETWEEN 1 AND 10485760)
) STRICT;
CREATE TABLE statement_events (
 id TEXT PRIMARY KEY,
 file_id TEXT NOT NULL REFERENCES statement_files(id),
 actor_id TEXT REFERENCES users(id),
 action TEXT NOT NULL CHECK(action IN('RESERVED','UPLOADED','VALIDATING','VALIDATED','REJECTED','ABANDONED','APPROVED','DECLINED','WITHDRAWN','RETRY_VALIDATION')),
 reason TEXT NOT NULL,
 occurred_at INTEGER NOT NULL,
 version INTEGER NOT NULL,
 UNIQUE(file_id,version)
) STRICT;
CREATE TABLE statement_publications (
 id TEXT PRIMARY KEY,
 group_id TEXT NOT NULL REFERENCES statement_groups(id),
 file_id TEXT NOT NULL REFERENCES statement_files(id),
 target_json TEXT NOT NULL CHECK(json_valid(target_json)),
 preview_hash TEXT NOT NULL CHECK(length(preview_hash)=64),
 target_people INTEGER NOT NULL CHECK(target_people>0 AND target_people<=2000),
 state TEXT NOT NULL DEFAULT 'PENDING' CHECK(state IN('PENDING','PUBLISHED','DECLINED','WITHDRAWN','REVOKED','SUPERSEDED')),
 proposed_by TEXT NOT NULL REFERENCES users(id),
 proposed_at INTEGER NOT NULL,
 reviewed_by TEXT REFERENCES users(id),
 reviewed_at INTEGER NOT NULL DEFAULT 0,
 version INTEGER NOT NULL DEFAULT 1 CHECK(version>=1),
 CHECK(state<>'PUBLISHED' OR (reviewed_by IS NOT NULL AND reviewed_by<>proposed_by AND reviewed_at>0))
) STRICT;
CREATE UNIQUE INDEX statement_one_pending_publication ON statement_publications(group_id) WHERE state='PENDING';
CREATE TABLE statement_publication_events (
 id TEXT PRIMARY KEY,
 publication_id TEXT NOT NULL REFERENCES statement_publications(id),
 actor_id TEXT NOT NULL REFERENCES users(id),
 action TEXT NOT NULL CHECK(action IN('PROPOSED','PUBLISHED','DECLINED','WITHDRAWN','REVOKED','SUPERSEDED')),
 reason TEXT NOT NULL,
 occurred_at INTEGER NOT NULL,
 version INTEGER NOT NULL,
 UNIQUE(publication_id,version)
) STRICT;
CREATE TABLE statement_accesses (
 id TEXT PRIMARY KEY,
 file_id TEXT NOT NULL REFERENCES statement_files(id),
 publication_id TEXT REFERENCES statement_publications(id),
 actor_id TEXT NOT NULL REFERENCES users(id),
 occurred_at INTEGER NOT NULL,
 bytes INTEGER NOT NULL CHECK(bytes>0),
 sha256 TEXT NOT NULL CHECK(length(sha256)=64)
) STRICT;
CREATE TRIGGER statement_file_identity BEFORE UPDATE ON statement_files WHEN
 NEW.id<>OLD.id OR NEW.group_id<>OLD.group_id OR NEW.replaces_id IS NOT OLD.replaces_id OR NEW.title<>OLD.title OR NEW.kind<>OLD.kind
 OR NEW.period_start<>OLD.period_start OR NEW.period_end<>OLD.period_end OR NEW.prepared_by<>OLD.prepared_by OR NEW.source<>OLD.source
 OR NEW.filename<>OLD.filename OR NEW.size_bytes<>OLD.size_bytes OR NEW.sha256<>OLD.sha256 OR NEW.uploaded_by<>OLD.uploaded_by OR NEW.created_at<>OLD.created_at OR NEW.expires_at<>OLD.expires_at
 OR (OLD.content_type<>'' AND NEW.content_type<>OLD.content_type) OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'statement original identity and versions are preserved'); END;
CREATE TRIGGER statement_file_retain BEFORE DELETE ON statement_files BEGIN SELECT RAISE(ABORT,'retain statement versions'); END;
CREATE TRIGGER statement_object_identity BEFORE UPDATE ON statement_objects BEGIN SELECT RAISE(ABORT,'retain unchanged statement originals'); END;
CREATE TRIGGER statement_object_retain BEFORE DELETE ON statement_objects BEGIN SELECT RAISE(ABORT,'retain statement originals'); END;
CREATE TRIGGER statement_event_identity BEFORE UPDATE ON statement_events BEGIN SELECT RAISE(ABORT,'retain statement review proof'); END;
CREATE TRIGGER statement_event_retain BEFORE DELETE ON statement_events BEGIN SELECT RAISE(ABORT,'retain statement review proof'); END;
CREATE TRIGGER statement_publication_identity BEFORE UPDATE ON statement_publications WHEN
 NEW.id<>OLD.id OR NEW.group_id<>OLD.group_id OR NEW.file_id<>OLD.file_id OR NEW.target_json<>OLD.target_json OR NEW.preview_hash<>OLD.preview_hash OR NEW.target_people<>OLD.target_people OR NEW.proposed_by<>OLD.proposed_by OR NEW.proposed_at<>OLD.proposed_at OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'publication freezes a reviewed original and audience'); END;
CREATE TRIGGER statement_publication_retain BEFORE DELETE ON statement_publications BEGIN SELECT RAISE(ABORT,'retain publication decisions'); END;
CREATE TRIGGER statement_publication_event_identity BEFORE UPDATE ON statement_publication_events BEGIN SELECT RAISE(ABORT,'retain publication history'); END;
CREATE TRIGGER statement_publication_event_retain BEFORE DELETE ON statement_publication_events BEGIN SELECT RAISE(ABORT,'retain publication history'); END;
CREATE TRIGGER statement_access_identity BEFORE UPDATE ON statement_accesses BEGIN SELECT RAISE(ABORT,'retain original access proof'); END;
CREATE TRIGGER statement_access_retain BEFORE DELETE ON statement_accesses BEGIN SELECT RAISE(ABORT,'retain original access proof'); END;
CREATE TRIGGER statement_group_head BEFORE UPDATE OF current_file_id ON statement_groups WHEN NEW.current_file_id IS NOT NULL
 AND NOT EXISTS(SELECT 1 FROM statement_files f WHERE f.id=NEW.current_file_id AND f.group_id=NEW.id AND f.state='APPROVED' AND f.validation='AVAILABLE')
 BEGIN SELECT RAISE(ABORT,'statement head requires an approved original'); END;
CREATE TRIGGER statement_group_publication BEFORE UPDATE OF current_publication_id ON statement_groups WHEN NEW.current_publication_id IS NOT NULL
 AND NOT EXISTS(SELECT 1 FROM statement_publications p JOIN statement_files f ON f.id=p.file_id WHERE p.id=NEW.current_publication_id AND p.group_id=NEW.id AND p.state='PUBLISHED' AND f.group_id=NEW.id AND f.state='APPROVED' AND f.validation='AVAILABLE')
 BEGIN SELECT RAISE(ABORT,'resident publication requires a separate approved release'); END;
