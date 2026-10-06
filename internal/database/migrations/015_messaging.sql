CREATE TABLE message_batches (
 id TEXT PRIMARY KEY,
 source_kind TEXT NOT NULL CHECK(source_kind IN('NOTICE','RECEIPT')),
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
 CHECK((source_kind='NOTICE' AND purpose='COMMUNITY') OR (source_kind='RECEIPT' AND purpose='FINANCE')),
 CHECK(state<>'APPROVED' OR (reviewed_by IS NOT NULL AND reviewed_by<>proposed_by AND reviewed_at>0))
) STRICT;
CREATE INDEX message_batch_queue ON message_batches(state,updated_at,id);
CREATE TRIGGER message_batch_delete BEFORE DELETE ON message_batches BEGIN SELECT RAISE(ABORT,'delivery proposals are retained'); END;
CREATE TRIGGER message_batch_identity BEFORE UPDATE ON message_batches WHEN
 NEW.id<>OLD.id OR NEW.source_kind<>OLD.source_kind OR NEW.source_id<>OLD.source_id OR NEW.channel<>OLD.channel OR NEW.purpose<>OLD.purpose OR NEW.proposed_by<>OLD.proposed_by OR NEW.proposed_at<>OLD.proposed_at OR NEW.portal_origin<>OLD.portal_origin OR NEW.target_json<>OLD.target_json OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'delivery identity and version are preserved'); END;
CREATE TRIGGER message_batch_frozen BEFORE UPDATE ON message_batches WHEN
 (NEW.source_json<>OLD.source_json OR NEW.counts_json<>OLD.counts_json OR NEW.preview_hash<>OLD.preview_hash OR NEW.envelope<>OLD.envelope OR NEW.snapshot_version<>OLD.snapshot_version)
 AND (OLD.state<>'PENDING' OR NEW.state<>'PENDING' OR NEW.snapshot_version<>OLD.snapshot_version+1)
 BEGIN SELECT RAISE(ABORT,'only a new pending snapshot can refresh recipients'); END;

CREATE TABLE message_deliveries (
 id TEXT PRIMARY KEY,
 batch_id TEXT NOT NULL REFERENCES message_batches(id),
 snapshot_version INTEGER NOT NULL,
 destination TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN('QUEUED','CLAIMED','ACCEPTED','DELIVERED','READ','FAILED','SKIPPED','OPTED_OUT','CANCELLED','UNKNOWN')),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3),
 provider_id TEXT NOT NULL DEFAULT '',
 reason TEXT NOT NULL DEFAULT '',
 accepted_at INTEGER NOT NULL DEFAULT 0,
 delivered_at INTEGER NOT NULL DEFAULT 0,
 read_at INTEGER NOT NULL DEFAULT 0,
 updated_at INTEGER NOT NULL,
 UNIQUE(batch_id,snapshot_version,destination)
) STRICT;
CREATE INDEX message_delivery_queue ON message_deliveries(batch_id,snapshot_version,state,id);
CREATE UNIQUE INDEX message_provider_identity ON message_deliveries(provider_id) WHERE provider_id<>'';
CREATE TRIGGER message_delivery_delete BEFORE DELETE ON message_deliveries BEGIN SELECT RAISE(ABORT,'delivery outcomes are retained'); END;
CREATE TRIGGER message_delivery_identity BEFORE UPDATE ON message_deliveries WHEN NEW.id<>OLD.id OR NEW.batch_id<>OLD.batch_id OR NEW.snapshot_version<>OLD.snapshot_version OR NEW.destination<>OLD.destination OR NEW.attempts<OLD.attempts OR NEW.attempts>OLD.attempts+1 BEGIN SELECT RAISE(ABORT,'frozen destination and attempt identity are preserved'); END;
CREATE TRIGGER message_delivery_no_regression BEFORE UPDATE ON message_deliveries WHEN
 (OLD.state='READ' AND NEW.state<>'READ') OR (OLD.state='DELIVERED' AND NEW.state NOT IN('DELIVERED','READ')) OR
 (OLD.state IN('SKIPPED','OPTED_OUT','CANCELLED') AND NEW.state<>OLD.state)
 BEGIN SELECT RAISE(ABORT,'known delivered and suppressed outcomes do not regress'); END;

CREATE TABLE message_recipients (
 batch_id TEXT NOT NULL REFERENCES message_batches(id),
 snapshot_version INTEGER NOT NULL,
 resident_id TEXT NOT NULL REFERENCES residents(id),
 name TEXT NOT NULL,
 contact_version INTEGER NOT NULL,
 frozen_reason TEXT NOT NULL,
 delivery_id TEXT REFERENCES message_deliveries(id),
 disposition TEXT NOT NULL CHECK(disposition IN('ELIGIBLE','OMITTED','SKIPPED','OPTED_OUT','HANDED_OFF','CANCELLED')),
 reason TEXT NOT NULL,
 PRIMARY KEY(batch_id,snapshot_version,resident_id),
 CHECK((frozen_reason='' AND delivery_id IS NOT NULL) OR (frozen_reason<>'' AND delivery_id IS NULL))
) STRICT;
CREATE INDEX message_person_history ON message_recipients(resident_id,batch_id,snapshot_version);
CREATE TRIGGER message_recipient_delete BEFORE DELETE ON message_recipients BEGIN SELECT RAISE(ABORT,'recipient decisions are retained'); END;
CREATE TRIGGER message_recipient_frozen BEFORE UPDATE ON message_recipients WHEN
 NEW.batch_id<>OLD.batch_id OR NEW.snapshot_version<>OLD.snapshot_version OR NEW.resident_id<>OLD.resident_id OR NEW.name<>OLD.name OR NEW.contact_version<>OLD.contact_version OR NEW.frozen_reason<>OLD.frozen_reason OR NEW.delivery_id IS NOT OLD.delivery_id OR
 (OLD.disposition<>'ELIGIBLE' AND (NEW.disposition<>OLD.disposition OR NEW.reason<>OLD.reason) AND NOT (
 OLD.disposition='HANDED_OFF' AND NEW.disposition='ELIGIBLE' AND NEW.reason='' AND EXISTS(SELECT 1 FROM message_deliveries d WHERE d.id=OLD.delivery_id AND d.state='FAILED')))
 BEGIN SELECT RAISE(ABORT,'frozen and handed-off recipient decisions are preserved'); END;

