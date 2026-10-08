package backup

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/messaging"
	"society.local/portal/internal/security"
)

func TestMeetingRecoveryRetainsPendingAgendaMinutesCancellationPersonalOriginalsAndHeldKeys(t *testing.T) {
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
	a, b, o := upkeepRecoveryLogin(t, s, "admin@demo.society"), upkeepRecoveryLogin(t, s, "committee@demo.society"), upkeepRecoveryLogin(t, s, "owner@demo.society")
	money, e := s.CreateEntry(ctx, a, database.EntryInput{OperationKey: "meeting-recovery-money-12345", FlatID: "demo-flat-A-101", Kind: "RECEIVED", Amount: "432.19", Date: "2026-01-01", Description: "Fictional previously received amount", Payer: "Fictional owner", Method: "CASH"})
	check(e)
	_, e = s.PostEntry(ctx, a, money, database.EntryAction{OperationKey: "meeting-recovery-post-12345", Confirmed: true})
	check(e)
	wantMoney, e := s.EntryFor(ctx, a, money)
	check(e)
	if wantMoney.AmountPaise != 43219 || wantMoney.ReceiptID == "" {
		t.Fatal(wantMoney)
	}
	options, e := s.CommunityOptionsFor(ctx, a)
	check(e)
	sequence := 0
	next := func() string {
		sequence++
		return fmt.Sprintf("meeting-recovery-%04d", sequence)
	}
	propose := func(id string, in database.MeetingInput) string {
		t.Helper()
		in.OperationKey = next()
		in.Confirmed = true
		in.Reason = "PRIVATE exact supplied meeting information checked"
		result, e := s.ProposeMeeting(ctx, a, id, in)
		check(e)
		return result
	}
	approve := func(id string, version int) {
		t.Helper()
		_, e := s.DecideMeeting(ctx, b, id, database.MeetingAction{OperationKey: next(), Version: version, Action: "APPROVED", Reason: "PRIVATE separately reviewed the exact meeting version", Confirmed: true})
		check(e)
	}
	acknowledge := func(id string) {
		t.Helper()
		x, e := s.MeetingFor(ctx, o, id, false, 1)
		check(e)
		_, e = s.AcknowledgeMeeting(ctx, o, id, database.MeetingAcknowledgementInput{OperationKey: next(), Version: x.Version, Fingerprint: x.Acknowledgement.Fingerprint, Confirmed: true})
		check(e)
	}
	agenda := database.MeetingInput{Action: "AGENDA", Title: "Fictional recovery agenda", Body: "The original agenda and supplied time are retained in recovery.", Location: "Fictional community room", Scope: "ALL", AreaKey: options.AreaKey, StartAt: time.Now().Add(time.Hour).Unix(), AckRequired: true}
	pending := propose("", agenda)
	approve(pending, 1)
	acknowledge(pending)
	agenda.Version = 2
	agenda.Body = "PRIVATE pending replacement which has not been approved."
	propose(pending, agenda)
	agenda.Version = 0
	agenda.StartAt = time.Now().Add(-2 * time.Hour).Unix()
	minutes := propose("", agenda)
	approve(minutes, 1)
	acknowledge(minutes)
	propose(minutes, database.MeetingInput{Version: 2, Action: "MINUTES", HeldAt: time.Now().Add(-time.Hour).Unix(), Minutes: "These are the supplied fictional minutes, separately reviewed and retained.", AckRequired: true})
	approve(minutes, 3)
	acknowledge(minutes)
	agenda.StartAt = time.Now().Add(3 * time.Hour).Unix()
	cancelled := propose("", agenda)
	approve(cancelled, 1)
	propose(cancelled, database.MeetingInput{Version: 2, Action: "CANCEL", UpdateText: "The society supplied this explicit fictional meeting cancellation."})
	approve(cancelled, 3)
	wants := map[string]database.MeetingDetail{}
	for _, id := range []string{pending, minutes, cancelled} {
		x, e := s.MeetingFor(ctx, a, id, true, 1)
		check(e)
		wants[id] = x
	}
	wantPublic, e := s.MeetingFor(ctx, o, pending, false, 1)
	check(e)
	wantMinutes, e := s.MeetingFor(ctx, o, minutes, false, 1)
	check(e)
	wantResponses, e := s.MeetingResponsesFor(ctx, a, minutes, 1, 1)
	check(e)
	if !wantPublic.Acknowledgement.Acknowledged || !wantMinutes.Acknowledgement.Acknowledged || wantResponses.HistoricalTotal != 2 {
		t.Fatal("missing populated originals", wantResponses)
	}
	bundle := filepath.Join(root, "meetings-checkpoint")
	_, e = Snapshot(ctx, s, bundle, "0.22.0-dev")
	check(e)
	approve(pending, 3)
	target := filepath.Join(root, "restored-meetings", "society.db")
	_, e = Restore(ctx, bundle, target)
	check(e)
	r, e := database.Open(ctx, target)
	check(e)
	defer r.Close()
	r.MFA = s.MFA
	for _, table := range []string{"sessions", "account_tokens", "mfa_pending", "mfa_recovery_codes"} {
		var n int
		check(r.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n))
		if n != 0 {
			t.Fatal("temporary credentials revived", table, n)
		}
	}
	if _, e = r.CheckSession(ctx, a); !errors.Is(e, database.ErrUnauthenticated) {
		t.Fatal("old session survived", e)
	}
	check(r.VerifySchema(ctx))
	check(r.VerifyMFAKey(ctx))
	restoredEngine, e := messaging.New(r, key)
	check(e)
	check(restoredEngine.VerifyKey(ctx, false))
	freshA, freshO := upkeepRecoveryLogin(t, r, "admin@demo.society"), upkeepRecoveryLogin(t, r, "owner@demo.society")
	for id, want := range wants {
		got, e := r.MeetingFor(ctx, freshA, id, true, 1)
		check(e)
		if !reflect.DeepEqual(want, got) {
			t.Fatal("meeting original/history changed", id, got)
		}
	}
	for _, tc := range []struct {
		id   string
		want database.MeetingDetail
	}{{pending, wantPublic}, {minutes, wantMinutes}} {
		got, e := r.MeetingFor(ctx, freshO, tc.id, false, 1)
		check(e)
		if !reflect.DeepEqual(tc.want, got) {
			t.Fatal("personal exact publication changed", tc.id, got)
		}
	}
	gotResponses, e := r.MeetingResponsesFor(ctx, freshA, minutes, 1, 1)
	check(e)
	if !reflect.DeepEqual(wantResponses, gotResponses) {
		t.Fatal("current/historical response evidence changed", gotResponses)
	}
	gotMoney, e := r.EntryFor(ctx, freshA, money)
	check(e)
	if !reflect.DeepEqual(wantMoney, gotMoney) {
		t.Fatal("original received money/receipt changed", gotMoney)
	}
	var n int
	check(r.DB.QueryRow("SELECT COUNT(*) FROM meeting_acknowledgements").Scan(&n))
	if n != 3 {
		t.Fatal("personal originals lost or duplicated", n)
	}
}
