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

func upkeepWork() UpkeepTaskInput {
	return UpkeepTaskInput{OperationKey: randomToken(), Title: "Service the fictional water pump", Body: "Private supplied instructions and the original work evidence.", Category: "WATER", Priority: "HIGH", DueDate: today(), Confirmed: true}
}
func upkeepRegisterInput(kind string) UpkeepRegisterInput {
	in := UpkeepRegisterInput{OperationKey: randomToken(), Name: "Fictional service team", Category: "WATER", SourceReference: "PRIVATE-CONTRACT-REF", State: "ACTIVE", Reason: "Verified the supplied fictional register details", Confirmed: true}
	if kind == "VENDOR" {
		in.Contact = "PRIVATE-CONTACT"
		in.Phone = "+91 98765 43210"
		in.Email = "vendor@example.test"
	} else {
		in.Name = "Fictional pump"
		in.Location = "PRIVATE-BASEMENT-ROOM"
	}
	return in
}
func createWork(t *testing.T, s *Store, token string, in UpkeepTaskInput) string {
	t.Helper()
	id, err := s.CreateUpkeepTask(context.Background(), token, in)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func workDetail(t *testing.T, s *Store, token, id string) UpkeepDetail {
	t.Helper()
	x, err := s.UpkeepTaskFor(context.Background(), token, id, 1)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func workAction(t *testing.T, s *Store, token, id, action string) UpkeepDetail {
	t.Helper()
	x := workDetail(t, s, token, id)
	_, err := s.UpdateUpkeepTask(context.Background(), token, id, UpkeepAction{OperationKey: randomToken(), Version: x.Version, Action: action, Reason: "PRIVATE checked original work before this action", Confirmed: true})
	if err != nil {
		t.Fatal(action, err)
	}
	return workDetail(t, s, token, id)
}
func publishWork(t *testing.T, s *Store, token, id, audience, wing string) UpkeepDetail {
	t.Helper()
	x := workDetail(t, s, token, id)
	_, err := s.UpdateUpkeepTask(context.Background(), token, id, UpkeepAction{OperationKey: randomToken(), Version: x.Version, Action: "PUBLISH", Reason: "Reviewed resident wording and chosen audience", PublicTitle: "Water supply care update", PublicBody: "A fictional service visit is planned for the shared water pump.", Audience: audience, BuildingCode: wing, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	return workDetail(t, s, token, id)
}

func TestUpkeepSeparateCompletionCheckerImmutableHistoryAndReopen(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	committee := reviewLogin(t, s, "committee@demo.society")
	in := upkeepWork()
	id := createWork(t, s, admin, in)
	if got, err := s.CreateUpkeepTask(ctx, admin, in); err != nil || got != id {
		t.Fatal("create replay", got, err)
	}
	in.Title += " changed"
	if _, err := s.CreateUpkeepTask(ctx, admin, in); !errors.Is(err, ErrConflict) {
		t.Fatal("changed retry", err)
	}
	workAction(t, s, admin, id, "START")
	workAction(t, s, admin, id, "WAIT")
	workAction(t, s, admin, id, "START")
	x := workAction(t, s, admin, id, "SUBMIT_CHECK")
	action := UpkeepAction{OperationKey: randomToken(), Version: x.Version, Action: "CONFIRM_DONE", Reason: "Checked the separately submitted work evidence", Confirmed: true}
	if _, err := s.UpdateUpkeepTask(ctx, admin, id, action); !errors.Is(err, ErrForbidden) {
		t.Fatal("self completion", err)
	}
	for i := 0; i < 2; i++ {
		if got, err := s.UpdateUpkeepTask(ctx, committee, id, action); err != nil || got != id {
			t.Fatal("completion replay", got, err)
		}
	}
	x = workDetail(t, s, admin, id)
	if x.State != "DONE" || x.Version != 6 || x.EventTotal != 6 || x.ReadyBy != "demo-user-admin" || x.CheckedBy != "demo-user-committee" {
		t.Fatal("independent check evidence", x)
	}
	if out := mustOverview(t, s, admin, "upkeep"); out.Counts["open"] != 0 || out.Counts["overdue"] != 0 {
		t.Fatal("completed open work", out)
	}
	for _, query := range []string{"UPDATE upkeep_tasks SET title='overwrite',version=version+1 WHERE id=?", "UPDATE upkeep_tasks SET state='IN_PROGRESS',version=version+1 WHERE id=?", "DELETE FROM upkeep_tasks WHERE id=?", "UPDATE upkeep_task_events SET reason='overwrite' WHERE task_id=?", "DELETE FROM upkeep_task_events WHERE task_id=?"} {
		if _, err := s.DB.Exec(query, id); err == nil {
			t.Fatal("mutable work", query)
		}
	}
	x = workAction(t, s, admin, id, "REOPEN")
	if x.State != "PLANNED" || x.ReadyBy != "" || x.CheckedBy != "" || x.EventTotal != 7 {
		t.Fatal("reopen loses history", x)
	}
	workAction(t, s, admin, id, "START")
	workAction(t, s, admin, id, "SUBMIT_CHECK")
	x = workAction(t, s, committee, id, "RETURN_WORK")
	if x.State != "IN_PROGRESS" || x.ReadyBy != "" || x.EventTotal != 10 {
		t.Fatal("return to work", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}

func TestUpkeepPublicationSnapshotPreventsPrivateVersionCountSearchAndAudienceLeaks(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	committee := reviewLogin(t, s, "committee@demo.society")
	in := upkeepWork()
	in.DueDate = time.Now().In(societyZone).AddDate(0, 0, -1).Format("2006-01-02")
	id := createWork(t, s, admin, in)
	if _, err := s.UpkeepTaskFor(ctx, owner, id, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("internal read", err)
	}
	publishWork(t, s, admin, id, "BUILDING", "B")
	if _, err := s.UpkeepTaskFor(ctx, owner, id, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("other wing", err)
	}
	publishWork(t, s, admin, id, "BUILDING", "A")
	before := workDetail(t, s, owner, id)
	counts := mustOverview(t, s, owner, "upkeep")
	if before.Version != 2 || before.State != "PLANNED" || counts.Counts["overdue"] != 1 || counts.Counts["unassigned"] != 0 {
		t.Fatal("public expected snapshot", before, counts)
	}
	workAction(t, s, admin, id, "COMMENT")
	workAction(t, s, admin, id, "START")
	workAction(t, s, admin, id, "SUBMIT_CHECK")
	workAction(t, s, committee, id, "CONFIRM_DONE")
	after := workDetail(t, s, owner, id)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("private activity leaked", before, after)
	}
	if got := mustOverview(t, s, owner, "upkeep"); !reflect.DeepEqual(got.Counts, counts.Counts) || !reflect.DeepEqual(got.Items, counts.Items) {
		t.Fatal("private count/activity", counts, got)
	}
	page, err := s.UpkeepTasksFor(ctx, owner, "Private supplied", "DONE", 1)
	if err != nil || page.Total != 0 {
		t.Fatal("private search/status oracle", page, err)
	}
	data, _ := json.Marshal(after)
	for _, private := range []string{"PRIVATE", "created_by", "assigned_to", "ready_by", "checked_by", "events", "public_snapshot", "repeat_days"} {
		if strings.Contains(string(data), private) {
			t.Fatal("private response field", private, string(data))
		}
	}
	publishWork(t, s, admin, id, "ALL_RESIDENTS", "")
	if x := workDetail(t, s, owner, id); x.State != "DONE" || x.Version != 3 {
		t.Fatal(x)
	}
	if got := mustOverview(t, s, owner, "upkeep"); got.Counts["open"] != 0 || got.Counts["overdue"] != 0 {
		t.Fatal("published completion not reflected", got)
	}
	if _, err = s.UpkeepTaskFor(ctx, tenant, id, 1); err != nil {
		t.Fatal("all current residents", err)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101'", today())
	if _, err = s.UpkeepTaskFor(ctx, owner, id, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("ended membership", err)
	}
	workAction(t, s, admin, id, "UNPUBLISH")
	if _, err = s.UpkeepTaskFor(ctx, tenant, id, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("unpublish immediate scope", err)
	}
}

func TestUpkeepActiveReferencesCurrentAssignmentsAndPrivateRegister(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	committee := reviewLogin(t, s, "committee@demo.society")
	owner := reviewLogin(t, s, "owner@demo.society")
	vin := upkeepRegisterInput("VENDOR")
	originalVendor := vin
	vendor, err := s.SaveUpkeepRegister(ctx, admin, "VENDOR", "", vin)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.SaveUpkeepRegister(ctx, admin, "VENDOR", "", vin); err != nil || got != vendor {
		t.Fatal("register replay", got, err)
	}
	ain := upkeepRegisterInput("ASSET")
	ain.VendorID = vendor
	asset, err := s.SaveUpkeepRegister(ctx, admin, "ASSET", "", ain)
	if err != nil {
		t.Fatal(err)
	}
	in := upkeepWork()
	in.AssetID, in.VendorID, in.AssignedTo = asset, vendor, "demo-user-committee"
	id := createWork(t, s, admin, in)
	for _, kind := range []string{"ASSET", "VENDOR"} {
		if _, err := s.UpkeepRegisterFor(ctx, owner, kind, "", "", 1); !errors.Is(err, ErrForbidden) {
			t.Fatal("resident private directory", err)
		}
	}
	vin.OperationKey, vin.Version, vin.State = randomToken(), 1, "INACTIVE"
	if _, err = s.SaveUpkeepRegister(ctx, admin, "VENDOR", vendor, vin); err != nil {
		t.Fatal(err)
	}
	ain.OperationKey, ain.Version, ain.State = randomToken(), 1, "INACTIVE"
	if _, err = s.SaveUpkeepRegister(ctx, admin, "ASSET", asset, ain); err != nil {
		t.Fatal("historical inactive vendor link", err)
	}
	if x := workDetail(t, s, admin, id); x.Asset != "Fictional pump" || x.Vendor != "Fictional service team" || !x.AssigneeEligible {
		t.Fatal("historical links", x)
	}
	in.OperationKey = randomToken()
	if _, err = s.CreateUpkeepTask(ctx, admin, in); !errors.Is(err, ErrInvalid) {
		t.Fatal("inactive new source", err)
	}
	grantAppointment(t, s, admin, "demo-user-committee", "TREASURER", 30)
	committee = reviewLogin(t, s, "committee@demo.society")
	accessExec(t, s, "UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-committee' AND role='COMMITTEE'", time.Now().Unix()-60, time.Now().Unix()-1)
	if _, err = s.UpkeepTasksFor(ctx, committee, "", "", 1); err != nil {
		t.Fatal("ordinary publication reads allowed", err)
	}
	if _, err = s.UpkeepOptionsFor(ctx, committee); !errors.Is(err, ErrForbidden) {
		t.Fatal("treasury implies operations", err)
	}
	if _, err = s.UpdateUpkeepTask(ctx, committee, id, UpkeepAction{OperationKey: randomToken(), Version: 1, Action: "START", Reason: "Fictional expired appointment check", Confirmed: true}); !errors.Is(err, ErrForbidden) {
		t.Fatal("expired operational write", err)
	}
	if x := workDetail(t, s, admin, id); x.AssigneeEligible || x.AssignedName == "" {
		t.Fatal("historical assignee eligibility", x)
	}
	action := UpkeepAction{OperationKey: randomToken(), Version: 1, Action: "ASSIGN", AssignedTo: "demo-user-committee", Reason: "Fictional reassignment must use current authority", Confirmed: true}
	if _, err = s.UpdateUpkeepTask(ctx, admin, id, action); !errors.Is(err, ErrInvalid) {
		t.Fatal("expired new assignment", err)
	}
	options, err := s.UpkeepOptionsFor(ctx, admin)
	if err != nil || len(options.Assets) != 0 || len(options.Vendors) != 0 || len(options.Handlers) != 1 {
		t.Fatal("current choices", options, err)
	}
	accessExec(t, s, "DELETE FROM mfa_factors WHERE user_id='demo-user-admin'")
	if _, err = s.SaveUpkeepRegister(ctx, admin, "VENDOR", "", originalVendor); !errors.Is(err, ErrMFARequired) {
		t.Fatal("factorless register replay", err)
	}
}

func TestUpkeepCalendarDeadlinesExplicitRepeatAndBoundedPages(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	yesterday := time.Now().In(societyZone).AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().In(societyZone).AddDate(0, 0, 1).Format("2006-01-02")
	in := upkeepWork()
	in.DueDate, in.VisitDate = tomorrow, tomorrow
	createWork(t, s, admin, in)
	in.OperationKey, in.DueDate, in.RepeatDays = randomToken(), yesterday, 30
	id := createWork(t, s, admin, in)
	asset := upkeepRegisterInput("ASSET")
	asset.AMCStart, asset.AMCEnd, asset.InspectionDate = "2020-01-01", yesterday, tomorrow
	if _, err := s.SaveUpkeepRegister(ctx, admin, "ASSET", "", asset); err != nil {
		t.Fatal(err)
	}
	x := mustOverview(t, s, admin, "upkeep")
	for key, want := range map[string]int64{"open": 2, "overdue": 1, "unassigned": 2, "ready_for_check": 0, "upcoming_visits": 2, "amc_expired": 1, "amc_expiring": 0, "inspection_overdue": 0, "inspection_upcoming": 1} {
		if x.Counts[key] != want {
			t.Fatal(key, x.Counts[key], want, x)
		}
	}
	workAction(t, s, admin, id, "CANCEL")
	original := workDetail(t, s, admin, id)
	repeat := upkeepWork()
	repeat.ParentID, repeat.RepeatDays, repeat.DueDate = id, 30, tomorrow
	next := createWork(t, s, admin, repeat)
	if next == id || !reflect.DeepEqual(original, workDetail(t, s, admin, id)) {
		t.Fatal("repeat changed source")
	}
	if again, err := s.CreateUpkeepTask(ctx, admin, repeat); err != nil || again != next {
		t.Fatal("repeat retry", again, err)
	}
	repeat.OperationKey = randomToken()
	if _, err := s.CreateUpkeepTask(ctx, admin, repeat); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate dated occurrence", err)
	}
	repeat.DueDate = "2026-02-30"
	if _, err := s.CreateUpkeepTask(ctx, admin, repeat); !errors.Is(err, ErrInvalid) {
		t.Fatal("impossible date", err)
	}
	asset.AMCStart = "2027-01-01"
	asset.AMCEnd = "2026-12-31"
	if _, err := s.SaveUpkeepRegister(ctx, admin, "ASSET", "", asset); !errors.Is(err, ErrInvalid) {
		t.Fatal("inverted AMC", err)
	}
	for i := 0; i < 11; i++ {
		createWork(t, s, admin, upkeepWork())
	}
	page, err := s.UpkeepTasksFor(ctx, admin, "", "", 99999)
	if err != nil || page.Total != 14 || page.Page != 2 || len(page.Items) != 2 || page.Counts["open"] != 13 {
		t.Fatal("bounded page counts", page, err)
	}
	if _, err = s.UpkeepTasksFor(ctx, admin, "", "", 0); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid page", err)
	}
}

func TestUpkeepConcurrentCurrentVersionAndAuthorityCheckedOnReplays(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	id := createWork(t, s, admin, upkeepWork())
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.UpdateUpkeepTask(ctx, admin, id, UpkeepAction{OperationKey: randomToken(), Version: 1, Action: "START", Reason: "Concurrent independent supplied work action", Confirmed: true})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal(successes, conflicts)
	}
	action := UpkeepAction{OperationKey: randomToken(), Version: 2, Action: "COMMENT", Reason: "Response loss should retain exactly this comment", Confirmed: true}
	for i := 0; i < 2; i++ {
		if _, err := s.UpdateUpkeepTask(ctx, admin, id, action); err != nil {
			t.Fatal(err)
		}
	}
	if x := workDetail(t, s, admin, id); x.Version != 3 || x.EventTotal != 3 {
		t.Fatal("duplicate comment", x)
	}
	action.Reason += "changed"
	if _, err := s.UpdateUpkeepTask(ctx, admin, id, action); !errors.Is(err, ErrConflict) {
		t.Fatal("changed comment retry", err)
	}
	action.Reason = strings.TrimSuffix(action.Reason, "changed")
	accessExec(t, s, "UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-admin'", time.Now().Unix()-60, time.Now().Unix()-1)
	if _, err := s.UpdateUpkeepTask(ctx, admin, id, action); !errors.Is(err, ErrForbidden) {
		t.Fatal("expired replay", err)
	}
}
