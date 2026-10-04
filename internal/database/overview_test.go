package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

func mustOverview(t *testing.T, s *Store, token, section string) Overview {
	t.Helper()
	x, err := s.OverviewFor(context.Background(), token, section)
	if err != nil {
		t.Fatal(section, err)
	}
	if x.Calendar != "Asia/Kolkata" || x.AsOf == 0 || len(x.Items) > 4 {
		t.Fatal("invalid bounded snapshot", x)
	}
	return x
}

func TestOverviewCalendarIncludesIndiaMidnightInsteadOfPreviousUTCDay(t *testing.T) {
	midnight, err := time.Parse(time.RFC3339, "2026-10-04T18:30:01Z")
	if err != nil {
		t.Fatal(err)
	}
	day, start := overviewWindow(midnight)
	if day != "2026-10-05" || start != "2026-09-06" {
		t.Fatal(day, start)
	}
}

func TestOverviewFinanceKeepsPerHomeCreditsSeparateAndExcludesReversalsAndDrafts(t *testing.T) {
	s, treasury := recordFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	post(t, s, treasury, supplied("CHARGE", "100.10"))
	in := supplied("OPENING_CREDIT", "500.00")
	in.FlatID = "demo-flat-A-102"
	post(t, s, treasury, in)
	in = received("25.00")
	in.Date = today()
	payment := post(t, s, treasury, in)
	in = received("9.99")
	in.Date = today()
	reversed := post(t, s, treasury, in)
	if _, err := s.ReverseEntry(ctx, treasury, reversed, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Duplicate fictional confirmation corrected"}); err != nil {
		t.Fatal(err)
	}
	in = supplied("CHARGE", "300.00")
	in.FlatID = "demo-flat-A-103"
	post(t, s, treasury, in)
	if _, err := s.CreateEntry(ctx, treasury, supplied("CHARGE", "777.00")); err != nil {
		t.Fatal(err)
	}
	in = received("20.00")
	in.Date = time.Now().In(societyZone).AddDate(0, 0, -30).Format("2006-01-02")
	post(t, s, treasury, in)
	// A failed PDF is confirmed money, not a failed payment.
	if _, err := s.DB.Exec(`UPDATE receipt_jobs SET state='FAILED' WHERE receipt_id=(SELECT id FROM receipts WHERE entry_id=?)`, payment); err != nil {
		t.Fatal(err)
	}
	x := mustOverview(t, s, owner, "finance")
	for key, want := range map[string]int64{"balance_paise": -44490, "positive_balance_paise": 5510, "credit_balance_paise": 50000, "homes_with_balance": 1, "received_paise": 2500, "awaiting_confirmation": 0, "receipt_failed": 1, "receipt_pending": 1} {
		if x.Counts[key] != want {
			t.Fatalf("%s=%d want %d", key, x.Counts[key], want)
		}
	}
	staff := mustOverview(t, s, treasury, "finance")
	if staff.Counts["positive_balance_paise"] != 35510 || staff.Counts["awaiting_confirmation"] != 1 {
		t.Fatal(staff)
	}
	if _, err := s.OverviewFor(ctx, tenant, "finance"); !errors.Is(err, ErrForbidden) {
		t.Fatal("unentitled finance", err)
	}
	if _, err := s.DB.Exec(`UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OverviewFor(ctx, owner, "finance"); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked finance cached", err)
	}
}

func TestOverviewReviewsCountAllActionableItemsAndExcludeSelfReview(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	var changed string
	for i := 0; i < 6; i++ {
		in := proposal("MAINTENANCE")
		in.Title = fmt.Sprintf("Fictional review request %d", i)
		id, err := s.SubmitReview(ctx, owner, "", in)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			changed = id
		}
	}
	if _, err := s.SubmitReview(ctx, admin, "", proposal("NOTICE")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecideReview(ctx, admin, changed, decision(1, "CHANGES_REQUESTED")); err != nil {
		t.Fatal(err)
	}
	x := mustOverview(t, s, admin, "reviews")
	if x.Counts["needs_your_decision"] != 5 || x.Counts["awaiting_others"] != 1 || len(x.Items) != 4 {
		t.Fatal(x)
	}
	x = mustOverview(t, s, owner, "reviews")
	if x.Counts["needs_your_decision"] != 0 || x.Counts["awaiting_others"] != 5 || x.Counts["changes_requested"] != 1 || len(x.Items) != 1 || x.Items[0].ID != changed {
		t.Fatal(x)
	}
	if _, err := s.DB.Exec(`UPDATE role_grants SET revoked_at=? WHERE user_id='demo-user-admin' AND role='ADMINISTRATOR' AND revoked_at IS NULL`, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	x = mustOverview(t, s, admin, "reviews")
	if x.Counts["needs_your_decision"] != 0 || len(x.Items) != 0 || x.Counts["awaiting_others"] != 1 {
		t.Fatal("revoked reviewer queue retained", x)
	}
}

func TestOverviewServiceDoesNotExposeStaffOnlyActivityOrCoflatCases(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	var urgent string
	for i := 0; i < 6; i++ {
		in := complaintInput()
		in.Subject = fmt.Sprintf("Fictional overview repair %d", i)
		if i == 5 {
			in.Priority = "URGENT"
		}
		id, err := s.CreateComplaint(ctx, owner, in)
		if err != nil {
			t.Fatal(err)
		}
		if i == 5 {
			urgent = id
		}
	}
	x := mustOverview(t, s, owner, "service")
	if x.Counts["active"] != 6 || x.Counts["urgent"] != 1 || x.Counts["unassigned"] != 0 || len(x.Items) != 4 || x.Items[0].ID != urgent {
		t.Fatal(x)
	}
	before, _ := json.Marshal(x.Items)
	note := complaintAction(1, "COMMENT", "Sensitive staff-only observation never shown on overview")
	note.Visibility = "STAFF_ONLY"
	mustComplaintUpdate(t, s, admin, urgent, note)
	y := mustOverview(t, s, owner, "service")
	after, _ := json.Marshal(y.Items)
	if string(before) != string(after) {
		t.Fatal("private note affected public overview", string(after))
	}
	y = mustOverview(t, s, tenant, "service")
	if y.Counts["active"] != 0 || len(y.Items) != 0 {
		t.Fatal("personal case leaked", y)
	}
	y = mustOverview(t, s, admin, "service")
	if y.Counts["unassigned"] != 6 {
		t.Fatal(y)
	}
	for i, status := range []string{"IN_PROGRESS", "RESOLVED"} {
		in := complaintAction(2+i, "STATUS", "A clear fictional progress and resolution explanation")
		in.Status = status
		mustComplaintUpdate(t, s, admin, urgent, in)
	}
	y = mustOverview(t, s, owner, "service")
	if y.Counts["active"] != 5 || y.Counts["urgent"] != 0 || y.Counts["needs_closure"] != 1 {
		t.Fatal(y)
	}
}

func TestOverviewNoticesUseApprovedCurrentAudiencesAndMemberships(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	for i, audience := range []string{"ALL_RESIDENTS", "OWNERS_ONLY", "TENANTS_ONLY", "COMMITTEE_ONLY", "BUILDING"} {
		in := proposal("NOTICE")
		in.Audience = audience
		in.Title = fmt.Sprintf("Fictional audience notice %d", i)
		if audience == "BUILDING" {
			in.BuildingCode = "B"
		}
		id, err := s.SubmitReview(ctx, owner, "", in)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.DecideReview(ctx, admin, id, decision(1, "APPROVED")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.SubmitReview(ctx, owner, "", proposal("NOTICE")); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{owner, tenant} {
		x := mustOverview(t, s, token, "notices")
		if x.Counts["total"] != 2 || x.Counts["recent"] != 2 || len(x.Items) != 2 {
			t.Fatal(x)
		}
	}
	if x := mustOverview(t, s, admin, "notices"); x.Counts["total"] != 5 || len(x.Items) != 4 {
		t.Fatal(x)
	}
	if _, err := s.DB.Exec(`UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101'`, today()); err != nil {
		t.Fatal(err)
	}
	if x := mustOverview(t, s, owner, "notices"); x.Counts["total"] != 0 || len(x.Items) != 0 {
		t.Fatal("ended audience cached", x)
	}
}

