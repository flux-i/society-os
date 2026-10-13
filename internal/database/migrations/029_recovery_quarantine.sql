CREATE TABLE recovery_epochs (
 id TEXT PRIMARY KEY CHECK(length(id)=43 AND id NOT GLOB '*[^A-Za-z0-9_-]*'),
 format_version INTEGER NOT NULL CHECK(format_version=1),
 source_sha256 TEXT NOT NULL CHECK(length(source_sha256)=64 AND source_sha256 NOT GLOB '*[^a-f0-9]*'),
 source_bytes INTEGER NOT NULL CHECK(source_bytes BETWEEN 1 AND 536870912),
 source_schema INTEGER NOT NULL CHECK(source_schema>0),
 source_application TEXT NOT NULL CHECK(length(source_application) BETWEEN 1 AND 80),
 source_created_at INTEGER NOT NULL CHECK(source_created_at>0),
 restored_at INTEGER NOT NULL CHECK(restored_at>0)
) STRICT;
CREATE TRIGGER preserve_recovery_epoch_update BEFORE UPDATE ON recovery_epochs
BEGIN SELECT RAISE(ABORT,'recovery origins are immutable'); END;
CREATE TRIGGER preserve_recovery_epoch_delete BEFORE DELETE ON recovery_epochs
BEGIN SELECT RAISE(ABORT,'recovery origins are retained'); END;

CREATE TABLE recovery_external_jobs (
 epoch_id TEXT NOT NULL REFERENCES recovery_epochs(id),
 job_kind TEXT NOT NULL CHECK(job_kind IN('OFFSITE','MESSAGE')),
 job_id TEXT NOT NULL,
 original_state TEXT NOT NULL,
 disposition TEXT NOT NULL CHECK(disposition='WITHHELD'),
 PRIMARY KEY(epoch_id,job_kind,job_id)
) STRICT;
CREATE TRIGGER verify_recovery_external_job BEFORE INSERT ON recovery_external_jobs
WHEN NOT (
 (NEW.job_kind='OFFSITE' AND EXISTS(SELECT 1 FROM offsite_backup_jobs WHERE id=NEW.job_id AND state=NEW.original_state AND state!='VERIFIED')) OR
 (NEW.job_kind='MESSAGE' AND EXISTS(SELECT 1 FROM message_deliveries WHERE id=NEW.job_id AND state=NEW.original_state AND state IN('QUEUED','CLAIMED','FAILED','UNKNOWN')))
)
BEGIN SELECT RAISE(ABORT,'recovery dispositions must match retained unresolved originals'); END;
CREATE TRIGGER preserve_recovery_external_update BEFORE UPDATE ON recovery_external_jobs
BEGIN SELECT RAISE(ABORT,'recovery dispositions are immutable'); END;
CREATE TRIGGER preserve_recovery_external_delete BEFORE DELETE ON recovery_external_jobs
BEGIN SELECT RAISE(ABORT,'recovery dispositions are retained'); END;

-- Entry into recovery does not establish current society authority. A later
-- reviewed release will be a separate retained decision, never an epoch edit.
CREATE VIEW recovery_quarantine AS SELECT EXISTS(SELECT 1 FROM recovery_epochs) AS required;

CREATE TRIGGER quarantine_offsite_update BEFORE UPDATE ON offsite_backup_jobs
WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_offsite_insert BEFORE INSERT ON offsite_backup_jobs
WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_message_attempt BEFORE INSERT ON message_attempts
WHEN (SELECT required FROM recovery_quarantine)=1 OR EXISTS(
 SELECT 1 FROM recovery_external_jobs WHERE job_kind='MESSAGE' AND job_id=NEW.delivery_id)
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_whatsapp_handoff BEFORE INSERT ON whatsapp_handoffs
WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_simulation_handoff BEFORE INSERT ON simulation_messages
WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;

-- These additional guards preserve all preceding SQL objects and independently
-- cover money effects even when a copied backup acknowledgement is fresh.
CREATE TRIGGER quarantine_entries_insert BEFORE INSERT ON entries WHEN NEW.state='POSTED' AND (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_entries_post BEFORE UPDATE ON entries WHEN NEW.state='POSTED' AND OLD.state!='POSTED' AND (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_entry_reversal BEFORE INSERT ON entry_reversals WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_receipt_counter_insert BEFORE INSERT ON receipt_counter WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_receipt_counter_update BEFORE UPDATE ON receipt_counter WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_receipt_insert BEFORE INSERT ON receipts WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_allocation_insert BEFORE INSERT ON entry_allocations WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_allocation_reverse BEFORE INSERT ON allocation_reversals WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_fund_attribution BEFORE INSERT ON fund_contributions WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_fund_attribution_reverse BEFORE INSERT ON fund_contribution_reversals WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_paid_expense BEFORE UPDATE ON paid_expense_resources WHEN NEW.approved_version IS NOT OLD.approved_version AND (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_payment_identity BEFORE INSERT ON verified_fund_payments WHEN (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_fund_report BEFORE UPDATE ON fund_reports WHEN NEW.state='CONFIRMED' AND OLD.state!='CONFIRMED' AND (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
CREATE TRIGGER quarantine_fine_report BEFORE UPDATE ON fine_reports WHEN NEW.state='CONFIRMED' AND OLD.state!='CONFIRMED' AND (SELECT required FROM recovery_quarantine)=1
BEGIN SELECT RAISE(ABORT,'society_recovery_review_required'); END;
