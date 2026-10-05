CREATE TABLE fund_campaigns (
 id TEXT PRIMARY KEY, title TEXT NOT NULL, purpose TEXT NOT NULL, contribution_type TEXT NOT NULL CHECK(contribution_type IN ('FIXED','VOLUNTARY')),
 start_date TEXT NOT NULL, due_date TEXT NOT NULL CHECK(due_date>=start_date), target_paise INTEGER NOT NULL CHECK(target_paise BETWEEN 0 AND 1000000000),
 source_reference TEXT NOT NULL, note TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('PENDING','PUBLISHED','CLOSED','DECLINED','WITHDRAWN')),
 version INTEGER NOT NULL CHECK(version>0), author_id TEXT NOT NULL REFERENCES users(id), created_at INTEGER NOT NULL,
 decided_by TEXT REFERENCES users(id), decided_at INTEGER, decision_reason TEXT NOT NULL DEFAULT '',
 CHECK(state='PENDING' OR (decided_by IS NOT NULL AND decided_at IS NOT NULL)), CHECK(state NOT IN ('PUBLISHED','CLOSED','DECLINED') OR decided_by!=author_id)
) STRICT;
CREATE INDEX fund_campaign_dates ON fund_campaigns(state,due_date);
CREATE TABLE fund_participants (
 campaign_id TEXT NOT NULL REFERENCES fund_campaigns(id), flat_id TEXT NOT NULL REFERENCES flats(id), requested_paise INTEGER NOT NULL CHECK(requested_paise BETWEEN 0 AND 1000000000),
 original_entry_id TEXT UNIQUE REFERENCES entries(id), current_entry_id TEXT UNIQUE REFERENCES entries(id), waived_paise INTEGER NOT NULL DEFAULT 0 CHECK(waived_paise>=0 AND waived_paise<=requested_paise), version INTEGER NOT NULL DEFAULT 1 CHECK(version>0),
 PRIMARY KEY(campaign_id,flat_id)
) STRICT;
CREATE INDEX fund_participant_home ON fund_participants(flat_id,campaign_id);
CREATE TABLE fund_reports (
 id TEXT PRIMARY KEY, campaign_id TEXT NOT NULL, flat_id TEXT NOT NULL, author_id TEXT NOT NULL REFERENCES users(id),
 amount_paise INTEGER NOT NULL CHECK(amount_paise>0 AND amount_paise<=1000000000), payment_date TEXT NOT NULL, payer TEXT NOT NULL,
 method TEXT NOT NULL CHECK(method IN ('CASH','BANK_TRANSFER','CHEQUE','UPI')), reference TEXT NOT NULL, comment TEXT NOT NULL,
 evidence_id TEXT REFERENCES library_documents(id), state TEXT NOT NULL CHECK(state IN ('PENDING','NEEDS_INFO','REJECTED','WITHDRAWN','CONFIRMED','DUPLICATE')),
 version INTEGER NOT NULL CHECK(version>0), created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 entry_id TEXT REFERENCES entries(id), allocation_id TEXT REFERENCES entry_allocations(id), duplicate_report_id TEXT REFERENCES fund_reports(id),
 reviewer_id TEXT REFERENCES users(id), reviewed_at INTEGER, decision_reason TEXT NOT NULL DEFAULT '',
 FOREIGN KEY(campaign_id,flat_id) REFERENCES fund_participants(campaign_id,flat_id),
 CHECK(state NOT IN ('CONFIRMED','DUPLICATE') OR (entry_id IS NOT NULL AND reviewer_id IS NOT NULL AND reviewer_id!=author_id AND reviewed_at IS NOT NULL)),
 CHECK(state!='DUPLICATE' OR duplicate_report_id IS NOT NULL)
) STRICT;
CREATE INDEX fund_report_queue ON fund_reports(state,created_at);
CREATE INDEX fund_report_scope ON fund_reports(author_id,flat_id,campaign_id);
CREATE TABLE verified_fund_payments (
 fingerprint TEXT PRIMARY KEY CHECK(length(fingerprint)=64), entry_id TEXT NOT NULL UNIQUE REFERENCES entries(id),
 verification_source TEXT NOT NULL, payment_identity TEXT NOT NULL, reviewer_id TEXT NOT NULL REFERENCES users(id), verified_at INTEGER NOT NULL
) STRICT;
CREATE TABLE fund_contributions (
 id TEXT PRIMARY KEY, campaign_id TEXT NOT NULL, flat_id TEXT NOT NULL, source_id TEXT NOT NULL REFERENCES entries(id),
 amount_paise INTEGER NOT NULL CHECK(amount_paise>0 AND amount_paise<=1000000000), actor_id TEXT NOT NULL REFERENCES users(id), reason TEXT NOT NULL, created_at INTEGER NOT NULL,
 FOREIGN KEY(campaign_id,flat_id) REFERENCES fund_participants(campaign_id,flat_id)
) STRICT;
CREATE TABLE fund_contribution_reversals (
 contribution_id TEXT PRIMARY KEY REFERENCES fund_contributions(id), actor_id TEXT NOT NULL REFERENCES users(id), reason TEXT NOT NULL, created_at INTEGER NOT NULL
) STRICT;
CREATE VIEW live_fund_contributions AS SELECT c.* FROM fund_contributions c JOIN entries e ON e.id=c.source_id
 WHERE e.state='POSTED' AND e.kind='RECEIVED' AND e.flat_id=c.flat_id AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id)
 AND NOT EXISTS(SELECT 1 FROM fund_contribution_reversals r WHERE r.contribution_id=c.id);
