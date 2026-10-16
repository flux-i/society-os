-- A supplied financial comparison is a retained assessment, not a posting.
CREATE TABLE recovery_gap_preparations (
 sequence INTEGER PRIMARY KEY CHECK(sequence>0),
 id TEXT NOT NULL UNIQUE,
 operation_key TEXT NOT NULL UNIQUE,
 epoch_id TEXT NOT NULL REFERENCES recovery_epochs(id),
 nomination_id TEXT NOT NULL REFERENCES recovery_nominations(operation_id),
 author_id TEXT NOT NULL REFERENCES users(id),
 author_identity TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version>0),
 predecessor TEXT UNIQUE REFERENCES recovery_gap_preparations(id),
 input_digest TEXT NOT NULL CHECK(length(input_digest)=64),
 input_json TEXT NOT NULL CHECK(json_valid(input_json) AND length(input_json)<=65536),
 comparison_json TEXT NOT NULL CHECK(json_valid(comparison_json) AND length(comparison_json)<=65536),
 known_count INTEGER NOT NULL CHECK(known_count BETWEEN 0 AND 24),
 unresolved_count INTEGER NOT NULL CHECK(unresolved_count BETWEEN 0 AND 25),
 created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX recovery_gap_scope ON recovery_gap_preparations(epoch_id,nomination_id,sequence);
CREATE TABLE recovery_gap_decisions (
 preparation_id TEXT PRIMARY KEY REFERENCES recovery_gap_preparations(id),
 operation_key TEXT NOT NULL UNIQUE,
 actor_id TEXT NOT NULL REFERENCES users(id),
 actor_identity TEXT NOT NULL,
 preparation_version INTEGER NOT NULL,
 decision TEXT NOT NULL CHECK(decision IN('APPROVED','DECLINED')),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 1000),
 attestation TEXT NOT NULL CHECK(length(attestation) BETWEEN 10 AND 300),
 input_digest TEXT NOT NULL CHECK(length(input_digest)=64),
 created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER verify_recovery_gap_preparation BEFORE INSERT ON recovery_gap_preparations
WHEN NEW.sequence!=(SELECT COALESCE(MAX(sequence),0)+1 FROM recovery_gap_preparations)
 OR NEW.operation_key IS NOT json_extract(NEW.input_json,'$.operation_key')
 OR json_type(NEW.input_json,'$.items') IS NOT 'array'
 OR json_array_length(NEW.input_json,'$.items') NOT BETWEEN 0 AND 24
 OR json_type(NEW.comparison_json,'$.homes') IS NOT 'array'
 OR json_array_length(NEW.comparison_json,'$.homes') NOT BETWEEN 0 AND 24
 OR json_type(NEW.comparison_json,'$.prepared_only') IS NOT 'true'
 OR NEW.known_count IS NOT json_extract(NEW.comparison_json,'$.known_items')
 OR NEW.unresolved_count IS NOT json_extract(NEW.comparison_json,'$.unresolved_items')
 OR EXISTS(SELECT 1 FROM recovery_gap_decisions WHERE operation_key=NEW.operation_key)
 OR NOT EXISTS(SELECT 1 FROM recovery_nominations n JOIN recovery_reviewers rr ON rr.nomination_id=n.operation_id JOIN users u ON u.id=rr.user_id
 WHERE n.operation_id=NEW.nomination_id AND n.epoch_id=NEW.epoch_id
 AND rr.user_id=NEW.author_id AND rr.identity_ref=NEW.author_identity
 AND n.epoch_id=(SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)
 AND n.sequence=(SELECT MAX(sequence) FROM recovery_nominations WHERE epoch_id=n.epoch_id)
 AND n.valid_from<=unixepoch('now') AND n.valid_until>unixepoch('now')
 AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id)
 AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL)
 OR (NEW.predecessor IS NULL AND NEW.version!=1)
 OR (NEW.predecessor IS NOT NULL AND NOT EXISTS(
 SELECT 1 FROM recovery_gap_preparations p LEFT JOIN recovery_gap_decisions d ON d.preparation_id=p.id
 WHERE p.id=NEW.predecessor AND p.epoch_id=NEW.epoch_id
 AND NEW.version=p.version+CASE WHEN d.preparation_id IS NULL THEN 0 ELSE 1 END+1
 AND NOT EXISTS(SELECT 1 FROM recovery_gap_preparations WHERE predecessor=p.id)))
BEGIN SELECT RAISE(ABORT,'current nominated financial-gap preparation required'); END;
CREATE TRIGGER verify_recovery_gap_decision BEFORE INSERT ON recovery_gap_decisions
WHEN EXISTS(SELECT 1 FROM recovery_gap_preparations WHERE operation_key=NEW.operation_key)
 OR NOT EXISTS(SELECT 1 FROM recovery_gap_preparations p WHERE p.id=NEW.preparation_id
 AND p.version=NEW.preparation_version AND p.author_id!=NEW.actor_id AND p.author_identity!=NEW.actor_identity
 AND p.epoch_id=(SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)
 AND p.nomination_id=(SELECT operation_id FROM recovery_nominations WHERE epoch_id=p.epoch_id ORDER BY sequence DESC LIMIT 1)
 AND NOT EXISTS(SELECT 1 FROM recovery_gap_preparations WHERE predecessor=p.id)
 AND EXISTS(SELECT 1 FROM recovery_reviewers rr JOIN recovery_nominations n ON n.operation_id=rr.nomination_id JOIN users u ON u.id=rr.user_id
 WHERE rr.nomination_id=p.nomination_id AND rr.user_id=NEW.actor_id AND rr.identity_ref=NEW.actor_identity
 AND n.valid_from<=unixepoch('now') AND n.valid_until>unixepoch('now')
 AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id)
 AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL))
BEGIN SELECT RAISE(ABORT,'a separate current reviewer must assess the frozen financial gap'); END;
CREATE TRIGGER preserve_recovery_gap_preparation_update BEFORE UPDATE ON recovery_gap_preparations BEGIN SELECT RAISE(ABORT,'financial-gap preparations are immutable'); END;
CREATE TRIGGER preserve_recovery_gap_preparation_delete BEFORE DELETE ON recovery_gap_preparations BEGIN SELECT RAISE(ABORT,'financial-gap preparations are retained'); END;
CREATE TRIGGER preserve_recovery_gap_decision_update BEFORE UPDATE ON recovery_gap_decisions BEGIN SELECT RAISE(ABORT,'financial-gap decisions are immutable'); END;
CREATE TRIGGER preserve_recovery_gap_decision_delete BEFORE DELETE ON recovery_gap_decisions BEGIN SELECT RAISE(ABORT,'financial-gap decisions are retained'); END;
