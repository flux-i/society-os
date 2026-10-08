CREATE TABLE meeting_resources (
 id TEXT PRIMARY KEY,
 version INTEGER NOT NULL CHECK(version>=1),
 latest_version INTEGER NOT NULL CHECK(latest_version BETWEEN 1 AND version),
 pending_version INTEGER,
 published_version INTEGER,
 decision TEXT NOT NULL CHECK(decision IN('PENDING','APPROVED','DECLINED','CANCELLED')),
 created_by TEXT NOT NULL REFERENCES users(id),
 created_at INTEGER NOT NULL,
 FOREIGN KEY(id,latest_version) REFERENCES meeting_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,pending_version) REFERENCES meeting_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,published_version) REFERENCES meeting_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 CHECK(pending_version IS NULL OR pending_version=latest_version),
 CHECK(published_version IS NULL OR published_version BETWEEN 1 AND version),
 CHECK((decision='PENDING')=(pending_version IS NOT NULL))
) STRICT;
CREATE TABLE meeting_versions (
 resource_id TEXT NOT NULL REFERENCES meeting_resources(id),
 version INTEGER NOT NULL CHECK(version>=1),
 action TEXT NOT NULL CHECK(action IN('AGENDA','MINUTES','CANCEL','WITHDRAW')),
 title TEXT NOT NULL CHECK(length(title) BETWEEN 5 AND 120),
 body TEXT NOT NULL CHECK(length(body) BETWEEN 10 AND 4000),
 location TEXT NOT NULL CHECK(length(location) BETWEEN 3 AND 200),
 scope TEXT NOT NULL CHECK(scope IN('ALL','WING','HOMES')),
 building_code TEXT NOT NULL,
 homes_json TEXT NOT NULL CHECK(json_valid(homes_json) AND json_type(homes_json)='array' AND json_array_length(homes_json) BETWEEN 1 AND 118),
 start_at INTEGER NOT NULL CHECK(start_at BETWEEN 946684800 AND 4102444799),
 end_at INTEGER NOT NULL CHECK(end_at=0 OR end_at BETWEEN start_at AND 4102444799),
 held_at INTEGER NOT NULL,
 minutes TEXT NOT NULL,
 update_text TEXT NOT NULL,
 ack_required INTEGER NOT NULL CHECK(ack_required IN(0,1)),
 ack_deadline INTEGER NOT NULL CHECK(ack_deadline=0 OR ack_deadline BETWEEN 946684800 AND 4102444799),
 submitted_by TEXT NOT NULL REFERENCES users(id),
 submitted_at INTEGER NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 PRIMARY KEY(resource_id,version),
 CHECK((scope='WING' AND length(building_code) BETWEEN 1 AND 20) OR (scope<>'WING' AND building_code='')),
 CHECK(ack_required=1 OR ack_deadline=0),
 CHECK(action NOT IN('CANCEL','WITHDRAW') OR ack_required=0),
 CHECK((action='AGENDA' AND held_at=0 AND minutes='' AND update_text='')
 OR (action='MINUTES' AND held_at BETWEEN start_at AND 4102444799 AND length(minutes) BETWEEN 10 AND 8000 AND update_text='')
 OR (action='CANCEL' AND held_at=0 AND minutes='' AND length(update_text) BETWEEN 10 AND 2000)
 OR action='WITHDRAW')
) STRICT;
CREATE TABLE meeting_events (
 resource_id TEXT NOT NULL REFERENCES meeting_resources(id),
 version INTEGER NOT NULL CHECK(version>=1),
 proposal_version INTEGER NOT NULL,
 action TEXT NOT NULL CHECK(action IN('PROPOSED','REVISED','APPROVED','DECLINED','CANCELLED')),
 actor_id TEXT NOT NULL REFERENCES users(id),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 occurred_at INTEGER NOT NULL,
 publication_fingerprint TEXT NOT NULL,
 PRIMARY KEY(resource_id,version),
 FOREIGN KEY(resource_id,proposal_version) REFERENCES meeting_versions(resource_id,version),
 CHECK((action='APPROVED' AND length(publication_fingerprint)=64) OR (action<>'APPROVED' AND publication_fingerprint=''))
) STRICT;
CREATE TABLE meeting_acknowledgements (
 id TEXT PRIMARY KEY,
 resource_id TEXT NOT NULL REFERENCES meeting_resources(id),
 publication_version INTEGER NOT NULL,
 publication_fingerprint TEXT NOT NULL CHECK(length(publication_fingerprint)=64),
 approval_event_version INTEGER NOT NULL,
 resident_id TEXT NOT NULL REFERENCES residents(id),
 actor_id TEXT NOT NULL REFERENCES users(id),
 homes_json TEXT NOT NULL CHECK(json_valid(homes_json) AND json_type(homes_json)='array' AND json_array_length(homes_json) BETWEEN 1 AND 118),
 occurred_at INTEGER NOT NULL,
 UNIQUE(resource_id,publication_version,resident_id),
 FOREIGN KEY(resource_id,publication_version) REFERENCES meeting_versions(resource_id,version),
 FOREIGN KEY(resource_id,approval_event_version) REFERENCES meeting_events(resource_id,version)
) STRICT;
CREATE INDEX meeting_published ON meeting_resources(published_version,id);
CREATE INDEX meeting_pending ON meeting_resources(decision,id);
CREATE INDEX meeting_responses ON meeting_acknowledgements(resource_id,publication_version,resident_id);
CREATE TRIGGER meeting_version_identity BEFORE UPDATE ON meeting_versions BEGIN SELECT RAISE(ABORT,'retain exact meeting proposals'); END;
CREATE TRIGGER meeting_version_retain BEFORE DELETE ON meeting_versions BEGIN SELECT RAISE(ABORT,'retain meeting originals'); END;
CREATE TRIGGER meeting_event_identity BEFORE UPDATE ON meeting_events BEGIN SELECT RAISE(ABORT,'retain meeting decisions'); END;
CREATE TRIGGER meeting_event_retain BEFORE DELETE ON meeting_events BEGIN SELECT RAISE(ABORT,'retain meeting decisions'); END;
CREATE TRIGGER meeting_ack_identity BEFORE UPDATE ON meeting_acknowledgements BEGIN SELECT RAISE(ABORT,'retain exact personal acknowledgement'); END;
CREATE TRIGGER meeting_ack_retain BEFORE DELETE ON meeting_acknowledgements BEGIN SELECT RAISE(ABORT,'retain exact personal acknowledgement'); END;
CREATE TRIGGER meeting_resource_retain BEFORE DELETE ON meeting_resources BEGIN SELECT RAISE(ABORT,'retain meeting history'); END;
CREATE TRIGGER meeting_resource_identity BEFORE UPDATE ON meeting_resources WHEN NEW.id<>OLD.id OR NEW.created_by<>OLD.created_by OR NEW.created_at<>OLD.created_at OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'retain meeting identity and version sequence'); END;
CREATE TRIGGER meeting_separate_review BEFORE INSERT ON meeting_events WHEN NEW.action IN('APPROVED','DECLINED') AND EXISTS(SELECT 1 FROM meeting_versions v WHERE v.resource_id=NEW.resource_id AND v.version=NEW.proposal_version AND v.submitted_by=NEW.actor_id)
 BEGIN SELECT RAISE(ABORT,'meeting publication requires a different reviewer'); END;
