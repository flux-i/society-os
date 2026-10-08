package database

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMeetingOverviewIndependentWindowPersonalAttentionAndSeparateReview(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	ids := []string{}
	for i, hours := range []int{1, 2, -2, 31 * 24, 1} {
		in := meetingInput(t, s, a)
		in.Title = fmt.Sprintf("Fictional attention meeting %d", i)
		in.StartAt = time.Now().Add(time.Duration(hours) * time.Hour).Unix()
		in.EndAt = in.StartAt + 1800
		in.AckRequired = i != 1 && i != 4
		if !in.AckRequired {
			in.AckDeadline = 0
		}
		if i == 4 {
			in.Scope, in.BuildingCode = "WING", "B"
		}
		id := meetingPropose(t, s, a, "", in)
		meetingApprove(t, s, b, id)
		ids = append(ids, id)
	}
	in := meetingInput(t, s, a)
	in.Version = 2
	in.Body = "PRIVATE replacement awaiting a separate reviewer, not the public agenda."
	meetingPropose(t, s, a, ids[0], in)
	x := mustOverview(t, s, o, "meetings")
	if x.Counts["upcoming"] != 2 || x.Counts["acknowledgements_needed"] != 3 || x.Counts["attention_items"] != 4 || len(x.Items) != 4 {
		t.Fatal("independent window, current audience and coalesced personal attention", x)
	}
	seen := map[string]int{}
	for _, item := range x.Items {
		seen[item.ID]++
		if item.ID == ids[4] || item.State == "PENDING" || strings.Contains(item.Title, "PRIVATE") {
			t.Fatal("unrelated/private attention", item)
		}
	}
	if seen[ids[0]] != 1 || seen[ids[1]] != 1 || seen[ids[2]] != 1 || seen[ids[3]] != 1 {
		t.Fatal("upcoming and acknowledgement duplicated one publication", seen)
	}
	y := mustOverview(t, s, b, "meetings")
	if y.Counts["upcoming"] != 3 || y.Counts["acknowledgements_needed"] != 0 || y.Counts["needs_your_decision"] != 1 || y.Counts["attention_items"] != 4 {
		t.Fatal("operator review and publication count meanings", y)
	}
	seen = map[string]int{}
	for _, item := range y.Items {
		seen[item.ID]++
	}
	if seen[ids[0]] != 2 {
		t.Fatal("published reading and private review destinations were conflated", seen)
	}
	y = mustOverview(t, s, a, "meetings")
	if y.Counts["needs_your_decision"] != 0 || y.Counts["awaiting_other_reviewer"] != 1 || y.Counts["attention_items"] != 3 {
		t.Fatal("self review surfaced as permitted decision", y)
	}
	if _, e := s.AcknowledgeMeeting(ctx, o, ids[2], meetingAck(meetingDetail(t, s, o, ids[2], false))); e != nil {
		t.Fatal(e)
	}
	x = mustOverview(t, s, o, "meetings")
	if x.Counts["acknowledgements_needed"] != 2 || x.Counts["attention_items"] != 3 || x.Counts["upcoming"] != 2 {
		t.Fatal("personal response did not remove just its attention", x)
	}
	for range 5 {
		in := meetingInput(t, s, a)
		in.AckRequired, in.AckDeadline = false, 0
		id := meetingPropose(t, s, a, "", in)
		meetingApprove(t, s, b, id)
	}
	x = mustOverview(t, s, o, "meetings")
	if x.Counts["upcoming"] != 7 || x.Counts["attention_items"] != 8 || len(x.Items) != 4 {
		t.Fatal("full counts were derived from bounded rows", x)
	}
}

