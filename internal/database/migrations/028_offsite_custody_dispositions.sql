CREATE TABLE offsite_custody_dispositions (
 job_id TEXT PRIMARY KEY REFERENCES offsite_backup_jobs(id),
 archive_id TEXT NOT NULL UNIQUE CHECK(length(archive_id)=43 AND archive_id NOT GLOB '*[^A-Za-z0-9_-]*'),
 archive_manifest_sha256 TEXT NOT NULL CHECK(length(archive_manifest_sha256)=64 AND archive_manifest_sha256 NOT GLOB '*[^a-f0-9]*'),
 note TEXT NOT NULL CHECK(length(note) BETWEEN 10 AND 300),
 archived_at INTEGER NOT NULL CHECK(archived_at>0)
) STRICT;
CREATE TRIGGER verify_offsite_custody_disposition BEFORE INSERT ON offsite_custody_dispositions
WHEN NOT EXISTS(
 SELECT 1 FROM offsite_backup_jobs j WHERE j.id=NEW.job_id AND j.state='FAILED' AND j.attempts=5
 AND j.lease_token='' AND j.lease_until=0 AND j.object_version='' AND j.acknowledged_at=0
 AND length(j.encrypted_sha256)=64 AND j.encrypted_sha256 NOT GLOB '*[^a-f0-9]*'
 AND j.encrypted_bytes>0 AND j.captured_at<=NEW.archived_at
 AND j.error_code IN('CAPTURE_FAILED','UPLOAD_DENIED','UPLOAD_REJECTED','LEASE_EXPIRED','STAGING_FULL')
)
BEGIN SELECT RAISE(ABORT,'only an exhausted known-failure encrypted candidate can be archived'); END;
CREATE TRIGGER preserve_offsite_custody_disposition_update BEFORE UPDATE ON offsite_custody_dispositions
BEGIN SELECT RAISE(ABORT,'offsite custody disposition is immutable'); END;
CREATE TRIGGER preserve_offsite_custody_disposition_delete BEFORE DELETE ON offsite_custody_dispositions
BEGIN SELECT RAISE(ABORT,'offsite custody evidence is preserved'); END;
CREATE TRIGGER preserve_custodied_offsite_job BEFORE UPDATE ON offsite_backup_jobs
WHEN EXISTS(SELECT 1 FROM offsite_custody_dispositions WHERE job_id=OLD.id)
BEGIN SELECT RAISE(ABORT,'the independently archived original job is preserved'); END;
