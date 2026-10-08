CREATE TABLE budget_resources (
 id TEXT PRIMARY KEY,
 version INTEGER NOT NULL CHECK(version>=1),
 latest_version INTEGER NOT NULL CHECK(latest_version BETWEEN 1 AND version),
 pending_version INTEGER,
 approved_version INTEGER,
 decision TEXT NOT NULL CHECK(decision IN('PENDING','APPROVED','DECLINED','CANCELLED')),
 created_by TEXT NOT NULL REFERENCES users(id),
 created_at INTEGER NOT NULL,
 FOREIGN KEY(id,latest_version) REFERENCES budget_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,pending_version) REFERENCES budget_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,approved_version) REFERENCES budget_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 CHECK(pending_version IS NULL OR pending_version=latest_version),
 CHECK((decision='PENDING')=(pending_version IS NOT NULL))
) STRICT;
CREATE TABLE budget_versions (
 resource_id TEXT NOT NULL REFERENCES budget_resources(id),
 version INTEGER NOT NULL CHECK(version>=1),
 action TEXT NOT NULL CHECK(action IN('PLAN','CLOSE')),
 snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
 submitted_by TEXT NOT NULL REFERENCES users(id),
 submitted_at INTEGER NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 PRIMARY KEY(resource_id,version),
 CHECK(json_extract(snapshot_json,'$.version') IS version AND json_extract(snapshot_json,'$.action') IS action),
 CHECK(json_extract(snapshot_json,'$.submitted_by') IS submitted_by AND json_extract(snapshot_json,'$.submitted_at') IS submitted_at AND json_extract(snapshot_json,'$.reason') IS reason),
 CHECK(json_type(snapshot_json,'$.plan_version') IS 'integer' AND json_extract(snapshot_json,'$.plan_version') BETWEEN 1 AND version),
 CHECK((action='PLAN' AND json_extract(snapshot_json,'$.plan_version')=version) OR (action='CLOSE' AND json_extract(snapshot_json,'$.plan_version')<version)),
 CHECK(json_type(snapshot_json,'$.collections_paise') IS 'integer' AND json_extract(snapshot_json,'$.collections_paise') BETWEEN 0 AND 1000000000),
 CHECK(json_type(snapshot_json,'$.expenses_paise') IS 'integer' AND json_extract(snapshot_json,'$.expenses_paise') BETWEEN 0 AND 1000000000),
 CHECK(json_type(snapshot_json,'$.title') IS 'text' AND length(json_extract(snapshot_json,'$.title')) BETWEEN 5 AND 120),
 CHECK(json_type(snapshot_json,'$.period_start') IS 'text' AND json_type(snapshot_json,'$.period_end') IS 'text' AND length(json_extract(snapshot_json,'$.period_start'))=10 AND length(json_extract(snapshot_json,'$.period_end'))=10)
) STRICT;
CREATE TABLE budget_events (
 resource_id TEXT NOT NULL REFERENCES budget_resources(id),
 version INTEGER NOT NULL CHECK(version>=1),
 proposal_version INTEGER NOT NULL,
 action TEXT NOT NULL CHECK(action IN('PROPOSED','REVISED','APPROVED','DECLINED','CANCELLED')),
 actor_id TEXT NOT NULL REFERENCES users(id),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 occurred_at INTEGER NOT NULL,
 PRIMARY KEY(resource_id,version),
 FOREIGN KEY(resource_id,proposal_version) REFERENCES budget_versions(resource_id,version)
) STRICT;
CREATE TABLE paid_expense_resources (
 id TEXT PRIMARY KEY,
 budget_id TEXT NOT NULL REFERENCES budget_resources(id),
 version INTEGER NOT NULL CHECK(version>=1),
 latest_version INTEGER NOT NULL CHECK(latest_version BETWEEN 1 AND version),
 pending_version INTEGER,
 approved_version INTEGER,
 decision TEXT NOT NULL CHECK(decision IN('PENDING','APPROVED','DECLINED','CANCELLED')),
 created_by TEXT NOT NULL REFERENCES users(id),
 created_at INTEGER NOT NULL,
 FOREIGN KEY(id,latest_version) REFERENCES paid_expense_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,pending_version) REFERENCES paid_expense_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,approved_version) REFERENCES paid_expense_versions(resource_id,version) DEFERRABLE INITIALLY DEFERRED,
 CHECK(pending_version IS NULL OR pending_version=latest_version),
 CHECK((decision='PENDING')=(pending_version IS NOT NULL))
) STRICT;
CREATE TABLE paid_expense_versions (
 resource_id TEXT NOT NULL REFERENCES paid_expense_resources(id),
 version INTEGER NOT NULL CHECK(version>=1),
 budget_id TEXT NOT NULL REFERENCES budget_resources(id),
 budget_version INTEGER NOT NULL,
 previous_version INTEGER,
 action TEXT NOT NULL CHECK(action IN('PAID','CORRECTION','VOID')),
 snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
 submitted_by TEXT NOT NULL REFERENCES users(id),
 submitted_at INTEGER NOT NULL,
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 PRIMARY KEY(resource_id,version),
 FOREIGN KEY(budget_id,budget_version) REFERENCES budget_versions(resource_id,version),
 FOREIGN KEY(resource_id,previous_version) REFERENCES paid_expense_versions(resource_id,version),
 CHECK((action='PAID')=(previous_version IS NULL)),
 CHECK(previous_version IS NULL OR previous_version<version),
 CHECK(json_extract(snapshot_json,'$.previous_version') IS COALESCE(previous_version,0)),
 CHECK(json_extract(snapshot_json,'$.version') IS version AND json_extract(snapshot_json,'$.action') IS action),
 CHECK(json_extract(snapshot_json,'$.submitted_by') IS submitted_by AND json_extract(snapshot_json,'$.submitted_at') IS submitted_at AND json_extract(snapshot_json,'$.reason') IS reason),
 CHECK(json_extract(snapshot_json,'$.budget_id') IS budget_id AND json_extract(snapshot_json,'$.budget_version') IS budget_version),
 CHECK(json_type(snapshot_json,'$.amount_paise') IS 'integer' AND json_extract(snapshot_json,'$.amount_paise') BETWEEN 0 AND 1000000000),
 CHECK((action='VOID')=(json_extract(snapshot_json,'$.amount_paise')=0)),
 CHECK(json_type(snapshot_json,'$.method') IS 'text' AND json_extract(snapshot_json,'$.method') IN('CASH','BANK_TRANSFER','CHEQUE','UPI')),
 CHECK(json_type(snapshot_json,'$.reference') IS 'text' AND length(json_extract(snapshot_json,'$.reference')) BETWEEN 1 AND 120),
 CHECK(json_type(snapshot_json,'$.paid_date') IS 'text' AND length(json_extract(snapshot_json,'$.paid_date'))=10)
) STRICT;
CREATE TABLE paid_expense_events (
 resource_id TEXT NOT NULL REFERENCES paid_expense_resources(id),
 version INTEGER NOT NULL CHECK(version>=1),
 proposal_version INTEGER NOT NULL,
 action TEXT NOT NULL CHECK(action IN('PROPOSED','REVISED','APPROVED','DECLINED','CANCELLED')),
 actor_id TEXT NOT NULL REFERENCES users(id),
 reason TEXT NOT NULL CHECK(length(reason) BETWEEN 10 AND 800),
 occurred_at INTEGER NOT NULL,
 PRIMARY KEY(resource_id,version),
 FOREIGN KEY(resource_id,proposal_version) REFERENCES paid_expense_versions(resource_id,version)
) STRICT;
CREATE TABLE paid_expense_identities (
 identity_hash TEXT PRIMARY KEY CHECK(length(identity_hash)=64),
 expense_id TEXT NOT NULL REFERENCES paid_expense_resources(id),
 source_version INTEGER NOT NULL,
 created_at INTEGER NOT NULL,
 FOREIGN KEY(expense_id,source_version) REFERENCES paid_expense_versions(resource_id,version)
) STRICT;
CREATE INDEX budget_decision_queue ON budget_resources(decision,id);
CREATE INDEX paid_expense_budget ON paid_expense_resources(budget_id,decision,id);
CREATE TRIGGER budget_versions_update BEFORE UPDATE ON budget_versions BEGIN SELECT RAISE(ABORT,'budget originals are immutable'); END;
CREATE TRIGGER budget_versions_delete BEFORE DELETE ON budget_versions BEGIN SELECT RAISE(ABORT,'budget originals are retained'); END;
CREATE TRIGGER budget_events_update BEFORE UPDATE ON budget_events BEGIN SELECT RAISE(ABORT,'budget decisions are immutable'); END;
CREATE TRIGGER budget_events_delete BEFORE DELETE ON budget_events BEGIN SELECT RAISE(ABORT,'budget decisions are retained'); END;
CREATE TRIGGER budget_resources_delete BEFORE DELETE ON budget_resources BEGIN SELECT RAISE(ABORT,'budget history is retained'); END;
CREATE TRIGGER budget_resources_identity BEFORE UPDATE ON budget_resources WHEN NEW.id<>OLD.id OR NEW.created_by<>OLD.created_by OR NEW.created_at<>OLD.created_at OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'budget identity and version sequence are retained'); END;
CREATE TRIGGER budget_separate_decision BEFORE INSERT ON budget_events WHEN NEW.action IN('APPROVED','DECLINED') AND EXISTS(SELECT 1 FROM budget_versions v WHERE v.resource_id=NEW.resource_id AND v.version=NEW.proposal_version AND v.submitted_by=NEW.actor_id)
 BEGIN SELECT RAISE(ABORT,'budget decisions require a different reviewer'); END;