func TestMeetingFrozenAreasCanonicalRetryAndImmediateCurrentAccess(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	ctx := context.Background()
	in := meetingInput(t, s, a)
	in.Scope, in.HomeIDs = "HOMES", []string{"demo-flat-A-202", "demo-flat-A-101"}
	wantIDs := append([]string{}, in.HomeIDs...)
	id := meetingPropose(t, s, a, "", in)
	if !reflect.DeepEqual(wantIDs, in.HomeIDs) {
		t.Fatal("normalisation mutated caller selection")
	}
	in.HomeIDs = []string{"demo-flat-A-101", "demo-flat-A-202"}
	if again, e := s.ProposeMeeting(ctx, a, "", in); e != nil || again != id {
		t.Fatal("canonical selection retry", again, e)
	}
	in.HomeIDs = []string{"demo-flat-A-101"}
	if _, e := s.ProposeMeeting(ctx, a, "", in); !errors.Is(e, ErrConflict) {
		t.Fatal("changed selection under original operation", e)
	}
	meetingApprove(t, s, b, id)
	if len(meetingDetail(t, s, o, id, false).Snapshot.Homes) != 2 {
		t.Fatal("frozen selection")
	}
	if _, e := s.MeetingFor(ctx, tenant, id, false, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("unrelated household read", e)
	}
	if _, e := s.AcknowledgeMeeting(ctx, tenant, id, meetingAck(meetingDetail(t, s, o, id, false))); !errors.Is(e, ErrForbidden) {
		t.Fatal("another household acknowledgement", e)
	}
	accessExec(t, s, "INSERT INTO flat_memberships VALUES('meeting-current-home','demo-flat-A-101','demo-tenant-A-103','TENANT','2020-01-01',NULL,0,0)")
	ack := meetingAck(meetingDetail(t, s, tenant, id, false))
	if _, e := s.AcknowledgeMeeting(ctx, tenant, id, ack); e != nil {
		t.Fatal(e)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE id='meeting-current-home'", today())
	if _, e := s.AcknowledgeMeeting(ctx, tenant, id, ack); !errors.Is(e, ErrForbidden) {
		t.Fatal("accepted retry bypassed ended membership", e)
	}
	all := meetingInput(t, s, a)
	allID := meetingPropose(t, s, a, "", all)
	meetingApprove(t, s, b, allID)
	want := meetingDetail(t, s, o, allID, false)
	accessExec(t, s, "INSERT INTO flats(id,building_id,flat_number,floor,status) SELECT 'meeting-new-flat',building_id,'998',9,'VACANT' FROM flats WHERE id='demo-flat-A-101'")
	if got := meetingDetail(t, s, o, allID, false); !reflect.DeepEqual(want, got) || len(got.Snapshot.Homes) != 118 {
		t.Fatal("published area silently expanded", got)
	}
	all.OperationKey = randomToken()
	if _, e := s.ProposeMeeting(ctx, a, "", all); !errors.Is(e, ErrConflict) {
		t.Fatal("stale flat-options fingerprint accepted", e)
	}
}

func TestMeetingAuthorityFreshnessAndDeclinedCancelledPredecessor(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	in := meetingInput(t, s, a)
	if _, e := s.ProposeMeeting(ctx, o, "", in); !errors.Is(e, ErrForbidden) {
		t.Fatal("resident prepared publication", e)
	}
	if _, e := s.MeetingsFor(ctx, o, "", "", true, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("resident private desk", e)
	}
	id := meetingPropose(t, s, a, "", in)
	approve := meetingDecision(meetingDetail(t, s, b, id, true), "APPROVED")
	if _, e := s.DecideMeeting(ctx, b, id, approve); e != nil {
		t.Fatal(e)
	}
	want := meetingDetail(t, s, o, id, false)
	for _, decision := range []string{"DECLINED", "CANCELLED"} {
		in.OperationKey = randomToken()
		in.Version = meetingDetail(t, s, a, id, true).Version
		in.Body = "PRIVATE proposed replacement which must never replace the approved agenda."
		meetingPropose(t, s, a, id, in)
		if _, e := s.DecideMeeting(ctx, b, id, meetingDecision(meetingDetail(t, s, b, id, true), "CANCELLED")); !errors.Is(e, ErrForbidden) {
			t.Fatal("other actor cancelled proposal", e)
		}
		token := b
		if decision == "CANCELLED" {
			token = a
		}
		if _, e := s.DecideMeeting(ctx, token, id, meetingDecision(meetingDetail(t, s, token, id, true), decision)); e != nil {
			t.Fatal(e)
		}
		if got := meetingDetail(t, s, o, id, false); !reflect.DeepEqual(want, got) {
			t.Fatal("rejected successor changed approved original", got)
		}
	}
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=0 WHERE token_hash=?", TokenHash(b))
	if _, e := s.DecideMeeting(ctx, b, id, approve); !errors.Is(e, ErrReauthRequired) {
		t.Fatal("accepted decision retry bypassed current freshness", e)
	}
	accessExec(t, s, "UPDATE role_grants SET valid_until=? WHERE user_id='demo-user-admin'", time.Now().Add(-time.Second).Unix())
	if _, e := s.ProposeMeeting(ctx, a, "", meetingInput(t, s, b)); !errors.Is(e, ErrForbidden) {
		t.Fatal("expired appointment prepared meeting", e)
	}
	if _, e := s.MeetingResponsesFor(ctx, a, id, 1, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("expired appointment inspected responses", e)
	}
	accessExec(t, s, "UPDATE users SET suspended_at=? WHERE id='demo-user-committee'", time.Now().Unix())
	if _, e := s.MeetingFor(ctx, b, id, true, 1); !errors.Is(e, ErrUnauthenticated) {
		t.Fatal("suspended reviewer read", e)
	}
}

func TestMeetingValidationSupportedTimeAndExplicitStateBoundaries(t *testing.T) {
	now := int64(1791360000)
	x := MeetingSnapshot{Action: "AGENDA", StartAt: now, EndAt: now + 60}
	for _, tc := range []struct {
		at    int64
		state string
	}{{now - 1, "UPCOMING"}, {now, "PAST"}, {now + 60, "PAST"}, {now + 86400, "PAST"}} {
		if got := meetingState(x, tc.at); got != tc.state || x.HeldAt != 0 || x.Minutes != "" {
			t.Fatal("time passage fabricated meeting outcome", tc, got)
		}
	}
	in := MeetingInput{OperationKey: randomToken(), Action: "AGENDA", Title: "A supplied agenda", Body: "Supplied agenda for the community meeting", Location: "Community room", Scope: "ALL", AreaKey: strings.Repeat("a", 64), StartAt: now, Reason: "PRIVATE checked exact information", Confirmed: true}
	for _, instant := range []int64{946684800, 4102444799} {
		valid := in
		valid.StartAt, valid.EndAt = instant, instant
		if _, e := normaliseMeeting(valid); e != nil {
			t.Fatal("supported inclusive boundary", instant, e)
		}
	}
	for i, alter := range []func(*MeetingInput){
		func(x *MeetingInput) { x.StartAt = 946684799 }, func(x *MeetingInput) { x.StartAt = 4102444800 },
		func(x *MeetingInput) { x.EndAt = x.StartAt - 1 }, func(x *MeetingInput) { x.Title = strings.Repeat("界", 121) },
		func(x *MeetingInput) { x.Body = "bad\x00control character" }, func(x *MeetingInput) { x.Location = "" },
		func(x *MeetingInput) { x.HeldAt = now }, func(x *MeetingInput) { x.Minutes = "Supplied minutes" },
		func(x *MeetingInput) { x.AckDeadline = now }, func(x *MeetingInput) { x.AckRequired = true; x.AckDeadline = 4102444800 },
		func(x *MeetingInput) { x.AreaKey = strings.Repeat("z", 64) }, func(x *MeetingInput) { x.Confirmed = false },
		func(x *MeetingInput) { x.Scope = "HOMES"; x.HomeIDs = []string{"same", "same"} },
		func(x *MeetingInput) { x.HomeIDs = []string{"another-home"} }, func(x *MeetingInput) { x.BuildingCode = "A" },
	} {
		bad := in
		alter(&bad)
		if _, e := normaliseMeeting(bad); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid agenda accepted", i, e)
		}
	}
}

