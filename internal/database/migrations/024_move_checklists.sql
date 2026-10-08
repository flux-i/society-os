CREATE TABLE move_checklist_resources (
 id TEXT PRIMARY KEY,
 flat_id TEXT NOT NULL REFERENCES flats(id),
 resident_id TEXT NOT NULL REFERENCES residents(id),
 kind TEXT NOT NULL CHECK(kind IN('MOVE_IN','MOVE_OUT','CONTACT_REVIEW')),
 author_id TEXT NOT NULL REFERENCES users(id),
 created_at INTEGER NOT NULL,
 version INTEGER NOT NULL CHECK(version>=1),
 latest_version INTEGER NOT NULL CHECK(latest_version=version),
 pending_version INTEGER,
 approved_version INTEGER,
 phase TEXT NOT NULL CHECK(phase IN('CHECKING','READY','INFO','COMPLETED','DECLINED','CANCELLED')),
 FOREIGN KEY(id,latest_version) REFERENCES move_checklist_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,pending_version) REFERENCES move_checklist_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,approved_version) REFERENCES move_checklist_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 CHECK(pending_version IS NULL OR pending_version=version),
 CHECK((pending_version IS NOT NULL)=(phase IN('CHECKING','READY','INFO'))),
 CHECK(approved_version IS NULL OR approved_version BETWEEN 1 AND version),
 CHECK(phase<>'COMPLETED' OR approved_version IS version),
 FOREIGN KEY(id,version) REFERENCES move_checklist_events(resource_id,version) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TABLE move_checklist_versions (
 resource_id TEXT NOT NULL REFERENCES move_checklist_resources(id),
 version INTEGER NOT NULL CHECK(version>=1),
 action TEXT NOT NULL CHECK(action IN('SUBMIT','REVISE','CHECK','READY','RETURN','INFO','APPROVED','DECLINED','CANCELLED','CORRECTION')),
 snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
 actor_id TEXT NOT NULL REFERENCES users(id),
 occurred_at INTEGER NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 PRIMARY KEY(resource_id,version),
 CHECK(json_type(snapshot_json,'$.version') IS 'integer' AND json_extract(snapshot_json,'$.version') IS version),
 CHECK(json_extract(snapshot_json,'$.action') IS action AND json_extract(snapshot_json,'$.actor_id') IS actor_id),
 CHECK(json_extract(snapshot_json,'$.occurred_at') IS occurred_at AND json_extract(snapshot_json,'$.reason') IS reason),
 CHECK(json_type(snapshot_json,'$.phase') IS 'text' AND json_extract(snapshot_json,'$.phase') IN('CHECKING','READY','INFO','COMPLETED','DECLINED','CANCELLED')),
 CHECK(json_type(snapshot_json,'$.effective_date') IS 'text' AND length(json_extract(snapshot_json,'$.effective_date'))=10),
 CHECK(json_type(snapshot_json,'$.note') IS 'text' AND length(json_extract(snapshot_json,'$.note')) BETWEEN 10 AND 800),
 CHECK(json_type(snapshot_json,'$.proposed_by') IS 'text' AND length(json_extract(snapshot_json,'$.proposed_by'))>0),
 CHECK(json_type(snapshot_json,'$.checks') IS 'array' AND json_array_length(snapshot_json,'$.checks')=5),
 CHECK(json_type(snapshot_json,'$.previous_approved_version') IS 'integer' AND json_extract(snapshot_json,'$.previous_approved_version') BETWEEN 0 AND version-1)
) STRICT;
CREATE TABLE move_checklist_events (
 resource_id TEXT NOT NULL,
 version INTEGER NOT NULL,
 action TEXT NOT NULL,
 actor_id TEXT NOT NULL REFERENCES users(id),
 occurred_at INTEGER NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 PRIMARY KEY(resource_id,version),
 FOREIGN KEY(resource_id,version) REFERENCES move_checklist_versions(resource_id,version)
) STRICT;
CREATE INDEX move_checklist_queue ON move_checklist_resources(phase,created_at,id);
CREATE INDEX move_checklist_personal ON move_checklist_resources(author_id,resident_id,created_at,id);
CREATE TRIGGER move_checklist_versions_update BEFORE UPDATE ON move_checklist_versions BEGIN SELECT RAISE(ABORT,'checklist originals are immutable'); END;
CREATE TRIGGER move_checklist_versions_delete BEFORE DELETE ON move_checklist_versions BEGIN SELECT RAISE(ABORT,'checklist originals are retained'); END;
CREATE TRIGGER move_checklist_events_update BEFORE UPDATE ON move_checklist_events BEGIN SELECT RAISE(ABORT,'checklist decisions are immutable'); END;
CREATE TRIGGER move_checklist_events_delete BEFORE DELETE ON move_checklist_events BEGIN SELECT RAISE(ABORT,'checklist decisions are retained'); END;
CREATE TRIGGER move_checklist_resources_delete BEFORE DELETE ON move_checklist_resources BEGIN SELECT RAISE(ABORT,'checklist history is retained'); END;
CREATE TRIGGER move_checklist_identity BEFORE UPDATE ON move_checklist_resources WHEN NEW.id<>OLD.id OR NEW.flat_id<>OLD.flat_id OR NEW.resident_id<>OLD.resident_id OR NEW.kind<>OLD.kind OR NEW.author_id<>OLD.author_id OR NEW.created_at<>OLD.created_at OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'checklist identity and sequence are retained'); END;
CREATE TRIGGER move_checklist_version_target BEFORE INSERT ON move_checklist_versions WHEN NOT EXISTS(
 SELECT 1 FROM move_checklist_resources h WHERE h.id=NEW.resource_id AND json_extract(NEW.snapshot_json,'$.flat_id') IS h.flat_id AND json_extract(NEW.snapshot_json,'$.resident_id') IS h.resident_id AND json_extract(NEW.snapshot_json,'$.kind') IS h.kind AND json_extract(NEW.snapshot_json,'$.author_id') IS h.author_id)
 BEGIN SELECT RAISE(ABORT,'checklist snapshots retain their exact target'); END;
