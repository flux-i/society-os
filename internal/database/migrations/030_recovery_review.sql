-- Current recovery is explicitly ordered; random IDs and wall clocks are not
-- authority. A single legacy origin is unambiguous, multiple ones are held.
CREATE TABLE recovery_epoch_heads (
 sequence INTEGER PRIMARY KEY CHECK(sequence>0),
 epoch_id TEXT NOT NULL UNIQUE REFERENCES recovery_epochs(id)
) STRICT;
INSERT INTO recovery_epoch_heads SELECT 1,id FROM recovery_epochs WHERE (SELECT COUNT(*) FROM recovery_epochs)=1;
CREATE TRIGGER recovery_head_order BEFORE INSERT ON recovery_epoch_heads
WHEN NEW.sequence!=(SELECT COALESCE(MAX(sequence),0)+1 FROM recovery_epoch_heads)
BEGIN SELECT RAISE(ABORT,'recovery heads must be appended in order'); END;
CREATE TRIGGER preserve_recovery_head_update BEFORE UPDATE ON recovery_epoch_heads BEGIN SELECT RAISE(ABORT,'recovery heads are immutable'); END;
CREATE TRIGGER preserve_recovery_head_delete BEFORE DELETE ON recovery_epoch_heads BEGIN SELECT RAISE(ABORT,'recovery heads are retained'); END;

CREATE TABLE recovery_nominations (
 sequence INTEGER PRIMARY KEY CHECK(sequence>0),
 operation_id TEXT NOT NULL UNIQUE,
 epoch_id TEXT NOT NULL REFERENCES recovery_epochs(id),
 source_sha256 TEXT NOT NULL,
 predecessor TEXT NOT NULL,
 input_digest TEXT NOT NULL CHECK(length(input_digest)=64),
 input_json TEXT NOT NULL CHECK(json_valid(input_json)),
 valid_from INTEGER NOT NULL,
 valid_until INTEGER NOT NULL CHECK(valid_until>valid_from AND valid_until-valid_from<=2592000),
 created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER verify_recovery_nomination BEFORE INSERT ON recovery_nominations
WHEN NEW.sequence!=(SELECT COALESCE(MAX(sequence),0)+1 FROM recovery_nominations)
 OR NEW.epoch_id IS NOT (SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)
 OR NEW.source_sha256 IS NOT (SELECT source_sha256 FROM recovery_epochs WHERE id=NEW.epoch_id)
 OR NEW.predecessor!=COALESCE((SELECT operation_id FROM recovery_nominations WHERE epoch_id=NEW.epoch_id ORDER BY sequence DESC LIMIT 1),'')
BEGIN SELECT RAISE(ABORT,'current recovery nomination binding required'); END;
CREATE TABLE recovery_reviewers (
 nomination_id TEXT NOT NULL REFERENCES recovery_nominations(operation_id),
 user_id TEXT NOT NULL REFERENCES users(id),
 identity_ref TEXT NOT NULL,
 evidence_ref TEXT NOT NULL,
 verified_at INTEGER NOT NULL,
 initial_password_hash TEXT NOT NULL,
 PRIMARY KEY(nomination_id,user_id),
 UNIQUE(nomination_id,identity_ref)
) STRICT;
CREATE TABLE recovery_nomination_revocations (
 operation_id TEXT PRIMARY KEY,
 nomination_id TEXT NOT NULL UNIQUE REFERENCES recovery_nominations(operation_id),
 input_digest TEXT NOT NULL CHECK(length(input_digest)=64),
 input_json TEXT NOT NULL CHECK(json_valid(input_json)),
 created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE recovery_proposals (
 id TEXT PRIMARY KEY,
 operation_key TEXT NOT NULL UNIQUE,
 epoch_id TEXT NOT NULL REFERENCES recovery_epochs(id),
 nomination_id TEXT NOT NULL REFERENCES recovery_nominations(operation_id),
 author_id TEXT NOT NULL REFERENCES users(id),
 author_identity TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN('ACCESS','EXTERNAL')),
 version INTEGER NOT NULL CHECK(version>0),
 predecessor TEXT UNIQUE REFERENCES recovery_proposals(id),
 input_digest TEXT NOT NULL CHECK(length(input_digest)=64),
 input_json TEXT NOT NULL CHECK(json_valid(input_json)),
 created_at INTEGER NOT NULL
) STRICT;
CREATE TABLE recovery_decisions (
 proposal_id TEXT PRIMARY KEY REFERENCES recovery_proposals(id),
 operation_key TEXT NOT NULL UNIQUE,
 actor_id TEXT NOT NULL REFERENCES users(id),
 actor_identity TEXT NOT NULL,
 proposal_version INTEGER NOT NULL,
 decision TEXT NOT NULL CHECK(decision IN('APPROVED','DECLINED')),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 1000),
 attestation TEXT NOT NULL CHECK(length(attestation) BETWEEN 10 AND 300),
 input_digest TEXT NOT NULL CHECK(length(input_digest)=64),
 created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER verify_recovery_proposal BEFORE INSERT ON recovery_proposals
WHEN EXISTS(SELECT 1 FROM recovery_decisions WHERE operation_key=NEW.operation_key) OR NOT EXISTS(
 SELECT 1 FROM recovery_nominations n JOIN recovery_reviewers rr ON rr.nomination_id=n.operation_id JOIN users u ON u.id=rr.user_id
 WHERE n.operation_id=NEW.nomination_id AND rr.user_id=NEW.author_id AND rr.identity_ref=NEW.author_identity
 AND n.epoch_id=NEW.epoch_id AND n.epoch_id=(SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)
 AND n.sequence=(SELECT MAX(sequence) FROM recovery_nominations WHERE epoch_id=n.epoch_id)
 AND n.valid_from<=unixepoch('now') AND n.valid_until>unixepoch('now')
 AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id)
 AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'current nominated proposal author required'); END;
