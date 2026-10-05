package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func maintenanceProposal() MaintenanceInput {
	return MaintenanceInput{OperationKey: randomToken(), Title: "October fictional maintenance", PeriodStart: "2026-01-01", PeriodEnd: "2026-01-31", DueDate: "2026-01-10", SourceReference: "Fictional approved register OCT-1", Note: "Private committee supporting note", Confirmed: true,
		Lines: []MaintenanceLineInput{{"demo-flat-A-101", "1000.00"}, {"demo-flat-A-102", "750.25"}}}
}
func maintenanceDecision(state string) MaintenanceAction {
	return MaintenanceAction{OperationKey: randomToken(), Version: 1, Decision: state, Reason: "Separately checked each fictional supplied amount", Confirmed: true}
}
func maintenanceReviewer(t *testing.T, s *Store, admin string) string {
	t.Helper()
	grantAppointment(t, s, admin, "demo-user-committee", "TREASURER", 30)
	return reviewLogin(t, s, "committee@demo.society")
}
func publishMaintenance(t *testing.T, s *Store, author, reviewer string, in MaintenanceInput) MaintenanceDetails {
	t.Helper()
	id, err := s.CreateMaintenanceCycle(context.Background(), author, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideMaintenanceCycle(context.Background(), reviewer, id, maintenanceDecision("PUBLISHED")); err != nil {
		t.Fatal(err)
	}
	details, err := s.MaintenanceCycleFor(context.Background(), author, id, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	return details
}
func allocate(t *testing.T, s *Store, token, source, charge, amount string) string {
	t.Helper()
	id, err := s.AllocateCredit(context.Background(), token, CreditAllocationInput{randomToken(), source, charge, amount, "Checked against the original receipt and charge", true})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func statement(t *testing.T, s *Store, token, home string) HomeStatement {
	t.Helper()
	x, err := s.HomeStatementFor(context.Background(), token, home, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func maintenanceCount(t *testing.T, s *Store, query string, want int) {
	t.Helper()
	var got int
	if err := s.DB.QueryRow(query).Scan(&got); err != nil || got != want {
		t.Fatalf("%s: %d, want %d: %v", query, got, want, err)
	}
}

func TestMaintenanceSeparateReviewFrozenProposalAndIdempotentPublication(t *testing.T) {
	s, admin := recordFixture(t)
	ctx := context.Background()
	in := maintenanceProposal()
	id, err := s.CreateMaintenanceCycle(ctx, admin, in)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := s.CreateMaintenanceCycle(ctx, admin, in)
	if err != nil || repeat != id {
		t.Fatal("submission replay", repeat, err)
	}
	in.Lines[0].Amount = "1000.01"
	if _, err = s.CreateMaintenanceCycle(ctx, admin, in); !errors.Is(err, ErrConflict) {
		t.Fatal("changed retry", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	pending, err := s.MaintenanceCyclesFor(ctx, admin, "", "", "", 1)
	if err != nil || pending.Total != 1 || pending.Totals.RequestedPaise != 175025 || pending.Totals.ActivePaise != 0 || pending.Totals.OutstandingPaise != 0 {
		t.Fatal("pending affected dues", pending, err)
	}
	if _, err = s.DecideMaintenanceCycle(ctx, admin, id, maintenanceDecision("PUBLISHED")); !errors.Is(err, ErrForbidden) {
		t.Fatal("self publication", err)
	}
	committee := reviewLogin(t, s, "committee@demo.society")
	if _, err = s.DecideMaintenanceCycle(ctx, committee, id, maintenanceDecision("PUBLISHED")); !errors.Is(err, ErrForbidden) {
		t.Fatal("committee implied treasury", err)
	}
	for _, query := range []string{
		"UPDATE maintenance_cycles SET title='Changed after submission' WHERE id=?",
		"UPDATE maintenance_lines SET amount_paise=1 WHERE cycle_id=?",
		"DELETE FROM maintenance_lines WHERE cycle_id=?",
		"INSERT INTO maintenance_lines(cycle_id,flat_id,amount_paise) VALUES(?,'demo-flat-A-103',100)",
	} {
		if _, err = s.DB.Exec(query, id); err == nil {
			t.Fatal("mutable proposal", query)
		}
	}
	reviewer := maintenanceReviewer(t, s, admin)
	action := maintenanceDecision("PUBLISHED")
	for i := 0; i < 2; i++ {
		got, e := s.DecideMaintenanceCycle(ctx, reviewer, id, action)
		if e != nil || got != id {
			t.Fatal("publish replay", got, e)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	x, err := s.MaintenanceCycleFor(ctx, admin, id, 1, 1)
	if err != nil || x.State != "PUBLISHED" || x.Version != 2 || x.ActivePaise != 175025 || x.OutstandingPaise != 175025 || len(x.Events) != 2 || x.Events[0].Actor == x.Author || len(x.Lines) != 2 {
		t.Fatal("published facts", x, err)
	}
	if _, err = s.DecideMaintenanceCycle(ctx, reviewer, id, maintenanceDecision("DECLINED")); !errors.Is(err, ErrConflict) {
		t.Fatal("stale terminal decision", err)
	}
	for _, query := range []string{"UPDATE maintenance_cycles SET state='PENDING' WHERE id=?", "DELETE FROM maintenance_cycles WHERE id=?", "DELETE FROM maintenance_events WHERE cycle_id=?", "UPDATE maintenance_events SET reason='Changed' WHERE cycle_id=?"} {
		if _, err = s.DB.Exec(query, id); err == nil {
			t.Fatal("mutable publication", query)
		}
	}
	accessExec(t, s, "UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-committee' AND role='TREASURER'", time.Now().Unix()-60, time.Now().Unix()-1)
	if _, err = s.DecideMaintenanceCycle(ctx, reviewer, id, action); !errors.Is(err, ErrForbidden) {
		t.Fatal("expired authority replay", err)
	}
}

func TestMaintenanceExactPartialAdvanceOpeningCreditAndLinkedSourceReversal(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	ctx := context.Background()
	one := publishMaintenance(t, s, admin, reviewer, maintenanceProposal())
	a101, a102 := one.Lines[0].EntryID, one.Lines[1].EntryID
	first := post(t, s, admin, received("400.00"))
	in := CreditAllocationInput{randomToken(), first, a101, "400.00", "Checked original receipt and October home charge", true}
	allocation, err := s.AllocateCredit(ctx, admin, in)
	if err != nil {
		t.Fatal(err)
	}
	if again, e := s.AllocateCredit(ctx, admin, in); e != nil || again != allocation {
		t.Fatal("allocation retry", again, e)
	}
	x := statement(t, s, admin, "demo-flat-A-101")
	if x.DebitPaise != 100000 || x.CreditPaise != 40000 || x.AllocatedPaise != 40000 || x.OutstandingPaise != 60000 || x.UnallocatedPaise != 0 {
		t.Fatal("partial allocation", x)
	}
	second := post(t, s, admin, received("800.00"))
	allocate(t, s, admin, second, a101, "600.00")
	x = statement(t, s, admin, "demo-flat-A-101")
	if x.OutstandingPaise != 0 || x.UnallocatedPaise != 20000 {
		t.Fatal("advance retained", x)
	}
	twoIn := maintenanceProposal()
	twoIn.Title = "February fictional maintenance"
	twoIn.PeriodStart = "2026-02-01"
	twoIn.PeriodEnd = "2026-02-28"
	twoIn.DueDate = "2026-02-10"
	twoIn.Lines = []MaintenanceLineInput{{"demo-flat-A-101", "900.00"}}
	two := publishMaintenance(t, s, admin, reviewer, twoIn)
	allocate(t, s, admin, second, two.Lines[0].EntryID, "200.00")
	credit := supplied("OPENING_CREDIT", "100.25")
	credit.FlatID = "demo-flat-A-102"
	opening := post(t, s, admin, credit)
	allocate(t, s, admin, opening, a102, "100.25")
	b := statement(t, s, admin, "demo-flat-A-102")
	if b.OutstandingPaise != 65000 || b.AllocatedPaise != 10025 || b.Credits[0].Receipt != "" {
		t.Fatal("opening credit", b)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 2)
	before, err := s.EntryFor(ctx, admin, first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReverseEntry(ctx, admin, first, EntryAction{randomToken(), true, "Received entry had an incorrect supplied reference"}); err != nil {
		t.Fatal(err)
	}
	after, err := s.EntryFor(ctx, admin, first)
	if err != nil || before.ReceiptID != after.ReceiptID || before.ReceiptNumber != after.ReceiptNumber || after.State != "REVERSED" {
		t.Fatal("receipt identity lost", after, err)
	}
	x = statement(t, s, admin, "demo-flat-A-101")
	if x.DebitPaise != 190000 || x.CreditPaise != 80000 || x.AllocatedPaise != 80000 || x.OutstandingPaise != 110000 || x.UnallocatedPaise != 0 || x.AllocationTotal != 3 {
		t.Fatal("source reversal effect", x)
	}
	o, e := s.MaintenanceCycleFor(ctx, admin, one.ID, 1, 1)
	if e != nil || o.OutstandingPaise != 105000 || o.AllocatedPaise != 70025 {
		t.Fatal("first period after reversal", o, e)
	}
	tw, e := s.MaintenanceCycleFor(ctx, admin, two.ID, 1, 1)
	if e != nil || tw.OutstandingPaise != 70000 || tw.AllocatedPaise != 20000 {
		t.Fatal("second period advance changed", tw, e)
	}
	found := false
	for _, a := range x.Allocations {
		if a.ID == allocation {
			found = true
			if a.State != "SOURCE_REVERSED" || a.Receipt != before.ReceiptNumber || a.AmountPaise != 40000 {
				t.Fatal("allocation history lost", a)
			}
		}
	}
	if !found {
		t.Fatal("missing source allocation history")
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 2)
}

func TestMaintenanceChargeReversalReleasesCreditAndCorrectionKeepsHistory(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	ctx := context.Background()
	cycle := publishMaintenance(t, s, admin, reviewer, maintenanceProposal())
	source := post(t, s, admin, received("400.00"))
	allocation := allocate(t, s, admin, source, cycle.Lines[0].EntryID, "400.00")
	correction := AllocationCorrection{randomToken(), "Allocation selected the wrong approved period", true}
	for i := 0; i < 2; i++ {
		if _, e := s.ReverseAllocation(ctx, admin, allocation, correction); e != nil {
			t.Fatal(e)
		}
	}
	x := statement(t, s, admin, "demo-flat-A-101")
	if x.OutstandingPaise != 100000 || x.UnallocatedPaise != 40000 || x.Allocations[0].State != "CORRECTED" || x.Allocations[0].CorrectionReason != correction.Reason {
		t.Fatal("correction facts", x)
	}
	allocation = allocate(t, s, admin, source, cycle.Lines[0].EntryID, "400.00")
	if _, err := s.ReverseEntry(ctx, admin, cycle.Lines[0].EntryID, EntryAction{randomToken(), true, "Wrong approved charge requires separately reviewed replacement"}); err != nil {
		t.Fatal(err)
	}
	c, err := s.MaintenanceCycleFor(ctx, admin, cycle.ID, 1, 1)
	if err != nil || c.RequestedPaise != 175025 || c.ActivePaise != 75025 || c.ReversedPaise != 100000 || c.AllocatedPaise != 0 || c.OutstandingPaise != 75025 || c.Lines[0].State != "REVERSED" {
		t.Fatal("frozen vs active amounts", c, err)
	}
	x = statement(t, s, admin, "demo-flat-A-101")
	if x.DebitPaise != 0 || x.UnallocatedPaise != 40000 || x.AllocationTotal != 2 {
		t.Fatal("reversal did not release", x)
	}
	for _, q := range []string{"UPDATE entry_allocations SET amount_paise=1 WHERE id=?", "DELETE FROM entry_allocations WHERE id=?", "DELETE FROM allocation_reversals WHERE allocation_id IN (SELECT id FROM entry_allocations WHERE source_id=?)"} {
		arg := allocation
		if q[0:6] == "DELETE" && q != "DELETE FROM entry_allocations WHERE id=?" {
			arg = source
		}
		if _, err = s.DB.Exec(q, arg); err == nil {
			t.Fatal("mutable allocation history", q)
		}
	}
}

func TestMaintenanceAllocationWriterReservationPreventsOverspending(t *testing.T) {
	s, admin := recordFixture(t)
	ctx := context.Background()
	source := post(t, s, admin, received("400.00"))
	charge := post(t, s, admin, supplied("CHARGE", "1000.00"))
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.AllocateCredit(ctx, admin, CreditAllocationInput{randomToken(), source, charge, "300.00", "Independent competing receipt allocation", true})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	x := statement(t, s, admin, "demo-flat-A-101")
	if success != 1 || conflict != 1 || x.AllocatedPaise != 30000 || x.OutstandingPaise != 70000 || x.UnallocatedPaise != 10000 || x.AllocationTotal != 1 {
		t.Fatal("concurrent source overspent", success, conflict, x)
	}
	foreign := supplied("CHARGE", "300")
	foreign.FlatID = "demo-flat-B-101"
	foreignID := post(t, s, admin, foreign)
	if _, err := s.AllocateCredit(ctx, admin, CreditAllocationInput{randomToken(), source, foreignID, "1.00", "Cross-home allocation must fail", true}); !errors.Is(err, ErrInvalid) {
		t.Fatal("cross-home credit", err)
	}
	if _, err := s.DB.Exec("INSERT INTO entry_allocations VALUES(?,?,?,?,?,?,?)", randomToken(), source, charge, 10001, "demo-user-admin", "Direct SQL exceeds remaining credit", time.Now().Unix()); err == nil {
		t.Fatal("DB permitted source overspend")
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
}

func TestMaintenanceDueDatesOpeningDuesAndCurrentResidentScope(t *testing.T) {
	s, admin := recordFixture(t)
	accessExec(t, s, "UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-102'")
	reviewer := maintenanceReviewer(t, s, admin)
	ctx := context.Background()
	future := maintenanceProposal()
	future.DueDate = time.Now().In(societyZone).AddDate(0, 0, 1).Format("2006-01-02")
	c := publishMaintenance(t, s, admin, reviewer, future)
	pending, err := s.CreateMaintenanceCycle(ctx, admin, maintenanceProposal())
	if err != nil {
		t.Fatal(err)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	page, err := s.MaintenanceCyclesFor(ctx, owner, "", "", "", 1)
	if err != nil || page.Total != 1 || page.Totals.ActivePaise != 100000 || page.Totals.OverduePaise != 0 || len(page.Homes) != 1 || page.Items[0].Participants != 1 || page.Items[0].SourceReference != "" || page.Items[0].Author != "" {
		t.Fatal("private scope leaked", page, err)
	}
	if _, err = s.MaintenanceCycleFor(ctx, owner, pending, 1, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("resident read pending", err)
	}
	detail, err := s.MaintenanceCycleFor(ctx, owner, c.ID, 1, 1)
	if err != nil || len(detail.Lines) != 1 || detail.Lines[0].Home != "A-101" || detail.Note != "" || len(detail.Events) != 0 {
		t.Fatal("resident details leaked", detail, err)
	}
	if _, err = s.HomeStatementFor(ctx, owner, "demo-flat-A-102", 1, 1, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("another home statement", err)
	}
	opening := post(t, s, admin, supplied("OPENING_DEBIT", "200.00"))
	source := post(t, s, admin, supplied("OPENING_CREDIT", "50.00"))
	allocate(t, s, admin, source, opening, "50.00")
	x := statement(t, s, owner, "demo-flat-A-101")
	if x.OutstandingPaise != 115000 || x.OverduePaise != 0 || x.Allocations[0].Reason != "" || x.Allocations[0].Actor != "" {
		t.Fatal("inferred due/private history", x)
	}
	noMatch, err := s.MaintenanceCyclesFor(ctx, owner, "absent title", "demo-flat-A-102", "PENDING", 1)
	if err != nil || noMatch.Total != 0 || noMatch.Totals.ActivePaise != 0 {
		t.Fatal("filtered scope leaked", noMatch, err)
	}
	accessExec(t, s, "UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101'")
	if _, err = s.MaintenanceCyclesFor(ctx, owner, "", "", "", 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("ended entitlement list", err)
	}
	if _, err = s.HomeStatementFor(ctx, owner, "demo-flat-A-101", 1, 1, 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("ended entitlement statement", err)
	}
}

func TestMaintenanceDeclineWithdrawalValidationAndBoundedPages(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	ctx := context.Background()
	for _, state := range []string{"DECLINED", "WITHDRAWN"} {
		id, e := s.CreateMaintenanceCycle(ctx, admin, maintenanceProposal())
		if e != nil {
			t.Fatal(e)
		}
		token := reviewer
		if state == "WITHDRAWN" {
			token = admin
		}
		action := maintenanceDecision(state)
		if _, e = s.DecideMaintenanceCycle(ctx, token, id, action); e != nil {
			t.Fatal(e)
		}
		x, e := s.MaintenanceCycleFor(ctx, admin, id, 1, 1)
		if e != nil || x.State != state || x.ActivePaise != 0 || x.OutstandingPaise != 0 || len(x.Events) != 2 {
			t.Fatal(state, x, e)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	base := maintenanceProposal()
	invalidInputs := []MaintenanceInput{base, base, base, base, base, base, base}
	invalidInputs[0].Confirmed = false
	invalidInputs[1].DueDate = "2025-12-31"
	invalidInputs[2].PeriodEnd = "2025-12-31"
	invalidInputs[3].Lines = nil
	invalidInputs[4].Lines = []MaintenanceLineInput{{"demo-flat-A-101", "1.001"}}
	invalidInputs[5].Lines = []MaintenanceLineInput{{"demo-flat-A-101", "1"}, {"demo-flat-A-101", "2"}}
	invalidInputs[6].Lines = []MaintenanceLineInput{{"missing-home", "1"}}
	for _, in := range invalidInputs {
		if _, err := s.CreateMaintenanceCycle(ctx, admin, in); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid proposal", in, err)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM maintenance_cycles", 2)
	for i := 0; i < 13; i++ {
		in := maintenanceProposal()
		in.Lines = []MaintenanceLineInput{{"demo-flat-A-101", "0.01"}}
		publishMaintenance(t, s, admin, reviewer, in)
	}
	page, e := s.MaintenanceCyclesFor(ctx, admin, "", "", "", 100000)
	if e != nil || page.Total != 15 || page.Page != 2 || len(page.Items) != 3 || page.Totals.ActivePaise != 13 {
		t.Fatal("bounded cycle page", page, e)
	}
	if _, e = s.MaintenanceCyclesFor(ctx, admin, "", "", "UNSUPPORTED", 1); !errors.Is(e, ErrInvalid) {
		t.Fatal("invalid state", e)
	}
	for i := 0; i < 21; i++ {
		post(t, s, admin, supplied("OPENING_CREDIT", "0.01"))
	}
	x, e := s.HomeStatementFor(ctx, admin, "demo-flat-A-101", 100000, 100000, 100000)
	if e != nil || x.ChargeTotal != 13 || x.CreditTotal != 21 || x.CreditPage != 2 || len(x.Credits) != 1 || x.UnallocatedPaise != 21 || x.OutstandingPaise != 13 {
		t.Fatal("bounded statement totals", x, e)
	}
}

func TestSchemaEightMaintenanceUpgradePreservesLedgerAndAccountAuthority(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "schema-eight.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	accessExec(t, s, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT")
	checks := map[int]string{}
	for version := 1; version <= 8; version++ {
		body, e := migrationBody(version)
		if e != nil {
			t.Fatal(e)
		}
		accessExec(t, s, string(body))
		sum := sha256.Sum256(body)
		checks[version] = hex.EncodeToString(sum[:])
		accessExec(t, s, "INSERT INTO schema_migrations VALUES(?,?,?)", version, checks[version], "2026-10-05")
	}
	if err = s.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	accessExec(t, s, "INSERT INTO entries VALUES('old-charge','demo-flat-A-101','CHARGE',12345,'2026-01-01','Existing supplied charge','','','','Legacy source','POSTED','demo-user-admin',1,'demo-user-admin',2)")
	before, e := CountRecords(ctx, s.DB)
	if e != nil {
		t.Fatal(e)
	}
	var audit, grants, legacy string
	accessSnapshot := func() (string, string, string) {
		var a, g, l string
		for i, item := range []struct {
			query string
			dst   *string
		}{
			{"SELECT json_group_array(json_array(id,action,reason,before_json,after_json)) FROM audit_events", &a},
			{"SELECT json_group_array(json_array(id,user_id,role,valid_from,valid_until,revoked_at,granted_by)) FROM role_grants", &g},
			{"SELECT json_group_array(json_array(id,amount_paise,state,created_by,posted_by,source_note)) FROM entries", &l},
		} {
			if err = s.DB.QueryRow(item.query).Scan(item.dst); err != nil {
				t.Fatal(i, err)
			}
		}
		return a, g, l
	}
	audit, grants, legacy = accessSnapshot()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	after, e := CountRecords(ctx, s.DB)
	if e != nil || before != after {
		t.Fatal("record changes", before, after, e)
	}
	a, g, l := accessSnapshot()
	if a != audit || g != grants || l != legacy {
		t.Fatal("historical authority/ledger changed")
	}
	for version, want := range checks {
		var got string
		if e = s.DB.QueryRow("SELECT checksum FROM schema_migrations WHERE version=?", version).Scan(&got); e != nil || got != want {
			t.Fatal(version, e)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM maintenance_cycles", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entry_allocations", 0)
}

func TestMaintenanceOverviewCountsCurrentReviewAndExplicitDueAmounts(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	ctx := context.Background()
	id, err := s.CreateMaintenanceCycle(ctx, admin, maintenanceProposal())
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.OverviewFor(ctx, admin, "maintenance")
	if err != nil || a.Counts["pending_review"] != 0 || a.Counts["awaiting_other_reviewer"] != 1 || a.Counts["outstanding_paise"] != 0 || len(a.Items) != 0 {
		t.Fatal("author queue", a, err)
	}
	r, err := s.OverviewFor(ctx, reviewer, "maintenance")
	if err != nil || r.Counts["pending_review"] != 1 || len(r.Items) != 1 || r.Items[0].ID != id || r.Items[0].State != "PENDING" {
		t.Fatal("current separate queue", r, err)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	o, err := s.OverviewFor(ctx, owner, "maintenance")
	if err != nil || o.Counts["pending_review"] != 0 || o.Counts["published_periods"] != 0 || len(o.Items) != 0 {
		t.Fatal("private overview leak", o, err)
	}
	if _, err = s.DecideMaintenanceCycle(ctx, reviewer, id, maintenanceDecision("PUBLISHED")); err != nil {
		t.Fatal(err)
	}
	d, err := s.MaintenanceCycleFor(ctx, admin, id, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	credit := post(t, s, admin, received("400.00"))
	allocate(t, s, admin, credit, d.Lines[0].EntryID, "400.00")
	o, err = s.OverviewFor(ctx, owner, "maintenance")
	if err != nil || o.Counts["active_paise"] != 175025 || o.Counts["allocated_paise"] != 40000 || o.Counts["outstanding_paise"] != 135025 || o.Counts["overdue_paise"] != 135025 || o.Counts["overdue_periods"] != 1 || o.Items[0].AmountPaise != 135025 {
		t.Fatal("exact overview", o, err)
	}
	future := maintenanceProposal()
	future.DueDate = time.Now().In(societyZone).AddDate(0, 0, 1).Format("2006-01-02")
	future.Lines = []MaintenanceLineInput{{"demo-flat-A-101", "123.45"}}
	publishMaintenance(t, s, admin, reviewer, future)
	o, err = s.OverviewFor(ctx, owner, "maintenance")
	if err != nil || o.Counts["outstanding_paise"] != 147370 || o.Counts["overdue_paise"] != 135025 || o.Counts["published_periods"] != 2 || o.Counts["overdue_periods"] != 1 {
		t.Fatal("future due falsely overdue", o, err)
	}
	tenant := reviewLogin(t, s, "tenant@demo.society")
	if _, err = s.OverviewFor(ctx, tenant, "maintenance"); !errors.Is(err, ErrForbidden) {
		t.Fatal("unentitled maintenance overview", err)
	}
}
