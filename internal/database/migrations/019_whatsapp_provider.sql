CREATE TABLE message_provider_bindings (
 batch_id TEXT NOT NULL REFERENCES message_batches(id),
 snapshot_version INTEGER NOT NULL CHECK(snapshot_version>0),
 provider_json TEXT NOT NULL CHECK(json_valid(provider_json) AND length(provider_json)<=8192),
 PRIMARY KEY(batch_id,snapshot_version)
) STRICT;
CREATE TRIGGER message_provider_binding_insert BEFORE INSERT ON message_provider_bindings WHEN NOT EXISTS(
 SELECT 1 FROM message_batches b WHERE b.id=NEW.batch_id AND b.channel='WHATSAPP' AND b.state='PENDING'
 AND ((NEW.snapshot_version=b.snapshot_version AND b.version=1) OR NEW.snapshot_version=b.snapshot_version+1))
 BEGIN SELECT RAISE(ABORT,'provider snapshots belong to a new pending proposal or refresh'); END;
CREATE TRIGGER message_provider_binding_update BEFORE UPDATE ON message_provider_bindings BEGIN SELECT RAISE(ABORT,'provider content requires a new pending snapshot'); END;
CREATE TRIGGER message_provider_binding_delete BEFORE DELETE ON message_provider_bindings BEGIN SELECT RAISE(ABORT,'provider snapshots are retained'); END;

CREATE TABLE whatsapp_handoffs (
 attempt_id TEXT PRIMARY KEY REFERENCES message_attempts(id),
 provider_json TEXT NOT NULL CHECK(json_valid(provider_json)),
 started_at INTEGER NOT NULL CHECK(started_at>0),
 provider_id TEXT NOT NULL DEFAULT '',
 result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
 completed_at INTEGER NOT NULL DEFAULT 0,
 retry_at INTEGER NOT NULL DEFAULT 0,
 CHECK((result_json IS NULL AND completed_at=0) OR (result_json IS NOT NULL AND completed_at>0))
) STRICT;
CREATE TRIGGER whatsapp_handoff_insert BEFORE INSERT ON whatsapp_handoffs WHEN NOT EXISTS(
 SELECT 1 FROM message_attempts a JOIN message_deliveries d ON d.id=a.delivery_id
 JOIN message_batches b ON b.id=d.batch_id JOIN message_provider_bindings p ON p.batch_id=b.id AND p.snapshot_version=b.snapshot_version
 WHERE a.id=NEW.attempt_id AND a.attempt_number=d.attempts AND d.snapshot_version=b.snapshot_version
 AND d.state='CLAIMED' AND b.state='APPROVED' AND p.provider_json=NEW.provider_json)
 BEGIN SELECT RAISE(ABORT,'provider handoffs require the current approved frozen claim'); END;
CREATE UNIQUE INDEX whatsapp_provider_message ON whatsapp_handoffs(provider_id) WHERE provider_id<>'';
CREATE TRIGGER whatsapp_handoff_frozen BEFORE UPDATE ON whatsapp_handoffs WHEN
 NEW.attempt_id<>OLD.attempt_id OR NEW.provider_json<>OLD.provider_json OR NEW.started_at<>OLD.started_at OR
 (OLD.provider_id<>'' AND NEW.provider_id<>OLD.provider_id) OR
 (OLD.result_json IS NOT NULL AND (NEW.result_json IS NOT OLD.result_json OR NEW.completed_at<>OLD.completed_at OR NEW.retry_at<>OLD.retry_at))
 BEGIN SELECT RAISE(ABORT,'provider attempts and recorded responses are preserved'); END;
CREATE TRIGGER whatsapp_handoff_delete BEFORE DELETE ON whatsapp_handoffs BEGIN SELECT RAISE(ABORT,'provider attempts are retained'); END;

CREATE TABLE whatsapp_status_events (
 event_hash TEXT PRIMARY KEY CHECK(length(event_hash)=64),
 attempt_id TEXT NOT NULL REFERENCES whatsapp_handoffs(attempt_id),
 provider_id TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN('ACCEPTED','DELIVERED','READ','FAILED')),
 occurred_at INTEGER NOT NULL CHECK(occurred_at>0),
 received_at INTEGER NOT NULL CHECK(received_at>0),
 status_json TEXT NOT NULL CHECK(json_valid(status_json) AND length(status_json)<=16384)
) STRICT;
CREATE TRIGGER whatsapp_status_update BEFORE UPDATE ON whatsapp_status_events BEGIN SELECT RAISE(ABORT,'provider evidence is immutable'); END;
CREATE TRIGGER whatsapp_status_delete BEFORE DELETE ON whatsapp_status_events BEGIN SELECT RAISE(ABORT,'provider evidence is retained'); END;