CREATE TABLE message_events (
 id INTEGER PRIMARY KEY,
 batch_id TEXT NOT NULL REFERENCES message_batches(id),
 version INTEGER NOT NULL,
 actor_id TEXT NOT NULL REFERENCES users(id),
 action TEXT NOT NULL,
 reason TEXT NOT NULL,
 occurred_at INTEGER NOT NULL,
 snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
 UNIQUE(batch_id,version)
) STRICT;
CREATE TRIGGER message_event_update BEFORE UPDATE ON message_events BEGIN SELECT RAISE(ABORT,'message decisions are immutable'); END;
CREATE TRIGGER message_event_delete BEFORE DELETE ON message_events BEGIN SELECT RAISE(ABORT,'message decisions are retained'); END;

CREATE TABLE message_attempts (
 id TEXT PRIMARY KEY,
 delivery_id TEXT NOT NULL REFERENCES message_deliveries(id),
 attempt_number INTEGER NOT NULL CHECK(attempt_number BETWEEN 1 AND 3),
 actor_id TEXT NOT NULL REFERENCES users(id),
 operation_key TEXT NOT NULL,
 started_at INTEGER NOT NULL,
 UNIQUE(delivery_id,attempt_number)
) STRICT;
CREATE TRIGGER message_attempt_update BEFORE UPDATE ON message_attempts BEGIN SELECT RAISE(ABORT,'message attempt identities are immutable'); END;
CREATE TRIGGER message_attempt_delete BEFORE DELETE ON message_attempts BEGIN SELECT RAISE(ABORT,'message attempts are retained'); END;

CREATE TABLE message_attempt_recipients (
 attempt_id TEXT NOT NULL REFERENCES message_attempts(id),
 resident_id TEXT NOT NULL REFERENCES residents(id),
 state TEXT NOT NULL CHECK(state IN('CLAIMED','HANDED_OFF','REJECTED','SKIPPED','OPTED_OUT')),
 reason TEXT NOT NULL,
 PRIMARY KEY(attempt_id,resident_id)
) STRICT;
CREATE TRIGGER message_attempt_recipient_delete BEFORE DELETE ON message_attempt_recipients BEGIN SELECT RAISE(ABORT,'attempt recipient outcomes are retained'); END;
CREATE TRIGGER message_attempt_recipient_update BEFORE UPDATE ON message_attempt_recipients WHEN NEW.attempt_id<>OLD.attempt_id OR NEW.resident_id<>OLD.resident_id OR OLD.state<>'CLAIMED' OR NEW.state='CLAIMED' BEGIN SELECT RAISE(ABORT,'completed attempt recipient outcomes are immutable'); END;

CREATE TABLE message_delivery_events (
 id INTEGER PRIMARY KEY,
 delivery_id TEXT NOT NULL REFERENCES message_deliveries(id),
 attempt_id TEXT REFERENCES message_attempts(id),
 state TEXT NOT NULL,
 reason TEXT NOT NULL,
 occurred_at INTEGER NOT NULL,
 provider_id TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE TRIGGER message_delivery_event_update BEFORE UPDATE ON message_delivery_events BEGIN SELECT RAISE(ABORT,'delivery events are immutable'); END;
CREATE TRIGGER message_delivery_event_delete BEFORE DELETE ON message_delivery_events BEGIN SELECT RAISE(ABORT,'delivery events are retained'); END;

CREATE TABLE simulation_messages (
 provider_id TEXT PRIMARY KEY,
 attempt_id TEXT NOT NULL UNIQUE REFERENCES message_attempts(id),
 outcome TEXT NOT NULL CHECK(outcome IN('ACCEPTED','DELIVERED','READ','REJECTED','UNKNOWN')),
 accepted_at INTEGER NOT NULL,
 simulation INTEGER NOT NULL CHECK(simulation=1)
) STRICT;
CREATE TRIGGER simulation_message_update BEFORE UPDATE ON simulation_messages BEGIN SELECT RAISE(ABORT,'synthetic handoffs are immutable'); END;
CREATE TRIGGER simulation_message_delete BEFORE DELETE ON simulation_messages BEGIN SELECT RAISE(ABORT,'synthetic handoffs are retained'); END;

CREATE TABLE message_callbacks (
 event_id TEXT PRIMARY KEY,
 provider_id TEXT NOT NULL REFERENCES simulation_messages(provider_id),
 delivery_id TEXT NOT NULL REFERENCES message_deliveries(id),
 state TEXT NOT NULL CHECK(state IN('ACCEPTED','DELIVERED','READ','FAILED')),
 occurred_at INTEGER NOT NULL,
 received_at INTEGER NOT NULL,
 payload_hash TEXT NOT NULL
) STRICT;
CREATE TRIGGER message_callback_update BEFORE UPDATE ON message_callbacks BEGIN SELECT RAISE(ABORT,'callback events are immutable'); END;
CREATE TRIGGER message_callback_delete BEFORE DELETE ON message_callbacks BEGIN SELECT RAISE(ABORT,'callback events are retained'); END;
