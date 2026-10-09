package database

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func financeVisibilityInput(person string, version int, action string) HouseholdFinanceInput {
	return HouseholdFinanceInput{OperationKey: randomToken(), PersonID: person, Version: version, Action: action, Confirmed: true, Note: "Verified current household and independently supplied financial visibility"}
}

func householdFinanceCount(t *testing.T, s *Store, query string, expected int, args ...any) {
	t.Helper()
	var count int
	if err := s.DB.QueryRow(query, args...).Scan(&count); err != nil || count != expected {
		t.Fatal("independent financial visibility count", count, expected, err)
	}
}

func TestHouseholdFinanceSeparateFreshAuthorityNoImplicitOwnerGrantAndPrivateAudit(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	if err := s.AddMembership(ctx, admin, "demo-flat-B-101", AddMembership{RegistryChange: RegistryChange{Version: 1, Reason: "Verified additional current owner"}, Name: "Supplied new owner", Relationship: "OWNER", StartDate: "2020-01-01"}); err != nil {
		t.Fatal(err)
	}
	var person string
	if err := s.DB.QueryRow("SELECT id FROM residents WHERE full_name='Supplied new owner'").Scan(&person); err != nil {
		t.Fatal(err)
	}
	householdFinanceCount(t, s, "SELECT COUNT(*) FROM flat_memberships WHERE resident_id=? AND can_view_finances=1", 0, person)
	input := financeVisibilityInput(person, 2, "GRANT")
	if _, err := s.HouseholdFinanceFor(ctx, admin, "demo-flat-B-101", "", 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("registry read finance controls", err)
	}
	if _, err := s.ChangeHouseholdFinance(ctx, admin, "demo-flat-B-101", input); !errors.Is(err, ErrForbidden) {
		t.Fatal("registry granted finances", err)
	}
	if err := s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := s.ChangeHouseholdFinance(ctx, admin, "demo-flat-B-101", input)
	if err != nil || !result.Visible || result.Version != 3 || len(result.After) != 1 {
		t.Fatal("explicit confirmed grant", result, err)
	}
	householdFinanceCount(t, s, "SELECT COUNT(*) FROM flat_memberships WHERE resident_id=? AND can_view_finances=1", 1, person)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM household_finance_actions", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=?,mfa_verified_at=? WHERE token_hash=?", time.Now().Add(-6*time.Minute).Unix(), time.Now().Add(-6*time.Minute).Unix(), TokenHash(admin))
	if _, err = s.ChangeHouseholdFinance(ctx, admin, "demo-flat-B-101", input); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("stale authentication replayed grant", err)
	}
	accessExec(t, s, "UPDATE role_grants SET revoked_at=? WHERE user_id='demo-user-admin' AND role='TREASURER'", time.Now().Unix())
	if _, err = s.HouseholdFinanceActionFor(ctx, admin, "demo-flat-B-101", input.OperationKey); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked finance authority read retained result", err)
	}
	history, err := s.ActivityFor(ctx, admin, "demo-flat-B-101")
	if err != nil || len(history) != 1 || history[0].Action != "MEMBERSHIP_ADDED" {
		t.Fatal("registry history exposed private financial decision", history, err)
	}
}

