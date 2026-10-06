CREATE TABLE fines (
 id TEXT PRIMARY KEY, incident_id TEXT NOT NULL REFERENCES incidents(id), replaces_id TEXT REFERENCES fines(id),
 flat_id TEXT NOT NULL REFERENCES flats(id), rule_id TEXT NOT NULL REFERENCES society_rules(id),
 material_key TEXT NOT NULL CHECK(length(material_key)=64), source_key TEXT NOT NULL CHECK(length(source_key)=64), source_json TEXT NOT NULL CHECK(json_valid(source_json)),
 title TEXT NOT NULL, amount_paise INTEGER NOT NULL CHECK(amount_paise BETWEEN 1 AND 1000000000),
 policy_reference TEXT NOT NULL, reason TEXT NOT NULL, due_date TEXT NOT NULL, response_by TEXT NOT NULL,
 notice_body TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN('PENDING','NOTIFIED','ISSUED','WAIVED','DECLINED','WITHDRAWN')),
 author_id TEXT NOT NULL REFERENCES users(id), created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 version INTEGER NOT NULL CHECK(version>0), public_version INTEGER NOT NULL DEFAULT 0 CHECK(public_version>=0), public_updated_at INTEGER NOT NULL DEFAULT 0,
 notice_id TEXT REFERENCES fine_notices(id), original_entry_id TEXT UNIQUE REFERENCES entries(id), current_entry_id TEXT UNIQUE REFERENCES entries(id),
 charge_version INTEGER NOT NULL DEFAULT 0 CHECK(charge_version>=0), waived_paise INTEGER NOT NULL DEFAULT 0 CHECK(waived_paise BETWEEN 0 AND amount_paise),
 resolution TEXT NOT NULL DEFAULT '', resolution_key TEXT NOT NULL DEFAULT '', early_issue_reference TEXT NOT NULL DEFAULT '',
 resolved_by TEXT REFERENCES users(id), resolved_at INTEGER, issued_by TEXT REFERENCES users(id), issued_at INTEGER, issued_context_key TEXT NOT NULL DEFAULT '', issued_incident_notice_id TEXT NOT NULL DEFAULT '', issued_response_count INTEGER NOT NULL DEFAULT 0 CHECK(issued_response_count>=0),
 pause_until TEXT NOT NULL DEFAULT '', pause_appeal_id TEXT REFERENCES fine_appeals(id),
 CHECK(state NOT IN('NOTIFIED','ISSUED','WAIVED') OR notice_id IS NOT NULL),
 CHECK(state NOT IN('ISSUED','WAIVED') OR (original_entry_id IS NOT NULL AND issued_by IS NOT NULL AND issued_by!=author_id AND issued_at IS NOT NULL AND charge_version>0)),
 CHECK(state!='WAIVED' OR (current_entry_id IS NULL AND waived_paise=amount_paise))
) STRICT;
CREATE UNIQUE INDEX fine_active_incident ON fines(incident_id) WHERE state IN('PENDING','NOTIFIED','ISSUED','WAIVED');
CREATE INDEX fine_home_queue ON fines(flat_id,state,due_date);
CREATE TABLE fine_notices (
 id TEXT PRIMARY KEY, fine_id TEXT NOT NULL UNIQUE REFERENCES fines(id), flat_id TEXT NOT NULL REFERENCES flats(id),
 title TEXT NOT NULL, body TEXT NOT NULL, amount_paise INTEGER NOT NULL CHECK(amount_paise>0),
 policy_reference TEXT NOT NULL, due_date TEXT NOT NULL, response_by TEXT NOT NULL,
 issuer_id TEXT NOT NULL REFERENCES users(id), created_at INTEGER NOT NULL, version INTEGER NOT NULL CHECK(version=1)
) STRICT;
CREATE TABLE fine_responses (
 id TEXT PRIMARY KEY, notice_id TEXT NOT NULL REFERENCES fine_notices(id), author_id TEXT NOT NULL REFERENCES users(id), body TEXT NOT NULL, created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX fine_response_history ON fine_responses(notice_id,created_at,id);
CREATE TABLE fine_reports (
 id TEXT PRIMARY KEY, fine_id TEXT NOT NULL REFERENCES fines(id), flat_id TEXT NOT NULL REFERENCES flats(id), author_id TEXT NOT NULL REFERENCES users(id),
 amount_paise INTEGER NOT NULL CHECK(amount_paise BETWEEN 1 AND 1000000000), payment_date TEXT NOT NULL, payer TEXT NOT NULL,
 method TEXT NOT NULL CHECK(method IN('CASH','BANK_TRANSFER','CHEQUE','UPI')), reference TEXT NOT NULL, comment TEXT NOT NULL,
 evidence_id TEXT REFERENCES library_documents(id), state TEXT NOT NULL CHECK(state IN('PENDING','NEEDS_INFO','REJECTED','WITHDRAWN','CONFIRMED','DUPLICATE')),
 version INTEGER NOT NULL CHECK(version>0), created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 entry_id TEXT REFERENCES entries(id), allocation_id TEXT REFERENCES entry_allocations(id), duplicate_report_id TEXT REFERENCES fine_reports(id),
 reviewer_id TEXT REFERENCES users(id), reviewed_at INTEGER, decision_reason TEXT NOT NULL DEFAULT '',
 CHECK(state NOT IN('CONFIRMED','DUPLICATE') OR (entry_id IS NOT NULL AND reviewer_id IS NOT NULL AND reviewer_id!=author_id AND reviewed_at IS NOT NULL)),
 CHECK(state!='DUPLICATE' OR duplicate_report_id IS NOT NULL)
) STRICT;
CREATE INDEX fine_report_queue ON fine_reports(fine_id,state,author_id);
CREATE TABLE fine_appeals (
 id TEXT PRIMARY KEY, fine_id TEXT NOT NULL REFERENCES fines(id), author_id TEXT NOT NULL REFERENCES users(id), body TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN('PENDING','PAUSED','RESOLVED','DECLINED','WITHDRAWN')), version INTEGER NOT NULL CHECK(version>0),
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, reviewer_id TEXT REFERENCES users(id), reviewed_at INTEGER,
 decision_reason TEXT NOT NULL DEFAULT '', policy_reference TEXT NOT NULL DEFAULT '', pause_until TEXT NOT NULL DEFAULT '',
 CHECK(state NOT IN('PAUSED','RESOLVED','DECLINED') OR (reviewer_id IS NOT NULL AND reviewer_id!=author_id AND reviewed_at IS NOT NULL))
) STRICT;
CREATE INDEX fine_appeal_queue ON fine_appeals(fine_id,state,author_id);
CREATE UNIQUE INDEX fine_pending_appeal ON fine_appeals(fine_id,author_id) WHERE state IN('PENDING','PAUSED');
CREATE TABLE fine_waivers (
 id TEXT PRIMARY KEY, fine_id TEXT NOT NULL REFERENCES fines(id), charge_version INTEGER NOT NULL CHECK(charge_version>0),
 amount_paise INTEGER NOT NULL CHECK(amount_paise BETWEEN 1 AND 1000000000), kind TEXT NOT NULL CHECK(kind IN('WAIVER','REVERSAL')),
 policy_reference TEXT NOT NULL, reason TEXT NOT NULL, author_id TEXT NOT NULL REFERENCES users(id),
 state TEXT NOT NULL CHECK(state IN('PENDING','APPROVED','DECLINED','WITHDRAWN')), version INTEGER NOT NULL CHECK(version>0),
 created_at INTEGER NOT NULL, reviewer_id TEXT REFERENCES users(id), reviewed_at INTEGER, decision_reason TEXT NOT NULL DEFAULT '', replacement_entry_id TEXT REFERENCES entries(id),
 CHECK(state NOT IN('APPROVED','DECLINED') OR (reviewer_id IS NOT NULL AND reviewer_id!=author_id AND reviewed_at IS NOT NULL))
) STRICT;
CREATE INDEX fine_waiver_queue ON fine_waivers(fine_id,state);
CREATE TABLE fine_events (
 id INTEGER PRIMARY KEY, fine_id TEXT NOT NULL REFERENCES fines(id), subject_type TEXT NOT NULL CHECK(subject_type IN('FINE','REPORT','APPEAL','WAIVER')),
 subject_id TEXT NOT NULL, version INTEGER NOT NULL CHECK(version>0), action TEXT NOT NULL,
 actor_id TEXT NOT NULL REFERENCES users(id), reason TEXT NOT NULL, snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)), occurred_at INTEGER NOT NULL,
 UNIQUE(subject_type,subject_id,version)
) STRICT;
CREATE INDEX fine_event_history ON fine_events(subject_type,subject_id,id DESC);
CREATE TRIGGER fine_insert BEFORE INSERT ON fines WHEN NEW.state!='PENDING' OR NEW.version!=1 OR NEW.public_version!=0 OR NEW.notice_id IS NOT NULL OR NEW.original_entry_id IS NOT NULL OR NEW.current_entry_id IS NOT NULL OR NEW.charge_version!=0 OR NEW.waived_paise!=0
BEGIN SELECT RAISE(ABORT,'fine starts as a frozen unissued proposal'); END;
CREATE TRIGGER fine_frozen BEFORE UPDATE ON fines WHEN NEW.id!=OLD.id OR NEW.incident_id!=OLD.incident_id OR NEW.replaces_id IS NOT OLD.replaces_id OR NEW.flat_id!=OLD.flat_id OR NEW.rule_id!=OLD.rule_id OR NEW.material_key!=OLD.material_key OR NEW.source_key!=OLD.source_key OR NEW.source_json!=OLD.source_json OR NEW.title!=OLD.title OR NEW.amount_paise!=OLD.amount_paise OR NEW.policy_reference!=OLD.policy_reference OR NEW.reason!=OLD.reason OR NEW.due_date!=OLD.due_date OR NEW.response_by!=OLD.response_by OR NEW.notice_body!=OLD.notice_body OR NEW.author_id!=OLD.author_id OR NEW.created_at!=OLD.created_at OR NEW.version!=OLD.version+1 OR (OLD.original_entry_id IS NOT NULL AND NEW.original_entry_id IS NOT OLD.original_entry_id) OR (OLD.notice_id IS NOT NULL AND NEW.notice_id IS NOT OLD.notice_id)
 OR NOT ((OLD.state='PENDING' AND NEW.state IN('PENDING','NOTIFIED','DECLINED','WITHDRAWN')) OR (OLD.state='NOTIFIED' AND NEW.state IN('NOTIFIED','ISSUED','DECLINED','WITHDRAWN')) OR (OLD.state='ISSUED' AND NEW.state IN('ISSUED','WAIVED')) OR (OLD.state='WAIVED' AND NEW.state='WAIVED'))
