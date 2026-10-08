package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func reminderTestInput(kind, id string) MessageInput {
	in := messageInput(id)
	in.SourceKind = kind
	in.ReminderBasis = "OUTSTANDING"
	in.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}
	return in
}

func TestReminderRefreshCannotReplaceReviewedMeetingPublicationOrExpandHomes(t *testing.T) {
	s, a, b, _ := messageFixture(t)
	ctx := context.Background()
	original := meetingInput(t, s, a)
	original.Scope = "HOMES"
	original.HomeIDs = []string{"demo-flat-A-101"}
	meeting := meetingPropose(t, s, a, "", original)
	meetingApprove(t, s, b, meeting)
	in := reminderTestInput("MEETING_REMINDER", meeting)
	in.Target = MessageTarget{Kind: "ALL"}
	id := messagePropose(t, s, a, in)
	before := messageDetail(t, s, a, id)
	if before.Counts.SourcePeople != 2 || before.Source.Reminder.PublicationVersion != 1 || len(before.Source.PublicationTarget.IDs) != 1 {
		t.Fatal("original frozen publication", before)
	}
	successor := meetingInput(t, s, a)
	successor.Version = 2
	successor.Title = "PRIVATE new society-wide meeting publication"
	meetingPropose(t, s, a, meeting, successor)
	meetingApprove(t, s, b, meeting)
	if _, e := s.ActOnMessage(ctx, a, id, messageDecision(messageDetail(t, s, a, id), "REFRESH")); !errors.Is(e, ErrConflict) {
		t.Fatal("recipient refresh replaced the reviewed publication or expanded its frozen area", e)
	}
	after := messageDetail(t, s, a, id)
	if after.SnapshotVersion != 1 || after.Source.Version != before.Source.Version || len(after.Source.PublicationTarget.IDs) != 1 || after.CanRefresh || !after.CanWithdraw || after.ReviewProblem != "SOURCE_CHANGED" {
		t.Fatal("old binding or safe withdrawal changed", after)
	}
	fresh := reminderPreview(t, s, a, in)
	if fresh.Source.Reminder.PublicationVersion != 3 || fresh.Counts.EligiblePeople != 3 || fresh.Counts.Destinations != 2 {
		t.Fatal("deliberate new proposal cannot select new publication", fresh)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_batches", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_reminder_recipients", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 0)
}
func reminderPreview(t *testing.T, s *Store, token string, in MessageInput) MessagePreview {
	t.Helper()
	x, e := s.MessagePreviewFor(context.Background(), token, in, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func reminderDispatch(t *testing.T, s *Store, token, id, outcome string) []MessageClaim {
	t.Helper()
	ctx := context.Background()
	action := messageDecision(messageDetail(t, s, token, id), "DISPATCH")
	action.Outcome = outcome
	claims, result, e := s.ClaimMessageDispatch(ctx, token, id, action)
	if e != nil || result != id {
		t.Fatal(claims, result, e)
	}
	for _, claim := range claims {
		if _, e = s.SyntheticMessageHandoff(ctx, token, claim.ID, outcome); e != nil {
			t.Fatal(e)
		}
		if e = s.CompleteSyntheticMessage(ctx, claim.ID); e != nil {
			t.Fatal(e)
		}
	}
	again, result, e := s.ClaimMessageDispatch(ctx, token, id, action)
	if e != nil || len(again) != 0 || result != id {
		t.Fatal("same dispatch duplicated", again, result, e)
	}
	return claims
}
func TestReminderMaintenanceIndependentPaiseCreditsMultipleHomesAndExactSelectedArea(t *testing.T) {
	s, a, _, _ := messageFixture(t)
	b := maintenanceReviewer(t, s, a)
	ctx := context.Background()
	cycle := publishMaintenance(t, s, a, b, maintenanceProposal())
	receipt := post(t, s, a, received("600.00"))
	allocate(t, s, a, receipt, cycle.Lines[0].EntryID, "400.00")
	in := reminderTestInput("MAINTENANCE_REMINDER", cycle.ID)
	in.Target = MessageTarget{Kind: "OWNERS"}
	in.ReminderBasis = "DEADLINE_PASSED"
	x := reminderPreview(t, s, a, in)
	if x.Counts.TargetPeople != 118 || x.Counts.SourcePeople != 2 || x.Counts.EligiblePeople != 2 || x.Counts.Destinations != 1 || x.Counts.OmittedPeople != 116 {
		t.Fatal("independent distinct-person/destination counts", x.Counts)
	}
	bindings := map[string]*MessageReminderBinding{}
	for _, r := range x.Recipients {
		if r.Reminder != nil {
			bindings[r.ID] = r.Reminder
		}
	}
	if bindings["demo-owner-A-101"] == nil || bindings["demo-owner-A-101"].OutstandingPaise != 135025 || len(bindings["demo-owner-A-101"].Obligations) != 2 || bindings["demo-joint-owner"].OutstandingPaise != 60000 {
		t.Fatal("unused20000credit must not settle or double-count any obligation", bindings)
	}
	in.Target = MessageTarget{Kind: "HOMES", IDs: []string{"demo-flat-A-101"}}
	x = reminderPreview(t, s, a, in)
	for _, r := range x.Recipients {
		if r.Reminder == nil || r.Reminder.OutstandingPaise != 60000 || len(r.Reminder.Obligations) != 1 || len(r.Reminder.Homes) != 1 {
			t.Fatal("selected home expanded to another home", r)
		}
	}
	if strings.Contains(x.Envelope, "600") || strings.Contains(x.Envelope, "1350") || strings.Contains(x.Envelope, cycle.Title) || strings.Contains(x.Envelope, "Demo Owner") {
		t.Fatal("private amount/title/person in shared envelope", x.Envelope)
	}
	id := messageApproved(t, s, a, b, in)
	detail := messageDetail(t, s, a, id)
	if len(detail.Recipients) != 2 || detail.Recipients[0].Reminder == nil {
		t.Fatal("exact private binding missing", detail)
	}
	for _, query := range []string{"UPDATE message_reminder_recipients SET outstanding_paise=1", "DELETE FROM message_reminder_recipients"} {
		if _, e := s.DB.Exec(query); e == nil {
			t.Fatal("mutable binding", query)
		}
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	personal := messageDetail(t, s, owner, id)
	encoded, _ := json.Marshal(personal)
	for _, secret := range []string{"outstanding_paise", "fingerprint", "charge_id", "Demo Joint Owner", "135025", "60000"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("private binding in resident delivery", secret, string(encoded))
		}
	}
	if _, e := s.MessagePreviewFor(ctx, owner, in, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("resident composed private reminder", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 0)
}
func TestReminderSeparateReviewChangedAmountsRefreshAndConcurrentExactRetries(t *testing.T) {
	s, a, _, _ := messageFixture(t)
	b := maintenanceReviewer(t, s, a)
	ctx := context.Background()
	cycle := publishMaintenance(t, s, a, b, maintenanceProposal())
	in := reminderTestInput("MAINTENANCE_REMINDER", cycle.ID)
	id := messagePropose(t, s, a, in)
	original := messageDetail(t, s, a, id)
	oldBinding := original.Recipients[0].Reminder.Fingerprint
	if _, e := s.ActOnMessage(ctx, a, id, messageDecision(original, "APPROVED")); !errors.Is(e, ErrForbidden) {
		t.Fatal("self review", e)
	}
	credit := post(t, s, a, received("400.00"))
	allocate(t, s, a, credit, cycle.Lines[0].EntryID, "400.00")
	stale := messageDetail(t, s, b, id)
	if stale.CanApprove || stale.ReviewProblem != "PREVIEW_CHANGED" {
		t.Fatal("amount changed without stale review", stale)
	}
	if _, e := s.ActOnMessage(ctx, b, id, messageDecision(stale, "APPROVED")); !errors.Is(e, ErrConflict) {
		t.Fatal("approved stale amount", e)
	}
	if _, e := s.ActOnMessage(ctx, a, id, messageDecision(messageDetail(t, s, a, id), "REFRESH")); e != nil {
		t.Fatal(e)
	}
	fresh := messageDetail(t, s, b, id)
	if fresh.SnapshotVersion != 2 || fresh.Recipients[0].Reminder.OutstandingPaise != 135025 || fresh.Recipients[0].Reminder.Fingerprint == oldBinding {
		t.Fatal("new current exact binding", fresh)
	}
	var retained string
	if e := s.DB.QueryRow("SELECT fingerprint FROM message_reminder_recipients WHERE batch_id=? AND snapshot_version=1", id).Scan(&retained); e != nil || retained != oldBinding {
		t.Fatal("old review replaced", retained, e)
	}
	action := messageDecision(fresh, "APPROVED")
	errs := make([]error, 2)
	results := make([]string, 2)
	var group sync.WaitGroup
	for i := range errs {
		group.Add(1)
		go func(i int) { defer group.Done(); results[i], errs[i] = s.ActOnMessage(ctx, b, id, action) }(i)
	}
	group.Wait()
	if errs[0] != nil || errs[1] != nil || results[0] != id || results[1] != id {
		t.Fatal("same concurrent approval failed", results, errs)
	}
	action.Reason += " changed"
	if _, e := s.ActOnMessage(ctx, b, id, action); !errors.Is(e, ErrConflict) {
		t.Fatal("changed accepted operation", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_events WHERE action='APPROVED'", 1)
}
func TestReminderBeforeClaimAndBeforeHandoffRejectSettledChangedOrEndedEligibility(t *testing.T) {
	for _, when := range []string{"before_claim", "after_claim"} {
		for _, change := range []string{"SETTLED", "PARTIAL", "MEMBERSHIP", "AUTHORITY"} {
			t.Run(when+"/"+change, func(t *testing.T) {
				s, a, _, _ := messageFixture(t)
				b := maintenanceReviewer(t, s, a)
				ctx := context.Background()
				cycle := publishMaintenance(t, s, a, b, maintenanceProposal())
				in := reminderTestInput("MAINTENANCE_REMINDER", cycle.ID)
				in.Target = MessageTarget{Kind: "HOMES", IDs: []string{"demo-flat-A-101"}}
				id := messageApproved(t, s, a, b, in)
				action := messageDecision(messageDetail(t, s, a, id), "DISPATCH")
				action.Outcome = "ACCEPTED"
				claims := []MessageClaim{}
				if when == "after_claim" {
					var e error
					claims, _, e = s.ClaimMessageDispatch(ctx, a, id, action)
					if e != nil || len(claims) != 1 {
						t.Fatal(claims, e)
					}
				}
				switch change {
				case "SETTLED", "PARTIAL":
					amount := "1000.00"
					if change == "PARTIAL" {
						amount = "0.01"
					}
					credit := post(t, s, a, received(amount))
					allocate(t, s, a, credit, cycle.Lines[0].EntryID, amount)
				case "MEMBERSHIP":
					accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE flat_id='demo-flat-A-101'", today())
				case "AUTHORITY":
					accessExec(t, s, "UPDATE role_grants SET revoked_at=? WHERE user_id='demo-user-committee' AND role='TREASURER'", time.Now().Unix())
				}
				if when == "before_claim" {
					var e error
					claims, _, e = s.ClaimMessageDispatch(ctx, a, id, action)
					if e != nil || len(claims) != 0 {
						t.Fatal("new ineligible claim", claims, e)
					}
				} else {
					handoff, e := s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "ACCEPTED")
					if e != nil || handoff.Outcome != "SKIPPED" {
						t.Fatal("ineligible handoff", handoff, e)
					}
				}
				maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 0)
				if x := messageDetail(t, s, a, id); x.Outcomes["SKIPPED"] != 1 {
					t.Fatal("omission missing", x)
				}
			})
		}
	}
}
func TestReminderFixedFundUnverifiedClaimsExemptionsCorrectionsClosedAndVoluntary(t *testing.T) {
	s, a, _, _ := messageFixture(t)
	b := maintenanceReviewer(t, s, a)
	ctx := context.Background()
	fund := createPublishedFund(t, s, a, b, fundProposal())
	owner := reviewLogin(t, s, "owner@demo.society")
	createFundClaim(t, s, owner, fundClaim(fund.ID, "1000.00", "UNCONFIRMED-REMINDER-CLAIM"))
	in := reminderTestInput("FUND_REMINDER", fund.ID)
	x := reminderPreview(t, s, a, in)
	if x.Recipients[0].Reminder.OutstandingPaise != 150025 {
		t.Fatal("pending report treated as paid", x)
	}
	in.Target = MessageTarget{Kind: "HOMES", IDs: []string{"demo-flat-A-101"}}
	id := messageApproved(t, s, a, b, in)
	waiver, e := s.ProposeFundWaiver(ctx, a, FundWaiverInput{randomToken(), fund.ID, "demo-flat-A-101", 1, "1000.00", "Approved fictional full exemption", "Separately supplied exemption removes the obligation", true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideFundWaiver(ctx, b, waiver, fundDecision("APPROVED", 1)); e != nil {
		t.Fatal(e)
	}
	if claims := reminderDispatch(t, s, a, id, "ACCEPTED"); len(claims) != 0 {
		t.Fatal("exempt reminder handed off", claims)
	}
	if x = reminderPreview(t, s, a, in); x.Counts.EligiblePeople != 0 || x.Counts.Reasons["NO_OUTSTANDING"] != 2 {
		t.Fatal("full exemption still outstanding", x)
	}
	closed := createPublishedFund(t, s, a, b, fundProposal())
	closedInput := reminderTestInput("FUND_REMINDER", closed.ID)
	closedID := messageApproved(t, s, a, b, closedInput)
	if _, e = s.DecideFundCampaign(ctx, a, closed.ID, fundDecision("CLOSED", 2)); e != nil {
		t.Fatal(e)
	}
	if len(reminderDispatch(t, s, a, closedID, "ACCEPTED")) != 0 {
		t.Fatal("closed fund sent new reminder")
	}
	voluntary := fundProposal()
	voluntary.ContributionType = "VOLUNTARY"
	for i := range voluntary.Lines {
		voluntary.Lines[i].Amount = ""
	}
	v := createPublishedFund(t, s, a, b, voluntary)
	if _, e = s.MessagePreviewFor(ctx, a, reminderTestInput("FUND_REMINDER", v.ID), 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("voluntary contribution became debt", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 0)
}
func TestReminderMeetingAcknowledgementAfterClaimSharedDestinationAndCurrentPublication(t *testing.T) {
	s, a, b, _ := messageFixture(t)
	ctx := context.Background()
	input := meetingInput(t, s, a)
	input.AckDeadline = time.Now().Add(-time.Hour).Unix()
	meeting := meetingPropose(t, s, a, "", input)
	meetingApprove(t, s, b, meeting)
	in := reminderTestInput("MEETING_REMINDER", meeting)
	in.Target = MessageTarget{Kind: "OWNERS"}
	in.ReminderBasis = "DEADLINE_PASSED"
	preview := reminderPreview(t, s, a, in)
	if preview.Counts.EligiblePeople != 2 || preview.Counts.Destinations != 1 {
		t.Fatal("current distinct meeting people", preview.Counts)
	}
	id := messageApproved(t, s, a, b, in)
	action := messageDecision(messageDetail(t, s, a, id), "DISPATCH")
	action.Outcome = "ACCEPTED"
	claims, _, e := s.ClaimMessageDispatch(ctx, a, id, action)
	if e != nil || len(claims) != 1 {
		t.Fatal(claims, e)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	if _, e = s.AcknowledgeMeeting(ctx, owner, meeting, meetingAck(meetingDetail(t, s, owner, meeting, false))); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "ACCEPTED"); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteSyntheticMessage(ctx, claims[0].ID); e != nil {
		t.Fatal(e)
	}
	personal := messageDetail(t, s, owner, id)
	if personal.Outcomes["SKIPPED"] != 1 || personal.Outcomes["ACCEPTED"] != 0 {
		t.Fatal("shared send credited acknowledged person", personal)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE state='SKIPPED' AND reason='ACKNOWLEDGED'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE state='HANDED_OFF'", 1)
	input.OperationKey, input.Version, input.Title = randomToken(), 2, "PRIVATE pending successor meeting"
	meetingPropose(t, s, a, meeting, input)
	if x := reminderPreview(t, s, a, in); x.Source.Reminder.PublicationVersion != 1 || x.Counts.EligiblePeople != 1 {
		t.Fatal("pending replacement hid approved predecessor", x)
	}
	old := messageApproved(t, s, a, b, in)
	meetingApprove(t, s, b, meeting)
	if len(reminderDispatch(t, s, a, old, "ACCEPTED")) != 0 {
		t.Fatal("new publication accepted predecessor reminder")
	}
	if x := reminderPreview(t, s, a, in); x.Source.Reminder.PublicationVersion != 3 || x.Counts.EligiblePeople != 2 {
		t.Fatal("successor did not reset exact acknowledgements", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM meeting_acknowledgements", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}
func TestReminderDeadlineAuthorityAndUnknownReconciliationAfterSourceWithdrawal(t *testing.T) {
	s, a, b, _ := messageFixture(t)
	ctx := context.Background()
	meetingInput := meetingInput(t, s, a)
	meetingInput.AckDeadline = 0
	meeting := meetingPropose(t, s, a, "", meetingInput)
	meetingApprove(t, s, b, meeting)
	in := reminderTestInput("MEETING_REMINDER", meeting)
	in.ReminderBasis = "DEADLINE_PASSED"
	if x := reminderPreview(t, s, a, in); x.Counts.EligiblePeople != 0 || x.Counts.Reasons["DEADLINE_NOT_PASSED"] != 1 {
		t.Fatal("missing deadline inferred", x)
	}
	in.ReminderBasis = "OUTSTANDING"
	id := messageApproved(t, s, a, b, in)
	claims := reminderDispatch(t, s, a, id, "UNKNOWN")
	if len(claims) != 1 {
		t.Fatal(claims)
	}
	before := messageDetail(t, s, a, id)
	withdraw := MeetingInput{OperationKey: randomToken(), Version: 2, Action: "WITHDRAW", Reason: "PRIVATE deliberate withdrawal retains the approved original", Confirmed: true}
	meetingPropose(t, s, a, meeting, withdraw)
	meetingApprove(t, s, b, meeting)
	queue, e := s.MessageExceptionsFor(ctx, a, "UNKNOWN", 1)
	if e != nil || queue.Total != 1 || !queue.Items[0].CanReconcile || queue.Items[0].CanRetry || queue.Items[0].EligibilityProblem != "SOURCE_UNAVAILABLE" {
		t.Fatal("existing uncertain proof lost with source", queue, e)
	}
	action := messageDecision(messageDetail(t, s, a, id), "RECONCILE")
	delivery := before.Deliveries[0].ID
	for i := 0; i < 2; i++ {
		if _, e = s.ReconcileMessage(ctx, a, id, delivery, action); e != nil {
			t.Fatal(e)
		}
	}
	if got := messageDetail(t, s, a, id); got.Outcomes["ACCEPTED"] != 1 || got.Deliveries[0].Attempts != 1 || got.ReminderCurrentKey == before.ReminderCurrentKey {
		t.Fatal("same attempt not retained", got)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempts", 1)
	if _, e = s.MessageSourcesFor(ctx, b, "MAINTENANCE_REMINDER", "", 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("registry/community implied finance", e)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	if _, e = s.MessageExceptionsFor(ctx, owner, "", 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("resident saw private exception queue", e)
	}
}