CREATE TRIGGER budget_approved_head BEFORE UPDATE OF approved_version ON budget_resources WHEN NEW.approved_version IS NOT OLD.approved_version AND (NEW.approved_version IS NULL OR NOT EXISTS(SELECT 1 FROM budget_events e JOIN budget_versions v ON v.resource_id=e.resource_id AND v.version=e.proposal_version WHERE e.resource_id=NEW.id AND e.action='APPROVED' AND e.version=NEW.version AND e.proposal_version=NEW.approved_version AND e.actor_id<>v.submitted_by))
 BEGIN SELECT RAISE(ABORT,'budget approval requires its exact separate decision'); END;
CREATE TRIGGER paid_expense_versions_update BEFORE UPDATE ON paid_expense_versions BEGIN SELECT RAISE(ABORT,'paid expense originals are immutable'); END;
CREATE TRIGGER paid_expense_versions_delete BEFORE DELETE ON paid_expense_versions BEGIN SELECT RAISE(ABORT,'paid expense originals are retained'); END;
CREATE TRIGGER paid_expense_events_update BEFORE UPDATE ON paid_expense_events BEGIN SELECT RAISE(ABORT,'paid expense decisions are immutable'); END;
CREATE TRIGGER paid_expense_events_delete BEFORE DELETE ON paid_expense_events BEGIN SELECT RAISE(ABORT,'paid expense decisions are retained'); END;
CREATE TRIGGER paid_expense_resources_delete BEFORE DELETE ON paid_expense_resources BEGIN SELECT RAISE(ABORT,'paid expense history is retained'); END;
CREATE TRIGGER paid_expense_resources_identity BEFORE UPDATE ON paid_expense_resources WHEN NEW.id<>OLD.id OR NEW.budget_id<>OLD.budget_id OR NEW.created_by<>OLD.created_by OR NEW.created_at<>OLD.created_at OR NEW.version<>OLD.version+1
 BEGIN SELECT RAISE(ABORT,'paid expense identity and version sequence are retained'); END;