CREATE TRIGGER move_checklist_check_shape BEFORE INSERT ON move_checklist_versions WHEN
 (SELECT COUNT(DISTINCT json_extract(value,'$.kind')) FROM json_each(NEW.snapshot_json,'$.checks'))<>5
 OR EXISTS(SELECT 1 FROM json_each(NEW.snapshot_json,'$.checks') WHERE json_extract(value,'$.kind') NOT IN('IDENTITY','REGISTRY','CONTACT','DOCUMENTS','HANDOVER') OR json_type(value,'$.kind') IS NOT 'text' OR json_type(value,'$.state') IS NOT 'text' OR json_extract(value,'$.state') NOT IN('INCOMPLETE','CHECKED','NOT_APPLICABLE')
 OR (json_extract(value,'$.state')='NOT_APPLICABLE' AND json_extract(value,'$.kind') NOT IN('DOCUMENTS','HANDOVER'))
 OR (json_extract(value,'$.state')<>'INCOMPLETE' AND (json_type(value,'$.reference') IS NOT 'text' OR length(json_extract(value,'$.reference')) NOT BETWEEN 5 AND 300 OR json_type(value,'$.checked_by') IS NOT 'text' OR length(json_extract(value,'$.checked_by'))=0 OR json_type(value,'$.checked_at') IS NOT 'integer' OR json_type(value,'$.source_key') IS NOT 'text' OR length(json_extract(value,'$.source_key'))<>64)))
 BEGIN SELECT RAISE(ABORT,'checklist evidence has five explicit permitted checks'); END;
