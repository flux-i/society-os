CREATE TABLE resident_contacts (
 resident_id TEXT PRIMARY KEY REFERENCES residents(id),
 phone TEXT NOT NULL,
 email TEXT NOT NULL,
 preferred_channel TEXT NOT NULL CHECK(preferred_channel IN('NONE','WHATSAPP','EMAIL')),
 community_whatsapp INTEGER NOT NULL CHECK(community_whatsapp IN(0,1)),
 community_email INTEGER NOT NULL CHECK(community_email IN(0,1)),
 finance_whatsapp INTEGER NOT NULL CHECK(finance_whatsapp IN(0,1)),
 finance_email INTEGER NOT NULL CHECK(finance_email IN(0,1)),
 consent_source TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN('PENDING','VERIFIED','DECLINED','WITHDRAWN')),
 submitted_by TEXT NOT NULL REFERENCES users(id),
 submitted_at INTEGER NOT NULL,
 reviewed_by TEXT REFERENCES users(id),
 reviewed_at INTEGER NOT NULL DEFAULT 0,
 decision_reason TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 version INTEGER NOT NULL CHECK(version>0),
 CHECK(phone<>'' OR email<>''),
 CHECK(phone<>'' OR (community_whatsapp=0 AND finance_whatsapp=0 AND preferred_channel<>'WHATSAPP')),
 CHECK(email<>'' OR (community_email=0 AND finance_email=0 AND preferred_channel<>'EMAIL')),
 CHECK(state<>'VERIFIED' OR (reviewed_by IS NOT NULL AND reviewed_by<>submitted_by AND reviewed_at>0))
) STRICT;
CREATE INDEX resident_contact_queue ON resident_contacts(state,updated_at,resident_id);
CREATE TRIGGER resident_contact_delete BEFORE DELETE ON resident_contacts BEGIN SELECT RAISE(ABORT,'contact history is retained'); END;
CREATE TRIGGER resident_contact_identity BEFORE UPDATE ON resident_contacts WHEN NEW.resident_id<>OLD.resident_id OR NEW.created_at<>OLD.created_at OR NEW.version<>OLD.version+1 BEGIN SELECT RAISE(ABORT,'contact identity and version are preserved'); END;
CREATE TRIGGER resident_contact_verify BEFORE UPDATE ON resident_contacts WHEN NEW.state='VERIFIED' AND OLD.state<>'VERIFIED' AND (
 OLD.state<>'PENDING' OR NEW.phone<>OLD.phone OR NEW.email<>OLD.email OR NEW.preferred_channel<>OLD.preferred_channel OR NEW.community_whatsapp<>OLD.community_whatsapp OR NEW.community_email<>OLD.community_email OR NEW.finance_whatsapp<>OLD.finance_whatsapp OR NEW.finance_email<>OLD.finance_email OR NEW.consent_source<>OLD.consent_source OR NEW.submitted_by<>OLD.submitted_by OR NEW.submitted_at<>OLD.submitted_at
) BEGIN SELECT RAISE(ABORT,'verification preserves the proposed contact and consent'); END;
CREATE TRIGGER resident_contact_verified_update BEFORE UPDATE ON resident_contacts WHEN NEW.state='VERIFIED' AND OLD.state='VERIFIED' AND (
 NEW.phone<>OLD.phone OR NEW.email<>OLD.email OR NEW.preferred_channel<>OLD.preferred_channel OR NEW.community_whatsapp>OLD.community_whatsapp OR NEW.community_email>OLD.community_email OR NEW.finance_whatsapp>OLD.finance_whatsapp OR NEW.finance_email>OLD.finance_email OR NEW.consent_source<>OLD.consent_source OR NEW.submitted_by<>OLD.submitted_by OR NEW.submitted_at<>OLD.submitted_at OR NEW.reviewed_by IS NOT OLD.reviewed_by OR NEW.reviewed_at<>OLD.reviewed_at OR NEW.decision_reason<>OLD.decision_reason
) BEGIN SELECT RAISE(ABORT,'verified destinations cannot change or regain consent without review'); END;
CREATE TABLE contact_events (
 id INTEGER PRIMARY KEY,
 resident_id TEXT NOT NULL REFERENCES resident_contacts(resident_id),
 version INTEGER NOT NULL,
 actor_id TEXT NOT NULL REFERENCES users(id),
 action TEXT NOT NULL CHECK(action IN('REGISTERED','VERIFIED','DECLINED','WITHDRAWN','OPTED_OUT')),
 reason TEXT NOT NULL,
 occurred_at INTEGER NOT NULL,
 snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
 UNIQUE(resident_id,version)
) STRICT;
CREATE TRIGGER contact_event_update BEFORE UPDATE ON contact_events BEGIN SELECT RAISE(ABORT,'contact decisions are immutable'); END;
CREATE TRIGGER contact_event_delete BEFORE DELETE ON contact_events BEGIN SELECT RAISE(ABORT,'contact decisions are retained'); END;
