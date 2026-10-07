package backup

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"testing"
	"time"
)

func TestCommunityRecoveryRetainsApprovedContactPendingReplacementAndExplicitResolution(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if e := s.SeedDemoAccounts(ctx); e != nil {
		t.Fatal(e)
	}
	var e error
	s.MFA, e = security.LoadKey(filepath.Join(root, "keys", "mfa.key"), true)
	if e != nil {
		t.Fatal(e)
	}
	a, b, o := upkeepRecoveryLogin(t, s, "admin@demo.society"), upkeepRecoveryLogin(t, s, "committee@demo.society"), upkeepRecoveryLogin(t, s, "owner@demo.society")
	options, e := s.CommunityOptionsFor(ctx, a)
	if e != nil {
		t.Fatal(e)
	}
	in := database.CommunityInput{OperationKey: "community-recovery-contact-12345", Kind: "CONTACT", Action: "PUBLISH", Title: "Fictional retained directory contact", Body: "The supplied contact is deliberately published with its exact approved area.", Service: "OTHER", Phone: "+919000000101", Availability: "Supplied weekday availability", Attestation: "PRIVATE retained contact authority and permission", Scope: "ALL", AreaKey: options.AreaKey, Reason: "PRIVATE prepare this original supplied contact", Confirmed: true}
	contact, e := s.ProposeCommunity(ctx, a, "", in)
	if e != nil {
		t.Fatal(e)
	}
	approve := func(id string, version int, key string) {
		t.Helper()
		_, e := s.DecideCommunity(ctx, b, id, database.CommunityAction{OperationKey: key, Version: version, Action: "APPROVED", Reason: "PRIVATE independently checked the exact supplied publication", Confirmed: true})
		if e != nil {
			t.Fatal(e)
		}
	}
	approve(contact, 1, "community-recovery-contact-review-12345")
	in.Version = 2
	in.OperationKey = "community-recovery-replacement-12345"
	in.Phone = "+919000000202"
	if _, e = s.ProposeCommunity(ctx, a, contact, in); e != nil {
		t.Fatal(e)
	}
	interruption := database.CommunityInput{OperationKey: "community-recovery-outage-12345", Kind: "INTERRUPTION", Action: "PUBLISH", Title: "Fictional restored water interruption", Body: "This supplied interruption has its original service description and frozen area.", Service: "WATER", Scope: "WING", BuildingCode: "A", AreaKey: options.AreaKey, StartAt: time.Now().Add(-time.Hour).Unix(), EstimatedEnd: time.Now().Add(-time.Minute).Unix(), Reason: "PRIVATE supplied original interruption timing", Confirmed: true}
	outage, e := s.ProposeCommunity(ctx, a, "", interruption)
	if e != nil {
		t.Fatal(e)
	}
	approve(outage, 1, "community-recovery-outage-review-12345")
	resolution := database.CommunityInput{OperationKey: "community-recovery-resolution-12345", Version: 2, Action: "RESOLVE", ResolvedAt: time.Now().Add(-30 * time.Second).Unix(), UpdateText: "The operator supplied this explicit restoration time and approved update.", Reason: "PRIVATE supplied the exact restoration and frozen predecessor", Confirmed: true}
	if _, e = s.ProposeCommunity(ctx, a, outage, resolution); e != nil {
		t.Fatal(e)
	}
	approve(outage, 3, "community-recovery-resolution-review-12345")
	wantContact, e := s.CommunityResourceFor(ctx, a, contact, true, 1)
	if e != nil {
		t.Fatal(e)
	}
	wantPublic, e := s.CommunityResourceFor(ctx, o, contact, false, 1)
	if e != nil || wantPublic.Snapshot.Phone != "+919000000101" {
		t.Fatal(wantPublic, e)
	}
	wantResolved, e := s.CommunityResourceFor(ctx, o, outage, false, 1)
	if e != nil || wantResolved.State != "RESOLVED" {
		t.Fatal(wantResolved, e)
	}
	bundle := filepath.Join(root, "community-snapshot")
	if _, e = Snapshot(ctx, s, bundle, "0.21.0-dev"); e != nil {
		t.Fatal(e)
	}
	approve(contact, 3, "community-recovery-later-approve-12345")
	target := filepath.Join(root, "restored-community", "society.db")
	if _, e = Restore(ctx, bundle, target); e != nil {
		t.Fatal(e)
	}
	restored, e := database.Open(ctx, target)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	restored.MFA = s.MFA
	if _, e = restored.CheckSession(ctx, a); !errors.Is(e, database.ErrUnauthenticated) {
		t.Fatal("old session survived", e)
	}
	// Check invalidated temporary credentials before the fixture deliberately creates fresh login recovery material.
	for _, table := range []string{"account_tokens", "mfa_recovery_codes", "mfa_pending"} {
		var n int
		if e = restored.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal(table, n, e)
		}
	}
	freshA := upkeepRecoveryLogin(t, restored, "admin@demo.society")
	freshO := upkeepRecoveryLogin(t, restored, "owner@demo.society")
	for _, tc := range []struct {
		token, id string
		desk      bool
		want      database.CommunityDetail
	}{{freshA, contact, true, wantContact}, {freshO, contact, false, wantPublic}, {freshO, outage, false, wantResolved}} {
		got, e := restored.CommunityResourceFor(ctx, tc.token, tc.id, tc.desk, 1)
		if e != nil || !reflect.DeepEqual(tc.want, got) {
			t.Fatal("community snapshot changed", tc.id, got, e)
		}
	}
	if e = restored.VerifyMFAKey(ctx); e != nil {
		t.Fatal(e)
	}
}