BEGIN SELECT RAISE(ABORT,'fine content and transitions retain original decisions'); END;
CREATE TRIGGER fine_notice_insert BEFORE INSERT ON fine_notices WHEN NOT EXISTS(SELECT 1 FROM fines f WHERE f.id=NEW.fine_id AND f.state='PENDING' AND f.flat_id=NEW.flat_id AND f.title=NEW.title AND f.notice_body=NEW.body AND f.amount_paise=NEW.amount_paise AND f.policy_reference=NEW.policy_reference AND f.due_date=NEW.due_date AND f.response_by=NEW.response_by AND f.author_id!=NEW.issuer_id)
BEGIN SELECT RAISE(ABORT,'fine notice requires independent matching publication'); END;
CREATE TRIGGER fine_effect_guard BEFORE UPDATE ON fines WHEN NEW.state IN('ISSUED','WAIVED') AND (NOT EXISTS(SELECT 1 FROM entries e WHERE e.id=NEW.original_entry_id AND e.flat_id=NEW.flat_id AND e.kind='CHARGE' AND e.state='POSTED' AND e.amount_paise=NEW.amount_paise) OR (NEW.state='ISSUED' AND NOT EXISTS(SELECT 1 FROM entries e WHERE e.id=NEW.current_entry_id AND e.flat_id=NEW.flat_id AND e.kind='CHARGE' AND e.state='POSTED' AND e.amount_paise=NEW.amount_paise-NEW.waived_paise AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id))))
BEGIN SELECT RAISE(ABORT,'fine effects require the matching retained charge'); END;
CREATE TRIGGER fine_charge_reverse_guard BEFORE INSERT ON entry_reversals WHEN EXISTS(SELECT 1 FROM fines f WHERE NEW.entry_id IN(f.original_entry_id,f.current_entry_id)) AND NOT EXISTS(SELECT 1 FROM fines f JOIN fine_waivers w ON w.fine_id=f.id JOIN incidents i ON i.id=f.incident_id WHERE f.current_entry_id=NEW.entry_id AND f.state='ISSUED' AND w.state='APPROVED' AND w.reviewer_id=NEW.actor_id AND w.charge_version=f.charge_version AND w.author_id!=NEW.actor_id AND f.author_id!=NEW.actor_id AND i.reporter_id!=NEW.actor_id)
BEGIN SELECT RAISE(ABORT,'fine charge requires a separately reviewed correction'); END;
CREATE TRIGGER fine_report_insert BEFORE INSERT ON fine_reports WHEN NEW.state!='PENDING' OR NEW.version!=1 OR NEW.entry_id IS NOT NULL OR NEW.reviewer_id IS NOT NULL OR NOT EXISTS(SELECT 1 FROM fines f WHERE f.id=NEW.fine_id AND f.flat_id=NEW.flat_id AND f.state IN('ISSUED','WAIVED'))
BEGIN SELECT RAISE(ABORT,'fine payment starts as an unconfirmed own-home claim'); END;
CREATE TRIGGER fine_report_update BEFORE UPDATE ON fine_reports WHEN NEW.id!=OLD.id OR NEW.fine_id!=OLD.fine_id OR NEW.flat_id!=OLD.flat_id OR NEW.author_id!=OLD.author_id OR NEW.created_at!=OLD.created_at OR NEW.version!=OLD.version+1 OR NOT ((OLD.state='PENDING' AND NEW.state IN('NEEDS_INFO','REJECTED','WITHDRAWN','CONFIRMED','DUPLICATE')) OR (OLD.state IN('NEEDS_INFO','REJECTED') AND NEW.state IN('PENDING','WITHDRAWN'))) OR (NEW.state!='PENDING' AND (NEW.amount_paise!=OLD.amount_paise OR NEW.payment_date!=OLD.payment_date OR NEW.payer!=OLD.payer OR NEW.method!=OLD.method OR NEW.reference!=OLD.reference OR NEW.comment!=OLD.comment OR NEW.evidence_id IS NOT OLD.evidence_id))
BEGIN SELECT RAISE(ABORT,'fine report revisions preserve the original claim'); END;
CREATE TRIGGER fine_report_confirm BEFORE UPDATE ON fine_reports WHEN NEW.state IN('CONFIRMED','DUPLICATE') AND NOT EXISTS(SELECT 1 FROM entries e JOIN receipts r ON r.entry_id=e.id JOIN verified_fund_payments v ON v.entry_id=e.id WHERE e.id=NEW.entry_id AND e.flat_id=OLD.flat_id AND e.kind='RECEIVED' AND e.state='POSTED' AND e.amount_paise=OLD.amount_paise AND e.entry_date=OLD.payment_date AND e.method=OLD.method AND NOT EXISTS(SELECT 1 FROM entry_reversals x WHERE x.entry_id=e.id))
BEGIN SELECT RAISE(ABORT,'fine confirmation requires the verified original receipt'); END;
CREATE TRIGGER fine_waiver_insert BEFORE INSERT ON fine_waivers WHEN NEW.state!='PENDING' OR NEW.version!=1 OR NEW.reviewer_id IS NOT NULL
BEGIN SELECT RAISE(ABORT,'fine correction starts as a separate proposal'); END;
CREATE TRIGGER fine_waiver_update BEFORE UPDATE ON fine_waivers WHEN OLD.state!='PENDING' OR NEW.state='PENDING' OR NEW.version!=OLD.version+1 OR NEW.id!=OLD.id OR NEW.fine_id!=OLD.fine_id OR NEW.charge_version!=OLD.charge_version OR NEW.amount_paise!=OLD.amount_paise OR NEW.kind!=OLD.kind OR NEW.policy_reference!=OLD.policy_reference OR NEW.reason!=OLD.reason OR NEW.author_id!=OLD.author_id OR NEW.created_at!=OLD.created_at OR (NEW.state='WITHDRAWN' AND NEW.reviewer_id!=OLD.author_id)
BEGIN SELECT RAISE(ABORT,'fine correction and its decision are frozen'); END;
CREATE TRIGGER fine_waiver_approval BEFORE UPDATE ON fine_waivers WHEN NEW.state='APPROVED' AND NOT EXISTS(
 SELECT 1 FROM fines f JOIN incidents i ON i.id=f.incident_id JOIN entries e ON e.id=f.current_entry_id
 WHERE f.id=OLD.fine_id AND f.state='ISSUED' AND f.charge_version=OLD.charge_version
 AND NEW.reviewer_id!=f.author_id AND NEW.reviewer_id!=i.reporter_id AND NEW.reviewed_at IS NOT NULL
 AND e.kind='CHARGE' AND e.state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id)
 AND ((OLD.amount_paise=e.amount_paise AND NEW.replacement_entry_id IS NULL)
 OR (OLD.kind='WAIVER' AND OLD.amount_paise<e.amount_paise AND EXISTS(
 SELECT 1 FROM entries replacement WHERE replacement.id=NEW.replacement_entry_id AND replacement.flat_id=f.flat_id
 AND replacement.kind='CHARGE' AND replacement.state='POSTED' AND replacement.amount_paise=e.amount_paise-OLD.amount_paise
 AND replacement.posted_by=NEW.reviewer_id AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=replacement.id)))))