CREATE VIEW live_credit_uses AS SELECT source_id,amount_paise FROM live_entry_allocations UNION ALL SELECT source_id,amount_paise FROM live_fund_contributions;
DROP TRIGGER allocation_insert_valid;
CREATE TRIGGER allocation_insert_valid BEFORE INSERT ON entry_allocations
WHEN NOT EXISTS(SELECT 1 FROM entries source JOIN entries charge ON charge.id=NEW.charge_id
 WHERE source.id=NEW.source_id AND source.state='POSTED' AND charge.state='POSTED' AND source.flat_id=charge.flat_id
 AND source.kind IN ('RECEIVED','OPENING_CREDIT') AND charge.kind IN ('CHARGE','OPENING_DEBIT')
 AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id IN (source.id,charge.id))
 AND NEW.amount_paise+COALESCE((SELECT SUM(amount_paise) FROM live_credit_uses WHERE source_id=source.id),0)<=source.amount_paise
 AND NEW.amount_paise+COALESCE((SELECT SUM(amount_paise) FROM live_entry_allocations WHERE charge_id=charge.id),0)<=charge.amount_paise)
BEGIN SELECT RAISE(ABORT,'allocation exceeds a current same-home credit or charge'); END;
CREATE TRIGGER fund_contribution_insert BEFORE INSERT ON fund_contributions
WHEN NOT EXISTS(SELECT 1 FROM fund_campaigns f JOIN entries e ON e.id=NEW.source_id WHERE f.id=NEW.campaign_id AND f.contribution_type='VOLUNTARY' AND f.state IN ('PUBLISHED','CLOSED')
 AND e.flat_id=NEW.flat_id AND e.kind='RECEIVED' AND e.state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id)
 AND NEW.amount_paise+COALESCE((SELECT SUM(amount_paise) FROM live_credit_uses WHERE source_id=e.id),0)<=e.amount_paise)
