CREATE TABLE offsite_policy (
 id INTEGER PRIMARY KEY CHECK(id=1),
 config_hash TEXT NOT NULL UNIQUE CHECK(length(config_hash)=64),
 storage_mode TEXT NOT NULL CHECK(storage_mode IN('FIXTURE','S3')),
 configured_at INTEGER NOT NULL CHECK(configured_at>0),
 highest_clock_at INTEGER NOT NULL CHECK(highest_clock_at>=configured_at)
) STRICT;
CREATE TRIGGER preserve_offsite_policy_update BEFORE UPDATE ON offsite_policy
WHEN NEW.id!=OLD.id OR NEW.config_hash!=OLD.config_hash OR NEW.storage_mode!=OLD.storage_mode OR
     NEW.configured_at!=OLD.configured_at OR NEW.highest_clock_at<OLD.highest_clock_at
BEGIN SELECT RAISE(ABORT,'offsite policy binding is immutable'); END;
CREATE TRIGGER preserve_offsite_policy_delete BEFORE DELETE ON offsite_policy
BEGIN SELECT RAISE(ABORT,'offsite protection cannot be silently disabled'); END;
CREATE TABLE offsite_backup_jobs (
 id TEXT PRIMARY KEY,
 object_key TEXT NOT NULL UNIQUE,
 config_hash TEXT NOT NULL REFERENCES offsite_policy(config_hash),
 captured_at INTEGER NOT NULL CHECK(captured_at>0),
 state TEXT NOT NULL CHECK(state IN('CAPTURING','SEALED','UPLOADING','VERIFIED','FAILED','UNCERTAIN')),
 attempts INTEGER NOT NULL CHECK(attempts BETWEEN 1 AND 5),
 available_at INTEGER NOT NULL,
 lease_token TEXT NOT NULL,
 lease_until INTEGER NOT NULL,
 encrypted_sha256 TEXT NOT NULL DEFAULT '' CHECK(length(encrypted_sha256) IN(0,64)),
 encrypted_bytes INTEGER NOT NULL DEFAULT 0 CHECK(encrypted_bytes BETWEEN 0 AND 268435456),
 object_version TEXT NOT NULL DEFAULT '' CHECK(length(object_version)<=1024),
 acknowledged_at INTEGER NOT NULL DEFAULT 0,
 error_code TEXT NOT NULL DEFAULT '' CHECK(error_code IN('','CAPTURE_FAILED','UPLOAD_DENIED','UPLOAD_REJECTED','UPLOAD_UNCERTAIN','LEASE_EXPIRED','STAGING_FULL')),
 CHECK((state='VERIFIED')=(object_version!='' AND acknowledged_at>=captured_at)),
 CHECK(state NOT IN('SEALED','UPLOADING','VERIFIED') OR (length(encrypted_sha256)=64 AND encrypted_bytes>0)),
 CHECK(state!='VERIFIED' OR (lease_token='' AND lease_until=0 AND error_code=''))
) STRICT;
CREATE INDEX offsite_backup_schedule ON offsite_backup_jobs(state,available_at,captured_at);
CREATE TRIGGER preserve_verified_offsite_job BEFORE UPDATE ON offsite_backup_jobs
WHEN OLD.state='VERIFIED' OR NEW.id!=OLD.id OR NEW.object_key!=OLD.object_key OR
     NEW.config_hash!=OLD.config_hash OR NEW.captured_at!=OLD.captured_at OR NEW.attempts<OLD.attempts OR
     (OLD.encrypted_sha256!='' AND (NEW.encrypted_sha256!=OLD.encrypted_sha256 OR NEW.encrypted_bytes!=OLD.encrypted_bytes))
