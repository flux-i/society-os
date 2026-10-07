-- Rebuild only the constrained parent. Migrate changes foreign-key mode
-- before its dedicated transaction, checks retained references, and restores it.
CREATE TABLE new_message_batches (
 id TEXT PRIMARY KEY,
 source_kind TEXT NOT NULL CHECK(source_kind IN('NOTICE','RECEIPT','STATEMENT')),
 source_id TEXT NOT NULL,
 source_json TEXT NOT NULL CHECK(json_valid(source_json)),
 target_json TEXT NOT NULL CHECK(json_valid(target_json)),
 counts_json TEXT NOT NULL CHECK(json_valid(counts_json)),
 preview_hash TEXT NOT NULL,
 channel TEXT NOT NULL CHECK(channel IN('WHATSAPP','EMAIL')),
 purpose TEXT NOT NULL CHECK(purpose IN('COMMUNITY','FINANCE')),
 portal_origin TEXT NOT NULL,
 envelope TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN('PENDING','APPROVED','DECLINED','WITHDRAWN','CANCELLED')),
 proposed_by TEXT NOT NULL REFERENCES users(id),
 proposed_at INTEGER NOT NULL,
 reviewed_by TEXT REFERENCES users(id),
 reviewed_at INTEGER NOT NULL DEFAULT 0,
 updated_at INTEGER NOT NULL,
 snapshot_version INTEGER NOT NULL CHECK(snapshot_version>0),
 version INTEGER NOT NULL CHECK(version>0),
 CHECK((source_kind='NOTICE' AND purpose='COMMUNITY') OR (source_kind IN('RECEIPT','STATEMENT') AND purpose='FINANCE')),
 CHECK(state<>'APPROVED' OR (reviewed_by IS NOT NULL AND reviewed_by<>proposed_by AND reviewed_at>0))
) STRICT;
INSERT INTO new_message_batches SELECT * FROM message_batches ORDER BY rowid;
DROP TABLE message_batches;
ALTER TABLE new_message_batches RENAME TO message_batches;
CREATE INDEX message_batch_queue ON message_batches(state,updated_at,id);
CREATE TRIGGER message_batch_delete BEFORE DELETE ON message_batches BEGIN SELECT RAISE(ABORT,'delivery proposals are retained'); END;
CREATE TRIGGER message_batch_identity BEFORE UPDATE ON message_batches WHEN
 NEW.id<>OLD.id OR NEW.source_kind<>OLD.source_kind OR NEW.source_id<>OLD.source_id OR NEW.channel<>OLD.channel OR NEW.purpose<>OLD.purpose OR NEW.proposed_by<>OLD.proposed_by OR NEW.proposed_at<>OLD.proposed_at OR NEW.portal_origin<>OLD.portal_origin OR NEW.target_json<>OLD.target_json OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'delivery identity and version are preserved'); END;
CREATE TRIGGER message_batch_frozen BEFORE UPDATE ON message_batches WHEN
 (NEW.source_json<>OLD.source_json OR NEW.counts_json<>OLD.counts_json OR NEW.preview_hash<>OLD.preview_hash OR NEW.envelope<>OLD.envelope OR NEW.snapshot_version<>OLD.snapshot_version)
 AND (OLD.state<>'PENDING' OR NEW.state<>'PENDING' OR NEW.snapshot_version<>OLD.snapshot_version+1)
 BEGIN SELECT RAISE(ABORT,'only a new pending snapshot can refresh recipients'); END;