CREATE TRIGGER verify_recovery_decision BEFORE INSERT ON recovery_decisions
WHEN EXISTS(SELECT 1 FROM recovery_proposals WHERE operation_key=NEW.operation_key) OR NOT EXISTS(SELECT 1 FROM recovery_proposals p WHERE p.id=NEW.proposal_id
 AND p.version=NEW.proposal_version AND p.author_id!=NEW.actor_id AND p.author_identity!=NEW.actor_identity
 AND p.epoch_id=(SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)
 AND p.nomination_id=(SELECT operation_id FROM recovery_nominations WHERE epoch_id=p.epoch_id ORDER BY sequence DESC LIMIT 1)
 AND EXISTS(SELECT 1 FROM recovery_reviewers rr JOIN recovery_nominations n ON n.operation_id=rr.nomination_id JOIN users u ON u.id=rr.user_id
 WHERE rr.nomination_id=p.nomination_id AND rr.user_id=NEW.actor_id AND rr.identity_ref=NEW.actor_identity
 AND n.valid_from<=unixepoch('now') AND n.valid_until>unixepoch('now')
 AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id)
 AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL))
BEGIN SELECT RAISE(ABORT,'a separate current reviewer must decide the frozen version'); END;

CREATE TRIGGER preserve_nomination_update BEFORE UPDATE ON recovery_nominations BEGIN SELECT RAISE(ABORT,'recovery nominations are immutable'); END;
CREATE TRIGGER preserve_nomination_delete BEFORE DELETE ON recovery_nominations BEGIN SELECT RAISE(ABORT,'recovery nominations are retained'); END;
CREATE TRIGGER preserve_reviewer_update BEFORE UPDATE ON recovery_reviewers BEGIN SELECT RAISE(ABORT,'recovery reviewers are immutable'); END;
CREATE TRIGGER preserve_reviewer_delete BEFORE DELETE ON recovery_reviewers BEGIN SELECT RAISE(ABORT,'recovery reviewers are retained'); END;
CREATE TRIGGER preserve_revocation_update BEFORE UPDATE ON recovery_nomination_revocations BEGIN SELECT RAISE(ABORT,'recovery revocations are immutable'); END;
CREATE TRIGGER preserve_revocation_delete BEFORE DELETE ON recovery_nomination_revocations BEGIN SELECT RAISE(ABORT,'recovery revocations are retained'); END;
CREATE TRIGGER preserve_proposal_update BEFORE UPDATE ON recovery_proposals BEGIN SELECT RAISE(ABORT,'recovery proposals are immutable'); END;
CREATE TRIGGER preserve_proposal_delete BEFORE DELETE ON recovery_proposals BEGIN SELECT RAISE(ABORT,'recovery proposals are retained'); END;
CREATE TRIGGER preserve_decision_update BEFORE UPDATE ON recovery_decisions BEGIN SELECT RAISE(ABORT,'recovery decisions are immutable'); END;
CREATE TRIGGER preserve_decision_delete BEFORE DELETE ON recovery_decisions BEGIN SELECT RAISE(ABORT,'recovery decisions are retained'); END;