CREATE TRIGGER move_checklist_ready_evidence BEFORE INSERT ON move_checklist_versions WHEN json_extract(NEW.snapshot_json,'$.phase') IN('READY','COMPLETED') AND (
 json_type(NEW.snapshot_json,'$.ready_by') IS NOT 'text' OR length(json_extract(NEW.snapshot_json,'$.ready_by'))=0 OR json_type(NEW.snapshot_json,'$.source_key') IS NOT 'text' OR length(json_extract(NEW.snapshot_json,'$.source_key'))<>64
 OR EXISTS(SELECT 1 FROM json_each(NEW.snapshot_json,'$.checks') WHERE json_extract(value,'$.state')='INCOMPLETE' OR json_extract(value,'$.source_key') IS NOT json_extract(NEW.snapshot_json,'$.source_key')))
 BEGIN SELECT RAISE(ABORT,'ready checklists retain current checked evidence'); END;
CREATE TRIGGER move_checklist_separate_approval BEFORE INSERT ON move_checklist_versions WHEN NEW.action='APPROVED' AND (
 json_extract(NEW.snapshot_json,'$.phase') IS NOT 'COMPLETED' OR NEW.actor_id=json_extract(NEW.snapshot_json,'$.author_id') OR NEW.actor_id=json_extract(NEW.snapshot_json,'$.proposed_by') OR NEW.actor_id=json_extract(NEW.snapshot_json,'$.ready_by'))
 BEGIN SELECT RAISE(ABORT,'checklist completion requires a different reviewer'); END;
CREATE TRIGGER move_checklist_event_source BEFORE INSERT ON move_checklist_events WHEN NOT EXISTS(SELECT 1 FROM move_checklist_versions v WHERE v.resource_id=NEW.resource_id AND v.version=NEW.version AND v.action=NEW.action AND v.actor_id=NEW.actor_id AND v.occurred_at=NEW.occurred_at AND v.reason=NEW.reason)
 BEGIN SELECT RAISE(ABORT,'checklist events retain their exact immutable source'); END;
CREATE TRIGGER move_checklist_head_event BEFORE UPDATE ON move_checklist_resources WHEN NOT EXISTS(SELECT 1 FROM move_checklist_events e JOIN move_checklist_versions v ON v.resource_id=e.resource_id AND v.version=e.version WHERE e.resource_id=NEW.id AND e.version=NEW.version AND json_extract(v.snapshot_json,'$.phase') IS NEW.phase)
 BEGIN SELECT RAISE(ABORT,'checklist head changes retain their exact event'); END;
CREATE TRIGGER move_checklist_approved_head BEFORE UPDATE OF approved_version ON move_checklist_resources WHEN NEW.approved_version IS NOT OLD.approved_version AND (NEW.approved_version IS NULL OR NEW.approved_version<>NEW.version OR NOT EXISTS(SELECT 1 FROM move_checklist_events e WHERE e.resource_id=NEW.id AND e.version=NEW.version AND e.action='APPROVED'))
 BEGIN SELECT RAISE(ABORT,'accepted checklists require their exact separate decision'); END;
CREATE TRIGGER move_checklist_initial_head BEFORE INSERT ON move_checklist_resources WHEN NEW.version<>1 OR NEW.latest_version<>1 OR NEW.pending_version IS NOT 1 OR NEW.approved_version IS NOT NULL OR NEW.phase<>'CHECKING'
 BEGIN SELECT RAISE(ABORT,'new checklists start as an unchecked submission'); END;
CREATE TRIGGER move_checklist_version_sequence BEFORE INSERT ON move_checklist_versions WHEN NOT EXISTS(SELECT 1 FROM move_checklist_resources h WHERE h.id=NEW.resource_id AND ((NEW.action='SUBMIT' AND NEW.version=1 AND h.version=1 AND h.phase='CHECKING') OR (NEW.action<>'SUBMIT' AND NEW.version=h.version+1)))
 BEGIN SELECT RAISE(ABORT,'checklist versions retain their exact preceding head'); END;
