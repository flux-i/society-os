package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func communityInput(kind string) CommunityInput {
	in := CommunityInput{OperationKey: randomToken(), Kind: kind, Action: "PUBLISH", Title: "Fictional water service", Body: "The society supplied this fictional service information for residents.", Service: "WATER", Scope: "ALL", AreaKey: strings.Repeat("a", 64), Reason: "PRIVATE checked the fictional supplied service information", Confirmed: true}
	if kind == "CONTACT" {
		in.Phone = "+91 (90000) 00101"
		in.Availability = "Weekdays 09:00–17:00; supplied hours"
		in.Attestation = "PRIVATE supplied contact permission reference C-21"
	} else {
		in.StartAt = time.Now().Add(-time.Hour).Unix()
		in.EstimatedEnd = time.Now().Add(-time.Minute).Unix()
	}
	return in
}
func communityTestInput(t *testing.T, s *Store, token, kind string) CommunityInput {
	t.Helper()
	in := communityInput(kind)
	options, e := s.CommunityOptionsFor(context.Background(), token)
	if e != nil {
		t.Fatal(e)
	}
	in.AreaKey = options.AreaKey
	return in
}
func communityDetail(t *testing.T, s *Store, token, id string, desk bool) CommunityDetail {
	t.Helper()
	x, e := s.CommunityResourceFor(context.Background(), token, id, desk, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func communityPropose(t *testing.T, s *Store, token, id string, in CommunityInput) string {
	t.Helper()
	result, e := s.ProposeCommunity(context.Background(), token, id, in)
	if e != nil {
		t.Fatal(e)
	}
	return result
}
func communityDecision(x CommunityDetail, action string) CommunityAction {
	return CommunityAction{OperationKey: randomToken(), Version: x.Version, Action: action, Reason: "PRIVATE independently checked the exact supplied proposal and audience", Confirmed: true}
}
func communityApprove(t *testing.T, s *Store, token, id string) {
	t.Helper()
	if _, e := s.DecideCommunity(context.Background(), token, id, communityDecision(communityDetail(t, s, token, id, true), "APPROVED")); e != nil {
		t.Fatal(e)
	}
}

func TestCommunityContactSeparatePublicationPendingPrivacyAndRetainedOriginals(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	in := communityTestInput(t, s, a, "CONTACT")
	id := communityPropose(t, s, a, "", in)
	if _, e := s.CommunityResourceFor(ctx, o, id, false, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("pending contact visible", e)
	}
	if _, e := s.DecideCommunity(ctx, a, id, communityDecision(communityDetail(t, s, a, id, true), "APPROVED")); !errors.Is(e, ErrForbidden) {
		t.Fatal("self approval", e)
	}
	approve := communityDecision(communityDetail(t, s, b, id, true), "APPROVED")
	for range 2 {
		if _, e := s.DecideCommunity(ctx, b, id, approve); e != nil {
			t.Fatal("accepted decision retry", e)
		}
	}
	want := communityDetail(t, s, o, id, false)
	if want.State != "AVAILABLE" || want.Version != 1 || want.Snapshot.Phone != "+919000000101" || len(want.Snapshot.Homes) != 118 || len(want.Events) > 0 || want.Published != nil {
		t.Fatal("approved public snapshot", want)
	}
	public, _ := json.Marshal(want)
	for _, secret := range []string{"PRIVATE", "submitted_by", "submitted_at", "attestation", "reason", "events", "demo-user"} {
		if strings.Contains(string(public), secret) {
			t.Fatal("private community data", secret, string(public))
		}
	}
	in.OperationKey = randomToken()
	in.Version = 2
	in.Phone = "+919000000202"
	in.Body = "PRIVATE replacement must remain private while it awaits its separate review."
	communityPropose(t, s, a, id, in)
	if got := communityDetail(t, s, o, id, false); !reflect.DeepEqual(want, got) {
		t.Fatal("pending changed public version/time/text", got)
	}
	page, e := s.CommunityFor(ctx, o, "CONTACT", "PRIVATE", "", false, 1)
	if e != nil || page.Total != 0 {
		t.Fatal("pending leaked through search/count", page, e)
	}
	communityApprove(t, s, b, id)
	after := communityDetail(t, s, o, id, false)
	if after.Version != 3 || after.Snapshot.Phone != "+919000000202" {
		t.Fatal("replacement not deliberately published", after)
	}
	history := communityDetail(t, s, a, id, true)
	if history.Version != 4 || history.EventTotal != 4 || history.Events[3].Snapshot.Phone != "+919000000101" || history.Events[1].Snapshot.Phone != "+919000000202" {
		t.Fatal("retained originals", history)
	}
	for _, query := range []string{"UPDATE community_versions SET phone='+919000000999'", "DELETE FROM community_versions", "UPDATE community_events SET reason='changed'", "DELETE FROM community_events", "DELETE FROM community_resources", "UPDATE community_resources SET published_version=1,version=version+1"} {
		if _, e = s.DB.Exec(query); e == nil {
			t.Fatal("immutable history/head changed", query)
		}
	}
	var audit string
	if e = s.DB.QueryRow("SELECT group_concat(reason||before_json||after_json) FROM audit_events WHERE action LIKE 'COMMUNITY_%'").Scan(&audit); e != nil || strings.Contains(audit, "PRIVATE") || strings.Contains(audit, "900000") {
		t.Fatal("broad audit private content", audit, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_batches", 0)
}
func TestCommunityFrozenAreasCanonicalRetriesAndCurrentHouseholdIntersection(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	ctx := context.Background()
	in := communityTestInput(t, s, a, "CONTACT")
	in.Scope = "HOMES"
	in.HomeIDs = []string{"demo-flat-A-202", "demo-flat-A-101"}
	originalIDs := append([]string{}, in.HomeIDs...)
	id := communityPropose(t, s, a, "", in)
	if !reflect.DeepEqual(originalIDs, in.HomeIDs) {
		t.Fatal("normalisation mutated caller")
	}
	in.HomeIDs = []string{"demo-flat-A-101", "demo-flat-A-202"}
	if again, e := s.ProposeCommunity(ctx, a, "", in); e != nil || again != id {
		t.Fatal("equivalent retry selection", again, e)
	}
	in.HomeIDs = []string{"demo-flat-A-101"}
	if _, e := s.ProposeCommunity(ctx, a, "", in); !errors.Is(e, ErrConflict) {
		t.Fatal("changed identity accepted", e)
	}
	communityApprove(t, s, b, id)
	x := communityDetail(t, s, o, id, false)
	if len(x.Snapshot.Homes) != 2 {
		t.Fatal("area snapshot", x)
	}
	if _, e := s.CommunityResourceFor(ctx, tenant, id, false, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("unrelated household", e)
	}
	accessExec(t, s, "INSERT INTO flat_memberships VALUES('community-current-home','demo-flat-A-101','demo-tenant-A-103','TENANT','2020-01-01',NULL,0,0)")
	if got := communityDetail(t, s, tenant, id, false); got.ID != id {
		t.Fatal(got)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date='2026-01-01' WHERE id='community-current-home'")
	if _, e := s.CommunityResourceFor(ctx, tenant, id, false, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("ended membership", e)
	}
	wing := communityTestInput(t, s, a, "INTERRUPTION")
	wing.Scope = "WING"
	wing.BuildingCode = "B"
	wingID := communityPropose(t, s, a, "", wing)
	communityApprove(t, s, b, wingID)
	wx := communityDetail(t, s, a, wingID, false)
	if len(wx.Snapshot.Homes) != 40 || wx.Snapshot.Homes[0].Label[:2] != "B-" {
		t.Fatal("independent wing size", wx)
	}
	allID := communityPropose(t, s, a, "", communityTestInput(t, s, a, "CONTACT"))
	communityApprove(t, s, b, allID)
	before := communityDetail(t, s, o, allID, false)
	accessExec(t, s, "INSERT INTO flats(id,building_id,flat_number,floor,status) SELECT 'community-new-flat',building_id,'999',9,'VACANT' FROM flats WHERE id='demo-flat-A-101'")
	if after := communityDetail(t, s, o, allID, false); !reflect.DeepEqual(before, after) || len(after.Snapshot.Homes) != 118 {
		t.Fatal("all area silently expanded", after)
	}
}
func TestCommunityInterruptionEstimatesResolutionAndOverviewUseApprovedTime(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	in := communityTestInput(t, s, a, "INTERRUPTION")
	id := communityPropose(t, s, a, "", in)
	communityApprove(t, s, b, id)
	want := communityDetail(t, s, o, id, false)
	if want.State != "UPDATE_NEEDED" || want.Snapshot.ResolvedAt != 0 {
		t.Fatal("estimate fabricated restoration", want)
	}
	future := communityTestInput(t, s, a, "INTERRUPTION")
	future.Title = "Future fictional power work"
	future.Service = "POWER"
	future.StartAt = time.Now().Add(time.Hour).Unix()
	future.EstimatedEnd = 0
	planned := communityPropose(t, s, a, "", future)
	communityApprove(t, s, b, planned)
	overview, e := s.OverviewFor(ctx, o, "community")
	if e != nil || overview.Counts["active"] != 1 || overview.Counts["update_needed"] != 1 || len(overview.Items) != 1 || overview.Items[0].ID != id {
		t.Fatal("independent current interruption count", overview, e)
	}
	resolution := CommunityInput{OperationKey: randomToken(), Version: 2, Action: "RESOLVE", ResolvedAt: time.Now().Add(-30 * time.Second).Unix(), UpdateText: "The supplied operator update confirms that the fictional water service is restored.", Reason: "PRIVATE verified supplied restoration time and original area", Confirmed: true}
	communityPropose(t, s, a, id, resolution)
	if got := communityDetail(t, s, o, id, false); !reflect.DeepEqual(want, got) {
		t.Fatal("pending restoration hid active original", got)
	}
	review, e := s.OverviewFor(ctx, b, "community")
	if e != nil || review.Counts["active"] != 1 || review.Counts["needs_your_decision"] != 1 || len(review.Items) != 2 {
		t.Fatal("active and independent review", review, e)
	}
	communityApprove(t, s, b, id)
	got := communityDetail(t, s, o, id, false)
	if got.State != "RESOLVED" || got.Snapshot.ResolvedAt != resolution.ResolvedAt || got.Snapshot.StartAt != in.StartAt || got.Snapshot.EstimatedEnd != in.EstimatedEnd || got.Snapshot.UpdateText != resolution.UpdateText {
		t.Fatal("exact supplied restoration", got)
	}
	overview, e = s.OverviewFor(ctx, o, "community")
	if e != nil || overview.Counts["active"] != 0 || len(overview.Items) != 0 {
		t.Fatal("restored/future still active", overview, e)
	}
	resolution.OperationKey = randomToken()
	resolution.Version = 2
	resolution.ResolvedAt = future.StartAt
	if _, e = s.ProposeCommunity(ctx, a, planned, resolution); !errors.Is(e, ErrInvalid) {
		t.Fatal("future restoration", e)
	}
	in.OperationKey = randomToken()
	in.Version = 4
	if _, e = s.ProposeCommunity(ctx, a, id, in); !errors.Is(e, ErrInvalid) {
		t.Fatal("restored original reopened", e)
	}
	withdraw := CommunityInput{OperationKey: randomToken(), Version: 2, Action: "WITHDRAW", Reason: "PRIVATE supplied withdrawal rather than a service restoration", Confirmed: true}
	communityPropose(t, s, a, planned, withdraw)
	communityApprove(t, s, b, planned)
	if _, e = s.CommunityResourceFor(ctx, o, planned, false, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("withdrawn public access", e)
	}
	private := communityDetail(t, s, a, planned, true)
	if private.State != "WITHDRAWN" || private.Snapshot.ResolvedAt != 0 {
		t.Fatal("withdrawal became restoration", private)
	}
}
func TestCommunityCurrentAuthorityFreshnessAndDeclinedCancelledPredecessor(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	in := communityTestInput(t, s, a, "CONTACT")
	if _, e := s.ProposeCommunity(ctx, o, "", in); !errors.Is(e, ErrForbidden) {
		t.Fatal("resident published", e)
	}
	if _, e := s.CommunityOptionsFor(ctx, o); !errors.Is(e, ErrForbidden) {
		t.Fatal("options inventory", e)
	}
	if _, e := s.CommunityFor(ctx, o, "", "", "", true, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("private desk", e)
	}
	id := communityPropose(t, s, a, "", in)
	approve := communityDecision(communityDetail(t, s, b, id, true), "APPROVED")
	communityApprove(t, s, b, id)
	want := communityDetail(t, s, o, id, false)
	in.Version = 2
	in.OperationKey = randomToken()
	in.Phone = "+919000000202"
	communityPropose(t, s, a, id, in)
	if _, e := s.DecideCommunity(ctx, b, id, communityDecision(communityDetail(t, s, b, id, true), "CANCELLED")); !errors.Is(e, ErrForbidden) {
		t.Fatal("other actor cancelled own proposal", e)
	}
	if _, e := s.DecideCommunity(ctx, b, id, communityDecision(communityDetail(t, s, b, id, true), "DECLINED")); e != nil {
		t.Fatal(e)
	}
	if got := communityDetail(t, s, o, id, false); !reflect.DeepEqual(want, got) {
		t.Fatal("declined replacement changed original", got)
	}
	in.Version = 4
	in.OperationKey = randomToken()
	communityPropose(t, s, a, id, in)
	cancel := communityDecision(communityDetail(t, s, a, id, true), "CANCELLED")
	if _, e := s.DecideCommunity(ctx, a, id, cancel); e != nil {
		t.Fatal(e)
	}
	if got := communityDetail(t, s, o, id, false); !reflect.DeepEqual(want, got) {
		t.Fatal("cancelled replacement changed original", got)
	}
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=0 WHERE token_hash=?", TokenHash(b))
	if _, e := s.DecideCommunity(ctx, b, id, approve); !errors.Is(e, ErrReauthRequired) {
		t.Fatal("stale privilege before retry", e)
	}
	accessExec(t, s, "UPDATE role_grants SET valid_until=? WHERE user_id='demo-user-admin'", time.Now().Add(-time.Second).Unix())
	if _, e := s.ProposeCommunity(ctx, a, "", communityInput("CONTACT")); !errors.Is(e, ErrForbidden) {
		t.Fatal("expired appointment", e)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date='2026-01-01' WHERE resident_id='demo-owner-A-101'")
	if _, e := s.CommunityResourceFor(ctx, o, id, false, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("former reader", e)
	}
	accessExec(t, s, "UPDATE users SET suspended_at=? WHERE id='demo-user-committee'", time.Now().Unix())
	if _, e := s.CommunityResourceFor(ctx, b, id, true, 1); !errors.Is(e, ErrUnauthenticated) {
		t.Fatal("suspended reader", e)
	}
}
func TestCommunityCompetingDecisionsAndRetryIdentityAppendOnce(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	ctx := context.Background()
	id := communityPropose(t, s, a, "", communityTestInput(t, s, a, "CONTACT"))
	x := communityDetail(t, s, b, id, true)
	actions := []CommunityAction{communityDecision(x, "APPROVED"), communityDecision(x, "DECLINED")}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range actions {
		wg.Add(1)
		go func() { defer wg.Done(); _, errs[i] = s.DecideCommunity(ctx, b, id, actions[i]) }()
	}
	wg.Wait()
	winner := -1
	for i, e := range errs {
		if e == nil {
			if winner >= 0 {
				t.Fatal("both decisions succeeded")
			}
			winner = i
		} else if !errors.Is(e, ErrConflict) {
			t.Fatal(e)
		}
	}
	if winner < 0 {
		t.Fatal(errs)
	}
	if _, e := s.DecideCommunity(ctx, b, id, actions[winner]); e != nil {
		t.Fatal("accepted retry", e)
	}
	changed := actions[winner]
	changed.Reason = "PRIVATE different reason under the original retry identity"
	if _, e := s.DecideCommunity(ctx, b, id, changed); !errors.Is(e, ErrConflict) {
		t.Fatal("changed retry", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM community_versions", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM community_events", 2)
	maintenanceCount(t, s, "SELECT version FROM community_resources", 2)
}
func TestCommunityBoundedPagesHistoryAndPublicPrivateSearch(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	var id string
	for i := range 13 {
		in := communityTestInput(t, s, a, "CONTACT")
		in.Title = fmt.Sprintf("Fictional directory service %02d", i)
		id = communityPropose(t, s, a, "", in)
		communityApprove(t, s, b, id)
	}
	page, e := s.CommunityFor(ctx, o, "CONTACT", "directory", "", false, 1)
	if e != nil || page.Total != 13 || len(page.Items) != 12 || page.Counts["AVAILABLE"] != 13 {
		t.Fatal("pagination/count", page, e)
	}
	last, e := s.CommunityFor(ctx, o, "CONTACT", "directory", "", false, 9999)
	if e != nil || last.Page != 2 || len(last.Items) != 1 || last.Total != 13 {
		t.Fatal("bounded/clamped page", last, e)
	}
	for range 11 {
		in := communityTestInput(t, s, a, "CONTACT")
		in.Version = communityDetail(t, s, a, id, true).Version
		communityPropose(t, s, a, id, in)
		communityApprove(t, s, b, id)
	}
	history, e := s.CommunityResourceFor(ctx, a, id, true, 2)
	if e != nil || history.EventTotal != 24 || len(history.Events) != 4 || history.EventPage != 2 || history.Events[3].Version != 1 {
		t.Fatal("history pages retained originals", history, e)
	}
	options, e := s.CommunityOptionsFor(ctx, a)
	if e != nil || len(options.Homes) != 118 || len(options.Buildings) != 3 {
		t.Fatal("flat-only options", options, e)
	}
	blob, _ := json.Marshal(options)
	if strings.Contains(string(blob), "Owner") || strings.Contains(string(blob), "phone") {
		t.Fatal("person inventory in area options")
	}
	for _, f := range []func() error{func() error { _, e := s.CommunityFor(ctx, o, "UNKNOWN", "", "", false, 1); return e }, func() error { _, e := s.CommunityFor(ctx, o, "", "", "PENDING", false, 1); return e }, func() error { _, e := s.CommunityFor(ctx, o, "", "", "", false, 10001); return e }, func() error { _, e := s.CommunityResourceFor(ctx, a, id, true, 0); return e }} {
		if e := f(); !errors.Is(e, ErrInvalid) {
			t.Fatal("unbounded/invalid selector", e)
		}
	}
}
func TestCommunityValidationTimeBoundariesAndNoImplicitRestoration(t *testing.T) {
	now := int64(1791360000)
	x := CommunitySnapshot{Kind: "INTERRUPTION", Action: "PUBLISH", StartAt: now, EstimatedEnd: now + 60}
	for _, tc := range []struct {
		at    int64
		state string
	}{{now - 1, "PLANNED"}, {now, "ACTIVE"}, {now + 59, "ACTIVE"}, {now + 60, "UPDATE_NEEDED"}, {now + 86400, "UPDATE_NEEDED"}} {
		if got := communityState(x, tc.at); got != tc.state {
			t.Fatal(tc, got)
		}
	}
	x.Action = "RESOLVE"
	x.ResolvedAt = now + 60
	if communityState(x, now+61) != "RESOLVED" {
		t.Fatal("explicit restoration")
	}
	contact := communityInput("CONTACT")
	for _, alter := range []func(*CommunityInput){func(x *CommunityInput) { x.Phone = "javascript:alert(1)" }, func(x *CommunityInput) { x.Phone = "+91\n9000000101" }, func(x *CommunityInput) { x.Attestation = "" }, func(x *CommunityInput) { x.Availability = "" }, func(x *CommunityInput) { x.Body = "bad\x00control character" }, func(x *CommunityInput) { x.Title = strings.Repeat("界", 121) }, func(x *CommunityInput) { x.HomeIDs = []string{"demo-flat-A-101"} }, func(x *CommunityInput) { x.Scope = "HOMES"; x.HomeIDs = []string{"same", "same"} }, func(x *CommunityInput) { x.Confirmed = false }, func(x *CommunityInput) { x.StartAt = now }} {
		bad := contact
		alter(&bad)
		if _, e := normaliseCommunity(bad); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid contact", bad, e)
		}
	}
	interruption := communityInput("INTERRUPTION")
	for _, alter := range []func(*CommunityInput){func(x *CommunityInput) { x.StartAt = 0 }, func(x *CommunityInput) { x.StartAt = 4102444800 }, func(x *CommunityInput) { x.EstimatedEnd = x.StartAt - 1 }, func(x *CommunityInput) { x.Phone = contact.Phone }, func(x *CommunityInput) { x.Service = "MEDICAL" }, func(x *CommunityInput) { x.ResolvedAt = now }} {
		bad := interruption
		alter(&bad)
		if _, e := normaliseCommunity(bad); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid interruption", bad, e)
		}
	}
}