BEGIN SELECT RAISE(ABORT,'voluntary attribution exceeds current confirmed credit'); END;
CREATE TABLE fund_waivers (
 id TEXT PRIMARY KEY, campaign_id TEXT NOT NULL, flat_id TEXT NOT NULL, participant_version INTEGER NOT NULL CHECK(participant_version>0),
 amount_paise INTEGER NOT NULL CHECK(amount_paise>0 AND amount_paise<=1000000000), source_reference TEXT NOT NULL, reason TEXT NOT NULL,
 author_id TEXT NOT NULL REFERENCES users(id), state TEXT NOT NULL CHECK(state IN ('PENDING','APPROVED','DECLINED','WITHDRAWN')), version INTEGER NOT NULL CHECK(version>0),
 created_at INTEGER NOT NULL, reviewer_id TEXT REFERENCES users(id), reviewed_at INTEGER, decision_reason TEXT NOT NULL DEFAULT '', replacement_entry_id TEXT REFERENCES entries(id),
 FOREIGN KEY(campaign_id,flat_id) REFERENCES fund_participants(campaign_id,flat_id), CHECK(state NOT IN ('APPROVED','DECLINED') OR (reviewer_id IS NOT NULL AND reviewer_id!=author_id))
) STRICT;
CREATE INDEX fund_waiver_queue ON fund_waivers(state,campaign_id,flat_id);
CREATE TABLE fund_events (
 id INTEGER PRIMARY KEY, campaign_id TEXT NOT NULL REFERENCES fund_campaigns(id), subject_type TEXT NOT NULL CHECK(subject_type IN ('CAMPAIGN','REPORT','WAIVER','CONTRIBUTION')),
 subject_id TEXT NOT NULL, version INTEGER NOT NULL, action TEXT NOT NULL, actor_id TEXT NOT NULL REFERENCES users(id), reason TEXT NOT NULL, snapshot_json TEXT NOT NULL,
 occurred_at INTEGER NOT NULL, UNIQUE(subject_type,subject_id,version)
) STRICT;
CREATE INDEX fund_event_history ON fund_events(subject_type,subject_id,id DESC);
CREATE TRIGGER fund_campaign_insert BEFORE INSERT ON fund_campaigns WHEN NEW.state!='PENDING' OR NEW.version!=1 OR NEW.decided_by IS NOT NULL
BEGIN SELECT RAISE(ABORT,'campaign starts as a frozen proposal'); END;
CREATE TRIGGER fund_campaign_update BEFORE UPDATE ON fund_campaigns
WHEN NEW.id!=OLD.id OR NEW.title!=OLD.title OR NEW.purpose!=OLD.purpose OR NEW.contribution_type!=OLD.contribution_type OR NEW.start_date!=OLD.start_date OR NEW.due_date!=OLD.due_date
 OR NEW.target_paise!=OLD.target_paise OR NEW.source_reference!=OLD.source_reference OR NEW.note!=OLD.note OR NEW.author_id!=OLD.author_id OR NEW.created_at!=OLD.created_at OR NEW.version!=OLD.version+1
 OR NOT ((OLD.state='PENDING' AND NEW.state IN ('PUBLISHED','DECLINED','WITHDRAWN')) OR (OLD.state='PUBLISHED' AND NEW.state='CLOSED') OR (OLD.state='CLOSED' AND NEW.state='PUBLISHED'))
 OR (OLD.state!='PENDING' AND (NEW.decided_by!=OLD.decided_by OR NEW.decided_at!=OLD.decided_at OR NEW.decision_reason!=OLD.decision_reason))
BEGIN SELECT RAISE(ABORT,'campaign decisions preserve frozen identity and history'); END;
CREATE TRIGGER fund_participant_insert BEFORE INSERT ON fund_participants
WHEN NEW.original_entry_id IS NOT NULL OR NEW.current_entry_id IS NOT NULL OR NEW.waived_paise!=0 OR NEW.version!=1
 OR (SELECT state FROM fund_campaigns WHERE id=NEW.campaign_id)!='PENDING' OR EXISTS(SELECT 1 FROM fund_events WHERE subject_type='CAMPAIGN' AND subject_id=NEW.campaign_id)
 OR ((SELECT contribution_type FROM fund_campaigns WHERE id=NEW.campaign_id)='FIXED')!=(NEW.requested_paise>0)