func TestHouseholdFinancePersonScopeOriginalReplayAndRemainingHomeAuthority(t *testing.T) {
	s, actor := recordFixture(t)
	ctx := context.Background()
	if err := s.AddMembership(ctx, actor, "demo-flat-A-101", AddMembership{RegistryChange: RegistryChange{Version: 1, Reason: "Supplied second current relationship"}, ResidentID: "demo-owner-A-101", Relationship: "FAMILY", StartDate: "2020-01-01"}); err != nil {
		t.Fatal(err)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	before, _ := s.CheckSession(ctx, owner)
	money := post(t, s, actor, received("432.19"))
	grantInput := financeVisibilityInput("demo-owner-A-101", 2, "GRANT")
	grant, err := s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-101", grantInput)
	if err != nil || len(grant.Before) != 2 || len(grant.After) != 2 || !grant.Visible {
		t.Fatal("person grouping", grant, err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM flat_memberships WHERE flat_id='demo-flat-A-101' AND resident_id='demo-owner-A-101' AND can_view_finances=1", 2)
	revoke, err := s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-101", financeVisibilityInput("demo-owner-A-101", 3, "REVOKE"))
	if err != nil || revoke.Visible || revoke.Version != 4 {
		t.Fatal("explicit person revocation", revoke, err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM flat_memberships WHERE flat_id='demo-flat-A-101' AND resident_id='demo-owner-A-101' AND can_view_finances=1", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM flat_memberships WHERE flat_id='demo-flat-A-102' AND resident_id='demo-owner-A-101' AND can_view_finances=1", 1)
	after, _ := s.CheckSession(ctx, owner)
	if !before.CanReadRecords || !after.CanReadRecords || before.ScopeKey == after.ScopeKey {
		t.Fatal("remaining financial home hid scope change", before, after)
	}
	if _, err = s.EntryFor(ctx, owner, money); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("revoked home retained financial access", err)
	}
	for i := 0; i < 2; i++ {
		original, err := s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-101", grantInput)
		if err != nil || !reflect.DeepEqual(original, grant) {
			t.Fatal("original grant reply changed or replay failed", original, err)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM household_finance_actions", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM flat_memberships WHERE flat_id='demo-flat-A-101' AND resident_id='demo-owner-A-101' AND can_view_finances=1", 0)
	maintenanceCount(t, s, "SELECT version FROM flats WHERE id='demo-flat-A-101'", 4)
	changed := grantInput
	changed.Note = "Changed verification note under the original identity"
	if _, err = s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-101", changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed retry accepted", err)
	}
	if _, err = s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-102", grantInput); !errors.Is(err, ErrConflict) {
		t.Fatal("same key changed homes", err)
	}
	for _, query := range []string{"UPDATE household_finance_actions SET result_json='{}'", "DELETE FROM household_finance_actions"} {
		if _, err = s.DB.Exec(query); err == nil {
			t.Fatal("mutable access history")
		}
	}
}

func TestHouseholdFinanceAtomicFailureConcurrentRetryAndCurrentMembershipBounds(t *testing.T) {
	s, actor := recordFixture(t)
	ctx := context.Background()
	input := financeVisibilityInput("demo-owner-A-101", 1, "REVOKE")
	accessExec(t, s, "CREATE TRIGGER test_finance_late_failure BEFORE INSERT ON household_finance_actions BEGIN SELECT RAISE(ABORT,'forced late failure'); END")
	if _, err := s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-101", input); err == nil {
		t.Fatal("forced late error accepted")
	}
	maintenanceCount(t, s, "SELECT version FROM flats WHERE id='demo-flat-A-101'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM flat_memberships WHERE flat_id='demo-flat-A-101' AND resident_id='demo-owner-A-101' AND can_view_finances=1", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='HOUSEHOLD_FINANCE_REVOKE'", 0)
	accessExec(t, s, "DROP TRIGGER test_finance_late_failure")
	var wg sync.WaitGroup
	results := make(chan HouseholdFinanceAction, 2)
	errorsOut := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-101", input)
			results <- value
			errorsOut <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errorsOut)
	for err := range errorsOut {
		if err != nil {
			t.Fatal("concurrent original retry", err)
		}
	}
	var first string
	for value := range results {
		if first != "" && first != value.ID {
			t.Fatal("duplicate result identities")
		}
		first = value.ID
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM household_finance_actions", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='HOUSEHOLD_FINANCE_REVOKE'", 1)
	maintenanceCount(t, s, "SELECT version FROM flats WHERE id='demo-flat-A-101'", 2)
	if _, err := s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-101", financeVisibilityInput("demo-owner-B-101", 2, "GRANT")); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("another home's person granted", err)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE flat_id='demo-flat-A-101' AND resident_id='demo-owner-A-101'", today())
	if _, err := s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-101", financeVisibilityInput("demo-owner-A-101", 2, "GRANT")); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("ended person granted", err)
	}
	if _, err := s.HouseholdFinanceFor(ctx, actor, "demo-flat-A-101", "", 0); !errors.Is(err, ErrInvalid) {
		t.Fatal("unbounded invalid page", err)
	}
}

func TestHouseholdFinanceSelfGrantDeniedAndCompetingVersionsKeepOneDecision(t *testing.T) {
	s, actor := recordFixture(t)
	ctx := context.Background()
	grantAppointment(t, s, actor, "demo-user-owner", "TREASURER", 30)
	owner := reviewLogin(t, s, "owner@demo.society")
	if _, err := s.ChangeHouseholdFinance(ctx, owner, "demo-flat-A-101", financeVisibilityInput("demo-owner-A-101", 1, "GRANT")); !errors.Is(err, ErrInvalid) {
		t.Fatal("self-grant accepted", err)
	}
	input := financeVisibilityInput("demo-owner-A-101", 1, "REVOKE")
	if _, err := s.ChangeHouseholdFinance(ctx, owner, "demo-flat-A-101", input); err != nil {
		t.Fatal("self revocation", err)
	}
	a := financeVisibilityInput("demo-owner-A-101", 2, "GRANT")
	b := a
	b.OperationKey = randomToken()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, input := range []HouseholdFinanceInput{a, b} {
		wg.Add(1)
		go func(in HouseholdFinanceInput) {
			defer wg.Done()
			_, err := s.ChangeHouseholdFinance(ctx, actor, "demo-flat-A-101", in)
			results <- err
		}(input)
	}
	wg.Wait()
	close(results)
	passed, conflicts := 0, 0
	for err := range results {
		if err == nil {
			passed++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if passed != 1 || conflicts != 1 {
		t.Fatal("competing source versions", passed, conflicts)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM household_finance_actions", 2)
	page, err := s.HouseholdFinanceFor(ctx, actor, "demo-flat-A-101", "not a supplied person", 1)
	if err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Fatal("meaningful empty person search", page, err)
	}
}
