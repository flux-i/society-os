CREATE TABLE document_groups (
 id TEXT PRIMARY KEY,
 current_document_id TEXT REFERENCES library_documents(id) DEFERRABLE INITIALLY DEFERRED,
 approved_revision INTEGER NOT NULL DEFAULT 0 CHECK(approved_revision>=0),
 archived INTEGER NOT NULL DEFAULT 0 CHECK(archived IN(0,1))
) STRICT;
CREATE TABLE library_documents (
 id TEXT PRIMARY KEY,
 group_id TEXT NOT NULL REFERENCES document_groups(id),
 replaces_document_id TEXT REFERENCES library_documents(id),
 title TEXT NOT NULL CHECK(length(title) BETWEEN 5 AND 120),
 original_filename TEXT NOT NULL CHECK(length(original_filename) BETWEEN 1 AND 150),
 expected_size_bytes INTEGER NOT NULL CHECK(expected_size_bytes BETWEEN 1 AND 20971520),
 verified_sha256 TEXT NOT NULL CHECK(length(verified_sha256)=64),
 detected_content_type TEXT NOT NULL DEFAULT '' CHECK(detected_content_type IN('','application/pdf','image/png','image/jpeg')),
 category TEXT NOT NULL CHECK(category IN('RECEIPT','INVOICE','NOTICE','AGM_MINUTES','AUDIT_REPORT','CONTRACT','AMC','LEGAL','CIRCULAR','RESIDENT_DOCUMENT','COMMITTEE_DOCUMENT','PAYMENT_EVIDENCE','ACCOUNTING_EXPORT')),
 visibility TEXT NOT NULL CHECK(visibility IN('ALL_AUTHORIZED_RESIDENTS','COMMITTEE_ONLY','FLAT_SPECIFIC','RESIDENT_SPECIFIC','ACCOUNTING_ONLY')),
 flat_id TEXT REFERENCES flats(id),
 subject_resident_id TEXT REFERENCES residents(id),
 expiry_date TEXT,
 uploaded_by TEXT NOT NULL REFERENCES users(id),
 created_at INTEGER NOT NULL,
 expires_at INTEGER NOT NULL,
 uploaded_at INTEGER,
 available_at INTEGER,
 validation_status TEXT NOT NULL DEFAULT 'PENDING' CHECK(validation_status IN('PENDING','VALIDATING','AVAILABLE','REJECTED','ABANDONED')),
 validation_error_code TEXT NOT NULL DEFAULT '',
 validation_lease_until INTEGER NOT NULL DEFAULT 0,
 validation_lease_token TEXT NOT NULL DEFAULT '',
 validation_attempts INTEGER NOT NULL DEFAULT 0 CHECK(validation_attempts BETWEEN 0 AND 3),
 review_state TEXT NOT NULL DEFAULT 'PENDING' CHECK(review_state IN('PENDING','APPROVED','DECLINED','WITHDRAWN','ARCHIVED')),
 revision INTEGER NOT NULL DEFAULT 0 CHECK(revision>=0),
 version INTEGER NOT NULL DEFAULT 1 CHECK(version>=1),
 CHECK((visibility='FLAT_SPECIFIC' AND flat_id IS NOT NULL AND subject_resident_id IS NULL)
  OR (visibility='RESIDENT_SPECIFIC' AND subject_resident_id IS NOT NULL AND flat_id IS NULL)
  OR (visibility IN('ALL_AUTHORIZED_RESIDENTS','COMMITTEE_ONLY','ACCOUNTING_ONLY') AND flat_id IS NULL AND subject_resident_id IS NULL)),
 CHECK(category NOT IN('RECEIPT','INVOICE','PAYMENT_EVIDENCE','ACCOUNTING_EXPORT') OR visibility IN('FLAT_SPECIFIC','ACCOUNTING_ONLY')),
 CHECK(category!='PAYMENT_EVIDENCE' OR visibility='FLAT_SPECIFIC'),
 CHECK(category!='COMMITTEE_DOCUMENT' OR visibility='COMMITTEE_ONLY'),
 CHECK(expiry_date IS NULL OR category IN('CONTRACT','AMC')),
 CHECK(review_state!='APPROVED' OR (validation_status='AVAILABLE' AND revision>=1)),
 CHECK(validation_status!='AVAILABLE' OR (available_at IS NOT NULL AND detected_content_type!=''))
) STRICT;
CREATE INDEX library_scope ON library_documents(visibility,flat_id,subject_resident_id,review_state);
CREATE INDEX library_author ON library_documents(uploaded_by,created_at);
CREATE UNIQUE INDEX one_pending_document_version ON library_documents(group_id) WHERE review_state='PENDING' AND validation_status IN('PENDING','VALIDATING','AVAILABLE');
CREATE TABLE local_document_objects (
 document_id TEXT PRIMARY KEY REFERENCES library_documents(id),
 original_bytes BLOB NOT NULL CHECK(length(original_bytes) BETWEEN 1 AND 20971520)
) STRICT;
CREATE TABLE document_events (
 id TEXT PRIMARY KEY,
 document_id TEXT NOT NULL REFERENCES library_documents(id),
 actor_id TEXT REFERENCES users(id),
 action TEXT NOT NULL CHECK(action IN('RESERVED','UPLOADED','VALIDATED','VALIDATION_REJECTED','ABANDONED','APPROVED','DECLINED','WITHDRAWN','ARCHIVED','RETRY_VALIDATION')),
 reason TEXT NOT NULL,
 occurred_at INTEGER NOT NULL,
 version INTEGER NOT NULL,
 UNIQUE(document_id,version)
) STRICT;
CREATE TRIGGER library_original_immutable BEFORE UPDATE ON library_documents
 WHEN NEW.id!=OLD.id OR NEW.group_id!=OLD.group_id OR NEW.replaces_document_id IS NOT OLD.replaces_document_id
 OR NEW.title!=OLD.title OR NEW.original_filename!=OLD.original_filename OR NEW.expected_size_bytes!=OLD.expected_size_bytes
 OR NEW.verified_sha256!=OLD.verified_sha256 OR NEW.category!=OLD.category OR NEW.visibility!=OLD.visibility
 OR NEW.flat_id IS NOT OLD.flat_id OR NEW.subject_resident_id IS NOT OLD.subject_resident_id OR NEW.expiry_date IS NOT OLD.expiry_date
 OR NEW.uploaded_by!=OLD.uploaded_by OR NEW.created_at!=OLD.created_at OR NEW.expires_at!=OLD.expires_at
 OR (OLD.detected_content_type!='' AND NEW.detected_content_type!=OLD.detected_content_type)
 BEGIN SELECT RAISE(ABORT,'original document metadata is immutable'); END;
CREATE TRIGGER library_retain BEFORE DELETE ON library_documents BEGIN SELECT RAISE(ABORT,'retain document versions'); END;
CREATE TRIGGER original_bytes_immutable BEFORE UPDATE ON local_document_objects BEGIN SELECT RAISE(ABORT,'original bytes are immutable'); END;
CREATE TRIGGER original_bytes_retain BEFORE DELETE ON local_document_objects BEGIN SELECT RAISE(ABORT,'retain original bytes'); END;
CREATE TRIGGER document_events_immutable BEFORE UPDATE ON document_events BEGIN SELECT RAISE(ABORT,'document history is immutable'); END;
CREATE TRIGGER document_events_retain BEFORE DELETE ON document_events BEGIN SELECT RAISE(ABORT,'retain document history'); END;
CREATE TRIGGER document_head_scope BEFORE UPDATE OF current_document_id ON document_groups WHEN NEW.current_document_id IS NOT NULL
 AND NOT EXISTS(SELECT 1 FROM library_documents d WHERE d.id=NEW.current_document_id AND d.group_id=NEW.id AND d.review_state='APPROVED' AND d.validation_status='AVAILABLE')
 BEGIN SELECT RAISE(ABORT,'document head must be an approved version in this group'); END;
