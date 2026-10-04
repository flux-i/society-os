package database

import (
	"context"
	"errors"
	"society.local/portal/internal/security"
	"sync"
	"testing"
)

func adminFixture(t *testing.T) (*Store, string) {
	t.Helper()
	s := fixture(t)
	s.MFA, _ = security.NewBox(make([]byte, 32))
	ctx := context.Background()
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	token, _, err := s.Login(ctx, "admin@demo.society", DemoPassword, HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetupMFA(ctx, token); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.DemoVerificationCode(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmMFA(ctx, token, code); err != nil {
		t.Fatal(err)
	}
	return s, token
}

func TestRegistryChangesPreserveHistoryAndCurrentPeopleCounts(t *testing.T) {
	s, token := adminFixture(t)
	ctx := context.Background()
	flat := "demo-flat-A-101"
	change := AddMembership{RegistryChange: RegistryChange{Version: 1, Reason: "Recorded fictional tenancy"}, Name: "Preview Tenant", Relationship: "TENANT", StartDate: "2022-01-01", PrimaryContact: true}
	if err := s.AddMembership(ctx, token, flat, change); err != nil {
		t.Fatal(err)
	}
	detail, err := s.Flat(ctx, flat)
	if err != nil || detail.Version != 2 || len(detail.Members) != 3 {
		t.Fatalf("saved relationship: %+v %v", detail, err)
	}
	var member Member
	primaryCount := 0
	for _, m := range detail.Members {
		if m.Name == change.Name {
			member = m
		}
		if m.PrimaryContact && m.Active {
			primaryCount++
		}
	}
	if !member.PrimaryContact || primaryCount != 1 {
		t.Fatal("primary contact replacement not atomic")
	}
	summary, err := s.Summary(ctx)
	if err != nil || summary.Community.Tenants != 36 || summary.Community.Owners != 118 {
		t.Fatalf("active counts after addition: %+v %v", summary.Community, err)
	}
	if err := s.EndMembership(ctx, token, flat, member.MembershipID, EndMembership{RegistryChange: RegistryChange{Version: 2, Reason: "Fictional tenant moved out"}, EndDate: today()}); err != nil {
		t.Fatal(err)
	}
	summary, err = s.Summary(ctx)
	if err != nil || summary.Community.Tenants != 35 {
		t.Fatalf("ended tenant still counted: %+v %v", summary.Community, err)
	}
	detail, err = s.Flat(ctx, flat)
	if err != nil || len(detail.Members) != 3 || detail.Version != 3 {
		t.Fatal("ending membership destroyed original history")
	}
	for _, m := range detail.Members {
		if m.MembershipID == member.MembershipID && (m.Active || m.EndDate == nil) {
			t.Fatal("ended membership remains active")
		}
	}
	events, err := s.ActivityFor(ctx, token, flat)
	if err != nil || len(events) != 2 || events[0].Reason != "Fictional tenant moved out" || events[1].Actor != "Demo Registry Officer" {
		t.Fatalf("audit: %+v %v", events, err)
	}
	for _, query := range []string{"UPDATE audit_events SET reason = 'tampered'", "DELETE FROM audit_events"} {
		if _, err := s.DB.Exec(query); err == nil {
			t.Fatal("immutable audit was changed")
		}
	}
	// Reuse a person for another home without inflating the owner count.
	if err := s.AddMembership(ctx, token, "demo-flat-B-101", AddMembership{RegistryChange: RegistryChange{Version: 1, Reason: "Same fictional owner, another home"}, ResidentID: "demo-owner-A-101", Relationship: "OWNER", StartDate: "2020-01-01"}); err != nil {
		t.Fatal(err)
	}
	summary, err = s.Summary(ctx)
	if err != nil || summary.Community.Owners != 118 || summary.Buildings[1].Owners != 41 {
		t.Fatalf("multi-wing owners must be deduplicated society-wide: %+v %v", summary, err)
	}
}

func TestConcurrentEditsCannotOverwriteEachOther(t *testing.T) {
	s, token := adminFixture(t)
	ctx := context.Background()
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, status := range []string{"VACANT", "RENTED"} {
		wg.Add(1)
		go func(status string) {
			defer wg.Done()
			errs <- s.ChangeOccupancy(ctx, token, "demo-flat-A-101", OccupancyChange{RegistryChange: RegistryChange{Version: 1, Reason: "Concurrent fictional edit"}, Status: status})
		}(status)
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("writes: %d successful, %d conflicts", success, conflict)
	}
	detail, err := s.Flat(ctx, "demo-flat-A-101")
	if err != nil || detail.Version != 2 {
		t.Fatal("version lost under concurrent edits")
	}
	events, err := s.ActivityFor(ctx, token, detail.ID)
	if err != nil || len(events) != 1 {
		t.Fatal("audit and change were not committed together")
	}
}

func TestInvalidRelationshipRollsBackVersionPersonAndAudit(t *testing.T) {
	s, token := adminFixture(t)
	ctx := context.Background()
	before, _ := s.Summary(ctx)
	for _, change := range []AddMembership{
		{RegistryChange: RegistryChange{Version: 1, Reason: "Invalid empty name"}, Name: " ", Relationship: "TENANT", StartDate: "2020-01-01"},
		{RegistryChange: RegistryChange{Version: 1, Reason: "Unknown person selection"}, ResidentID: "does-not-exist", Relationship: "OWNER", StartDate: "2020-01-01"},
		{RegistryChange: RegistryChange{Version: 1, Reason: "Duplicate relationship attempt"}, ResidentID: "demo-owner-A-101", Relationship: "OWNER", StartDate: "2021-01-01"},
		{RegistryChange: RegistryChange{Version: 1, Reason: "Future relationship attempt"}, Name: "Future Person", Relationship: "TENANT", StartDate: "2099-01-01"},
	} {
		if err := s.AddMembership(ctx, token, "demo-flat-A-101", change); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid relationship accepted: %v", err)
		}
	}
	after, _ := s.Summary(ctx)
	detail, _ := s.Flat(ctx, "demo-flat-A-101")
	events, _ := s.ActivityFor(ctx, token, detail.ID)
	if before.Counts != after.Counts || detail.Version != 1 || len(events) != 0 {
		t.Fatal("failed write left registry or audit changes")
	}
}
