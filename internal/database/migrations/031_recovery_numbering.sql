-- A numbering preparation never releases quarantine or changes old counters.
CREATE TABLE recovery_numbering_policies (
 sequence INTEGER PRIMARY KEY CHECK(sequence>0),
 id TEXT NOT NULL UNIQUE,
 operation_key TEXT NOT NULL UNIQUE,
 epoch_id TEXT NOT NULL REFERENCES recovery_epochs(id),
 nomination_id TEXT NOT NULL REFERENCES recovery_nominations(operation_id),
 author_id TEXT NOT NULL REFERENCES users(id),
 author_identity TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version>0),
 predecessor TEXT UNIQUE REFERENCES recovery_numbering_policies(id),
 input_digest TEXT NOT NULL CHECK(length(input_digest)=64),
 input_json TEXT NOT NULL CHECK(json_valid(input_json)),
 series_prefix TEXT NOT NULL CHECK(length(series_prefix) BETWEEN 3 AND 40),
 created_at INTEGER NOT NULL
) STRICT;
CREATE TABLE recovery_numbering_decisions (
 policy_id TEXT PRIMARY KEY REFERENCES recovery_numbering_policies(id),
 operation_key TEXT NOT NULL UNIQUE,
 actor_id TEXT NOT NULL REFERENCES users(id),
 actor_identity TEXT NOT NULL,
 policy_version INTEGER NOT NULL,
 decision TEXT NOT NULL CHECK(decision IN('APPROVED','DECLINED')),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 1000),
 attestation TEXT NOT NULL CHECK(length(attestation) BETWEEN 10 AND 300),
 input_digest TEXT NOT NULL CHECK(length(input_digest)=64),
 created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER verify_recovery_numbering_policy BEFORE INSERT ON recovery_numbering_policies
WHEN NEW.sequence!=(SELECT COALESCE(MAX(sequence),0)+1 FROM recovery_numbering_policies)
 OR NEW.series_prefix IS NOT json_extract(NEW.input_json,'$.policy.prefix')
 OR EXISTS(SELECT 1 FROM recovery_numbering_decisions WHERE operation_key=NEW.operation_key)
 OR NOT EXISTS(SELECT 1 FROM recovery_nominations n JOIN recovery_reviewers rr ON rr.nomination_id=n.operation_id JOIN users u ON u.id=rr.user_id
 WHERE n.operation_id=NEW.nomination_id AND n.epoch_id=NEW.epoch_id
 AND rr.user_id=NEW.author_id AND rr.identity_ref=NEW.author_identity
 AND n.epoch_id=(SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)
 AND n.sequence=(SELECT MAX(sequence) FROM recovery_nominations WHERE epoch_id=n.epoch_id)
 AND n.valid_from<=unixepoch('now') AND n.valid_until>unixepoch('now')
 AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id)
 AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'current nominated numbering author required'); END;
CREATE TRIGGER verify_recovery_numbering_decision BEFORE INSERT ON recovery_numbering_decisions
WHEN EXISTS(SELECT 1 FROM recovery_numbering_policies WHERE operation_key=NEW.operation_key)
 OR NOT EXISTS(SELECT 1 FROM recovery_numbering_policies p WHERE p.id=NEW.policy_id
 AND p.version=NEW.policy_version AND p.author_id!=NEW.actor_id AND p.author_identity!=NEW.actor_identity
 AND p.epoch_id=(SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)
 AND p.nomination_id=(SELECT operation_id FROM recovery_nominations WHERE epoch_id=p.epoch_id ORDER BY sequence DESC LIMIT 1)
 AND NOT EXISTS(SELECT 1 FROM recovery_numbering_policies WHERE predecessor=p.id)
 AND EXISTS(SELECT 1 FROM recovery_reviewers rr JOIN recovery_nominations n ON n.operation_id=rr.nomination_id JOIN users u ON u.id=rr.user_id
 WHERE rr.nomination_id=p.nomination_id AND rr.user_id=NEW.actor_id AND rr.identity_ref=NEW.actor_identity
 AND n.valid_from<=unixepoch('now') AND n.valid_until>unixepoch('now')
 AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id)
 AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL))
BEGIN SELECT RAISE(ABORT,'a separate current reviewer must decide the frozen numbering policy'); END;

-- The exact encrypted intent is retained before any external handoff. Its key
-- collides on stable society/prefix, independently of restored local counters.
CREATE TABLE recovery_series_intents (
 operation_id TEXT PRIMARY KEY,
 policy_id TEXT NOT NULL UNIQUE REFERENCES recovery_numbering_policies(id),
 policy_version INTEGER NOT NULL,
 epoch_id TEXT NOT NULL REFERENCES recovery_epochs(id),
 nomination_id TEXT NOT NULL REFERENCES recovery_nominations(operation_id),
 source_sha256 TEXT NOT NULL CHECK(length(source_sha256)=64),
 series_prefix TEXT NOT NULL UNIQUE,
 society_key TEXT NOT NULL,
 config_hash TEXT NOT NULL CHECK(length(config_hash)=64),
 object_key TEXT NOT NULL,
 manifest_json TEXT NOT NULL CHECK(json_valid(manifest_json)),
 input_digest TEXT NOT NULL CHECK(length(input_digest)=64),
 cipher_sha256 TEXT NOT NULL CHECK(length(cipher_sha256)=64),
 cipher_bytes INTEGER NOT NULL CHECK(cipher_bytes BETWEEN 32 AND 65536),
 ciphertext BLOB NOT NULL CHECK(length(ciphertext)=cipher_bytes),
 created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER verify_recovery_series_intent BEFORE INSERT ON recovery_series_intents
WHEN NEW.epoch_id IS NOT (SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)
 OR NEW.source_sha256 IS NOT (SELECT source_sha256 FROM recovery_epochs WHERE id=NEW.epoch_id)
 OR NOT EXISTS(SELECT 1 FROM recovery_numbering_policies p JOIN recovery_numbering_decisions d ON d.policy_id=p.id JOIN recovery_nominations n ON n.operation_id=p.nomination_id
 WHERE p.id=NEW.policy_id AND p.epoch_id=NEW.epoch_id AND p.nomination_id=NEW.nomination_id
 AND d.decision='APPROVED' AND d.policy_version+1=NEW.policy_version AND p.series_prefix=NEW.series_prefix
 AND NOT EXISTS(SELECT 1 FROM recovery_numbering_policies WHERE predecessor=p.id)
 AND n.operation_id=(SELECT operation_id FROM recovery_nominations WHERE epoch_id=p.epoch_id ORDER BY sequence DESC LIMIT 1)
 AND n.valid_from<=unixepoch('now') AND n.valid_until>unixepoch('now')
 AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id))
