CREATE TABLE upkeep_register (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK(kind IN ('VENDOR','ASSET')),
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    location TEXT NOT NULL DEFAULT '',
    contact TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    vendor_id TEXT REFERENCES upkeep_register(id),
    source_reference TEXT NOT NULL DEFAULT '',
    amc_start TEXT NOT NULL DEFAULT '',
    amc_end TEXT NOT NULL DEFAULT '',
    inspection_date TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK(state IN ('ACTIVE','INACTIVE')),
    version INTEGER NOT NULL DEFAULT 1 CHECK(version>0),
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK((amc_start='' AND amc_end='') OR (amc_start!='' AND amc_end>=amc_start)),
    CHECK(kind!='VENDOR' OR (location='' AND vendor_id IS NULL AND amc_start='' AND amc_end='' AND inspection_date='')),
    CHECK(kind!='ASSET' OR (contact='' AND phone='' AND email=''))
) STRICT;
CREATE INDEX upkeep_register_queue ON upkeep_register(kind,state,name);
CREATE TRIGGER upkeep_register_vendor_insert BEFORE INSERT ON upkeep_register
WHEN NEW.vendor_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM upkeep_register WHERE id=NEW.vendor_id AND kind='VENDOR' AND state='ACTIVE')
BEGIN SELECT RAISE(ABORT,'select a current active vendor'); END;
CREATE TRIGGER upkeep_register_vendor_update BEFORE UPDATE ON upkeep_register
WHEN NEW.vendor_id IS NOT NULL AND NEW.vendor_id IS NOT OLD.vendor_id AND NOT EXISTS(SELECT 1 FROM upkeep_register WHERE id=NEW.vendor_id AND kind='VENDOR' AND state='ACTIVE')
BEGIN SELECT RAISE(ABORT,'select a current active vendor'); END;
CREATE TRIGGER upkeep_register_identity BEFORE UPDATE ON upkeep_register
WHEN NEW.id!=OLD.id OR NEW.kind!=OLD.kind OR NEW.created_by!=OLD.created_by OR NEW.created_at!=OLD.created_at OR NEW.version!=OLD.version+1
BEGIN SELECT RAISE(ABORT,'retain register identity and use a current version'); END;
CREATE TRIGGER upkeep_register_delete BEFORE DELETE ON upkeep_register
BEGIN SELECT RAISE(ABORT,'upkeep register history is preserved'); END;
CREATE TABLE upkeep_register_events (
    id INTEGER PRIMARY KEY,
    register_id TEXT NOT NULL REFERENCES upkeep_register(id),
    version INTEGER NOT NULL,
    actor_id TEXT NOT NULL REFERENCES users(id),
    action TEXT NOT NULL,
    reason TEXT NOT NULL,
    occurred_at INTEGER NOT NULL,
    snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
    UNIQUE(register_id,version)
) STRICT;
CREATE TRIGGER upkeep_register_event_update BEFORE UPDATE ON upkeep_register_events
BEGIN SELECT RAISE(ABORT,'register activity is immutable'); END;
CREATE TRIGGER upkeep_register_event_delete BEFORE DELETE ON upkeep_register_events
BEGIN SELECT RAISE(ABORT,'register activity is preserved'); END;
CREATE TABLE upkeep_tasks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    category TEXT NOT NULL,
    priority TEXT NOT NULL CHECK(priority IN ('NORMAL','HIGH','URGENT')),
    due_date TEXT NOT NULL,
    visit_date TEXT NOT NULL DEFAULT '',
    asset_id TEXT REFERENCES upkeep_register(id),
    vendor_id TEXT REFERENCES upkeep_register(id),
    assigned_to TEXT REFERENCES users(id),
    complaint_id TEXT REFERENCES complaints(id),
    repeat_days INTEGER NOT NULL DEFAULT 0 CHECK(repeat_days BETWEEN 0 AND 365),
    parent_id TEXT REFERENCES upkeep_tasks(id),
    state TEXT NOT NULL CHECK(state IN ('PLANNED','IN_PROGRESS','WAITING','READY_FOR_CHECK','DONE','CANCELLED')),
    version INTEGER NOT NULL DEFAULT 1 CHECK(version>0),
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    ready_by TEXT REFERENCES users(id),
    ready_at INTEGER,
    checked_by TEXT REFERENCES users(id),
    checked_at INTEGER,
    audience TEXT NOT NULL DEFAULT 'INTERNAL' CHECK(audience IN ('INTERNAL','ALL_RESIDENTS','BUILDING')),
    building_code TEXT NOT NULL DEFAULT '',
    public_version INTEGER NOT NULL DEFAULT 0 CHECK(public_version BETWEEN 0 AND version),
    public_snapshot_json TEXT NOT NULL DEFAULT '' CHECK(public_snapshot_json='' OR json_valid(public_snapshot_json)),
    published_by TEXT REFERENCES users(id),
    published_at INTEGER,
    CHECK(audience='BUILDING' OR building_code=''),
    CHECK(audience='INTERNAL' OR (public_version>0 AND public_snapshot_json!='' AND published_at IS NOT NULL)),
    CHECK(state!='READY_FOR_CHECK' OR (ready_by IS NOT NULL AND ready_at IS NOT NULL)),
    CHECK(state!='DONE' OR (ready_by IS NOT NULL AND checked_by IS NOT NULL AND checked_at IS NOT NULL AND ready_by!=checked_by))
) STRICT;
CREATE INDEX upkeep_task_queue ON upkeep_tasks(state,due_date,priority);
CREATE UNIQUE INDEX upkeep_repeat_occurrence ON upkeep_tasks(parent_id,due_date) WHERE parent_id IS NOT NULL;
CREATE TRIGGER upkeep_task_insert BEFORE INSERT ON upkeep_tasks
WHEN NEW.state!='PLANNED' OR NEW.version!=1 OR NEW.audience!='INTERNAL' OR NEW.public_version!=0 OR NEW.public_snapshot_json!='' OR NEW.ready_by IS NOT NULL OR NEW.checked_by IS NOT NULL
BEGIN SELECT RAISE(ABORT,'new work starts as private planned work'); END;
CREATE TRIGGER upkeep_task_references BEFORE INSERT ON upkeep_tasks
WHEN (NEW.asset_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM upkeep_register WHERE id=NEW.asset_id AND kind='ASSET' AND state='ACTIVE')) OR (NEW.vendor_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM upkeep_register WHERE id=NEW.vendor_id AND kind='VENDOR' AND state='ACTIVE'))
BEGIN SELECT RAISE(ABORT,'new work requires active assets and vendors'); END;
CREATE INDEX upkeep_task_audience ON upkeep_tasks(audience,building_code);
CREATE TRIGGER upkeep_task_identity BEFORE UPDATE ON upkeep_tasks
WHEN NEW.id!=OLD.id OR NEW.title!=OLD.title OR NEW.body!=OLD.body OR NEW.category!=OLD.category OR NEW.priority!=OLD.priority OR
     NEW.created_by!=OLD.created_by OR NEW.created_at!=OLD.created_at OR NEW.repeat_days!=OLD.repeat_days OR
     NEW.parent_id IS NOT OLD.parent_id OR NEW.asset_id IS NOT OLD.asset_id OR NEW.vendor_id IS NOT OLD.vendor_id OR
     NEW.complaint_id IS NOT OLD.complaint_id OR NEW.version!=OLD.version+1
