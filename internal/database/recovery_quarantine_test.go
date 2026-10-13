package database

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func controlledRecoveryOrigin() RecoveryOrigin {
	return RecoveryOrigin{FormatVersion: 1, SourceSHA256: strings.Repeat("a", 64), SourceBytes: 1585152,
		SourceSchema: SchemaVersion, SourceApplication: "0.32.0-dev", SourceCreatedAt: time.Now().Add(-time.Minute).Unix()}
}

func withoutRecoveryRows(rows map[string]string) map[string]string {
	delete(rows, "recovery_epochs")
	delete(rows, "recovery_external_jobs")
	return rows
}

func TestRecoveryQuarantineWithholdsCopiedCurrentAuthorityAndMoneyDespiteFreshProtection(t *testing.T) {
	s, token := recordFixture(t)
	ctx := context.Background()
	originalID := post(t, s, token, received("400.00"))
	original, err := s.EntryFor(ctx, token, originalID)
	if err != nil || original.AmountPaise != 40000 || original.ReceiptID == "" {
		t.Fatal("Independent original receipt was missing", err)
	}
	draft, err := s.CreateEntry(ctx, token, received("400.01"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Second)
	bindOffsite(t, s, now.Add(-time.Minute))
	offsiteAcknowledgement(t, s, now, now)
	maintenanceCount(t, s, "SELECT allowed FROM offsite_financial_protection", 1)
	before := withoutRecoveryRows(moneyRows(t, s))
	old, err := s.CheckSession(ctx, token)
	if err != nil || !old.CanManageRecords {
		t.Fatal("Fixture lacked actual pre-quarantine Treasury authority", err)
	}
	if _, err := s.QuarantineRecovery(ctx, controlledRecoveryOrigin(), now); err != nil {
		t.Fatal(err)
	}
	p, err := s.CheckSession(ctx, token)
	if err != nil || !p.RecoveryRequired || p.ScopeKey == old.ScopeKey || len(p.Roles) != 0 || p.Fresh {
		t.Fatal("Quarantine did not replace the copied authority boundary", err)
	}
	body, _ := json.Marshal(p)
	var fields map[string]any
	json.Unmarshal(body, &fields)
	for key, value := range fields {
		if strings.HasPrefix(key, "can_") && value != false {
			t.Fatal("Checkpoint permission survived quarantine", key)
		}
	}
	for _, check := range []struct {
		name string
		run  func() error
	}{
		{"registry", func() error { _, e := s.SummaryFor(ctx, token); return e }},
		{"household", func() error { _, e := s.FlatFor(ctx, "demo-flat-A-101", token); return e }},
		{"money read", func() error { _, e := s.EntryFor(ctx, token, originalID); return e }},
		{"money post", func() error {
			_, e := s.PostEntry(ctx, token, draft, EntryAction{OperationKey: randomToken(), Confirmed: true})
			return e
		}},
		{"money reversal", func() error {
			_, e := s.ReverseEntry(ctx, token, originalID, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Supplied linked correction waits for current recovery review"})
			return e
		}},
		{"private recovery history", func() error { _, e := s.OffsiteStatusFor(ctx, token, "", 1); return e }},
		{"receipt background work", func() error { _, e := s.ClaimReceipt(ctx, now); return e }},
		{"document background work", func() error { _, e := s.ClaimDocumentCheck(ctx, now); return e }},
		{"statement background work", func() error { _, e := s.ClaimStatementCheck(ctx, now); return e }},
	} {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); !errors.Is(err, ErrRecoveryRequired) {
				t.Fatal("Copied current authority crossed recovery quarantine", err)
			}
		})
	}
	for _, query := range []string{
		"UPDATE entries SET state='POSTED' WHERE id='" + draft + "'",
		"INSERT INTO receipt_counter(year,next_number) VALUES('2099',1)",
		"UPDATE receipt_counter SET next_number=next_number+1",
		"INSERT INTO entry_reversals(entry_id,actor_id,reason,created_at) VALUES('" + originalID + "','" + p.ID + "','Reviewed original correction',unixepoch('now'))",
	} {
		if _, err := s.DB.Exec(query); !IsRecoveryRequired(err) {
			t.Fatal("Direct SQL crossed the independently required money boundary", query, err)
		}
	}
	var amount int64
	var state string
	if err := s.DB.QueryRow("SELECT amount_paise,state FROM entries WHERE id=?", draft).Scan(&amount, &state); err != nil || amount != 40001 || state != "DRAFT" {
		t.Fatal("Denied confirmation altered the independently supplied draft", amount, state, err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entry_reversals", 0)
	if !reflect.DeepEqual(before, withoutRecoveryRows(moneyRows(t, s))) {
		t.Fatal("Quarantine changed original business rows, BLOBs or provenance")
	}
}

func TestRecoveryQuarantineRetainsOldCaptureAndRejectsReplayInsteadOfExpiringItsIdentity(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	start := time.Now().Add(-time.Hour)
	bindOffsite(t, s, start)
	j, err := s.ClaimOffsite(ctx, testOffsiteBinding, "private-retained-capture", start)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.OffsiteJobForCustody(ctx, j.ID)
	if err != nil || before.State != "CAPTURING" {
		t.Fatal(err)
	}
	if _, err := s.QuarantineRecovery(ctx, controlledRecoveryOrigin(), time.Now()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := s.ClaimOffsite(ctx, testOffsiteBinding, "private-retained-capture", time.Now().Add(time.Hour)); !errors.Is(err, ErrRecoveryRequired) {
			t.Fatal("An expired restored capture resumed or reused its original key", err)
		}
		if err := s.RecoverMessageClaims(ctx); err != nil {
			t.Fatal("Quarantined startup did not remain idle", err)
		}
	}
	for _, query := range []string{"UPDATE offsite_backup_jobs SET available_at=0", "UPDATE offsite_backup_jobs SET state='FAILED',lease_token='',lease_until=0,error_code='LEASE_EXPIRED'"} {
		if _, err := s.DB.Exec(query); !IsRecoveryRequired(err) {
			t.Fatal("Direct SQL rewrote the restored capture instead of preserving it", err)
		}
	}
	after, err := s.OffsiteJobForCustody(ctx, j.ID)
	if err != nil || before != after {
		t.Fatal("Original capture identity, lease or state changed", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM offsite_backup_jobs", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM offsite_backup_events", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_external_jobs WHERE job_kind='OFFSITE' AND original_state='CAPTURING' AND disposition='WITHHELD'", 1)
}

func TestRecoveryJournalRejectsInvalidOriginsRollsBackWholeDispositionAndPreservesEpochs(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	bindOffsite(t, s, time.Now().Add(-time.Hour))
	if _, err := s.ClaimOffsite(ctx, testOffsiteBinding, "private-retained-capture", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name string
		run  func(*RecoveryOrigin)
	}{
		{"preassigned identity", func(x *RecoveryOrigin) { x.ID = randomToken() }},
		{"unknown format", func(x *RecoveryOrigin) { x.FormatVersion = 2 }},
		{"nonhex digest", func(x *RecoveryOrigin) { x.SourceSHA256 = strings.Repeat("z", 64) }},
		{"empty source", func(x *RecoveryOrigin) { x.SourceBytes = 0 }},
		{"unbounded source", func(x *RecoveryOrigin) { x.SourceBytes = 512*1024*1024 + 1 }},
		{"older schema", func(x *RecoveryOrigin) { x.SourceSchema-- }},
		{"missing version", func(x *RecoveryOrigin) { x.SourceApplication = "" }},
		{"control version", func(x *RecoveryOrigin) { x.SourceApplication = "unsafe\nversion" }},
		{"missing capture", func(x *RecoveryOrigin) { x.SourceCreatedAt = 0 }},
		{"supplied restore time", func(x *RecoveryOrigin) { x.RestoredAt = time.Now().Unix() }},
	} {
		t.Run(change.name, func(t *testing.T) {
			x := controlledRecoveryOrigin()
			change.run(&x)
			if _, err := s.QuarantineRecovery(ctx, x, time.Now()); !errors.Is(err, ErrInvalid) {
				t.Fatal("Invalid recovery origin was committed", err)
			}
			maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epochs", 0)
		})
	}
	if _, err := s.DB.Exec("CREATE TRIGGER injected_recovery_failure BEFORE INSERT ON recovery_external_jobs BEGIN SELECT RAISE(ABORT,'injected whole-journal failure'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QuarantineRecovery(ctx, controlledRecoveryOrigin(), time.Now()); err == nil || !strings.Contains(err.Error(), "injected whole-journal failure") {
		t.Fatal("Injected disposition failure was not preserved", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epochs", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_external_jobs", 0)
	if _, err := s.DB.Exec("DROP TRIGGER injected_recovery_failure"); err != nil {
		t.Fatal(err)
	}
	first, err := s.QuarantineRecovery(ctx, controlledRecoveryOrigin(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.QuarantineRecovery(ctx, controlledRecoveryOrigin(), time.Now())
	if err != nil || first.ID == second.ID {
		t.Fatal("New recovery replaced its predecessor identity", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epochs", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_external_jobs", 2)
	for _, query := range []string{"UPDATE recovery_epochs SET source_bytes=1", "DELETE FROM recovery_epochs", "UPDATE recovery_external_jobs SET original_state='FAILED'", "DELETE FROM recovery_external_jobs"} {
		if _, err := s.DB.Exec(query); err == nil {
			t.Fatal("Recovery origin or disposition could be silently changed", query)
		}
	}
}
