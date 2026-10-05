package backup

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func TestAccountRecoveryKeepsSuspensionTermsAndAuditAtCheckpointWithoutRestoringCredentials(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	var err error
	s.MFA, err = security.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	login := func(store *database.Store, address string) string {
		t.Helper()
		token, p, err := store.Login(ctx, address, database.DemoPassword, database.HashPassword("dummy"))
		if err != nil {
			t.Fatal(err)
		}
		if p.MFAPending {
			if !p.MFAEnrolled {
				if _, err = store.SetupMFA(ctx, token); err != nil {
					t.Fatal(err)
				}
			}
			code, recovery, err := store.DemoVerificationCode(ctx, token)
			if err != nil {
				t.Fatal(err)
			}
			if p.MFAEnrolled {
				_, err = store.VerifyMFA(ctx, token, code, recovery)
			} else {
				_, err = store.ConfirmMFA(ctx, token, code)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		return token
	}
	change := func(version int, reason string) database.AccessChange {
		return database.AccessChange{Version: version, Confirmed: true, Reason: reason}
	}
	admin := login(s, "admin@demo.society")
	for _, target := range []struct {
		id, role string
		days     int
	}{{"demo-user-owner", "AUDITOR", 3}, {"demo-user-tenant", "TREASURER", 30}} {
		if _, err = s.GrantAppointment(ctx, admin, target.id, database.AppointmentInput{
			AccessChange: change(1, "Verified fictional appointment before the recovery checkpoint"), Role: target.role, TermDays: target.days,
		}); err != nil {
			t.Fatal(err)
		}
	}
	owner := login(s, "owner@demo.society")
	tenant := login(s, "tenant@demo.society")
	link, err := s.IssueAccountLink(ctx, admin, "demo-user-owner", "PASSWORD_RESET", true, "Verified fictional identity before the snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangeAccountStatus(ctx, admin, "demo-user-tenant", database.AccountStatusInput{
		AccessChange: change(2, "Confirmed suspension before the recovery checkpoint"), Action: "SUSPEND",
	}); err != nil {
		t.Fatal(err)
	}
	wantOwner, err := s.AccountDetailsFor(ctx, admin, "demo-user-owner", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	wantTenant, err := s.AccountDetailsFor(ctx, admin, "demo-user-tenant", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if wantOwner.Version != 2 || len(wantOwner.Grants) != 1 || wantOwner.Grants[0].Role != "AUDITOR" || wantOwner.Grants[0].ValidUntil-wantOwner.Grants[0].ValidFrom != 259200 {
		t.Fatal("unexpected auditor appointment before snapshot", wantOwner)
	}
	if wantTenant.Version != 3 || wantTenant.Account.State != "SUSPENDED" || !wantTenant.Account.MFA || len(wantTenant.Grants) != 1 || wantTenant.Grants[0].State != "REVOKED" || wantTenant.Grants[0].RevokedBy != "demo-user-admin" {
		t.Fatal("unexpected suspended identity before snapshot", wantTenant)
	}
	bundle := filepath.Join(root, "account-checkpoint")
	if _, err = Snapshot(ctx, s, bundle, "account-test"); err != nil {
		t.Fatal(err)
	}
	// Later decisions must not cross the recovery boundary.
	if _, err = s.ChangeAccountStatus(ctx, admin, "demo-user-tenant", database.AccountStatusInput{
		AccessChange: change(3, "Verified resumption after the recovery checkpoint"), Action: "RESUME",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RevokeAppointment(ctx, admin, "demo-user-owner", wantOwner.Grants[0].ID,
		change(2, "Ended auditor appointment after the recovery checkpoint")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored-accounts.db")
	if _, err = Restore(ctx, bundle, target); err != nil {
		t.Fatal(err)
	}
	recovered, err := database.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	recovered.MFA = s.MFA
	if err = recovered.VerifyMFAKey(ctx); err != nil {
		t.Fatal("confirmed factors did not survive", err)
	}
	for _, oldToken := range []string{admin, owner, tenant} {
		if _, err = recovered.CheckSession(ctx, oldToken); !errors.Is(err, database.ErrUnauthenticated) {
			t.Fatal("old session survived recovery", err)
		}
	}
	if _, err = recovered.InspectAccountLink(ctx, link.Token); !errors.Is(err, database.ErrLink) {
		t.Fatal("old recovery link survived", err)
	}
	if _, _, err = recovered.Login(ctx, "tenant@demo.society", database.DemoPassword, database.HashPassword("dummy")); !errors.Is(err, database.ErrUnauthenticated) {
		t.Fatal("later resumption leaked into the restored suspended identity", err)
	}
	newAdmin := login(recovered, "admin@demo.society")
	for _, want := range []database.AccountDetails{wantOwner, wantTenant} {
		got, err := recovered.AccountDetailsFor(ctx, newAdmin, want.Account.ID, 1, 1)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("checkpoint account version, term, revocation or audit changed", got, want, err)
		}
	}
	newOwner := login(recovered, "owner@demo.society")
	p, err := recovered.CheckSession(ctx, newOwner)
	if err != nil || !p.CanReadAllRecords || p.CanManageRecords || p.CanManageAccounts || p.CanReviewRequests {
		t.Fatal("restored auditor's current permissions changed", p, err)
	}
	if _, err = recovered.AccountDetailsFor(ctx, newOwner, "demo-user-tenant", 1, 1); !errors.Is(err, database.ErrForbidden) {
		t.Fatal("restored auditor administered accounts", err)
	}
	if _, err = recovered.DB.Exec("UPDATE audit_events SET reason='changed after recovery' WHERE action='ACCOUNT_SUSPENDED'"); err == nil {
		t.Fatal("restored access audit became mutable")
	}
}
