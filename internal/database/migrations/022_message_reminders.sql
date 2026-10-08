-- Retained child triggers are restored verbatim inside this same transaction.
DROP TRIGGER message_provider_binding_insert;
DROP TRIGGER whatsapp_handoff_insert;
-- Rebuild only the constrained parent. Migrate changes foreign-key mode
-- before its dedicated transaction, checks retained references, and restores it.
CREATE TABLE new_message_batches (
 id TEXT PRIMARY KEY,
 source_kind TEXT NOT NULL CHECK(source_kind IN('NOTICE','RECEIPT','STATEMENT','MAINTENANCE_REMINDER','FUND_REMINDER','MEETING_REMINDER')),
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
 CHECK((source_kind IN('NOTICE','MEETING_REMINDER') AND purpose='COMMUNITY') OR (source_kind IN('RECEIPT','STATEMENT','MAINTENANCE_REMINDER','FUND_REMINDER') AND purpose='FINANCE')),
 CHECK(state<>'APPROVED' OR (reviewed_by IS NOT NULL AND reviewed_by<>proposed_by AND reviewed_at>0))
) STRICT;
INSERT INTO new_message_batches SELECT * FROM message_batches ORDER BY rowid;
DROP TABLE message_batches;
ALTER TABLE new_message_batches RENAME TO message_batches;
CREATE TRIGGER message_provider_binding_insert BEFORE INSERT ON message_provider_bindings WHEN NOT EXISTS(
 SELECT 1 FROM message_batches b WHERE b.id=NEW.batch_id AND b.channel='WHATSAPP' AND b.state='PENDING'
 AND ((NEW.snapshot_version=b.snapshot_version AND b.version=1) OR NEW.snapshot_version=b.snapshot_version+1))
 BEGIN SELECT RAISE(ABORT,'provider snapshots belong to a new pending proposal or refresh'); END;
CREATE TRIGGER whatsapp_handoff_insert BEFORE INSERT ON whatsapp_handoffs WHEN NOT EXISTS(
 SELECT 1 FROM message_attempts a JOIN message_deliveries d ON d.id=a.delivery_id
 JOIN message_batches b ON b.id=d.batch_id JOIN message_provider_bindings p ON p.batch_id=b.id AND p.snapshot_version=b.snapshot_version
 WHERE a.id=NEW.attempt_id AND a.attempt_number=d.attempts AND d.snapshot_version=b.snapshot_version
 AND d.state='CLAIMED' AND b.state='APPROVED' AND p.provider_json=NEW.provider_json)
 BEGIN SELECT RAISE(ABORT,'provider handoffs require the current approved frozen claim'); END;
CREATE INDEX message_batch_queue ON message_batches(state,updated_at,id);
CREATE TRIGGER message_batch_delete BEFORE DELETE ON message_batches BEGIN SELECT RAISE(ABORT,'delivery proposals are retained'); END;
CREATE TRIGGER message_batch_identity BEFORE UPDATE ON message_batches WHEN
 NEW.id<>OLD.id OR NEW.source_kind<>OLD.source_kind OR NEW.source_id<>OLD.source_id OR NEW.channel<>OLD.channel OR NEW.purpose<>OLD.purpose OR NEW.proposed_by<>OLD.proposed_by OR NEW.proposed_at<>OLD.proposed_at OR NEW.portal_origin<>OLD.portal_origin OR NEW.target_json<>OLD.target_json OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'delivery identity and version are preserved'); END;
CREATE TRIGGER message_batch_frozen BEFORE UPDATE ON message_batches WHEN
 (NEW.source_json<>OLD.source_json OR NEW.counts_json<>OLD.counts_json OR NEW.preview_hash<>OLD.preview_hash OR NEW.envelope<>OLD.envelope OR NEW.snapshot_version<>OLD.snapshot_version)
 AND (OLD.state<>'PENDING' OR NEW.state<>'PENDING' OR NEW.snapshot_version<>OLD.snapshot_version+1)
 BEGIN SELECT RAISE(ABORT,'only a new pending snapshot can refresh recipients'); END;

CREATE TABLE message_reminder_recipients (
 batch_id TEXT NOT NULL,
 snapshot_version INTEGER NOT NULL CHECK(snapshot_version>0),
 resident_id TEXT NOT NULL,
 source_kind TEXT NOT NULL CHECK(source_kind IN('MAINTENANCE_REMINDER','FUND_REMINDER','MEETING_REMINDER')),
 source_id TEXT NOT NULL,
 basis TEXT NOT NULL CHECK(basis IN('OUTSTANDING','DEADLINE_PASSED')),
 fingerprint TEXT NOT NULL CHECK(length(fingerprint)=64),
 outstanding_paise INTEGER NOT NULL CHECK(outstanding_paise>=0),
 binding_json TEXT NOT NULL CHECK(json_valid(binding_json)),
 PRIMARY KEY(batch_id,snapshot_version,resident_id),
 FOREIGN KEY(batch_id,snapshot_version,resident_id) REFERENCES message_recipients(batch_id,snapshot_version,resident_id),
 CHECK((source_kind='MEETING_REMINDER' AND outstanding_paise=0) OR (source_kind<>'MEETING_REMINDER' AND outstanding_paise>0)),
 CHECK(json_extract(binding_json,'$.fingerprint')=fingerprint AND json_extract(binding_json,'$.outstanding_paise')=outstanding_paise)
) STRICT;
CREATE TRIGGER message_reminder_recipient_update BEFORE UPDATE ON message_reminder_recipients BEGIN SELECT RAISE(ABORT,'reviewed reminder bindings are immutable'); END;
CREATE TRIGGER message_reminder_recipient_delete BEFORE DELETE ON message_reminder_recipients BEGIN SELECT RAISE(ABORT,'reviewed reminder bindings are retained'); END;
CREATE TRIGGER message_reminder_recipient_source BEFORE INSERT ON message_reminder_recipients WHEN NOT EXISTS(
 SELECT 1 FROM message_batches b JOIN message_recipients r ON r.batch_id=b.id AND r.snapshot_version=NEW.snapshot_version AND r.resident_id=NEW.resident_id
 WHERE b.id=NEW.batch_id AND b.source_kind=NEW.source_kind AND b.source_id=NEW.source_id AND r.frozen_reason='' AND r.disposition='ELIGIBLE'
) BEGIN SELECT RAISE(ABORT,'reminder binding requires its exact eligible frozen recipient'); END;
