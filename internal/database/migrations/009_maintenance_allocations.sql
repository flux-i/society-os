CREATE TABLE maintenance_cycles (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL CHECK(period_end >= period_start),
    due_date TEXT NOT NULL CHECK(due_date >= period_start),
    source_reference TEXT NOT NULL,
    note TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('PENDING','PUBLISHED','DECLINED','WITHDRAWN')),
    version INTEGER NOT NULL CHECK(version > 0),
    submitted_by TEXT NOT NULL REFERENCES users(id),
    submitted_at INTEGER NOT NULL,
    decided_by TEXT REFERENCES users(id),
    decided_at INTEGER,
    decision_reason TEXT NOT NULL DEFAULT '',
    CHECK((state='PENDING' AND version=1 AND decided_by IS NULL AND decided_at IS NULL) OR
          (state!='PENDING' AND version=2 AND decided_by IS NOT NULL AND decided_at IS NOT NULL)),
    CHECK(state NOT IN ('PUBLISHED','DECLINED') OR decided_by!=submitted_by),
    CHECK(state!='WITHDRAWN' OR decided_by=submitted_by)
) STRICT;
CREATE TRIGGER maintenance_cycle_insert BEFORE INSERT ON maintenance_cycles
WHEN NEW.state!='PENDING' OR NEW.version!=1
BEGIN SELECT RAISE(ABORT,'maintenance starts as an unposted proposal'); END;
CREATE INDEX maintenance_periods ON maintenance_cycles(period_start DESC,state);
CREATE TABLE maintenance_lines (
    cycle_id TEXT NOT NULL REFERENCES maintenance_cycles(id),
    flat_id TEXT NOT NULL REFERENCES flats(id),
    amount_paise INTEGER NOT NULL CHECK(amount_paise > 0 AND amount_paise <= 1000000000),
    entry_id TEXT UNIQUE REFERENCES entries(id),
    PRIMARY KEY(cycle_id,flat_id)
) STRICT;
CREATE INDEX maintenance_flat ON maintenance_lines(flat_id,cycle_id);
CREATE TRIGGER maintenance_line_insert BEFORE INSERT ON maintenance_lines
WHEN (SELECT state FROM maintenance_cycles WHERE id=NEW.cycle_id)!='PENDING' OR NEW.entry_id IS NOT NULL OR
     EXISTS(SELECT 1 FROM maintenance_events WHERE cycle_id=NEW.cycle_id AND action='SUBMITTED')
BEGIN SELECT RAISE(ABORT,'maintenance participants are frozen'); END;
CREATE TRIGGER maintenance_line_update BEFORE UPDATE ON maintenance_lines
WHEN NEW.cycle_id!=OLD.cycle_id OR NEW.flat_id!=OLD.flat_id OR NEW.amount_paise!=OLD.amount_paise OR
     OLD.entry_id IS NOT NULL OR NEW.entry_id IS NULL OR
     (SELECT state FROM maintenance_cycles WHERE id=OLD.cycle_id)!='PENDING' OR
     NOT EXISTS(SELECT 1 FROM entries e WHERE e.id=NEW.entry_id AND e.flat_id=OLD.flat_id AND
                e.kind='CHARGE' AND e.state='POSTED' AND e.amount_paise=OLD.amount_paise)
BEGIN SELECT RAISE(ABORT,'maintenance charges must match the approved proposal'); END;
CREATE TRIGGER maintenance_line_delete BEFORE DELETE ON maintenance_lines
BEGIN SELECT RAISE(ABORT,'maintenance participation history is preserved'); END;
CREATE TRIGGER maintenance_cycle_update BEFORE UPDATE ON maintenance_cycles
WHEN OLD.state!='PENDING' OR NEW.state='PENDING' OR NEW.version!=OLD.version+1 OR
     NEW.id!=OLD.id OR NEW.title!=OLD.title OR NEW.period_start!=OLD.period_start OR
     NEW.period_end!=OLD.period_end OR NEW.due_date!=OLD.due_date OR
     NEW.source_reference!=OLD.source_reference OR NEW.note!=OLD.note OR
     NEW.submitted_by!=OLD.submitted_by OR NEW.submitted_at!=OLD.submitted_at
BEGIN SELECT RAISE(ABORT,'maintenance proposals and decisions are immutable'); END;
CREATE TRIGGER maintenance_publish_complete BEFORE UPDATE ON maintenance_cycles
WHEN NEW.state='PUBLISHED' AND (NOT EXISTS(SELECT 1 FROM maintenance_lines WHERE cycle_id=OLD.id) OR
     EXISTS(SELECT 1 FROM maintenance_lines WHERE cycle_id=OLD.id AND entry_id IS NULL))