BEGIN SELECT RAISE(ABORT,'current separately approved numbering policy required'); END;

CREATE TABLE recovery_series_events (
 sequence INTEGER PRIMARY KEY CHECK(sequence>0),
 intent_id TEXT NOT NULL REFERENCES recovery_series_intents(operation_id),
 state TEXT NOT NULL CHECK(state IN('HANDOFF','UNKNOWN','REJECTED','VERIFIED')),
 attempt INTEGER NOT NULL CHECK(attempt BETWEEN 1 AND 3),
 attempt_key TEXT NOT NULL CHECK(length(attempt_key) BETWEEN 1 AND 80),
 object_version TEXT NOT NULL CHECK(length(object_version)<=1024),
 reason_code TEXT NOT NULL CHECK(length(reason_code) BETWEEN 1 AND 40),
 created_at INTEGER NOT NULL,
 UNIQUE(intent_id,attempt,state),
 CHECK((state='VERIFIED' AND length(object_version)>0 AND object_version!='null') OR (state!='VERIFIED' AND object_version=''))
) STRICT;
CREATE TRIGGER verify_recovery_series_event BEFORE INSERT ON recovery_series_events
WHEN NEW.sequence!=(SELECT COALESCE(MAX(sequence),0)+1 FROM recovery_series_events)
 OR (NEW.state='HANDOFF' AND (
  EXISTS(SELECT 1 FROM recovery_series_events WHERE state='HANDOFF' AND attempt_key=NEW.attempt_key)
  OR COALESCE((SELECT state FROM recovery_series_events WHERE intent_id=NEW.intent_id ORDER BY sequence DESC LIMIT 1),'PREPARED') NOT IN('PREPARED','REJECTED')
  OR NEW.attempt!=COALESCE((SELECT attempt FROM recovery_series_events WHERE intent_id=NEW.intent_id ORDER BY sequence DESC LIMIT 1),0)+1
  OR NOT EXISTS(SELECT 1 FROM recovery_series_intents i JOIN recovery_numbering_policies p ON p.id=i.policy_id JOIN recovery_nominations n ON n.operation_id=i.nomination_id
   WHERE i.operation_id=NEW.intent_id AND i.epoch_id=(SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)
   AND i.nomination_id=(SELECT operation_id FROM recovery_nominations WHERE epoch_id=i.epoch_id ORDER BY sequence DESC LIMIT 1)
   AND n.valid_from<=unixepoch('now') AND n.valid_until>unixepoch('now')
   AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id)
   AND NOT EXISTS(SELECT 1 FROM recovery_numbering_policies WHERE predecessor=p.id))))
 OR (NEW.state!='HANDOFF' AND NOT EXISTS(
  SELECT 1 FROM recovery_series_events e WHERE e.sequence=(SELECT MAX(sequence) FROM recovery_series_events WHERE intent_id=NEW.intent_id)
  AND e.attempt=NEW.attempt AND e.attempt_key=NEW.attempt_key
  AND (e.state='HANDOFF' OR (e.state='UNKNOWN' AND NEW.state='VERIFIED'))))
BEGIN SELECT RAISE(ABORT,'exact retained handoff transition required'); END;

CREATE TRIGGER preserve_numbering_policy_update BEFORE UPDATE ON recovery_numbering_policies BEGIN SELECT RAISE(ABORT,'numbering policies are immutable'); END;
CREATE TRIGGER preserve_numbering_policy_delete BEFORE DELETE ON recovery_numbering_policies BEGIN SELECT RAISE(ABORT,'numbering policies are retained'); END;
CREATE TRIGGER preserve_numbering_decision_update BEFORE UPDATE ON recovery_numbering_decisions BEGIN SELECT RAISE(ABORT,'numbering decisions are immutable'); END;
CREATE TRIGGER preserve_numbering_decision_delete BEFORE DELETE ON recovery_numbering_decisions BEGIN SELECT RAISE(ABORT,'numbering decisions are retained'); END;
CREATE TRIGGER preserve_series_intent_update BEFORE UPDATE ON recovery_series_intents BEGIN SELECT RAISE(ABORT,'original series ciphertext and intent are immutable'); END;
CREATE TRIGGER preserve_series_intent_delete BEFORE DELETE ON recovery_series_intents BEGIN SELECT RAISE(ABORT,'series intents are retained'); END;
CREATE TRIGGER preserve_series_event_update BEFORE UPDATE ON recovery_series_events BEGIN SELECT RAISE(ABORT,'series events are immutable'); END;
CREATE TRIGGER preserve_series_event_delete BEFORE DELETE ON recovery_series_events BEGIN SELECT RAISE(ABORT,'series events are retained'); END;