BEGIN SELECT RAISE(ABORT,'fine correction requires the matching live charge and independent approved replacement'); END;
CREATE TRIGGER fine_appeal_insert BEFORE INSERT ON fine_appeals WHEN NEW.state!='PENDING' OR NEW.version!=1 OR NEW.reviewer_id IS NOT NULL
BEGIN SELECT RAISE(ABORT,'fine appeal starts as a supplied pending account'); END;
CREATE TRIGGER fine_appeal_update BEFORE UPDATE ON fine_appeals WHEN NEW.id!=OLD.id OR NEW.fine_id!=OLD.fine_id OR NEW.author_id!=OLD.author_id OR NEW.body!=OLD.body OR NEW.created_at!=OLD.created_at OR NEW.version!=OLD.version+1 OR NOT ((OLD.state='PENDING' AND NEW.state IN('PAUSED','RESOLVED','DECLINED','WITHDRAWN')) OR (OLD.state='PAUSED' AND NEW.state IN('PAUSED','RESOLVED','DECLINED')))
BEGIN SELECT RAISE(ABORT,'fine appeal history is retained'); END;
CREATE TRIGGER fine_delete BEFORE DELETE ON fines BEGIN SELECT RAISE(ABORT,'fines are retained'); END;
CREATE TRIGGER fine_notice_update BEFORE UPDATE ON fine_notices BEGIN SELECT RAISE(ABORT,'fine notices are immutable'); END;
CREATE TRIGGER fine_notice_delete BEFORE DELETE ON fine_notices BEGIN SELECT RAISE(ABORT,'fine notices are retained'); END;
CREATE TRIGGER fine_response_update BEFORE UPDATE ON fine_responses BEGIN SELECT RAISE(ABORT,'fine responses are immutable'); END;
CREATE TRIGGER fine_response_delete BEFORE DELETE ON fine_responses BEGIN SELECT RAISE(ABORT,'fine responses are retained'); END;
CREATE TRIGGER fine_report_delete BEFORE DELETE ON fine_reports BEGIN SELECT RAISE(ABORT,'fine claims are retained'); END;
CREATE TRIGGER fine_appeal_delete BEFORE DELETE ON fine_appeals BEGIN SELECT RAISE(ABORT,'fine appeals are retained'); END;
CREATE TRIGGER fine_waiver_delete BEFORE DELETE ON fine_waivers BEGIN SELECT RAISE(ABORT,'fine corrections are retained'); END;
CREATE TRIGGER fine_event_update BEFORE UPDATE ON fine_events BEGIN SELECT RAISE(ABORT,'fine events are immutable'); END;
CREATE TRIGGER fine_event_delete BEFORE DELETE ON fine_events BEGIN SELECT RAISE(ABORT,'fine events are retained'); END;