BEGIN SELECT RAISE(ABORT,'publishing requires every approved charge'); END;
CREATE TRIGGER maintenance_cycle_delete BEFORE DELETE ON maintenance_cycles
BEGIN SELECT RAISE(ABORT,'maintenance history is preserved'); END;
CREATE TABLE maintenance_events (
    id INTEGER PRIMARY KEY,
    cycle_id TEXT NOT NULL REFERENCES maintenance_cycles(id),
    action TEXT NOT NULL CHECK(action IN ('SUBMITTED','PUBLISHED','DECLINED','WITHDRAWN')),
    version INTEGER NOT NULL,
    actor_id TEXT NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL,
    occurred_at INTEGER NOT NULL
) STRICT;
CREATE INDEX maintenance_history ON maintenance_events(cycle_id,id DESC);
CREATE TRIGGER maintenance_event_update BEFORE UPDATE ON maintenance_events
BEGIN SELECT RAISE(ABORT,'maintenance decisions are immutable'); END;
CREATE TRIGGER maintenance_event_delete BEFORE DELETE ON maintenance_events
BEGIN SELECT RAISE(ABORT,'maintenance decisions are preserved'); END;

-- One explicit ledger allocation mechanism, reusable for later approved funds.
-- It never creates a received entry, receipt, or inferred liability.
CREATE TABLE entry_allocations (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES entries(id),
    charge_id TEXT NOT NULL REFERENCES entries(id),
    amount_paise INTEGER NOT NULL CHECK(amount_paise > 0 AND amount_paise <= 1000000000),
    actor_id TEXT NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    CHECK(source_id!=charge_id)
) STRICT;
CREATE INDEX allocation_source ON entry_allocations(source_id);
CREATE INDEX allocation_charge ON entry_allocations(charge_id);
CREATE TABLE allocation_reversals (
    allocation_id TEXT PRIMARY KEY REFERENCES entry_allocations(id),
    actor_id TEXT NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;
CREATE VIEW live_entry_allocations AS
    SELECT a.* FROM entry_allocations a
    JOIN entries source ON source.id=a.source_id AND source.state='POSTED' AND source.kind IN ('RECEIVED','OPENING_CREDIT')
    JOIN entries charge ON charge.id=a.charge_id AND charge.state='POSTED' AND charge.kind IN ('CHARGE','OPENING_DEBIT') AND charge.flat_id=source.flat_id
    WHERE NOT EXISTS(SELECT 1 FROM allocation_reversals v WHERE v.allocation_id=a.id)
      AND NOT EXISTS(SELECT 1 FROM entry_reversals v WHERE v.entry_id IN (a.source_id,a.charge_id));
CREATE TRIGGER allocation_insert_valid BEFORE INSERT ON entry_allocations
WHEN NOT EXISTS(SELECT 1 FROM entries source JOIN entries charge ON charge.id=NEW.charge_id
    WHERE source.id=NEW.source_id AND source.state='POSTED' AND charge.state='POSTED'
      AND source.flat_id=charge.flat_id AND source.kind IN ('RECEIVED','OPENING_CREDIT') AND charge.kind IN ('CHARGE','OPENING_DEBIT')
      AND NOT EXISTS(SELECT 1 FROM entry_reversals v WHERE v.entry_id IN (source.id,charge.id))
      AND NEW.amount_paise+COALESCE((SELECT SUM(amount_paise) FROM live_entry_allocations WHERE source_id=source.id),0)<=source.amount_paise
      AND NEW.amount_paise+COALESCE((SELECT SUM(amount_paise) FROM live_entry_allocations WHERE charge_id=charge.id),0)<=charge.amount_paise)
BEGIN SELECT RAISE(ABORT,'allocation exceeds a current same-home credit or charge'); END;
CREATE TRIGGER allocation_update BEFORE UPDATE ON entry_allocations
BEGIN SELECT RAISE(ABORT,'allocation history is immutable'); END;
CREATE TRIGGER allocation_delete BEFORE DELETE ON entry_allocations
BEGIN SELECT RAISE(ABORT,'allocation history is preserved'); END;
CREATE TRIGGER allocation_reversal_update BEFORE UPDATE ON allocation_reversals
BEGIN SELECT RAISE(ABORT,'allocation corrections are immutable'); END;
CREATE TRIGGER allocation_reversal_delete BEFORE DELETE ON allocation_reversals
BEGIN SELECT RAISE(ABORT,'allocation corrections are preserved'); END;
