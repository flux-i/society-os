package backup

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"society.local/portal/internal/database"
	"society.local/portal/internal/messaging"
	"society.local/portal/internal/security"
)

func TestMessageRecoveryRetainsUnresolvedHandoffConsentPerAttemptProofMoneyAndSeparateSigningKey(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	for _, seed := range []func(context.Context) error{s.SeedDemoAccounts, s.SeedDemoTreasury} {
		if e := seed(ctx); e != nil {
			t.Fatal(e)
		}
	}
	var e error
	s.MFA, e = security.LoadKey(filepath.Join(root, "keys", "mfa.key"), true)
	if e != nil {
		t.Fatal(e)
	}
	key, e := messaging.LoadKey(filepath.Join(root, "keys", "messages.key"), true)
	if e != nil {
		t.Fatal(e)
	}
	engine, e := messaging.New(s, key)
	if e != nil {
		t.Fatal(e)
	}
	if e = engine.VerifyKey(ctx, true); e != nil {
		t.Fatal(e)
	}
	a, b, o := upkeepRecoveryLogin(t, s, "admin@demo.society"), upkeepRecoveryLogin(t, s, "committee@demo.society"), upkeepRecoveryLogin(t, s, "owner@demo.society")
	contact := database.ContactInput{OperationKey: "recovery-message-contact-12345", Email: "recovery-message@example.test", PreferredChannel: "EMAIL", ConsentSource: "PRIVATE deliberate identity and permission source", Reason: "PRIVATE intentionally supplied this fictional communication choice", ContactPreferences: database.ContactPreferences{CommunityEmail: true, FinanceEmail: true}, Confirmed: true}
	if _, e = s.RegisterContact(ctx, o, "me", contact); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnContact(ctx, a, "demo-owner-A-101", database.ContactAction{OperationKey: "recovery-message-contact-review-12345", Version: 1, Action: "VERIFIED", Reason: "PRIVATE independently verified this person, destination and permissions", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	wantContact, e := s.ContactFor(ctx, o, "me", 1)
	if e != nil {
		t.Fatal(e)
	}
	notice, e := s.SubmitReview(ctx, a, "", database.ReviewInput{OperationKey: "recovery-message-notice-12345", Kind: "NOTICE", Title: "Fictional recovery communication", Body: "Fictional approved community wording for the authenticated source audience.", Audience: "ALL_RESIDENTS"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideReview(ctx, b, notice, database.ReviewAction{OperationKey: "recovery-message-notice-review-12345", Version: 1, Decision: "APPROVED", Reason: "Independently checked the original notice audience", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	money, e := s.CreateEntry(ctx, a, database.EntryInput{OperationKey: "recovery-message-money-12345", FlatID: "demo-flat-A-101", Kind: "RECEIVED", Amount: "432.19", Date: "2026-01-01", Description: "Fictional previously received amount to preserve", Payer: "Demo Owner A-101", Method: "BANK_TRANSFER", Reference: "RECOVERY-MESSAGE-PAID"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.PostEntry(ctx, a, money, database.EntryAction{OperationKey: "recovery-message-post-12345", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	wantMoney, e := s.EntryFor(ctx, a, money)
	if e != nil || wantMoney.AmountPaise != 43219 || wantMoney.ReceiptID == "" {
		t.Fatal(wantMoney, e)
	}
	input := database.MessageInput{SourceKind: "NOTICE", SourceID: notice, Channel: "EMAIL", Target: database.MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}, PortalOrigin: "http://127.0.0.1:8080"}
	preview, e := s.MessagePreviewFor(ctx, a, input, 1)
	if e != nil {
		t.Fatal(e)
	}
	input.OperationKey, input.PreviewHash, input.Reason, input.Confirmed = "recovery-message-propose-12345", preview.PreviewHash, "Reviewed the exact fictional content and destination eligibility.", true
	id, e := s.ProposeMessage(ctx, a, input)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnMessage(ctx, b, id, database.MessageAction{OperationKey: "recovery-message-approve-12345", Version: 1, Action: "APPROVED", Reason: "Separately reviewed the frozen proposal and recipients.", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	claimAction := database.MessageAction{OperationKey: "recovery-message-dispatch-12345", Version: 2, Action: "DISPATCH", Outcome: "UNKNOWN", Reason: "Reviewed the approved local simulation handoff.", Confirmed: true}
	claims, _, e := s.ClaimMessageDispatch(ctx, a, id, claimAction)
	if e != nil || len(claims) != 1 {
		t.Fatal(claims, e)
	}
	h, e := s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "UNKNOWN")
	if e != nil {
		t.Fatal(e)
	}
	want, e := s.MessageFor(ctx, a, id, 1, 1, 1)
	if e != nil || want.Outcomes["CLAIMED"] != 1 || want.Deliveries[0].Attempts != 1 {
		t.Fatal(want, e)
	}
	bundle := filepath.Join(root, "message-checkpoint")
	if _, e = Snapshot(ctx, s, bundle, "message-test"); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteSyntheticMessage(ctx, claims[0].ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnContact(ctx, o, "me", database.ContactAction{OperationKey: "recovery-message-later-stop-12345", Version: 2, Action: "OPTED_OUT", Channel: "EMAIL", Purpose: "COMMUNITY", Reason: "PRIVATE later opt-out excluded from the older checkpoint", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(root, "restored", "society.db")
	if _, e = Restore(ctx, bundle, target); e != nil {
		t.Fatal(e)
	}
	r, e := database.Open(ctx, target)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	r.MFA = s.MFA
	for _, table := range []string{"sessions", "account_tokens", "mfa_pending", "mfa_recovery_codes"} {
		var n int
		if e = r.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal("temporary credentials recovered", table, n, e)
		}
	}
	if _, e = r.CheckSession(ctx, a); !errors.Is(e, database.ErrUnauthenticated) {
		t.Fatal("old session survived", e)
	}
	restored, e := messaging.New(r, key)
	if e != nil {
		t.Fatal(e)
	}
	if e = restored.VerifyKey(ctx, false); e != nil {
		t.Fatal("separate held key rejected", e)
	}
	wrong, _ := messaging.New(r, make([]byte, 32))
	if e = wrong.VerifyKey(ctx, true); e == nil {
		t.Fatal("wrong key accepted or rebound")
	}
	fresh := upkeepRecoveryLogin(t, r, "admin@demo.society")
	got, e := r.MessageFor(ctx, fresh, id, 1, 1, 1)
	if e != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("unresolved checkpoint lost", got, e)
	}
	if e = r.RecoverMessageClaims(ctx); e != nil {
		t.Fatal(e)
	}
	got, e = r.MessageFor(ctx, fresh, id, 1, 1, 1)
	if e != nil || got.Outcomes["UNKNOWN"] != 1 || got.CanDispatch {
		t.Fatal("restart invented a resolved send", got, e)
	}
	if _, e = r.ReconcileMessage(ctx, fresh, id, claims[0].DeliveryID, database.MessageAction{OperationKey: "recovery-message-reconcile-12345", Version: 3, Action: "RECONCILE", Reason: "Reconciled the existing durable local provider attempt.", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	got, e = r.MessageFor(ctx, fresh, id, 1, 1, 1)
	if e != nil || got.Outcomes["ACCEPTED"] != 1 || got.Deliveries[0].ProviderID != h.ProviderID || got.Deliveries[0].Attempts != 1 || got.Deliveries[0].DeliveredAt != 0 || got.Deliveries[0].ReadAt != 0 {
		t.Fatal("recovery fabricated send/delivery/read", got, e)
	}
	again, e := r.EntryFor(ctx, fresh, money)
	if e != nil || !reflect.DeepEqual(again, wantMoney) {
		t.Fatal("original amount/receipt changed", again, e)
	}
	owner := upkeepRecoveryLogin(t, r, "owner@demo.society")
	c, e := r.ContactFor(ctx, owner, "me", 1)
	if e != nil || !reflect.DeepEqual(c, wantContact) {
		t.Fatal("checkpoint contact/consent changed", c, e)
	}
	for _, tc := range []struct {
		query string
		want  int
	}{{"SELECT COUNT(*) FROM simulation_messages", 1}, {"SELECT COUNT(*) FROM message_attempts", 1}, {"SELECT COUNT(*) FROM message_attempt_recipients WHERE state='HANDED_OFF'", 1}, {"SELECT COUNT(*) FROM receipts", 1}} {
		var n int
		if e = r.DB.QueryRow(tc.query).Scan(&n); e != nil || n != tc.want {
			t.Fatal(tc.query, n, e)
		}
	}
}