func TestOverviewDocumentWorkRequiresValidatedOriginalSeparateReviewerAndCurrentHead(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	for i := 0; i < 6; i++ {
		in, data := documentFixtureInput()
		in.Title = fmt.Sprintf("Fictional ready document %d", i)
		checkedLibrary(t, s, owner, in, data)
	}
	in, _ := documentFixtureInput()
	if _, err := s.ReserveDocument(ctx, owner, in); err != nil {
		t.Fatal(err)
	}
	x := mustOverview(t, s, admin, "documents")
	if x.Counts["ready_review"] != 6 || x.Counts["upload_attention"] != 0 || len(x.Items) != 4 {
		t.Fatal(x)
	}
	x = mustOverview(t, s, owner, "documents")
	if x.Counts["ready_review"] != 0 || x.Counts["upload_attention"] != 1 || len(x.Items) != 1 {
		t.Fatal(x)
	}
	x = mustOverview(t, s, tenant, "documents")
	if len(x.Items) != 0 {
		t.Fatal("private filenames leaked", x)
	}
	in, data := documentFixtureInput()
	in.Category, in.Visibility = "AMC", "ALL_AUTHORIZED_RESIDENTS"
	in.Expiry = time.Now().In(societyZone).AddDate(0, 0, -1).Format("2006-01-02")
	original := checkedLibrary(t, s, owner, in, data)
	approveLibrary(t, s, admin, original)
	if x = mustOverview(t, s, tenant, "documents"); x.Counts["expired"] != 1 {
		t.Fatal(x)
	}
	in.OperationKey = randomToken()
	in.Replaces = original
	in.Version = mustLibrary(t, s, owner, original).Version
	in.Expiry = time.Now().In(societyZone).AddDate(0, 0, 60).Format("2006-01-02")
	replacement := checkedLibrary(t, s, owner, in, data)
	if x = mustOverview(t, s, tenant, "documents"); x.Counts["expired"] != 1 {
		t.Fatal("unapproved replacement hid deadline", x)
	}
	approveLibrary(t, s, admin, replacement)
	if x = mustOverview(t, s, tenant, "documents"); x.Counts["expired"] != 0 || len(x.Items) != 0 {
		t.Fatal("old head deadline retained", x)
	}
	for _, offset := range []int{0, 30, 31} {
		in, data := documentFixtureInput()
		in.Category, in.Visibility = "CONTRACT", "ALL_AUTHORIZED_RESIDENTS"
		in.Expiry = time.Now().In(societyZone).AddDate(0, 0, offset).Format("2006-01-02")
		id := checkedLibrary(t, s, owner, in, data)
		approveLibrary(t, s, admin, id)
	}
	if x = mustOverview(t, s, tenant, "documents"); x.Counts["expiring"] != 2 {
		t.Fatal("expiry boundaries", x)
	}
	deadlineID := x.Items[0].ID
	if _, err := s.DecideDocument(ctx, admin, deadlineID, libraryAction(mustLibrary(t, s, admin, deadlineID), "ARCHIVED")); err != nil {
		t.Fatal(err)
	}
	if x = mustOverview(t, s, tenant, "documents"); x.Counts["expiring"] != 1 {
		t.Fatal("archived deadline retained", x)
	}
}

func TestOverviewRejectsRevokedSessionsUnsupportedSectionsAndPendingMFA(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	if _, err := s.OverviewFor(ctx, admin, "../entries"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	token, p := loginTest(t, s, "committee@demo.society", DemoPassword)
	if !p.MFAPending {
		t.Fatal("missing MFA fixture")
	}
	if _, err := s.OverviewFor(ctx, token, "reviews"); !errors.Is(err, ErrMFARequired) {
		t.Fatal("MFA bypass", err)
	}
	if _, err := s.DB.Exec(`UPDATE role_grants SET revoked_at=1 WHERE user_id=? AND revoked_at IS NULL`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`DELETE FROM sessions WHERE token_hash=?`, TokenHash(admin)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OverviewFor(ctx, admin, "service"); err == nil {
		t.Fatal("revoked session cached")
	}
}