BEGIN SELECT RAISE(ABORT,'campaign participation is frozen before submission'); END;
CREATE TRIGGER fund_publish_complete BEFORE UPDATE ON fund_campaigns WHEN OLD.state='PENDING' AND NEW.state='PUBLISHED'
 AND (NOT EXISTS(SELECT 1 FROM fund_participants WHERE campaign_id=OLD.id) OR (OLD.contribution_type='FIXED' AND EXISTS(SELECT 1 FROM fund_participants WHERE campaign_id=OLD.id AND current_entry_id IS NULL)))
BEGIN SELECT RAISE(ABORT,'publication requires all reviewed home charges'); END;
CREATE TRIGGER fund_participant_update BEFORE UPDATE ON fund_participants
WHEN NEW.campaign_id!=OLD.campaign_id OR NEW.flat_id!=OLD.flat_id OR NEW.requested_paise!=OLD.requested_paise
 OR NOT ((OLD.original_entry_id IS NULL AND NEW.original_entry_id=NEW.current_entry_id AND NEW.waived_paise=0 AND NEW.version=1
 AND (SELECT state FROM fund_campaigns WHERE id=OLD.campaign_id)='PENDING' AND EXISTS(SELECT 1 FROM entries e WHERE e.id=NEW.current_entry_id AND e.flat_id=OLD.flat_id AND e.kind='CHARGE' AND e.state='POSTED' AND e.amount_paise=OLD.requested_paise))
 OR (NEW.original_entry_id=OLD.original_entry_id AND NEW.version=OLD.version+1 AND NEW.waived_paise>OLD.waived_paise
 AND EXISTS(SELECT 1 FROM fund_waivers w WHERE w.campaign_id=OLD.campaign_id AND w.flat_id=OLD.flat_id AND w.state='PENDING' AND w.participant_version=OLD.version AND w.amount_paise=NEW.waived_paise-OLD.waived_paise)
 AND EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=OLD.current_entry_id)
 AND ((NEW.waived_paise=OLD.requested_paise AND NEW.current_entry_id IS NULL) OR EXISTS(SELECT 1 FROM entries e WHERE e.id=NEW.current_entry_id AND e.flat_id=OLD.flat_id AND e.kind='CHARGE' AND e.state='POSTED' AND e.amount_paise=OLD.requested_paise-NEW.waived_paise))))
BEGIN SELECT RAISE(ABORT,'participant correction requires an explicit reviewed waiver'); END;
CREATE TRIGGER fund_report_insert BEFORE INSERT ON fund_reports
WHEN NEW.state!='PENDING' OR NEW.version!=1 OR NEW.entry_id IS NOT NULL OR NEW.reviewer_id IS NOT NULL OR (SELECT state FROM fund_campaigns WHERE id=NEW.campaign_id)!='PUBLISHED'
BEGIN SELECT RAISE(ABORT,'payment report starts as an unconfirmed claim'); END;
CREATE TRIGGER fund_report_update BEFORE UPDATE ON fund_reports
WHEN NEW.id!=OLD.id OR NEW.campaign_id!=OLD.campaign_id OR NEW.flat_id!=OLD.flat_id OR NEW.author_id!=OLD.author_id OR NEW.created_at!=OLD.created_at OR NEW.version!=OLD.version+1
 OR NOT ((OLD.state='PENDING' AND NEW.state IN ('NEEDS_INFO','REJECTED','WITHDRAWN','CONFIRMED','DUPLICATE')) OR (OLD.state IN ('NEEDS_INFO','REJECTED') AND NEW.state IN ('PENDING','WITHDRAWN')))
 OR (NEW.state!='PENDING' AND (NEW.amount_paise!=OLD.amount_paise OR NEW.payment_date!=OLD.payment_date OR NEW.payer!=OLD.payer OR NEW.method!=OLD.method OR NEW.reference!=OLD.reference OR NEW.comment!=OLD.comment OR NEW.evidence_id IS NOT OLD.evidence_id))
