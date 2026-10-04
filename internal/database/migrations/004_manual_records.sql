CREATE TABLE record_operations (
    actor_id TEXT NOT NULL REFERENCES users(id),
    operation_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY(actor_id, operation_key)
) STRICT;
CREATE TABLE entries (
    id TEXT PRIMARY KEY,
    flat_id TEXT NOT NULL REFERENCES flats(id),
    kind TEXT NOT NULL CHECK(kind IN ('CHARGE','OPENING_DEBIT','OPENING_CREDIT','RECEIVED')),
    amount_paise INTEGER NOT NULL CHECK(amount_paise > 0 AND amount_paise <= 1000000000),
    entry_date TEXT NOT NULL,
    description TEXT NOT NULL,
    payer TEXT NOT NULL,
    method TEXT NOT NULL,
    reference TEXT NOT NULL,
    source_note TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('DRAFT','POSTED')),
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at INTEGER NOT NULL,
    posted_by TEXT REFERENCES users(id),
    posted_at INTEGER
) STRICT;
CREATE INDEX entries_flat ON entries(flat_id,created_at);
CREATE TRIGGER preserve_posted_entry BEFORE UPDATE ON entries WHEN OLD.state = 'POSTED'
BEGIN SELECT RAISE(ABORT, 'posted entries are immutable'); END;
CREATE TRIGGER preserve_entry_delete BEFORE DELETE ON entries
BEGIN SELECT RAISE(ABORT, 'entry history is preserved'); END;
CREATE TABLE draft_discards (
    entry_id TEXT PRIMARY KEY REFERENCES entries(id),
    reason TEXT NOT NULL,
    actor_id TEXT NOT NULL REFERENCES users(id),
    created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER preserve_discard_update BEFORE UPDATE ON draft_discards BEGIN SELECT RAISE(ABORT,'immutable draft history'); END;
CREATE TRIGGER preserve_discard_delete BEFORE DELETE ON draft_discards BEGIN SELECT RAISE(ABORT,'immutable draft history'); END;
CREATE TRIGGER preserve_discarded_entry BEFORE UPDATE ON entries WHEN EXISTS(SELECT 1 FROM draft_discards WHERE entry_id=OLD.id)
BEGIN SELECT RAISE(ABORT,'discarded drafts are preserved'); END;
CREATE TABLE entry_reversals (
    entry_id TEXT PRIMARY KEY REFERENCES entries(id),
    reason TEXT NOT NULL,
    actor_id TEXT NOT NULL REFERENCES users(id),
    created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER preserve_reversal_update BEFORE UPDATE ON entry_reversals BEGIN SELECT RAISE(ABORT,'immutable correction'); END;
CREATE TRIGGER preserve_reversal_delete BEFORE DELETE ON entry_reversals BEGIN SELECT RAISE(ABORT,'immutable correction'); END;
CREATE TABLE receipt_counter (year TEXT PRIMARY KEY, next_number INTEGER NOT NULL) STRICT;
CREATE TABLE receipts (
    id TEXT PRIMARY KEY,
    entry_id TEXT NOT NULL UNIQUE REFERENCES entries(id),
    number TEXT NOT NULL UNIQUE,
    snapshot_json TEXT NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;
CREATE TRIGGER preserve_receipt_update BEFORE UPDATE ON receipts BEGIN SELECT RAISE(ABORT,'immutable receipt'); END;
CREATE TRIGGER preserve_receipt_delete BEFORE DELETE ON receipts BEGIN SELECT RAISE(ABORT,'immutable receipt'); END;
CREATE TABLE receipt_jobs (
    receipt_id TEXT PRIMARY KEY REFERENCES receipts(id),
    state TEXT NOT NULL CHECK(state IN ('PENDING','RUNNING','READY','FAILED')),
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at INTEGER NOT NULL,
    lease_until INTEGER NOT NULL DEFAULT 0,
    lease_token TEXT NOT NULL DEFAULT '',
    file_hash TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX receipt_job_queue ON receipt_jobs(state,available_at);
