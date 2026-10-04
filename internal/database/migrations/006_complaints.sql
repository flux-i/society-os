CREATE TABLE complaint_counters (
    year INTEGER PRIMARY KEY CHECK(year BETWEEN 2000 AND 9999),
    last_number INTEGER NOT NULL CHECK(last_number>0)
) STRICT;
CREATE TABLE complaints (
    id TEXT PRIMARY KEY,
    complaint_number TEXT NOT NULL UNIQUE,
    flat_id TEXT NOT NULL REFERENCES flats(id),
    reported_by TEXT NOT NULL REFERENCES users(id),
    category TEXT NOT NULL CHECK(category IN ('PLUMBING','LIFT','ELECTRICAL','SECURITY','CLEANING','WATER','PARKING','COMMON_AREA','OTHER')),
    subject TEXT NOT NULL,
    description TEXT NOT NULL,
    priority TEXT NOT NULL CHECK(priority IN ('NORMAL','HIGH','URGENT')),
    status TEXT NOT NULL CHECK(status IN ('OPEN','ACKNOWLEDGED','IN_PROGRESS','WAITING','RESOLVED','CLOSED')),
    assigned_to TEXT REFERENCES users(id),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    resolved_at INTEGER,
    closed_at INTEGER,
    version INTEGER NOT NULL DEFAULT 1 CHECK(version>0),
    public_version INTEGER NOT NULL DEFAULT 1 CHECK(public_version>0 AND public_version<=version),
    public_updated_at INTEGER NOT NULL
) STRICT;
CREATE INDEX complaint_queue ON complaints(status,updated_at);
CREATE INDEX complaint_reporter ON complaints(reported_by,updated_at);
CREATE TRIGGER preserve_complaint_delete BEFORE DELETE ON complaints BEGIN SELECT RAISE(ABORT,'service request history is preserved'); END;
CREATE TRIGGER preserve_complaint_identity BEFORE UPDATE ON complaints WHEN NEW.id<>OLD.id OR NEW.complaint_number<>OLD.complaint_number OR NEW.flat_id<>OLD.flat_id OR NEW.reported_by<>OLD.reported_by OR NEW.category<>OLD.category OR NEW.subject<>OLD.subject OR NEW.description<>OLD.description OR NEW.created_at<>OLD.created_at BEGIN SELECT RAISE(ABORT,'reported details are immutable'); END;
CREATE TRIGGER enforce_complaint_transition BEFORE UPDATE ON complaints WHEN NEW.status<>OLD.status AND NOT (
    (OLD.status='OPEN' AND NEW.status IN ('ACKNOWLEDGED','IN_PROGRESS','WAITING')) OR
    (OLD.status='ACKNOWLEDGED' AND NEW.status IN ('IN_PROGRESS','WAITING')) OR
    (OLD.status='IN_PROGRESS' AND NEW.status IN ('WAITING','RESOLVED')) OR
    (OLD.status='WAITING' AND NEW.status IN ('IN_PROGRESS','RESOLVED')) OR
    (OLD.status='RESOLVED' AND NEW.status IN ('IN_PROGRESS','CLOSED')) OR
    (OLD.status='CLOSED' AND NEW.status='OPEN')
) BEGIN SELECT RAISE(ABORT,'invalid service request transition'); END;
CREATE TABLE complaint_updates (
    id INTEGER PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    complaint_id TEXT NOT NULL REFERENCES complaints(id),
    actor_id TEXT NOT NULL REFERENCES users(id),
    action TEXT NOT NULL CHECK(action IN ('CREATED','COMMENT','STATUS','ASSIGN','PRIORITY')),
    message TEXT NOT NULL,
    visibility TEXT NOT NULL CHECK(visibility IN ('RESIDENT_VISIBLE','STAFF_ONLY')),
    occurred_at INTEGER NOT NULL,
    version INTEGER NOT NULL,
    snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
    UNIQUE(complaint_id,version),
    CHECK(action='COMMENT' OR visibility='RESIDENT_VISIBLE')
) STRICT;
CREATE INDEX complaint_update_scope ON complaint_updates(complaint_id,visibility,id);
CREATE TRIGGER preserve_complaint_update BEFORE UPDATE ON complaint_updates BEGIN SELECT RAISE(ABORT,'service request updates are immutable'); END;
CREATE TRIGGER preserve_complaint_update_delete BEFORE DELETE ON complaint_updates BEGIN SELECT RAISE(ABORT,'service request updates are immutable'); END;
