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
	"society.local/portal/internal/messaging"
	"society.local/portal/internal/security"
	"testing"
	"time"
)

func TestStatementRecoveryPreservesPublicationOriginalsUnknownMessageProofUnfinishedChecksAndMoneyWithoutSessions(t *testing.T) {
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
	// A separate consented finance delivery retains uncertain provider proof
	// alongside the publication and independently held key fingerprint.
	messageKey := bytes.Repeat([]byte{77}, 32)
	bound, err := messaging.New(s, messageKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = bound.VerifyKey(ctx, true); err != nil {
		t.Fatal(err)
	}
	contact := database.ContactInput{OperationKey: "statement-recovery-contact-12345", Email: "tenant@example.test", PreferredChannel: "EMAIL", ConsentSource: "Supplied fictional finance permission", Reason: "Deliberate fictional finance contact", Confirmed: true, ContactPreferences: database.ContactPreferences{FinanceEmail: true}}
	if _, err = s.RegisterContact(ctx, tenant, "me", contact); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ActOnContact(ctx, b, "demo-tenant-A-103", database.ContactAction{OperationKey: "statement-recovery-contact-check-12345", Version: 1, Action: "VERIFIED", Confirmed: true, Reason: "Separately checked this tenant's finance permission"}); err != nil {
		t.Fatal(err)
	}
	mi := database.MessageInput{SourceKind: "STATEMENT", SourceID: pub, Channel: "EMAIL", Target: database.MessageTarget{Kind: "ALL"}, PortalOrigin: "http://127.0.0.1:8080"}
	mp, err := s.MessagePreviewFor(ctx, a, mi, 1)
	if err != nil || mp.Counts.SourcePeople != 35 || mp.Counts.Destinations != 1 {
		t.Fatal(mp, err)
	}
	mi.OperationKey, mi.PreviewHash, mi.Confirmed, mi.Reason = "statement-recovery-message-12345", mp.PreviewHash, true, "Deliberately reviewed this exact publication and recipients"
	message, err := s.ProposeMessage(ctx, a, mi)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ActOnMessage(ctx, b, message, database.MessageAction{OperationKey: "statement-recovery-message-approve-12345", Version: 1, Action: "APPROVED", Confirmed: true, Reason: "Separately checked this exact finance delivery"}); err != nil {
		t.Fatal(err)
	}
	claims, _, err := s.ClaimMessageDispatch(ctx, a, message, database.MessageAction{OperationKey: "statement-recovery-message-dispatch-12345", Version: 2, Action: "DISPATCH", Outcome: "UNKNOWN", Confirmed: true, Reason: "Simulated a truthful uncertain handoff"})
	if err != nil || len(claims) != 1 {
		t.Fatal(claims, err)
	}
	proof, err := s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "UNKNOWN")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteSyntheticMessage(ctx, claims[0].ID); err != nil {
		t.Fatal(err)
	}
	wantMessage, err := s.MessageFor(ctx, a, message, 1, 1, 1)
	if err != nil || wantMessage.Outcomes["UNKNOWN"] != 1 {
		t.Fatal(wantMessage, err)
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
	wrong, err := messaging.New(restored, bytes.Repeat([]byte{78}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err = wrong.VerifyKey(ctx, true); err == nil {
		t.Fatal("replacement message key accepted")
	}
	recovered, err := messaging.New(restored, messageKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = recovered.VerifyKey(ctx, false); err != nil {
		t.Fatal("held matching message key not recovered", err)
	}
	gotMessage, err := restored.MessageFor(ctx, ra, message, 1, 1, 1)
	if err != nil || !reflect.DeepEqual(gotMessage, wantMessage) {
		t.Fatal("statement message source or unknown proof lost", gotMessage, err)
	}
	if _, err = restored.ReconcileMessage(ctx, ra, message, claims[0].DeliveryID, database.MessageAction{OperationKey: "statement-recovery-message-reconcile-12345", Version: gotMessage.Version, Action: "RECONCILE", Confirmed: true, Reason: "Reconciled retained original provider proof"}); err != nil {
		t.Fatal(err)
	}
	gotMessage, err = restored.MessageFor(ctx, ra, message, 1, 1, 1)
	if err != nil || gotMessage.Outcomes["ACCEPTED"] != 1 || gotMessage.Deliveries[0].ProviderID != proof.ProviderID || gotMessage.Outcomes["DELIVERED"] != 0 || gotMessage.Outcomes["READ"] != 0 {
		t.Fatal("recovered proof invented delivered/read", gotMessage, err)
	}
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
