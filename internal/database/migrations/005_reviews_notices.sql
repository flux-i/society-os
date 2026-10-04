CREATE TABLE review_requests (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK(kind IN ('NOTICE','MAINTENANCE','REGISTRY_CHANGE','EXPENSE')),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    flat_id TEXT REFERENCES flats(id),
    audience TEXT NOT NULL CHECK(audience IN ('','ALL_RESIDENTS','OWNERS_ONLY','TENANTS_ONLY','COMMITTEE_ONLY','BUILDING')),
    building_code TEXT NOT NULL,
    estimate_paise INTEGER NOT NULL DEFAULT 0 CHECK(estimate_paise BETWEEN 0 AND 1000000000),
    state TEXT NOT NULL CHECK(state IN ('PENDING','CHANGES_REQUESTED','APPROVED','REJECTED','WITHDRAWN','ARCHIVED')),
    submitted_by TEXT NOT NULL REFERENCES users(id),
    submitted_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK(version > 0),
    CHECK((kind='NOTICE' AND audience<>'' AND flat_id IS NULL AND estimate_paise=0) OR (kind<>'NOTICE' AND audience='' AND building_code='')),
    CHECK((audience='BUILDING' AND building_code IN ('A','B','C')) OR (audience<>'BUILDING' AND building_code='')),
    CHECK(kind='EXPENSE' OR estimate_paise=0)
) STRICT;
CREATE INDEX review_queue ON review_requests(state,updated_at);
CREATE INDEX review_author ON review_requests(submitted_by,updated_at);
CREATE TRIGGER preserve_review_delete BEFORE DELETE ON review_requests BEGIN SELECT RAISE(ABORT,'review history is preserved'); END;
CREATE TRIGGER preserve_review_identity BEFORE UPDATE ON review_requests WHEN NEW.id<>OLD.id OR NEW.kind<>OLD.kind OR NEW.submitted_by<>OLD.submitted_by OR NEW.submitted_at<>OLD.submitted_at BEGIN SELECT RAISE(ABORT,'submission identity is immutable'); END;
CREATE TRIGGER preserve_review_content BEFORE UPDATE ON review_requests WHEN OLD.state IN ('APPROVED','REJECTED','WITHDRAWN','ARCHIVED') AND (NEW.title<>OLD.title OR NEW.body<>OLD.body OR NEW.audience<>OLD.audience OR NEW.building_code<>OLD.building_code OR NEW.estimate_paise<>OLD.estimate_paise OR NEW.flat_id IS NOT OLD.flat_id) BEGIN SELECT RAISE(ABORT,'decided content is immutable'); END;
CREATE TRIGGER enforce_review_transition BEFORE UPDATE ON review_requests WHEN NEW.state<>OLD.state AND NOT (
    (OLD.state='PENDING' AND NEW.state IN ('APPROVED','REJECTED','CHANGES_REQUESTED','WITHDRAWN')) OR
    (OLD.state='CHANGES_REQUESTED' AND NEW.state IN ('PENDING','WITHDRAWN')) OR
    (OLD.state='APPROVED' AND NEW.state='ARCHIVED')
) BEGIN SELECT RAISE(ABORT,'invalid review transition'); END;
CREATE TABLE review_events (
    id INTEGER PRIMARY KEY,
    request_id TEXT NOT NULL REFERENCES review_requests(id),
    actor_id TEXT NOT NULL REFERENCES users(id),
    action TEXT NOT NULL CHECK(action IN ('SUBMITTED','RESUBMITTED','APPROVED','REJECTED','CHANGES_REQUESTED','WITHDRAWN','ARCHIVED')),
    reason TEXT NOT NULL,
    occurred_at INTEGER NOT NULL,
    version INTEGER NOT NULL,
    snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
    UNIQUE(request_id,version)
) STRICT;
CREATE TRIGGER preserve_review_event_update BEFORE UPDATE ON review_events BEGIN SELECT RAISE(ABORT,'review decisions are immutable'); END;
CREATE TRIGGER preserve_review_event_delete BEFORE DELETE ON review_events BEGIN SELECT RAISE(ABORT,'review decisions are immutable'); END;
CREATE TRIGGER prevent_self_review BEFORE INSERT ON review_events WHEN NEW.action IN ('APPROVED','REJECTED','CHANGES_REQUESTED') AND NEW.actor_id=(SELECT submitted_by FROM review_requests WHERE id=NEW.request_id) BEGIN SELECT RAISE(ABORT,'a submitter cannot review their own item'); END;
