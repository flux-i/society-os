package database

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

var custodyDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

type OffsiteCustodyInput struct {
	JobID, ArchiveID, ManifestSHA256, CipherSHA256, ConfigHash, Note string
	CipherBytes, CapturedAt                                          int64
}

type OffsiteCustodyDisposition struct {
	JobID          string `json:"job_id"`
	ArchiveID      string `json:"archive_id"`
	ManifestSHA256 string `json:"archive_manifest_sha256"`
	Note           string `json:"note"`
	ArchivedAt     int64  `json:"archived_at"`
}

func OffsiteCustodyEligible(j OffsiteJob) bool {
	knownFailure := j.ErrorCode == "CAPTURE_FAILED" || j.ErrorCode == "UPLOAD_DENIED" || j.ErrorCode == "UPLOAD_REJECTED" || j.ErrorCode == "LEASE_EXPIRED" || j.ErrorCode == "STAGING_FULL"
	return j.State == "FAILED" && j.Attempts == 5 && j.Lease == "" && j.LeaseUntil == 0 &&
		j.ObjectVersion == "" && j.AcknowledgedAt == 0 && custodyDigest.MatchString(j.SHA256) && j.Bytes > 0 && knownFailure
}

func (s *Store) OffsiteCustodyFor(ctx context.Context, id string) (OffsiteCustodyDisposition, error) {
	var out OffsiteCustodyDisposition
	if !operationPattern.MatchString(id) {
		return out, ErrInvalid
	}
	err := s.DB.QueryRowContext(ctx, "SELECT job_id,archive_id,archive_manifest_sha256,note,archived_at FROM offsite_custody_dispositions WHERE job_id=?", id).
		Scan(&out.JobID, &out.ArchiveID, &out.ManifestSHA256, &out.Note, &out.ArchivedAt)
	return out, err
}

// Offline custody calls this only after syncing and independently decrypting the
// exact retained copy. No HTTP or native mutation exposes this method.
func (s *Store) RecordOffsiteCustody(ctx context.Context, in OffsiteCustodyInput, now time.Time) (OffsiteCustodyDisposition, error) {
	var out OffsiteCustodyDisposition
	if !operationPattern.MatchString(in.JobID) || !operationPattern.MatchString(in.ArchiveID) ||
		!custodyDigest.MatchString(in.ManifestSHA256) || !custodyDigest.MatchString(in.CipherSHA256) ||
		!custodyDigest.MatchString(in.ConfigHash) || in.CipherBytes <= 0 || in.CapturedAt <= 0 || !validText(in.Note, 10, 300) {
		return out, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE offsite_policy SET highest_clock_at=highest_clock_at WHERE id=1"); err != nil {
		return out, err
	}
	j, err := scanOffsiteJob(tx.QueryRowContext(ctx, offsiteJobSelect+"WHERE id=?", in.JobID))
	if err != nil {
		return out, err
	}
	if !OffsiteCustodyEligible(j) || j.SHA256 != in.CipherSHA256 || j.Bytes != in.CipherBytes || j.CapturedAt != in.CapturedAt || j.ConfigHash != in.ConfigHash {
		return out, ErrConflict
	}
	err = tx.QueryRowContext(ctx, "SELECT job_id,archive_id,archive_manifest_sha256,note,archived_at FROM offsite_custody_dispositions WHERE job_id=?", in.JobID).
		Scan(&out.JobID, &out.ArchiveID, &out.ManifestSHA256, &out.Note, &out.ArchivedAt)
	if err == nil {
		if out.ArchiveID != in.ArchiveID || out.ManifestSHA256 != in.ManifestSHA256 || out.Note != in.Note {
			return OffsiteCustodyDisposition{}, ErrConflict
		}
		return out, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	var highest int64
	if err = tx.QueryRowContext(ctx, "SELECT highest_clock_at FROM offsite_policy WHERE id=1").Scan(&highest); err != nil {
		return out, err
	}
	if now.Unix() < highest || now.Unix() < j.CapturedAt {
		return out, ErrBackupProtection
	}
	if _, err = tx.ExecContext(ctx, "UPDATE offsite_policy SET highest_clock_at=? WHERE id=1", now.Unix()); err != nil {
		return out, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO offsite_custody_dispositions VALUES(?,?,?,?,?)", in.JobID, in.ArchiveID, in.ManifestSHA256, in.Note, now.Unix()); err != nil {
		return out, err
	}
	out = OffsiteCustodyDisposition{JobID: in.JobID, ArchiveID: in.ArchiveID, ManifestSHA256: in.ManifestSHA256, Note: in.Note, ArchivedAt: now.Unix()}
	return out, tx.Commit()
}