func TestMeetingBoundedPublicationHistoryAndPersonalResponsesKeepIndependentTotals(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	var id string
	for i := range 13 {
		in := meetingInput(t, s, a)
		in.Title = fmt.Sprintf("Fictional paged meeting %02d", i)
		id = meetingPropose(t, s, a, "", in)
		meetingApprove(t, s, b, id)
	}
	x, e := s.MeetingsFor(ctx, o, "paged", "UPCOMING", false, 1)
	if e != nil || x.Total != 13 || len(x.Items) != 12 || x.Counts["UPCOMING"] != 13 {
		t.Fatal("publication page truncated count", x, e)
	}
	x, e = s.MeetingsFor(ctx, o, "paged", "UPCOMING", false, 9999)
	if e != nil || x.Page != 2 || x.Total != 13 || len(x.Items) != 1 {
		t.Fatal("clamped page", x, e)
	}
	accessExec(t, s, `INSERT INTO users(id,resident_id,login,display_name,password_hash,status,auth_version,created_at,verified_at,password_changed_at,is_demo)
	 SELECT 'meeting-page-user-'||r.id,r.id,r.id||'@meeting.example.test',r.full_name,u.password_hash,'ACTIVE',1,u.created_at,u.verified_at,u.password_changed_at,0
	 FROM residents r CROSS JOIN users u WHERE u.login='owner@demo.society' AND r.id NOT IN(SELECT resident_id FROM users WHERE resident_id IS NOT NULL)
	 AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.resident_id=r.id AND m.start_date<=date('now','+330 minutes') AND (m.end_date IS NULL OR m.end_date>date('now','+330 minutes')))
	 ORDER BY r.id LIMIT 21`)
	responses, e := s.MeetingResponsesFor(ctx, a, id, 1, 1)
	if e != nil || responses.Expected != 23 || responses.Outstanding != 23 || len(responses.Items) != 20 {
		t.Fatal("current response total truncated", responses, e)
	}
	responses, e = s.MeetingResponsesFor(ctx, a, id, 9999, 9999)
	if e != nil || responses.Page != 2 || len(responses.Items) != 3 || responses.HistoryPage != 1 || responses.HistoricalTotal != 0 {
		t.Fatal("current response pages", responses, e)
	}
	for i := range 21 {
		if _, e := s.AcknowledgeMeeting(ctx, o, id, meetingAck(meetingDetail(t, s, o, id, false))); e != nil {
			t.Fatal(e)
		}
		if i < 20 {
			in := meetingInput(t, s, a)
			in.Version = meetingDetail(t, s, a, id, true).Version
			meetingPropose(t, s, a, id, in)
			meetingApprove(t, s, b, id)
		}
	}
	responses, e = s.MeetingResponsesFor(ctx, a, id, 1, 1)
	if e != nil || responses.Expected != 23 || responses.Acknowledged != 1 || responses.Outstanding != 22 || responses.HistoricalTotal != 21 || len(responses.History) != 20 {
		t.Fatal("current and historical response totals conflated", responses, e)
	}
	responses, e = s.MeetingResponsesFor(ctx, a, id, 1, 2)
	if e != nil || len(responses.History) != 1 || responses.History[0].Version != 1 {
		t.Fatal("old original response missing", responses, e)
	}
	history, e := s.MeetingFor(ctx, a, id, true, 9999)
	if e != nil || history.EventTotal != 42 || history.EventPage != 3 || len(history.Events) != 2 || history.Events[1].Version != 1 {
		t.Fatal("old publication history missing", history, e)
	}
	for _, check := range []func() error{
		func() error { _, e := s.MeetingsFor(ctx, o, "", "PENDING", false, 1); return e },
		func() error { _, e := s.MeetingsFor(ctx, o, "", "", false, 10001); return e },
		func() error { _, e := s.MeetingFor(ctx, a, id, true, 0); return e },
		func() error { _, e := s.MeetingResponsesFor(ctx, a, id, 1, 10001); return e },
	} {
		if e := check(); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid or unbounded selector", e)
		}
	}
}
