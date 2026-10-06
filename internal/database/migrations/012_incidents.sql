CREATE TABLE society_rules (
 id TEXT PRIMARY KEY,
 replaces_id TEXT REFERENCES society_rules(id),
 title TEXT NOT NULL,
 rule_text TEXT NOT NULL,
 policy_reference TEXT NOT NULL,
 effective_from TEXT NOT NULL,
 effective_until TEXT NOT NULL,
 fine_permitted INTEGER NOT NULL CHECK(fine_permitted IN(0,1)),
 state TEXT NOT NULL CHECK(state IN('PENDING','PUBLISHED','DECLINED','WITHDRAWN','RETIRED')),
 author_id TEXT NOT NULL REFERENCES users(id),
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 version INTEGER NOT NULL CHECK(version>0)
) STRICT;
CREATE INDEX society_rules_queue ON society_rules(state,updated_at);
CREATE UNIQUE INDEX society_rules_pending_replacement ON society_rules(replaces_id) WHERE state='PENDING' AND replaces_id IS NOT NULL;
CREATE TRIGGER society_rule_preserve_delete BEFORE DELETE ON society_rules BEGIN SELECT RAISE(ABORT,'rules are retained'); END;
CREATE TRIGGER society_rule_frozen BEFORE UPDATE ON society_rules WHEN NEW.id<>OLD.id OR NEW.replaces_id IS NOT OLD.replaces_id OR NEW.title<>OLD.title OR NEW.rule_text<>OLD.rule_text OR NEW.policy_reference<>OLD.policy_reference OR NEW.effective_from<>OLD.effective_from OR NEW.effective_until<>OLD.effective_until OR NEW.fine_permitted<>OLD.fine_permitted OR NEW.author_id<>OLD.author_id OR NEW.created_at<>OLD.created_at BEGIN SELECT RAISE(ABORT,'proposed rule content is frozen'); END;
CREATE TRIGGER society_rule_transition BEFORE UPDATE ON society_rules WHEN NEW.version<>OLD.version+1 OR NOT ((OLD.state='PENDING' AND NEW.state IN('PUBLISHED','DECLINED','WITHDRAWN')) OR (OLD.state='PUBLISHED' AND NEW.state='RETIRED')) BEGIN SELECT RAISE(ABORT,'invalid rule transition'); END;
CREATE TABLE society_rule_events (
 id INTEGER PRIMARY KEY,
 rule_id TEXT NOT NULL REFERENCES society_rules(id),
 version INTEGER NOT NULL,
 actor_id TEXT NOT NULL REFERENCES users(id),
 action TEXT NOT NULL,
 reason TEXT NOT NULL,
 occurred_at INTEGER NOT NULL,
 snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
 UNIQUE(rule_id,version)
) STRICT;
CREATE TRIGGER society_rule_event_update BEFORE UPDATE ON society_rule_events BEGIN SELECT RAISE(ABORT,'rule events are immutable'); END;
CREATE TRIGGER society_rule_event_delete BEFORE DELETE ON society_rule_events BEGIN SELECT RAISE(ABORT,'rule events are retained'); END;
CREATE TABLE incident_pictures (
 id TEXT PRIMARY KEY,
 uploader_id TEXT NOT NULL REFERENCES users(id),
 filename TEXT NOT NULL,
 media_type TEXT NOT NULL CHECK(media_type IN('image/png','image/jpeg')),
 width INTEGER NOT NULL CHECK(width>0 AND width<=4096),
 height INTEGER NOT NULL CHECK(height>0 AND height<=4096 AND width*height<=8000000),
 original_sha256 TEXT NOT NULL CHECK(length(original_sha256)=64),
 original_bytes BLOB NOT NULL CHECK(length(original_bytes)>0 AND length(original_bytes)<=5242880),
 preview_sha256 TEXT NOT NULL CHECK(length(preview_sha256)=64),
 preview_bytes BLOB NOT NULL CHECK(length(preview_bytes)>0),
 created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX incident_picture_uploader ON incident_pictures(uploader_id,created_at);
CREATE TRIGGER incident_picture_update BEFORE UPDATE ON incident_pictures BEGIN SELECT RAISE(ABORT,'incident originals and previews are immutable'); END;
CREATE TRIGGER incident_picture_delete BEFORE DELETE ON incident_pictures BEGIN SELECT RAISE(ABORT,'incident evidence is retained'); END;
CREATE TABLE incidents (
 id TEXT PRIMARY KEY,
 rule_id TEXT NOT NULL REFERENCES society_rules(id),
 flat_id TEXT NOT NULL REFERENCES flats(id),
 reporter_id TEXT NOT NULL REFERENCES users(id),
 incident_date TEXT NOT NULL,
 comment TEXT NOT NULL,
 picture_id TEXT REFERENCES incident_pictures(id),
 state TEXT NOT NULL CHECK(state IN('REPORTED','NEEDS_INFO','UNDER_REVIEW','DISMISSED','SUBSTANTIATED','WITHDRAWN')),
 duplicate_of TEXT REFERENCES incidents(id),
 notice_id TEXT REFERENCES incident_notices(id),
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 version INTEGER NOT NULL CHECK(version>0),
 reporter_version INTEGER NOT NULL CHECK(reporter_version>0 AND reporter_version<=version),
 reporter_updated_at INTEGER NOT NULL,
 CHECK(duplicate_of IS NULL OR duplicate_of<>id)
) STRICT;
CREATE INDEX incidents_reporter ON incidents(reporter_id,reporter_updated_at);
CREATE INDEX incidents_queue ON incidents(state,updated_at);
CREATE TRIGGER incident_preserve_identity BEFORE UPDATE ON incidents WHEN NEW.id<>OLD.id OR NEW.reporter_id<>OLD.reporter_id OR NEW.created_at<>OLD.created_at OR NEW.version<>OLD.version+1 BEGIN SELECT RAISE(ABORT,'incident identity and version are retained'); END;
CREATE TRIGGER incident_preserve_delete BEFORE DELETE ON incidents BEGIN SELECT RAISE(ABORT,'incident history is retained'); END;
CREATE TABLE incident_events (
 id INTEGER PRIMARY KEY,
 incident_id TEXT NOT NULL REFERENCES incidents(id),
 version INTEGER NOT NULL,
 reporter_version INTEGER NOT NULL,
 reporter_visible INTEGER NOT NULL CHECK(reporter_visible IN(0,1)),
 actor_id TEXT NOT NULL REFERENCES users(id),
 action TEXT NOT NULL,
 reason TEXT NOT NULL,
 occurred_at INTEGER NOT NULL,
 snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
 UNIQUE(incident_id,version)
) STRICT;
CREATE INDEX incident_event_scope ON incident_events(incident_id,reporter_visible,id);
CREATE TRIGGER incident_event_update BEFORE UPDATE ON incident_events BEGIN SELECT RAISE(ABORT,'incident events are immutable'); END;
CREATE TRIGGER incident_event_delete BEFORE DELETE ON incident_events BEGIN SELECT RAISE(ABORT,'incident events are retained'); END;
CREATE TABLE incident_notices (
 id TEXT PRIMARY KEY,
 incident_id TEXT NOT NULL REFERENCES incidents(id),
 flat_id TEXT NOT NULL REFERENCES flats(id),
 rule_id TEXT NOT NULL REFERENCES society_rules(id),
 incident_date TEXT NOT NULL,
 title TEXT NOT NULL,
 body TEXT NOT NULL,
 response_by TEXT NOT NULL,
 picture_id TEXT REFERENCES incident_pictures(id),
 issued_by TEXT NOT NULL REFERENCES users(id),
 created_at INTEGER NOT NULL,
 version INTEGER NOT NULL CHECK(version>0),
 UNIQUE(incident_id,version)
) STRICT;
CREATE INDEX incident_notice_home ON incident_notices(flat_id,created_at);
CREATE TRIGGER incident_notice_update BEFORE UPDATE ON incident_notices BEGIN SELECT RAISE(ABORT,'issued notices are frozen'); END;
CREATE TRIGGER incident_notice_delete BEFORE DELETE ON incident_notices BEGIN SELECT RAISE(ABORT,'issued notices are retained'); END;
CREATE TABLE incident_responses (
 id TEXT PRIMARY KEY,
 notice_id TEXT NOT NULL REFERENCES incident_notices(id),
 actor_id TEXT NOT NULL REFERENCES users(id),
 body TEXT NOT NULL,
 created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX incident_response_notice ON incident_responses(notice_id,created_at);
CREATE TRIGGER incident_response_update BEFORE UPDATE ON incident_responses BEGIN SELECT RAISE(ABORT,'responses are immutable'); END;
CREATE TRIGGER incident_response_delete BEFORE DELETE ON incident_responses BEGIN SELECT RAISE(ABORT,'responses are retained'); END;