CREATE TRIGGER paid_expense_separate_decision BEFORE INSERT ON paid_expense_events WHEN NEW.action IN('APPROVED','DECLINED') AND EXISTS(SELECT 1 FROM paid_expense_versions v WHERE v.resource_id=NEW.resource_id AND v.version=NEW.proposal_version AND v.submitted_by=NEW.actor_id)
 BEGIN SELECT RAISE(ABORT,'paid expense decisions require a different reviewer'); END;
CREATE TRIGGER paid_expense_approved_head BEFORE UPDATE OF approved_version ON paid_expense_resources WHEN NEW.approved_version IS NOT OLD.approved_version AND (NEW.approved_version IS NULL OR NOT EXISTS(SELECT 1 FROM paid_expense_events e JOIN paid_expense_versions v ON v.resource_id=e.resource_id AND v.version=e.proposal_version WHERE e.resource_id=NEW.id AND e.action='APPROVED' AND e.version=NEW.version AND e.proposal_version=NEW.approved_version AND e.actor_id<>v.submitted_by))
 BEGIN SELECT RAISE(ABORT,'paid expense approval requires its exact separate decision'); END;
CREATE TRIGGER paid_expense_budget_source BEFORE INSERT ON paid_expense_versions WHEN NOT EXISTS(SELECT 1 FROM paid_expense_resources r JOIN budget_versions v ON v.resource_id=r.budget_id AND v.version=NEW.budget_version WHERE r.id=NEW.resource_id AND r.budget_id=NEW.budget_id AND v.action='PLAN' AND EXISTS(SELECT 1 FROM budget_events e WHERE e.resource_id=v.resource_id AND e.proposal_version=v.version AND e.action='APPROVED'))
 BEGIN SELECT RAISE(ABORT,'paid expenses retain an original budget plan'); END;
CREATE TRIGGER paid_expense_identities_update BEFORE UPDATE ON paid_expense_identities BEGIN SELECT RAISE(ABORT,'expense payment identities are retained'); END;
CREATE TRIGGER paid_expense_identities_delete BEFORE DELETE ON paid_expense_identities BEGIN SELECT RAISE(ABORT,'expense payment identities are retained'); END;

CREATE TRIGGER budget_period_retained BEFORE INSERT ON budget_versions WHEN EXISTS(
 SELECT 1 FROM budget_resources h JOIN budget_versions v ON v.resource_id=h.id AND v.version=h.approved_version
 WHERE h.id=NEW.resource_id AND (json_extract(v.snapshot_json,'$.period_start') IS NOT json_extract(NEW.snapshot_json,'$.period_start') OR json_extract(v.snapshot_json,'$.period_end') IS NOT json_extract(NEW.snapshot_json,'$.period_end')))
 BEGIN SELECT RAISE(ABORT,'approved budget period boundaries are retained'); END;

CREATE TRIGGER budget_head_event BEFORE UPDATE ON budget_resources WHEN NOT EXISTS(SELECT 1 FROM budget_events e WHERE e.resource_id=NEW.id AND e.version=NEW.version)
 BEGIN SELECT RAISE(ABORT,'budget head changes retain their exact event'); END;
CREATE TRIGGER paid_expense_head_event BEFORE UPDATE ON paid_expense_resources WHEN NOT EXISTS(SELECT 1 FROM paid_expense_events e WHERE e.resource_id=NEW.id AND e.version=NEW.version)
 BEGIN SELECT RAISE(ABORT,'paid expense head changes retain their exact event'); END;
