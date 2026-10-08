package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func meetingInput(t *testing.T, s *Store, token string) MeetingInput {
	t.Helper()
	options, e := s.CommunityOptionsFor(context.Background(), token)
	if e != nil {
		t.Fatal(e)
	}
	return MeetingInput{OperationKey: randomToken(), Action: "AGENDA", Title: "Fictional community meeting", Body: "The supplied agenda includes maintenance updates and community questions.", Location: "Fictional community room", Scope: "ALL", AreaKey: options.AreaKey, StartAt: time.Now().Add(time.Hour).Unix(), EndAt: time.Now().Add(2 * time.Hour).Unix(), AckRequired: true, AckDeadline: time.Now().Add(24 * time.Hour).Unix(), Reason: "PRIVATE supplied agenda and its exact audience checked", Confirmed: true}
}
func TestMeetingCurrentMembershipDistinctPersonIdentityAndHistoricalResponseCounts(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	ctx := context.Background()
	id := meetingPropose(t, s, a, "", meetingInput(t, s, a))
	meetingApprove(t, s, b, id)
	original := meetingDetail(t, s, o, id, false)
	ack := meetingAck(original)
	first, e := s.AcknowledgeMeeting(ctx, o, id, ack)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(`INSERT INTO users(id,resident_id,login,display_name,password_hash,status,auth_version,created_at,verified_at,password_changed_at,is_demo) SELECT 'meeting-second-owner',resident_id,'meeting-owner@example.test',display_name,password_hash,status,auth_version,created_at,verified_at,password_changed_at,0 FROM users WHERE login='owner@demo.society'`); e == nil || !strings.Contains(e.Error(), "UNIQUE constraint failed: users.resident_id") {
		t.Fatal("existing one-account-per-person constraint lost", e)
	}
	second := meetingAck(meetingDetail(t, s, o, id, false))
	if result, err := s.AcknowledgeMeeting(ctx, o, id, second); err != nil || result != first {
		t.Fatal("same person fresh operation duplicated response", result, err)
	}
	responses, e := s.MeetingResponsesFor(ctx, a, id, 1, 1)
	if e != nil || responses.Expected != 2 || responses.Acknowledged != 1 || responses.HistoricalTotal != 1 {
		t.Fatal("distinct people expected, not accounts/homes", responses, e)
	}
	if _, e = s.AcknowledgeMeeting(ctx, a, id, meetingAck(meetingDetail(t, s, a, id, false))); !errors.Is(e, ErrForbidden) {
		t.Fatal("operational access acknowledged without household", e)
	}
	if _, e = s.DB.Exec("UPDATE flat_memberships SET end_date=? WHERE resident_id=(SELECT resident_id FROM users WHERE login='owner@demo.society')", today()); e != nil {
		t.Fatal(e)
	}
	if _, e = s.MeetingFor(ctx, o, id, false, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("ended member retained published read", e)
	}
	if _, e = s.AcknowledgeMeeting(ctx, o, id, ack); !errors.Is(e, ErrForbidden) {
		t.Fatal("ended member retained accepted retry access", e)
	}
	responses, e = s.MeetingResponsesFor(ctx, a, id, 1, 1)
	if e != nil || responses.Expected != 1 || responses.Acknowledged != 0 || responses.Outstanding != 1 || responses.HistoricalTotal != 1 {
		t.Fatal("current expected/historical response meaning", responses, e)
	}
	if _, e = s.AcknowledgeMeeting(ctx, tenant, id, meetingAck(meetingDetail(t, s, tenant, id, false))); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("UPDATE users SET status='DISABLED',auth_version=auth_version+1 WHERE login='tenant@demo.society'"); e != nil {
		t.Fatal(e)
	}
	responses, e = s.MeetingResponsesFor(ctx, a, id, 1, 1)
	if e != nil || responses.Expected != 0 || responses.Acknowledged != 0 || responses.HistoricalTotal != 2 || len(responses.History) != 2 {
		t.Fatal("inactive person stayed expected or erased evidence", responses, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM meeting_acknowledgements", 2)
}
func TestMeetingConcurrentReviewsAndIdenticalAcknowledgementsAppendOneOutcome(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	id := meetingPropose(t, s, a, "", meetingInput(t, s, a))
	current := meetingDetail(t, s, b, id, true)
	actions := []MeetingAction{meetingDecision(current, "APPROVED"), meetingDecision(current, "DECLINED")}
	var group sync.WaitGroup
	errs := make([]error, 2)
	for i := range actions {
		group.Add(1)
		go func(i int) { defer group.Done(); _, errs[i] = s.DecideMeeting(ctx, b, id, actions[i]) }(i)
	}
	group.Wait()
	successes, conflicts := 0, 0
	for _, e := range errs {
		if e == nil {
			successes++
		} else if errors.Is(e, ErrConflict) {
			conflicts++
		} else {
			t.Fatal("unexpected concurrent decision", e)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("competing decisions did not choose one head", errs)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM meeting_events", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM meeting_versions", 1)
	if meetingDetail(t, s, b, id, true).State == "DECLINED" {
		in := meetingInput(t, s, a)
		in.Version = 2
		meetingPropose(t, s, a, id, in)
		meetingApprove(t, s, b, id)
	}
	published := meetingDetail(t, s, o, id, false)
	ack := meetingAck(published)
	results := make([]string, 2)
	for i := range results {
		group.Add(1)
		go func(i int) { defer group.Done(); results[i], errs[i] = s.AcknowledgeMeeting(ctx, o, id, ack) }(i)
	}
	group.Wait()
	if errs[0] != nil || errs[1] != nil || results[0] == "" || results[0] != results[1] {
		t.Fatal("concurrent identical acknowledgement did not retain one result", results, errs)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM meeting_acknowledgements", 1)
}
func meetingDetail(t *testing.T, s *Store, token, id string, desk bool) MeetingDetail {
	t.Helper()
	x, e := s.MeetingFor(context.Background(), token, id, desk, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func meetingPropose(t *testing.T, s *Store, token, id string, in MeetingInput) string {
	t.Helper()
	x, e := s.ProposeMeeting(context.Background(), token, id, in)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func meetingDecision(x MeetingDetail, action string) MeetingAction {
	return MeetingAction{OperationKey: randomToken(), Version: x.Version, Action: action, Reason: "PRIVATE exact agenda and publication audience independently checked", Confirmed: true}
}
func meetingApprove(t *testing.T, s *Store, token, id string) {
	t.Helper()
	if _, e := s.DecideMeeting(context.Background(), token, id, meetingDecision(meetingDetail(t, s, token, id, true), "APPROVED")); e != nil {
		t.Fatal(e)
	}
}
func meetingAck(x MeetingDetail) MeetingAcknowledgementInput {
	return MeetingAcknowledgementInput{OperationKey: randomToken(), Version: x.Version, Fingerprint: x.Acknowledgement.Fingerprint, Confirmed: true}
}

func TestMeetingSeparatePublicationPersonalAcknowledgementAndCurrentResponses(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	ctx := context.Background()
	in := meetingInput(t, s, a)
	id := meetingPropose(t, s, a, "", in)
	if _, e := s.MeetingFor(ctx, o, id, false, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("private draft disclosed", e)
	}
	if _, e := s.AcknowledgeMeeting(ctx, o, id, MeetingAcknowledgementInput{OperationKey: randomToken(), Version: 1, Fingerprint: strings.Repeat("a", 64), Confirmed: true}); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("draft acknowledged", e)
	}
	if _, e := s.DecideMeeting(ctx, a, id, meetingDecision(meetingDetail(t, s, a, id, true), "APPROVED")); !errors.Is(e, ErrForbidden) {
		t.Fatal("self approval", e)
	}
	approve := meetingDecision(meetingDetail(t, s, b, id, true), "APPROVED")
	for range 2 {
		if _, e := s.DecideMeeting(ctx, b, id, approve); e != nil {
			t.Fatal("exact approval retry", e)
		}
	}
	original := meetingDetail(t, s, o, id, false)
	if original.Version != 1 || original.State != "UPCOMING" || len(original.Snapshot.Homes) != 118 || original.Published != nil || !original.Acknowledgement.Required || !original.Acknowledgement.CanAcknowledge || len(original.Acknowledgement.Fingerprint) != 64 {
		t.Fatal("wrong permitted publication", original)
	}
	encoded, _ := json.Marshal(original)
	for _, secret := range []string{"PRIVATE", "submitted_by", "submitted_at", "reason", "events", "resident_id", "actor_id"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("private publication metadata", secret)
		}
	}
	responses, e := s.MeetingResponsesFor(ctx, a, id, 1, 1)
	if e != nil || responses.Expected != 2 || responses.Acknowledged != 0 || responses.Outstanding != 2 || responses.HistoricalTotal != 0 {
		t.Fatal("distinct current responders must be two people despite owner's two homes", responses, e)
	}
	if _, e = s.MeetingResponsesFor(ctx, o, id, 1, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("resident inspected another person's responses", e)
	}
	ack := meetingAck(original)
	first, e := s.AcknowledgeMeeting(ctx, o, id, ack)
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		retry, err := s.AcknowledgeMeeting(ctx, o, id, ack)
		if err != nil || retry != first {
			t.Fatal("accepted ack retry", retry, err)
		}
	}
	changed := ack
	changed.Fingerprint = strings.Repeat("f", 64)
	if _, e = s.AcknowledgeMeeting(ctx, o, id, changed); !errors.Is(e, ErrConflict) {
		t.Fatal("changed retry accepted", e)
	}
	again := ack
	again.OperationKey = randomToken()
	if x, err := s.AcknowledgeMeeting(ctx, o, id, again); err != nil || x != first {
		t.Fatal("second operation duplicated exact personal response", x, err)
	}
	own := meetingDetail(t, s, o, id, false)
	other := meetingDetail(t, s, tenant, id, false)
	if !own.Acknowledgement.Acknowledged || own.Acknowledgement.CanAcknowledge || own.Acknowledgement.At == 0 || other.Acknowledgement.Acknowledged || !other.Acknowledgement.CanAcknowledge {
		t.Fatal("personal acknowledgement isolation", own.Acknowledgement, other.Acknowledgement)
	}
	responses, e = s.MeetingResponsesFor(ctx, a, id, 1, 1)
	if e != nil || responses.Expected != 2 || responses.Acknowledged != 1 || responses.Outstanding != 1 || responses.HistoricalTotal != 1 || len(responses.History[0].Homes) != 2 {
		t.Fatal("wrong current vs retained response", responses, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM meeting_acknowledgements", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	in.OperationKey = randomToken()
	in.Version = 2
	in.Body = "PRIVATE proposed successor must not change published agenda or acknowledgement state."
	meetingPropose(t, s, a, id, in)
	if got := meetingDetail(t, s, o, id, false); !reflect.DeepEqual(own, got) {
		t.Fatal("pending successor changed published content/time/ack", got)
	}
	page, e := s.MeetingsFor(ctx, o, "PRIVATE", "", false, 1)
	if e != nil || page.Total != 0 {
		t.Fatal("pending search/count leak", page, e)
	}
	if _, e = s.AcknowledgeMeeting(ctx, tenant, id, meetingAck(other)); e != nil {
		t.Fatal("pending replacement blocked predecessor acknowledgement", e)
	}
	meetingApprove(t, s, b, id)
	next := meetingDetail(t, s, o, id, false)
	if next.Version != 3 || next.Acknowledgement.Acknowledged || !next.Acknowledgement.CanAcknowledge || next.Acknowledgement.Fingerprint == own.Acknowledgement.Fingerprint {
		t.Fatal("old acknowledgement represented as successor response", next)
	}
	oldFirst := ack
	oldFirst.OperationKey = randomToken()
	if _, e = s.AcknowledgeMeeting(ctx, o, id, oldFirst); !errors.Is(e, ErrConflict) {
		t.Fatal("new acknowledgement of superseded version", e)
	}
	if result, err := s.AcknowledgeMeeting(ctx, o, id, ack); err != nil || result != first {
		t.Fatal("accepted exact retry should return retained original while current access persists", result, err)
	}
	responses, e = s.MeetingResponsesFor(ctx, a, id, 1, 1)
	if e != nil || responses.Expected != 2 || responses.Acknowledged != 0 || responses.HistoricalTotal != 2 {
		t.Fatal("successor response reset erased history", responses, e)
	}
	for _, query := range []string{"UPDATE meeting_versions SET title='changed original'", "DELETE FROM meeting_versions", "UPDATE meeting_events SET reason='changed reason'", "DELETE FROM meeting_events", "DELETE FROM meeting_resources", "UPDATE meeting_acknowledgements SET occurred_at=0", "DELETE FROM meeting_acknowledgements"} {
		if _, err := s.DB.Exec(query); err == nil {
			t.Fatal("immutable meeting history changed", query)
		}
	}
}
func TestMeetingSuppliedMinutesCancellationWithdrawalAndRetainedOriginalSchedule(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	in := meetingInput(t, s, a)
	in.StartAt = time.Now().Add(-2 * time.Hour).Unix()
	in.EndAt = in.StartAt + 3600
	id := meetingPropose(t, s, a, "", in)
	meetingApprove(t, s, b, id)
	original := meetingDetail(t, s, o, id, false)
	if original.State != "PAST" || original.Snapshot.Minutes != "" || original.Snapshot.HeldAt != 0 {
		t.Fatal("clock manufactured held meeting/minutes", original)
	}
	first, e := s.AcknowledgeMeeting(ctx, o, id, meetingAck(original))
	if e != nil {
		t.Fatal(e)
	}
	minutes := MeetingInput{OperationKey: randomToken(), Version: 2, Action: "MINUTES", HeldAt: time.Now().Add(-30 * time.Minute).Unix(), Minutes: "Supplied fictional minutes record the maintenance discussion and agreed next questions.", AckRequired: true, Reason: "PRIVATE checked the supplied minutes and original scheduled area", Confirmed: true}
	bad := minutes
	bad.HeldAt = time.Now().Add(time.Hour).Unix()
	if _, e = s.ProposeMeeting(ctx, a, id, bad); !errors.Is(e, ErrInvalid) {
		t.Fatal("future held time", e)
	}
	bad = minutes
	bad.HeldAt = in.StartAt - 1
	if _, e = s.ProposeMeeting(ctx, a, id, bad); !errors.Is(e, ErrInvalid) {
		t.Fatal("held before approved start", e)
	}
	bad = minutes
	bad.Scope = "ALL"
	if _, e = s.ProposeMeeting(ctx, a, id, bad); !errors.Is(e, ErrInvalid) {
		t.Fatal("minutes silently changed area", e)
	}
	meetingPropose(t, s, a, id, minutes)
	before := meetingDetail(t, s, o, id, false)
	if before.Snapshot.Minutes != "" || before.Version != 1 || !before.Acknowledgement.Acknowledged {
		t.Fatal("private minutes altered predecessor", before)
	}
	meetingApprove(t, s, b, id)
	published := meetingDetail(t, s, o, id, false)
	if published.State != "MINUTES" || published.Snapshot.HeldAt != minutes.HeldAt || published.Snapshot.Minutes != minutes.Minutes || published.Snapshot.Title != in.Title || published.Snapshot.Body != in.Body || published.Snapshot.Location != in.Location || published.Snapshot.StartAt != in.StartAt || published.Snapshot.EndAt != in.EndAt || len(published.Snapshot.Homes) != 118 || published.Acknowledgement.Acknowledged {
		t.Fatal("minutes lost original schedule/scope or reused acknowledgement", published)
	}
	if second, err := s.AcknowledgeMeeting(ctx, o, id, meetingAck(published)); err != nil || second == first {
		t.Fatal("minutes need their own exact-version response", second, err)
	}
	overview, e := s.OverviewFor(ctx, o, "meetings")
	if e != nil || overview.Counts["upcoming"] != 0 || overview.Counts["acknowledgements_needed"] != 0 {
		t.Fatal("completed personal response still outstanding", overview, e)
	}
	in.OperationKey = randomToken()
	in.Version = 4
	if _, e = s.ProposeMeeting(ctx, a, id, in); !errors.Is(e, ErrInvalid) {
		t.Fatal("minutes relabelled new agenda", e)
	}
	future := meetingInput(t, s, a)
	cancelID := meetingPropose(t, s, a, "", future)
	meetingApprove(t, s, b, cancelID)
	cancellation := MeetingInput{OperationKey: randomToken(), Version: 2, Action: "CANCEL", UpdateText: "The supplied fictional meeting has been cancelled and will be scheduled separately.", Reason: "PRIVATE cancellation text and original scope reviewed", Confirmed: true}
	meetingPropose(t, s, a, cancelID, cancellation)
	if meetingDetail(t, s, o, cancelID, false).State != "UPCOMING" {
		t.Fatal("pending cancellation hid approved agenda")
	}
	meetingApprove(t, s, b, cancelID)
	cancelled := meetingDetail(t, s, o, cancelID, false)
	if cancelled.State != "CANCELLED" || cancelled.Snapshot.StartAt != future.StartAt || cancelled.Snapshot.UpdateText != cancellation.UpdateText || cancelled.Acknowledgement.Required || cancelled.Acknowledgement.CanAcknowledge {
		t.Fatal("cancellation meaning", cancelled)
	}
	if _, e = s.AcknowledgeMeeting(ctx, o, cancelID, meetingAck(cancelled)); !errors.Is(e, ErrForbidden) {
		t.Fatal("cancelled publication acknowledged", e)
	}
	withdrawal := MeetingInput{OperationKey: randomToken(), Version: 4, Action: "WITHDRAW", Reason: "PRIVATE withdraw retained cancellation from new portal access", Confirmed: true}
	meetingPropose(t, s, a, cancelID, withdrawal)
	meetingApprove(t, s, b, cancelID)
	if _, e = s.MeetingFor(ctx, o, cancelID, false, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("withdrawn publication readable", e)
	}
	private := meetingDetail(t, s, a, cancelID, true)
	if private.State != "WITHDRAWN" || private.EventTotal != 6 || private.Events[5].Snapshot.Body != future.Body || private.Events[3].Snapshot.UpdateText != cancellation.UpdateText {
		t.Fatal("withdrawal erased original schedule/history", private)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM meeting_acknowledgements", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}
