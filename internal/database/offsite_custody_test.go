package database

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOffsiteCustodyDispositionReleasesExactlyOneSlotWithoutAcknowledgingTheOriginal(t *testing.T) {
	s, token := recordFixture(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	start := now.Add(-3 * time.Hour)
	bindOffsite(t, s, start)
	var originals []OffsiteJob
	for index := 0; index < 4; index++ {
		captured := start.Add(time.Duration(index) * 30 * time.Minute)
		j, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", captured)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SealOffsite(ctx, j, strings.Repeat("c", 64), 128, captured); err != nil {
			t.Fatal(err)
		}
		if err := s.FinishOffsite(ctx, j, "FAILED", "UPLOAD_DENIED", "", captured); err != nil {
			t.Fatal(err)
		}
		// The domain fixture supplies an exhausted known rejection. Separate
		// worker/protocol tests exercise the actual five signed attempts.
		if _, err := s.DB.Exec("UPDATE offsite_backup_jobs SET attempts=5 WHERE id=?", j.ID); err != nil {
			t.Fatal(err)
		}
		j, err = s.OffsiteJobForCustody(ctx, j.ID)
		if err != nil {
			t.Fatal(err)
		}
		originals = append(originals, j)
	}
	before := moneyRows(t, s)
	var events int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM offsite_backup_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", now); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("Four retained encrypted candidates did not stop a new capture", err)
	}
	j := originals[0]
	in := OffsiteCustodyInput{JobID: j.ID, ArchiveID: randomToken(), ManifestSHA256: strings.Repeat("d", 64), CipherSHA256: j.SHA256, CipherBytes: j.Bytes, CapturedAt: j.CapturedAt, ConfigHash: j.ConfigHash, Note: "Independent fictional custodian retained and verified the original encrypted bytes"}
	saved, err := s.RecordOffsiteCustody(ctx, in, now)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.RecordOffsiteCustody(ctx, in, now.Add(time.Hour))
	if err != nil || saved != again {
		t.Fatal("Exact retry did not return the one immutable disposition", saved, again, err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM offsite_custody_dispositions", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM offsite_backup_events", events)
	for _, original := range originals {
		current, err := s.OffsiteJobForCustody(ctx, original.ID)
		if err != nil || current != original {
			t.Fatal("Custody disposition altered a failed original or its acknowledgement", original, current, err)
		}
	}
	status, err := s.OffsiteStatusFor(ctx, token, "", 1)
	if err != nil || !status.FinancePaused || status.CaptureAt != 0 || status.AcknowledgedAt != 0 || status.ArchivedCount != 1 || status.StagingFull {
		t.Fatal("Independent archival fabricated financial protection", status, err)
	}
	if !reflect.DeepEqual(before, moneyRows(t, s)) {
		t.Fatal("Custody disposition changed a financial/business original")
	}
	fresh, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", now)
	if err != nil {
		t.Fatal("Verified archival did not release exactly one active staging slot", err)
	}
	if fresh.ID == j.ID || fresh.ObjectKey == j.ObjectKey || fresh.CapturedAt != now.Unix() {
		t.Fatal("New scheduled capture reused the exhausted original identity", fresh)
	}
	for _, statement := range []string{
		"UPDATE offsite_custody_dispositions SET note='Changed custody evidence'",
		"DELETE FROM offsite_custody_dispositions",
		"UPDATE offsite_backup_jobs SET error_code='CAPTURE_FAILED' WHERE id='" + j.ID + "'",
	} {
		if _, err := s.DB.Exec(statement); err == nil {
			t.Fatal("Committed custody evidence or its original was mutable", statement)
		}
	}
}

