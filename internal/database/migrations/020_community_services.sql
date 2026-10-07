CREATE TABLE community_resources (
 id TEXT PRIMARY KEY,
 kind TEXT NOT NULL CHECK(kind IN('CONTACT','INTERRUPTION')),
 version INTEGER NOT NULL CHECK(version>=1),
 latest_version INTEGER NOT NULL CHECK(latest_version BETWEEN 1 AND version),
 pending_version INTEGER,
 published_version INTEGER,
 decision TEXT NOT NULL CHECK(decision IN('PENDING','APPROVED','DECLINED','CANCELLED')),
 created_by TEXT NOT NULL REFERENCES users(id),
 created_at INTEGER NOT NULL,
 UNIQUE(id,kind),
 FOREIGN KEY(id,latest_version) REFERENCES community_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,pending_version) REFERENCES community_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,published_version) REFERENCES community_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 CHECK(pending_version IS NULL OR pending_version=latest_version),
 CHECK(published_version IS NULL OR published_version BETWEEN 1 AND version),
 CHECK((decision='PENDING')=(pending_version IS NOT NULL))
) STRICT;
CREATE TABLE community_versions (
 resource_id TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version>=1),
 kind TEXT NOT NULL CHECK(kind IN('CONTACT','INTERRUPTION')),
 action TEXT NOT NULL CHECK(action IN('PUBLISH','RESOLVE','WITHDRAW')),
 title TEXT NOT NULL CHECK(length(title) BETWEEN 5 AND 120),
 body TEXT NOT NULL CHECK(length(body) BETWEEN 10 AND 2000),
 service TEXT NOT NULL CHECK(service IN('WATER','POWER','LIFT','OTHER')),
 phone TEXT NOT NULL,
 availability TEXT NOT NULL,
 attestation TEXT NOT NULL,
 scope TEXT NOT NULL CHECK(scope IN('ALL','WING','HOMES')),
 building_code TEXT NOT NULL,
 homes_json TEXT NOT NULL CHECK(json_valid(homes_json) AND json_type(homes_json)='array' AND json_array_length(homes_json) BETWEEN 1 AND 118),
 start_at INTEGER NOT NULL,
 estimated_end INTEGER NOT NULL,
 resolved_at INTEGER NOT NULL,
 update_text TEXT NOT NULL,
 submitted_by TEXT NOT NULL REFERENCES users(id),
 submitted_at INTEGER NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 PRIMARY KEY(resource_id,version),
 FOREIGN KEY(resource_id,kind) REFERENCES community_resources(id,kind),
 CHECK((scope='WING' AND length(building_code) BETWEEN 1 AND 20) OR (scope<>'WING' AND building_code='')),
 CHECK((kind='CONTACT' AND length(phone) BETWEEN 9 AND 16 AND length(availability) BETWEEN 3 AND 200 AND length(attestation) BETWEEN 10 AND 800 AND start_at=0 AND estimated_end=0 AND resolved_at=0 AND update_text='' AND action<>'RESOLVE')
 OR (kind='INTERRUPTION' AND phone='' AND availability='' AND attestation='' AND start_at BETWEEN 946684800 AND 4102444799 AND (estimated_end=0 OR estimated_end BETWEEN start_at AND 4102444799)
 AND ((action='RESOLVE' AND resolved_at>=start_at AND resolved_at<=4102444799 AND length(update_text) BETWEEN 10 AND 2000) OR (action<>'RESOLVE' AND resolved_at=0 AND update_text=''))))
) STRICT;
CREATE TABLE community_events (
 resource_id TEXT NOT NULL REFERENCES community_resources(id),
 version INTEGER NOT NULL CHECK(version>=1),
 proposal_version INTEGER NOT NULL,
 action TEXT NOT NULL CHECK(action IN('PROPOSED','REVISED','APPROVED','DECLINED','CANCELLED')),
 actor_id TEXT NOT NULL REFERENCES users(id),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 occurred_at INTEGER NOT NULL,
 PRIMARY KEY(resource_id,version),
 FOREIGN KEY(resource_id,proposal_version) REFERENCES community_versions(resource_id,version)
) STRICT;
CREATE INDEX community_published ON community_resources(kind,published_version,id);
CREATE INDEX community_pending ON community_resources(decision,kind,id);
CREATE TRIGGER community_version_identity BEFORE UPDATE ON community_versions BEGIN SELECT RAISE(ABORT,'retain exact community proposals'); END;
CREATE TRIGGER community_version_retain BEFORE DELETE ON community_versions BEGIN SELECT RAISE(ABORT,'retain community publication history'); END;
CREATE TRIGGER community_event_identity BEFORE UPDATE ON community_events BEGIN SELECT RAISE(ABORT,'retain community decisions'); END;
CREATE TRIGGER community_event_retain BEFORE DELETE ON community_events BEGIN SELECT RAISE(ABORT,'retain community decisions'); END;
CREATE TRIGGER community_resource_retain BEFORE DELETE ON community_resources BEGIN SELECT RAISE(ABORT,'retain community resources'); END;
CREATE TRIGGER community_resource_identity BEFORE UPDATE ON community_resources WHEN NEW.id<>OLD.id OR NEW.kind<>OLD.kind OR NEW.created_by<>OLD.created_by OR NEW.created_at<>OLD.created_at OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'retain resource identity and version sequence'); END;
CREATE TRIGGER community_separate_review BEFORE INSERT ON community_events WHEN NEW.action IN('APPROVED','DECLINED') AND EXISTS(SELECT 1 FROM community_versions v WHERE v.resource_id=NEW.resource_id AND v.version=NEW.proposal_version AND v.submitted_by=NEW.actor_id)
 BEGIN SELECT RAISE(ABORT,'community publication requires a different reviewer'); END;
CREATE TRIGGER community_publication_head BEFORE UPDATE OF published_version ON community_resources WHEN NEW.published_version IS NOT OLD.published_version AND (NEW.published_version IS NULL OR NOT EXISTS(SELECT 1 FROM community_events e JOIN community_versions v ON v.resource_id=e.resource_id AND v.version=e.proposal_version WHERE e.resource_id=NEW.id AND e.action='APPROVED' AND e.version=NEW.version AND e.proposal_version=NEW.published_version AND e.actor_id<>v.submitted_by))
 BEGIN SELECT RAISE(ABORT,'community publication requires exact separate approval'); END;
