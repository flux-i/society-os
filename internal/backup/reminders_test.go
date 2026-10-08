package backup

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/messaging"
	"society.local/portal/internal/security"
)

func TestReminderRecoveryRetainsExactFinancialBindingsAcknowledgementUnknownProofOriginalReceiptAndHeldKeys(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	check := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	check(s.SeedDemoAccounts(ctx))
	check(s.SeedDemoTreasury(ctx))
	var e error
	s.MFA, e = security.LoadKey(filepath.Join(root, "keys", "mfa.key"), true)
	check(e)
	key, e := messaging.LoadKey(filepath.Join(root, "keys", "messages.key"), true)
	check(e)
	engine, e := messaging.New(s, key)
	check(e)
	check(engine.VerifyKey(ctx, true))
	a := upkeepRecoveryLogin(t, s, "admin@demo.society")
	_, e = s.GrantAppointment(ctx, a, "demo-user-committee", database.AppointmentInput{AccessChange: database.AccessChange{Version: 1, Confirmed: true, Reason: "Verified fictional separate finance reviewer for recovery"}, Role: "TREASURER", TermDays: 30})
	check(e)
	b, o := upkeepRecoveryLogin(t, s, "committee@demo.society"), upkeepRecoveryLogin(t, s, "owner@demo.society")
	_, e = s.RegisterContact(ctx, o, "me", database.ContactInput{OperationKey: "reminder-recovery-contact-12345", Email: "PRIVATE-reminder-recovery@example.test", PreferredChannel: "EMAIL", ConsentSource: "PRIVATE supplied separate finance and community preferences", Reason: "PRIVATE deliberate fictional contact choice", ContactPreferences: database.ContactPreferences{CommunityEmail: true, FinanceEmail: true}, Confirmed: true})
	check(e)
	_, e = s.ActOnContact(ctx, b, "demo-owner-A-101", database.ContactAction{OperationKey: "reminder-recovery-verify-12345", Version: 1, Action: "VERIFIED", Reason: "PRIVATE independently verified this fictional destination", Confirmed: true})
	check(e)
	cycle, e := s.CreateMaintenanceCycle(ctx, a, database.MaintenanceInput{OperationKey: "reminder-recovery-maintenance-12345", Title: "Fictional recovery maintenance", PeriodStart: "2026-01-01", PeriodEnd: "2026-01-31", DueDate: "2026-01-10", SourceReference: "PRIVATE supplied recovery obligation", Lines: []database.MaintenanceLineInput{{FlatID: "demo-flat-A-101", Amount: "1000.00"}}, Confirmed: true})
	check(e)
	_, e = s.DecideMaintenanceCycle(ctx, b, cycle, database.MaintenanceAction{OperationKey: "reminder-recovery-maintenance-review-12345", Version: 1, Decision: "PUBLISHED", Reason: "PRIVATE separately reviewed the supplied obligation", Confirmed: true})
	check(e)
	details, e := s.MaintenanceCycleFor(ctx, a, cycle, 1, 1)
	check(e)
	entry, e := s.CreateEntry(ctx, a, database.EntryInput{OperationKey: "reminder-recovery-money-12345", FlatID: "demo-flat-A-101", Kind: "RECEIVED", Amount: "432.19", Date: "2026-01-01", Description: "Fictional original received amount to preserve", Payer: "Fictional owner", Method: "CASH"})
	check(e)
	_, e = s.PostEntry(ctx, a, entry, database.EntryAction{OperationKey: "reminder-recovery-post-12345", Confirmed: true})
	check(e)
	_, e = s.AllocateCredit(ctx, a, database.CreditAllocationInput{OperationKey: "reminder-recovery-allocate-12345", SourceID: entry, ChargeID: details.Lines[0].EntryID, Amount: "400.00", Reason: "PRIVATE reviewed original receipt and supplied charge", Confirmed: true})
	check(e)
	wantMoney, e := s.EntryFor(ctx, a, entry)
	check(e)
	if wantMoney.AmountPaise != 43219 || wantMoney.ReceiptID == "" {
		t.Fatal(wantMoney)
	}
	input := database.MessageInput{SourceKind: "MAINTENANCE_REMINDER", SourceID: cycle, ReminderBasis: "DEADLINE_PASSED", Channel: "EMAIL", Target: database.MessageTarget{Kind: "HOMES", IDs: []string{"demo-flat-A-101"}}, PortalOrigin: "http://127.0.0.1:8080"}
	preview, e := s.MessagePreviewFor(ctx, a, input, 1)
	check(e)
	bindingFor := func(items []database.MessageRecipient) *database.MessageReminderBinding {
		t.Helper()
		for _, person := range items {
			if person.ID == "demo-owner-A-101" && person.Reminder != nil {
				return person.Reminder
			}
		}
		t.Fatal("explicit owner binding missing")
		return nil
	}
	if preview.Counts.EligiblePeople != 1 || bindingFor(preview.Recipients).OutstandingPaise != 60000 {
		t.Fatal("independent remaining amount", preview)
	}
	input.OperationKey, input.PreviewHash, input.Reason, input.Confirmed = "reminder-recovery-propose-12345", preview.PreviewHash, "PRIVATE exact source amounts and private recipient checked", true
	id, e := s.ProposeMessage(ctx, a, input)
	check(e)
	_, e = s.ActOnMessage(ctx, b, id, database.MessageAction{OperationKey: "reminder-recovery-review-12345", Version: 1, Action: "APPROVED", Reason: "PRIVATE independently reviewed the frozen reminder", Confirmed: true})
	check(e)
	claims, _, e := s.ClaimMessageDispatch(ctx, a, id, database.MessageAction{OperationKey: "reminder-recovery-dispatch-12345", Version: 2, Action: "DISPATCH", Outcome: "UNKNOWN", Reason: "PRIVATE deliberate local handoff for retained proof", Confirmed: true})
	check(e)
	if len(claims) != 1 {
		t.Fatal(claims)
	}
	_, e = s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "UNKNOWN")
	check(e)
	check(s.CompleteSyntheticMessage(ctx, claims[0].ID))
	options, e := s.CommunityOptionsFor(ctx, a)
	check(e)
	meeting, e := s.ProposeMeeting(ctx, a, "", database.MeetingInput{OperationKey: "reminder-recovery-meeting-12345", Action: "AGENDA", Title: "Fictional recovery meeting", Body: "The supplied agenda retains its exact personal acknowledgement.", Location: "Fictional community room", Scope: "ALL", AreaKey: options.AreaKey, StartAt: time.Now().Add(time.Hour).Unix(), AckRequired: true, Reason: "PRIVATE exact supplied meeting and audience checked", Confirmed: true})
	check(e)
	_, e = s.DecideMeeting(ctx, b, meeting, database.MeetingAction{OperationKey: "reminder-recovery-meeting-review-12345", Version: 1, Action: "APPROVED", Reason: "PRIVATE separately reviewed exact agenda and audience", Confirmed: true})
	check(e)
	publication, e := s.MeetingFor(ctx, o, meeting, false, 1)
	check(e)
	_, e = s.AcknowledgeMeeting(ctx, o, meeting, database.MeetingAcknowledgementInput{OperationKey: "reminder-recovery-ack-12345", Version: publication.Version, Fingerprint: publication.Acknowledgement.Fingerprint, Confirmed: true})
	check(e)
	want, e := s.MessageFor(ctx, a, id, 1, 1, 1)
	check(e)
	queue, e := s.MessageExceptionsFor(ctx, a, "", 1)
	check(e)
	if queue.Total != 1 || want.Outcomes["UNKNOWN"] != 1 {
		t.Fatal(queue, want)
	}
	bundle := filepath.Join(root, "reminders-checkpoint")
	_, e = Snapshot(ctx, s, bundle, "0.23.0-dev")
	check(e)
	target := filepath.Join(root, "restored-reminders", "society.db")
	_, e = Restore(ctx, bundle, target)
	check(e)
	r, e := database.Open(ctx, target)
	check(e)
	defer r.Close()
	r.MFA = s.MFA
	check(r.VerifySchema(ctx))
	check(r.VerifyMFAKey(ctx))
	restoredEngine, e := messaging.New(r, key)
	check(e)
	check(restoredEngine.VerifyKey(ctx, false))
	for _, table := range []string{"sessions", "account_tokens", "mfa_pending", "mfa_recovery_codes"} {
		var n int
		check(r.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n))
		if n != 0 {
			t.Fatal("temporary credential restored", table, n)
		}
	}
	if _, e = r.CheckSession(ctx, a); !errors.Is(e, database.ErrUnauthenticated) {
		t.Fatal("old session restored", e)
	}
	freshA, freshO := upkeepRecoveryLogin(t, r, "admin@demo.society"), upkeepRecoveryLogin(t, r, "owner@demo.society")
	got, e := r.MessageFor(ctx, freshA, id, 1, 1, 1)
	check(e)
	if !reflect.DeepEqual(want, got) {
		t.Fatal("binding/source/outcome original changed", want, got)
	}
	gotMoney, e := r.EntryFor(ctx, freshA, entry)
	check(e)
	if !reflect.DeepEqual(wantMoney, gotMoney) {
		t.Fatal("original receipt changed", gotMoney)
	}
	ack, e := r.MeetingFor(ctx, freshO, meeting, false, 1)
	check(e)
	if !ack.Acknowledgement.Acknowledged {
		t.Fatal("exact personal original lost", ack)
	}
	_, e = r.ReconcileMessage(ctx, freshA, id, got.Deliveries[0].ID, database.MessageAction{OperationKey: "reminder-recovery-reconcile-12345", Version: got.Version, Action: "RECONCILE", Reason: "PRIVATE reconcile the restored existing handoff only", Confirmed: true})
	check(e)
	got, e = r.MessageFor(ctx, freshA, id, 1, 1, 1)
	check(e)
	if got.Outcomes["ACCEPTED"] != 1 || got.Deliveries[0].Attempts != 1 || bindingFor(got.Recipients).OutstandingPaise != 60000 {
		t.Fatal("restored retry duplicated or changed original", got)
	}
	var count int
	check(r.DB.QueryRow("SELECT COUNT(*) FROM message_attempts").Scan(&count))
	if count != 1 {
		t.Fatal("restored handoff duplicated", count)
	}
}
