package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"reflect"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"testing"
	"time"
)

func TestStatementRecoveryPreservesSeparatePublicationOriginalsUnfinishedChecksAndMoneyWithoutSessions(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	for _, seed := range []func(context.Context) error{s.SeedDemoAccounts, s.SeedDemoTreasury} {
		if err := seed(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	a, b, tenant := upkeepRecoveryLogin(t, s, "admin@demo.society"), upkeepRecoveryLogin(t, s, "committee@demo.society"), upkeepRecoveryLogin(t, s, "tenant@demo.society")
	if _, err := s.GrantAppointment(ctx, a, "demo-user-committee", database.AppointmentInput{AccessChange: database.AccessChange{Version: 1, Reason: "Verified a fictional separate finance appointment", Confirmed: true}, Role: "TREASURER", TermDays: 30}); err != nil {
		t.Fatal(err)
	}
	b = upkeepRecoveryLogin(t, s, "committee@demo.society")
	money, err := s.CreateEntry(ctx, a, database.EntryInput{OperationKey: "statement-recovery-money-12345", FlatID: "demo-flat-A-101", Kind: "RECEIVED", Amount: "432.19", Date: "2026-01-01", Description: "Prior money retained through statement recovery", Payer: "Demo Owner A-101", Method: "BANK_TRANSFER", Reference: "FICTIONAL-RECOVERY-STATEMENT"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PostEntry(ctx, a, money, database.EntryAction{OperationKey: "statement-recovery-post-12345", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	wantMoney, err := s.EntryFor(ctx, a, money)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("Description,Amount\nFictional external accounts,50000.00\n")
	sha := sha256.Sum256(data)
	input := database.StatementInput{OperationKey: "statement-recovery-original-12345", Title: "Fictional recovery externally prepared income statement", Kind: "INCOME", PeriodStart: "2026-09-01", PeriodEnd: "2026-09-30", PreparedBy: "Fictional external accountant", Source: "Supplied fictional original September accounts", Filename: "original.csv", Size: int64(len(data)), SHA256: hex.EncodeToString(sha[:]), Confirmed: true, Reason: "Verified fictional original and explicit accounting period"}
	id, err := s.ReserveStatement(ctx, a, input)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteStatement(ctx, a, id, data); err != nil {
		t.Fatal(err)
	}
	job, err := s.ClaimStatementCheck(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishStatementCheck(ctx, job, "text/csv; charset=utf-8", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	x, err := s.StatementFor(ctx, b, id, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideStatement(ctx, b, id, database.StatementAction{OperationKey: "statement-recovery-internal-12345", Version: x.Version, Action: "APPROVED", Confirmed: true, Reason: "Independently checked this external original"}); err != nil {
		t.Fatal(err)
	}
	x, err = s.StatementFor(ctx, a, id, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	pi := database.StatementPublicationInput{OperationKey: "statement-recovery-publication-12345", FileID: id, FileVersion: x.Version, Target: database.MessageTarget{Kind: "TENANTS"}, Confirmed: true, Reason: "Intentionally publish to current tenant relationships"}
	preview, err := s.PreviewStatementPublication(ctx, a, pi)
	if err != nil {
		t.Fatal(err)
	}
	pi.PreviewHash = preview.PreviewHash
	pub, err := s.ProposeStatementPublication(ctx, a, pi)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideStatementPublication(ctx, b, pub, database.StatementAction{OperationKey: "statement-recovery-publish-12345", Version: 1, Action: "PUBLISHED", Confirmed: true, Reason: "Separately checked the exact original and tenant audience"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.DownloadStatement(ctx, tenant, id); err != nil {
		t.Fatal(err)
	}
	input.OperationKey = "statement-recovery-replacement-12345"
	input.Replaces = id
	input.Version = x.Version
	input.Title = "Fictional replacement still in original checking"
	replacement, err := s.ReserveStatement(ctx, a, input)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteStatement(ctx, a, replacement, data); err != nil {
		t.Fatal(err)
	}
	held, err := s.ClaimStatementCheck(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	want, err := s.StatementFor(ctx, a, id, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "statements")
	if _, err = Snapshot(ctx, s, bundle, "statement-test"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "restored", "society.db")
	if _, err = Restore(ctx, bundle, path); err != nil {
		t.Fatal(err)
	}
	restored, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restored.MFA = s.MFA
	if _, err = restored.CheckSession(ctx, a); !errors.Is(err, database.ErrUnauthenticated) {
		t.Fatal("restored session revived", err)
	}
	ra, rt := upkeepRecoveryLogin(t, restored, "admin@demo.society"), upkeepRecoveryLogin(t, restored, "tenant@demo.society")
	got, err := restored.StatementFor(ctx, ra, id, 1, 1, 1)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("statement original, separate review or publication history lost", got, err)
	}
	_, original, err := restored.DownloadStatement(ctx, rt, id)
	if err != nil || !bytes.Equal(original, data) {
		t.Fatal("published original recovery", err)
	}
	if _, err = restored.StatementFor(ctx, rt, replacement, 1, 1, 1); err == nil {
		t.Fatal("unfinished replacement exposed")
	}
	recoveredMoney, err := restored.EntryFor(ctx, ra, money)
	if err != nil || !reflect.DeepEqual(recoveredMoney, wantMoney) || recoveredMoney.AmountPaise != 43219 {
		t.Fatal("external figures changed money", recoveredMoney, err)
	}
	renewed, err := restored.ClaimStatementCheck(ctx, time.Now().Add(31*time.Second))
	if err != nil || renewed.ID != held.ID || renewed.LeaseToken == held.LeaseToken {
		t.Fatal("bounded unfinished check recovery", renewed, err)
	}
	if err = restored.FinishStatementCheck(ctx, held, "text/csv; charset=utf-8", "", time.Now().Add(32*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = restored.FinishStatementCheck(ctx, renewed, "text/csv; charset=utf-8", "", time.Now().Add(32*time.Second)); err != nil {
		t.Fatal(err)
	}
}