BEGIN SELECT RAISE(ABORT,'report revisions and decisions retain original history'); END;
CREATE TRIGGER fund_report_confirm BEFORE UPDATE ON fund_reports WHEN NEW.state IN ('CONFIRMED','DUPLICATE')
 AND NOT EXISTS(SELECT 1 FROM entries e JOIN receipts r ON r.entry_id=e.id JOIN verified_fund_payments v ON v.entry_id=e.id
 WHERE e.id=NEW.entry_id AND e.flat_id=OLD.flat_id AND e.kind='RECEIVED' AND e.state='POSTED' AND e.amount_paise=OLD.amount_paise AND e.entry_date=OLD.payment_date AND e.method=OLD.method
 AND NOT EXISTS(SELECT 1 FROM entry_reversals x WHERE x.entry_id=e.id))
BEGIN SELECT RAISE(ABORT,'confirmation requires the compatible verified original receipt'); END;
CREATE TRIGGER fund_waiver_insert BEFORE INSERT ON fund_waivers WHEN NEW.state!='PENDING' OR NEW.version!=1 OR NEW.reviewer_id IS NOT NULL
BEGIN SELECT RAISE(ABORT,'waiver starts as a separate review proposal'); END;
CREATE TRIGGER fund_waiver_update BEFORE UPDATE ON fund_waivers
WHEN OLD.state!='PENDING' OR NEW.state='PENDING' OR NEW.version!=OLD.version+1 OR NEW.id!=OLD.id OR NEW.campaign_id!=OLD.campaign_id OR NEW.flat_id!=OLD.flat_id
 OR NEW.participant_version!=OLD.participant_version OR NEW.amount_paise!=OLD.amount_paise OR NEW.source_reference!=OLD.source_reference OR NEW.reason!=OLD.reason OR NEW.author_id!=OLD.author_id OR NEW.created_at!=OLD.created_at
 OR (NEW.state='WITHDRAWN' AND NEW.reviewer_id!=OLD.author_id)
BEGIN SELECT RAISE(ABORT,'waiver proposal and decision are immutable'); END;
CREATE TRIGGER fund_campaign_delete BEFORE DELETE ON fund_campaigns BEGIN SELECT RAISE(ABORT,'campaign history is preserved'); END;
CREATE TRIGGER fund_participant_delete BEFORE DELETE ON fund_participants BEGIN SELECT RAISE(ABORT,'participation history is preserved'); END;
CREATE TRIGGER fund_report_delete BEFORE DELETE ON fund_reports BEGIN SELECT RAISE(ABORT,'claim history is preserved'); END;
CREATE TRIGGER fund_waiver_delete BEFORE DELETE ON fund_waivers BEGIN SELECT RAISE(ABORT,'waiver history is preserved'); END;
CREATE TRIGGER fund_event_update BEFORE UPDATE ON fund_events BEGIN SELECT RAISE(ABORT,'fund activity is immutable'); END;
CREATE TRIGGER fund_event_delete BEFORE DELETE ON fund_events BEGIN SELECT RAISE(ABORT,'fund activity is preserved'); END;
CREATE TRIGGER verified_fund_payment_update BEFORE UPDATE ON verified_fund_payments BEGIN SELECT RAISE(ABORT,'external verification identity is immutable'); END;
CREATE TRIGGER verified_fund_payment_delete BEFORE DELETE ON verified_fund_payments BEGIN SELECT RAISE(ABORT,'external verification identity is preserved'); END;
CREATE TRIGGER fund_contribution_update BEFORE UPDATE ON fund_contributions BEGIN SELECT RAISE(ABORT,'contribution attribution is immutable'); END;
CREATE TRIGGER fund_contribution_delete BEFORE DELETE ON fund_contributions BEGIN SELECT RAISE(ABORT,'contribution attribution is preserved'); END;
CREATE TRIGGER fund_contribution_reversal_update BEFORE UPDATE ON fund_contribution_reversals BEGIN SELECT RAISE(ABORT,'contribution correction is immutable'); END;
CREATE TRIGGER fund_contribution_reversal_delete BEFORE DELETE ON fund_contribution_reversals BEGIN SELECT RAISE(ABORT,'contribution correction is preserved'); END;