BEGIN SELECT RAISE(ABORT,'offsite snapshot and acknowledged original are preserved'); END;
CREATE TRIGGER preserve_offsite_job_delete BEFORE DELETE ON offsite_backup_jobs
BEGIN SELECT RAISE(ABORT,'offsite attempt history is preserved'); END;
CREATE TABLE offsite_backup_events (
 id INTEGER PRIMARY KEY,
 job_id TEXT REFERENCES offsite_backup_jobs(id),
 action TEXT NOT NULL CHECK(action IN('CONFIGURED','CLAIMED','SEALED','VERIFIED','FAILED','UNCERTAIN','RECONCILED','CLOCK_ERROR','STAGING_FULL')),
 actor_kind TEXT NOT NULL CHECK(actor_kind IN('SYSTEM','OFFLINE_CUSTODIAN')),
 note TEXT NOT NULL CHECK(length(note)<=300),
 occurred_at INTEGER NOT NULL
) STRICT;
CREATE INDEX offsite_backup_history ON offsite_backup_events(id DESC);
CREATE TRIGGER preserve_offsite_event_update BEFORE UPDATE ON offsite_backup_events
BEGIN SELECT RAISE(ABORT,'offsite history is immutable'); END;
CREATE TRIGGER preserve_offsite_event_delete BEFORE DELETE ON offsite_backup_events
BEGIN SELECT RAISE(ABORT,'offsite history is preserved'); END;
CREATE VIEW offsite_financial_protection AS SELECT CASE
 WHEN NOT EXISTS(SELECT 1 FROM offsite_policy) THEN CASE
   WHEN COALESCE((SELECT value FROM app_metadata WHERE key='data_kind'),'')='synthetic'
     AND NOT EXISTS(SELECT 1 FROM workspace_setup WHERE mode='LOCAL_WORKSPACE') THEN 1
   ELSE 0 END
 WHEN (COALESCE((SELECT value FROM app_metadata WHERE key='data_kind'),'')!='synthetic'
       OR EXISTS(SELECT 1 FROM workspace_setup WHERE mode='LOCAL_WORKSPACE'))
       AND (SELECT storage_mode FROM offsite_policy WHERE id=1)!='S3' THEN 0
 WHEN (SELECT highest_clock_at FROM offsite_policy WHERE id=1)>unixepoch('now') THEN 0
 WHEN EXISTS(SELECT 1 FROM offsite_backup_jobs j JOIN offsite_policy p ON p.config_hash=j.config_hash
   WHERE j.state='VERIFIED' AND j.captured_at BETWEEN unixepoch('now')-3599 AND unixepoch('now')
     AND j.acknowledged_at<=unixepoch('now')) THEN 1
 ELSE 0 END AS allowed;

CREATE TRIGGER protect_offsite_entries_insert BEFORE INSERT ON entries WHEN NEW.state='POSTED'
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_entries_post BEFORE UPDATE ON entries WHEN NEW.state='POSTED' AND OLD.state!='POSTED'
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_entry_reversal BEFORE INSERT ON entry_reversals WHEN 1
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_receipt_counter_insert BEFORE INSERT ON receipt_counter WHEN 1
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_receipt_counter_update BEFORE UPDATE ON receipt_counter WHEN 1
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_receipt_insert BEFORE INSERT ON receipts WHEN 1
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_allocation_insert BEFORE INSERT ON entry_allocations WHEN 1
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_allocation_reverse BEFORE INSERT ON allocation_reversals WHEN 1
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_fund_attribution BEFORE INSERT ON fund_contributions WHEN 1
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_fund_attribution_reverse BEFORE INSERT ON fund_contribution_reversals WHEN 1
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

CREATE TRIGGER protect_offsite_paid_expense_approval BEFORE UPDATE ON paid_expense_resources WHEN NEW.approved_version IS NOT OLD.approved_version
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;

-- A new authoritative payment verification also matters when it links an
-- existing receipt without creating another amount or allocation.
CREATE TRIGGER protect_offsite_payment_identity BEFORE INSERT ON verified_fund_payments
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;
CREATE TRIGGER protect_offsite_fund_report BEFORE UPDATE ON fund_reports
WHEN NEW.state='CONFIRMED' AND OLD.state!='CONFIRMED'
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;
CREATE TRIGGER protect_offsite_fine_report BEFORE UPDATE ON fine_reports
WHEN NEW.state='CONFIRMED' AND OLD.state!='CONFIRMED'
BEGIN
 SELECT CASE WHEN (SELECT allowed FROM offsite_financial_protection)!=1 THEN RAISE(ABORT,'society_backup_protection_required') END;
 UPDATE offsite_policy SET highest_clock_at=MAX(highest_clock_at,unixepoch('now')) WHERE id=1;
END;