CREATE TRIGGER meeting_publication_head BEFORE UPDATE OF published_version ON meeting_resources WHEN NEW.published_version IS NOT OLD.published_version AND (NEW.published_version IS NULL OR NOT EXISTS(SELECT 1 FROM meeting_events e JOIN meeting_versions v ON v.resource_id=e.resource_id AND v.version=e.proposal_version WHERE e.resource_id=NEW.id AND e.action='APPROVED' AND e.version=NEW.version AND e.proposal_version=NEW.published_version AND e.actor_id<>v.submitted_by))
 BEGIN SELECT RAISE(ABORT,'meeting publication requires exact separate approval'); END;
CREATE TRIGGER meeting_ack_current_publication BEFORE INSERT ON meeting_acknowledgements WHEN NOT EXISTS(
 SELECT 1 FROM meeting_resources h JOIN meeting_versions v ON v.resource_id=h.id AND v.version=h.published_version
 JOIN meeting_events e ON e.resource_id=h.id AND e.version=NEW.approval_event_version AND e.proposal_version=v.version
 JOIN users u ON u.id=NEW.actor_id AND u.resident_id=NEW.resident_id AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL
 WHERE h.id=NEW.resource_id AND v.version=NEW.publication_version AND v.ack_required=1 AND v.action IN('AGENDA','MINUTES') AND e.action='APPROVED' AND e.publication_fingerprint=NEW.publication_fingerprint
 AND NOT EXISTS(SELECT 1 FROM json_each(NEW.homes_json) a WHERE NOT EXISTS(SELECT 1 FROM json_each(v.homes_json) b JOIN flat_memberships m ON m.flat_id=json_extract(b.value,'$.id') AND m.resident_id=NEW.resident_id AND m.start_date<=date('now','+330 minutes') AND (m.end_date IS NULL OR m.end_date>date('now','+330 minutes')) WHERE json_extract(a.value,'$.id')=json_extract(b.value,'$.id'))))
 BEGIN SELECT RAISE(ABORT,'acknowledgement requires exact current publication and personal household'); END;