func TestOffsiteCustodyRejectsMismatchedProofAndConcurrentArchivesLeaveOneDisposition(t *testing.T) {
	s, _ := recordFixture(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	bindOffsite(t, s, now.Add(-time.Hour))
	j, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SealOffsite(ctx, j, strings.Repeat("c", 64), 128, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishOffsite(ctx, j, "FAILED", "UPLOAD_DENIED", "", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("UPDATE offsite_backup_jobs SET attempts=5 WHERE id=?", j.ID); err != nil {
		t.Fatal(err)
	}
	j, err = s.OffsiteJobForCustody(ctx, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	valid := OffsiteCustodyInput{JobID: j.ID, ArchiveID: randomToken(), ManifestSHA256: strings.Repeat("d", 64), CipherSHA256: j.SHA256, CipherBytes: j.Bytes, CapturedAt: j.CapturedAt, ConfigHash: j.ConfigHash, Note: "Independent fictional custody proof matched the complete original"}
	for _, change := range []struct {
		name   string
		mutate func(*OffsiteCustodyInput)
	}{
		{"cipher-hash", func(in *OffsiteCustodyInput) { in.CipherSHA256 = strings.Repeat("e", 64) }},
		{"cipher-bytes", func(in *OffsiteCustodyInput) { in.CipherBytes++ }},
		{"capture-time", func(in *OffsiteCustodyInput) { in.CapturedAt++ }},
		{"configuration", func(in *OffsiteCustodyInput) { in.ConfigHash = strings.Repeat("f", 64) }},
		{"archive-identity", func(in *OffsiteCustodyInput) { in.ArchiveID = "invalid" }},
		{"manifest-digest", func(in *OffsiteCustodyInput) { in.ManifestSHA256 = strings.Repeat("z", 64) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			in := valid
			change.mutate(&in)
			if _, err := s.RecordOffsiteCustody(ctx, in, now); err == nil {
				t.Fatal("Mismatched independently supplied proof was recorded")
			}
			maintenanceCount(t, s, "SELECT COUNT(*) FROM offsite_custody_dispositions", 0)
		})
	}
	if _, err := s.RecordOffsiteCustody(ctx, valid, time.Unix(j.CapturedAt-1, 0)); err == nil {
		t.Fatal("Archival accepted a clock before its retained policy high-water mark")
	}
	before := moneyRows(t, s)
	other := valid
	other.ArchiveID, other.ManifestSHA256 = randomToken(), strings.Repeat("e", 64)
	start := make(chan struct{})
	errorsOut := make(chan error, 2)
	var wait sync.WaitGroup
	for _, input := range []OffsiteCustodyInput{valid, other} {
		wait.Add(1)
		go func(in OffsiteCustodyInput) {
			defer wait.Done()
			<-start
			_, err := s.RecordOffsiteCustody(ctx, in, now)
			errorsOut <- err
		}(input)
	}
	close(start)
	wait.Wait()
	close(errorsOut)
	accepted, conflicts := 0, 0
	for err := range errorsOut {
		if err == nil {
			accepted++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal("Unexpected concurrent custody result", err)
		}
	}
	if accepted != 1 || conflicts != 1 {
		t.Fatal("Concurrent copies did not preserve exactly one immutable disposition", accepted, conflicts)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM offsite_custody_dispositions", 1)
	if !reflect.DeepEqual(before, moneyRows(t, s)) {
		t.Fatal("Concurrent custody changed business/financial originals")
	}
	current, err := s.OffsiteJobForCustody(ctx, j.ID)
	if err != nil || current != j {
		t.Fatal("Concurrent custody changed the exhausted original", err)
	}
}

func TestOffsiteCustodyDomainAndSQLBothDenyUncertainActiveUnsealedAndAcknowledgedOriginals(t *testing.T) {
	s, _ := recordFixture(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	bindOffsite(t, s, now.Add(-time.Hour))
	before := moneyRows(t, s)
	base := OffsiteJob{State: "FAILED", Attempts: 5, CapturedAt: now.Add(-time.Hour).Unix(), ConfigHash: testOffsiteBinding, SHA256: strings.Repeat("c", 64), Bytes: 128, ErrorCode: "UPLOAD_DENIED"}
	for _, scenario := range []struct {
		name   string
		change func(*OffsiteJob)
	}{
		{"capturing", func(j *OffsiteJob) { j.State = "CAPTURING" }},
		{"sealed", func(j *OffsiteJob) { j.State = "SEALED" }},
		{"uploading", func(j *OffsiteJob) { j.State = "UPLOADING" }},
		{"uncertain", func(j *OffsiteJob) { j.State, j.ErrorCode = "UNCERTAIN", "UPLOAD_UNCERTAIN" }},
		{"mislabeled-uncertainty", func(j *OffsiteJob) { j.ErrorCode = "UPLOAD_UNCERTAIN" }},
		{"missing-failure-evidence", func(j *OffsiteJob) { j.ErrorCode = "" }},
		{"retryable", func(j *OffsiteJob) { j.Attempts = 4 }},
		{"retained-lease", func(j *OffsiteJob) { j.Lease = randomToken() }},
		{"live-lease", func(j *OffsiteJob) { j.LeaseUntil = now.Add(time.Hour).Unix() }},
		{"no-sealed-copy", func(j *OffsiteJob) { j.SHA256, j.Bytes = "", 0 }},
		{"invalid-sealed-digest", func(j *OffsiteJob) { j.SHA256 = strings.Repeat("z", 64) }},
		{"acknowledgement", func(j *OffsiteJob) { j.AcknowledgedAt = now.Unix() }},
		{"retained-version", func(j *OffsiteJob) { j.ObjectVersion = "independently-retained-version" }},
		{"verified", func(j *OffsiteJob) {
			j.State, j.ErrorCode, j.ObjectVersion, j.AcknowledgedAt = "VERIFIED", "", "original-version", now.Unix()
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			j := base
			j.ID, j.ObjectKey = randomToken(), "recovery/society/"+scenario.name
			scenario.change(&j)
			if _, err := s.DB.Exec(`INSERT INTO offsite_backup_jobs(id,object_key,config_hash,captured_at,state,attempts,available_at,lease_token,lease_until,encrypted_sha256,encrypted_bytes,object_version,acknowledged_at,error_code) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, j.ID, j.ObjectKey, j.ConfigHash, j.CapturedAt, j.State, j.Attempts, j.AvailableAt, j.Lease, j.LeaseUntil, j.SHA256, j.Bytes, j.ObjectVersion, j.AcknowledgedAt, j.ErrorCode); err != nil {
				t.Fatal(err)
			}
			if OffsiteCustodyEligible(j) {
				t.Fatal("Ineligible original passed domain custody gate")
			}
			input := OffsiteCustodyInput{JobID: j.ID, ArchiveID: randomToken(), ManifestSHA256: strings.Repeat("d", 64), CipherSHA256: j.SHA256, CipherBytes: j.Bytes, CapturedAt: j.CapturedAt, ConfigHash: j.ConfigHash, Note: "Independent evidence must never dispose this ineligible original"}
			if _, err := s.RecordOffsiteCustody(ctx, input, now); err == nil {
				t.Fatal("Ineligible original was disposed through the domain")
			}
			if _, err := s.DB.Exec("INSERT INTO offsite_custody_dispositions VALUES(?,?,?,?,?)", j.ID, input.ArchiveID, input.ManifestSHA256, input.Note, now.Unix()); err == nil {
				t.Fatal("Direct SQL bypassed the independent eligibility gate")
			}
			current, err := s.OffsiteJobForCustody(ctx, j.ID)
			if err != nil || current != j {
				t.Fatal("Rejected disposal changed its original", current, err)
			}
			maintenanceCount(t, s, "SELECT COUNT(*) FROM offsite_custody_dispositions", 0)
		})
	}
	if !reflect.DeepEqual(before, moneyRows(t, s)) {
		t.Fatal("Rejected custody changed business originals")
	}
}