BEGIN SELECT RAISE(ABORT,'retain supplied work details and current versions'); END;
CREATE TRIGGER upkeep_task_transition BEFORE UPDATE ON upkeep_tasks
WHEN NEW.state!=OLD.state AND NOT (
    (OLD.state='PLANNED' AND NEW.state IN ('IN_PROGRESS','CANCELLED')) OR
    (OLD.state='IN_PROGRESS' AND NEW.state IN ('WAITING','READY_FOR_CHECK','CANCELLED')) OR
    (OLD.state='WAITING' AND NEW.state IN ('IN_PROGRESS','CANCELLED')) OR
    (OLD.state='READY_FOR_CHECK' AND NEW.state IN ('IN_PROGRESS','DONE','CANCELLED')) OR
    (OLD.state IN ('DONE','CANCELLED') AND NEW.state='PLANNED'))
BEGIN SELECT RAISE(ABORT,'invalid upkeep transition'); END;
CREATE TRIGGER upkeep_task_publication BEFORE UPDATE ON upkeep_tasks
WHEN (NEW.public_version=OLD.public_version AND (NEW.audience!=OLD.audience OR NEW.building_code!=OLD.building_code OR NEW.public_snapshot_json!=OLD.public_snapshot_json OR NEW.published_by IS NOT OLD.published_by OR NEW.published_at IS NOT OLD.published_at)) OR
     (NEW.public_version!=OLD.public_version AND NEW.public_version!=OLD.public_version+1) OR
     (NEW.public_version!=OLD.public_version AND NEW.audience!='INTERNAL' AND (json_extract(NEW.public_snapshot_json,'$.state')!=NEW.state OR json_extract(NEW.public_snapshot_json,'$.due_date')!=NEW.due_date))
BEGIN SELECT RAISE(ABORT,'resident publications require a separate frozen version'); END;
CREATE TRIGGER upkeep_task_delete BEFORE DELETE ON upkeep_tasks
BEGIN SELECT RAISE(ABORT,'upkeep work history is preserved'); END;
CREATE TABLE upkeep_task_events (
    id INTEGER PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES upkeep_tasks(id),
    version INTEGER NOT NULL,
    actor_id TEXT NOT NULL REFERENCES users(id),
    action TEXT NOT NULL,
    reason TEXT NOT NULL,
    occurred_at INTEGER NOT NULL,
    snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
    UNIQUE(task_id,version)
) STRICT;
CREATE TRIGGER upkeep_task_event_update BEFORE UPDATE ON upkeep_task_events
BEGIN SELECT RAISE(ABORT,'work activity is immutable'); END;
CREATE TRIGGER upkeep_task_event_delete BEFORE DELETE ON upkeep_task_events
BEGIN SELECT RAISE(ABORT,'work activity is preserved'); END;
