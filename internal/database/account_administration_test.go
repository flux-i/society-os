package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func accessInput(version int) AccessChange {
	return AccessChange{Version: version, Confirmed: true, Reason: "Verified fictional appointment and identity against the approved register"}
}
func accountDetails(t *testing.T, s *Store, token, id string) AccountDetails {
	t.Helper()
	x, err := s.AccountDetailsFor(context.Background(), token, id, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func grantAppointment(t *testing.T, s *Store, token, id, role string, days int) AccountDetails {
	t.Helper()
	x := accountDetails(t, s, token, id)
	if _, err := s.GrantAppointment(context.Background(), token, id, AppointmentInput{AccessChange: accessInput(x.Version), Role: role, TermDays: days}); err != nil {
		t.Fatal(err)
	}
	return accountDetails(t, s, token, id)
}
func accessExec(t *testing.T, s *Store, sql string, args ...any) {
	t.Helper()
	if _, err := s.DB.Exec(sql, args...); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaSevenAccountUpgradePreservesIdentityGrantsAndAudit(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "previous.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	accessExec(t, s, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT")
	checks := map[int]string{}
	for version := 1; version <= 7; version++ {
		body, err := migrationBody(version)
		if err != nil {
			t.Fatal(err)
		}
		accessExec(t, s, string(body))
		sum := sha256.Sum256(body)
		checks[version] = hex.EncodeToString(sum[:])
		accessExec(t, s, "INSERT INTO schema_migrations VALUES(?,?,?)", version, checks[version], "2026-10-04")
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
	before, err := CountRecords(ctx, s.DB)
	if err != nil {
		t.Fatal(err)
	}
	var grantUser, grantRole, audit string
	if err = s.DB.QueryRow("SELECT user_id,role FROM role_grants WHERE id='demo-treasury-grant'").Scan(&grantUser, &grantRole); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.QueryRow("SELECT after_json FROM audit_events WHERE action='DEMO_IDENTITY_BOOTSTRAP'").Scan(&audit); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := CountRecords(ctx, s.DB)
	if err != nil || before != after {
		t.Fatal(before, after, err)
	}
	var version, count int
	var suspended bool
	var gotUser, gotRole, gotAudit string
	if err = s.DB.QueryRow("SELECT access_version,suspended_at IS NOT NULL FROM users WHERE id='demo-user-admin'").Scan(&version, &suspended); err != nil || version != 1 || suspended {
		t.Fatal(version, suspended, err)
	}
	if err = s.DB.QueryRow("SELECT user_id,role FROM role_grants WHERE id='demo-treasury-grant'").Scan(&gotUser, &gotRole); err != nil || gotUser != grantUser || gotRole != grantRole {
		t.Fatal("changed legacy grant", err)
	}
	if err = s.DB.QueryRow("SELECT after_json FROM audit_events WHERE action='DEMO_IDENTITY_BOOTSTRAP'").Scan(&gotAudit); err != nil || audit != gotAudit {
		t.Fatal("changed audit", err)
	}
	for version, want := range checks {
		var got string
		if err = s.DB.QueryRow("SELECT checksum FROM schema_migrations WHERE version=?", version).Scan(&got); err != nil || got != want {
			t.Fatal("changed migration", version, err)
		}
	}
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM role_grants").Scan(&count); err != nil || count != 3 {
		t.Fatal("lost seed appointments", count, err)
	}
	accessExec(t, s, "INSERT INTO role_grants(id,user_id,role,valid_from,valid_until,granted_by) VALUES('new-auditor','demo-user-owner','AUDITOR',?,?,'demo-user-admin')", time.Now().Unix(), time.Now().Add(time.Hour).Unix())
}

func TestAccountAppointmentsSeparateFinancialAndAdministrativePowers(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	p, err := s.CheckSession(ctx, admin)
	if err != nil || !p.CanManageAccounts || p.CanManageRecords || p.CanReadAllRecords {
		t.Fatal("administrator inherited finance", p, err)
	}
	committee := reviewLogin(t, s, "committee@demo.society")
	p, err = s.CheckSession(ctx, committee)
	if err != nil || p.CanManageAccounts || p.CanManageRecords || !p.CanReviewRequests {
		t.Fatal("committee boundary", p, err)
	}
	grantAppointment(t, s, admin, "demo-user-tenant", "TREASURER", 2)
	token, p := loginTest(t, s, "tenant@demo.society", DemoPassword)
	if !p.MFAPending || p.CanManageRecords || p.CanManageAccounts {
		t.Fatal("new treasury bypassed factor", p)
	}
	if _, err = s.AccountDetailsFor(ctx, token, "demo-user-owner", 1, 1); !errors.Is(err, ErrMFARequired) {
		t.Fatal("pending-factor read", err)
	}
	if _, err = s.SetupMFA(ctx, token); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.DemoVerificationCode(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmMFA(ctx, token, code); err != nil {
		t.Fatal(err)
	}
	p, err = s.CheckSession(ctx, token)
	if err != nil || !p.CanManageRecords || !p.CanReadAllRecords || p.CanManageAccounts || p.CanReviewRequests || p.CanHandleComplaints {
		t.Fatal("treasurer boundary", p, err)
	}
	grantAppointment(t, s, admin, "demo-user-owner", "AUDITOR", 1)
	auditor := reviewLogin(t, s, "owner@demo.society")
	p, err = s.CheckSession(ctx, auditor)
	if err != nil || !p.CanReadAllRecords || !p.CanReadRecords || p.CanReadRegistry || p.CanManageRecords || p.CanManageAccounts || p.CanReviewRequests || p.CanHandleComplaints || p.CanManageDocuments {
		t.Fatal("auditor boundary", p, err)
	}
	if _, err = s.CreateEntry(ctx, auditor, supplied("CHARGE", "1.00")); !errors.Is(err, ErrForbidden) {
		t.Fatal("auditor wrote money", err)
	}
	accessExec(t, s, "UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-owner' AND role='AUDITOR'", time.Now().Add(-time.Hour).Unix(), time.Now().Add(-time.Second).Unix())
	p, err = s.CheckSession(ctx, auditor)
	if err != nil || p.CanReadAllRecords || !p.CanReadRecords {
		t.Fatal("expiry lost independent home entitlement", p, err)
	}
}

func TestCurrentScopeChangesWhenAuthorityOrOneHomeEntitlementChanges(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	read := func(token string) Principal {
		t.Helper()
		p, err := s.CheckSession(ctx, token)
		if err != nil || len(p.ScopeKey) != 64 {
			t.Fatal("missing current scope", p, err)
		}
		return p
	}
	initial := read(owner)
	if read(owner).ScopeKey != initial.ScopeKey || !initial.CanReadRecords || initial.CanReadAllRecords {
		t.Fatal("unstable personal scope", initial)
	}
	// Credential freshness and inactive relationships do not change access.
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=? WHERE token_hash=?", time.Now().Add(-10*time.Minute).Unix(), TokenHash(owner))
	accessExec(t, s, `INSERT INTO flat_memberships(id,flat_id,resident_id,relationship,start_date,end_date,is_primary_contact,can_view_finances) VALUES
        ('scope-history','demo-flat-A-103','demo-owner-A-101','OWNER','1900-01-01','1901-01-01',0,1),
        ('scope-future','demo-flat-A-103','demo-owner-A-101','FAMILY','2099-01-01',NULL,0,1)`)
	if read(owner).ScopeKey != initial.ScopeKey || read(owner).Fresh {
		t.Fatal("ordinary recheck would discard an unchanged form")
	}
	// A remaining financial home keeps the aggregate permission true.
	accessExec(t, s, "UPDATE flat_memberships SET can_view_finances=0 WHERE flat_id='demo-flat-A-102' AND resident_id='demo-owner-A-101' AND end_date IS NULL")
	changed := read(owner)
	if changed.ScopeKey == initial.ScopeKey || !changed.CanReadRecords {
		t.Fatal("financial subset change was hidden by a true permission", changed)
	}
	accessExec(t, s, "UPDATE flat_memberships SET can_view_finances=1 WHERE flat_id='demo-flat-A-102' AND resident_id='demo-owner-A-101' AND end_date IS NULL")
	if read(owner).ScopeKey != initial.ScopeKey {
		t.Fatal("restoring the same scope changed its fingerprint")
	}
	accessExec(t, s, "UPDATE flat_memberships SET relationship='FAMILY' WHERE flat_id='demo-flat-A-102' AND resident_id='demo-owner-A-101' AND end_date IS NULL")
	if read(owner).ScopeKey == initial.ScopeKey {
		t.Fatal("audience relationship change did not invalidate scope")
	}
	accessExec(t, s, "UPDATE flat_memberships SET relationship='OWNER',end_date=? WHERE flat_id='demo-flat-A-102' AND resident_id='demo-owner-A-101' AND end_date IS NULL", today())
	changed = read(owner)
	if changed.ScopeKey == initial.ScopeKey || !changed.CanReadRecords {
		t.Fatal("ending one home was hidden by the remaining home", changed)
	}
	grantAppointment(t, s, admin, "demo-user-owner", "AUDITOR", 1)
	owner = reviewLogin(t, s, "owner@demo.society")
	appointed := read(owner)
	if !appointed.CanReadAllRecords {
		t.Fatal("missing explicit auditor scope")
	}
	accessExec(t, s, "UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-owner' AND role='AUDITOR' AND revoked_at IS NULL", time.Now().Add(-48*time.Hour).Unix(), time.Now().Add(-time.Second).Unix())
	expired := read(owner)
	if expired.ScopeKey == appointed.ScopeKey || expired.ScopeKey != changed.ScopeKey || expired.CanReadAllRecords || !expired.CanReadRecords {
		t.Fatal("expiry did not restore the current independent home scope", expired)
	}
}

func TestAccountAppointmentValidationSelfDenialAndConcurrentVersion(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	target := "demo-user-owner"
	for _, in := range []AppointmentInput{{accessInput(1), "UNKNOWN", 30}, {accessInput(1), "TREASURER", 0}, {accessInput(1), "AUDITOR", 366}, {AccessChange{Version: 1, Reason: "A sufficiently long reason"}, "TREASURER", 30}, {AccessChange{Version: 1, Confirmed: true, Reason: "short"}, "TREASURER", 30}} {
		if _, err := s.GrantAppointment(ctx, admin, target, in); !errors.Is(err, ErrInvalid) {
			t.Fatal("accepted invalid appointment", in, err)
		}
	}
	if _, err := s.GrantAppointment(ctx, admin, "demo-user-admin", AppointmentInput{accessInput(1), "TREASURER", 30}); !errors.Is(err, ErrInvalid) {
		t.Fatal("self financial grant", err)
	}
	if _, err := s.ChangeAccountStatus(ctx, admin, "demo-user-admin", AccountStatusInput{accessInput(1), "SUSPEND"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("self suspension", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, role := range []string{"TREASURER", "AUDITOR"} {
		wg.Add(1)
		go func(role string) {
			defer wg.Done()
			_, err := s.GrantAppointment(ctx, admin, target, AppointmentInput{accessInput(1), role, 30})
			results <- err
		}(role)
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
	x := accountDetails(t, s, admin, target)
	if success != 1 || conflict != 1 || x.Version != 2 || x.GrantTotal != 1 || x.EventTotal != 1 {
		t.Fatal("partial or repeated concurrent write", x, success, conflict)
	}
	if _, err := s.GrantAppointment(ctx, admin, target, AppointmentInput{accessInput(2), x.Grants[0].Role, 30}); !errors.Is(err, ErrInvalid) {
		t.Fatal("duplicate term", err)
	}
	if _, err := s.DB.Exec("UPDATE audit_events SET reason='different' WHERE action='APPOINTMENT_GRANTED'"); err == nil {
		t.Fatal("mutable access audit")
	}
}

func TestAccountAppointmentRevocationIsTargetBoundAndKeepsHomeAccess(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	oldOwner := reviewLogin(t, s, "owner@demo.society")
	x := grantAppointment(t, s, admin, "demo-user-owner", "COMMITTEE", 90)
	if _, err := s.CheckSession(ctx, oldOwner); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("old recipient session survived grant", err)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	p, err := s.CheckSession(ctx, owner)
	if err != nil || !p.CanReviewRequests {
		t.Fatal(p, err)
	}
	if _, err = s.RevokeAppointment(ctx, admin, "demo-user-tenant", x.Grants[0].ID, accessInput(1)); !errors.Is(err, ErrConflict) {
		t.Fatal("cross-target grant revocation", err)
	}
	if _, err = s.RevokeAppointment(ctx, admin, "demo-user-owner", x.Grants[0].ID, accessInput(x.Version)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CheckSession(ctx, owner); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("old recipient session survived revocation", err)
	}
	owner = reviewLogin(t, s, "owner@demo.society")
	p, err = s.CheckSession(ctx, owner)
	if err != nil || p.CanReviewRequests || !p.CanReadRecords {
		t.Fatal("resident entitlement after term", p, err)
	}
	x = accountDetails(t, s, admin, "demo-user-owner")
	if x.Version != 3 || x.Grants[0].State != "REVOKED" || x.Grants[0].RevokedBy != "demo-user-admin" || x.EventTotal != 4 {
		t.Fatal("missing immutable ending", x)
	}
}

func TestAccountSuspensionRevokesCredentialsRolesAndResumeDoesNotRestoreThem(t *testing.T) {
	s, admin := recordFixture(t)
	ctx := context.Background()
	entry := post(t, s, admin, received("25.00"))
	x := grantAppointment(t, s, admin, "demo-user-owner", "TREASURER", 30)
	owner := reviewLogin(t, s, "owner@demo.society")
	link, err := s.IssueAccountLink(ctx, admin, "demo-user-owner", "PASSWORD_RESET", true, "Verified fictional identity for recovery")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangeAccountStatus(ctx, admin, "demo-user-owner", AccountStatusInput{accessInput(x.Version), "SUSPEND"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CheckSession(ctx, owner); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("suspended session survived", err)
	}
	if _, _, err = s.Login(ctx, "owner@demo.society", DemoPassword, HashPassword("dummy")); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("suspended login succeeded", err)
	}
	if _, err = s.InspectAccountLink(ctx, link.Token); !errors.Is(err, ErrLink) {
		t.Fatal("suspended recovery survived", err)
	}
	if _, err = s.IssueAccountLink(ctx, admin, "demo-user-owner", "PASSWORD_RESET", true, "Verified fictional identity for recovery"); !errors.Is(err, ErrInvalid) {
		t.Fatal("recovery bypassed suspension", err)
	}
	var beforeVersion, afterVersion, ended int
	var beforeState, afterState string
	if err = s.DB.QueryRow("SELECT json_extract(before_json,'$.version'),json_extract(after_json,'$.version'),json_extract(before_json,'$.state'),json_extract(after_json,'$.state'),json_extract(after_json,'$.appointments_ended') FROM audit_events WHERE action='ACCOUNT_SUSPENDED' AND json_extract(before_json,'$.user_id')='demo-user-owner'").Scan(&beforeVersion, &afterVersion, &beforeState, &afterState, &ended); err != nil || beforeVersion != 2 || afterVersion != 3 || beforeState != "ACTIVE" || afterState != "SUSPENDED" || ended != 1 {
		t.Fatal("suspension audit lacked exact before/after", beforeVersion, afterVersion, beforeState, afterState, ended, err)
	}
	x = accountDetails(t, s, admin, "demo-user-owner")
	if x.Account.State != "SUSPENDED" || len(x.Account.Roles) != 0 || !x.Account.MFA || x.Grants[0].State != "REVOKED" {
		t.Fatal(x)
	}
	var codes int
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM mfa_recovery_codes WHERE user_id='demo-user-owner'").Scan(&codes); err != nil || codes != 0 {
		t.Fatal("old factor codes survived", codes, err)
	}
	if _, err = s.ChangeAccountStatus(ctx, admin, "demo-user-owner", AccountStatusInput{accessInput(x.Version), "RESUME"}); err != nil {
		t.Fatal(err)
	}
	x = accountDetails(t, s, admin, "demo-user-owner")
	if x.Account.State != "ACTIVE" || len(x.Account.Roles) != 0 || !x.Account.MFA || x.Version != 4 {
		t.Fatal("implicit appointment restoration", x)
	}
	if err = s.DB.QueryRow("SELECT json_extract(before_json,'$.version'),json_extract(after_json,'$.version'),json_extract(before_json,'$.state'),json_extract(after_json,'$.state') FROM audit_events WHERE action='ACCOUNT_RESUMED' AND json_extract(before_json,'$.user_id')='demo-user-owner'").Scan(&beforeVersion, &afterVersion, &beforeState, &afterState); err != nil || beforeVersion != 3 || afterVersion != 4 || beforeState != "SUSPENDED" || afterState != "ACTIVE" {
		t.Fatal("resumption audit lacked exact before/after", beforeVersion, afterVersion, beforeState, afterState, err)
	}
	if _, err = s.InspectAccountLink(ctx, link.Token); !errors.Is(err, ErrLink) {
		t.Fatal("resumed old link", err)
	}
	owner = reviewLogin(t, s, "owner@demo.society")
	p, err := s.CheckSession(ctx, owner)
	if err != nil || p.CanManageRecords || !p.CanReadRecords {
		t.Fatal("resumed wrong scope", p, err)
	}
	if e, err := s.EntryFor(ctx, owner, entry); err != nil || e.AmountPaise != 2500 || e.ReceiptID == "" {
		t.Fatal("suspension changed receipt/financial history", e, err)
	}
}

func TestSuspendedInvitationRequiresExplicitResumeAndNewHandover(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	link := invitationTest(t, s, admin, "pending@example.test", "COMMITTEE")
	var id string
	if err := s.DB.QueryRow("SELECT id FROM users WHERE login='pending@example.test'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeAccountStatus(ctx, admin, id, AccountStatusInput{accessInput(1), "SUSPEND"}); err != nil {
		t.Fatal(err)
	}
	x := accountDetails(t, s, admin, id)
	if x.Account.State != "SUSPENDED" {
		t.Fatal("pending suspension concealed", x)
	}
	if err := s.CompleteAccountLink(ctx, link.Token, "A new fictional password!"); !errors.Is(err, ErrLink) {
		t.Fatal("suspended invitation activated", err)
	}
	if _, err := s.GrantAppointment(ctx, admin, id, AppointmentInput{accessInput(x.Version), "TREASURER", 30}); !errors.Is(err, ErrInvalid) {
		t.Fatal("pending/suspended role grant", err)
	}
	if _, err := s.IssueAccountLink(ctx, admin, id, "INVITE", true, "Verified pending identity again"); !errors.Is(err, ErrInvalid) {
		t.Fatal("new invitation bypassed pause", err)
	}
	if _, err := s.ChangeAccountStatus(ctx, admin, id, AccountStatusInput{accessInput(x.Version), "RESUME"}); err != nil {
		t.Fatal(err)
	}
	x = accountDetails(t, s, admin, id)
	if x.Account.State != "PENDING" || len(x.Account.Roles) != 0 {
		t.Fatal("resumed invitation restored appointment", x)
	}
	newLink, err := s.IssueAccountLink(ctx, admin, id, "INVITE", true, "Verified resumed identity and email")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.InspectAccountLink(ctx, link.Token); !errors.Is(err, ErrLink) {
		t.Fatal("old handover revived", err)
	}
	if err = s.CompleteAccountLink(ctx, newLink.Token, "A new fictional password!"); err != nil {
		t.Fatal(err)
	}
	x = accountDetails(t, s, admin, id)
	if x.Account.State != "ACTIVE" || len(x.Account.Roles) != 0 {
		t.Fatal("activation restored ended role", x)
	}
}

func TestAccountReadsAndChangesUseCurrentFactorsRolesAndBoundedHistory(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	committee := reviewLogin(t, s, "committee@demo.society")
	for _, token := range []string{owner, committee} {
		if _, err := s.AccountDetailsFor(ctx, token, "demo-user-admin", 1, 1); !errors.Is(err, ErrForbidden) {
			t.Fatal("non-admin account detail", err)
		}
		if _, err := s.GrantAppointment(ctx, token, "demo-user-tenant", AppointmentInput{accessInput(1), "TREASURER", 30}); !errors.Is(err, ErrForbidden) {
			t.Fatal("non-admin role write", err)
		}
	}
	for _, pages := range [][2]int{{0, 1}, {1, 10001}} {
		if _, err := s.AccountDetailsFor(ctx, admin, "demo-user-owner", pages[0], pages[1]); !errors.Is(err, ErrInvalid) {
			t.Fatal("unbounded page", err)
		}
	}
	now := time.Now().Unix()
	for i := 0; i < 23; i++ {
		accessExec(t, s, "INSERT INTO role_grants(id,user_id,role,valid_from,valid_until,granted_by) VALUES(?,'demo-user-owner','AUDITOR',?,?,'demo-user-admin')", fmt.Sprint("old-", i), now-5000-int64(i), now-1)
		accessExec(t, s, `INSERT INTO audit_events(actor_user_id,action,occurred_at,reason,before_json,after_json) VALUES('demo-user-admin','APPOINTMENT_GRANTED',?,'Historical fictional appointment','{"user_id":"demo-user-owner"}','{}')`, now-int64(i))
	}
	x := accountDetails(t, s, admin, "demo-user-owner")
	if x.GrantTotal != 23 || len(x.Grants) != 20 || x.EventTotal != 23 || len(x.Events) != 20 || len(x.Account.Roles) != 0 {
		t.Fatal("bounded first page", x)
	}
	x, err := s.AccountDetailsFor(ctx, admin, "demo-user-owner", 2, 2)
	if err != nil || len(x.Grants) != 3 || len(x.Events) != 3 {
		t.Fatal("bounded later page", x, err)
	}
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=? WHERE user_id='demo-user-admin'", time.Now().Add(-6*time.Minute).Unix())
	if _, err = s.ChangeAccountStatus(ctx, admin, "demo-user-owner", AccountStatusInput{accessInput(1), "SUSPEND"}); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("stale mutation", err)
	}
	accessExec(t, s, "UPDATE sessions SET mfa_verified_at=NULL WHERE user_id='demo-user-admin'")
	if _, err = s.AccountDetailsFor(ctx, admin, "demo-user-owner", 1, 1); !errors.Is(err, ErrMFARequired) {
		t.Fatal("factor-pending data", err)
	}
	accessExec(t, s, "UPDATE sessions SET mfa_verified_at=?,reauthenticated_at=? WHERE user_id='demo-user-admin'", now, now)
	var ciphertext []byte
	var confirmedAt, lastStep int64
	if err = s.DB.QueryRow("SELECT secret_ciphertext,confirmed_at,last_step FROM mfa_factors WHERE user_id='demo-user-admin'").Scan(&ciphertext, &confirmedAt, &lastStep); err != nil {
		t.Fatal(err)
	}
	accessExec(t, s, "DELETE FROM mfa_factors WHERE user_id='demo-user-admin'")
	if _, err = s.AccountDetailsFor(ctx, admin, "demo-user-owner", 1, 1); !errors.Is(err, ErrMFARequired) {
		t.Fatal("a past factor verification authorised an account without a current confirmed factor", err)
	}
	p, err := s.CheckSession(ctx, admin)
	if err != nil || !p.MFAPending || p.Fresh || p.CanManageAccounts {
		t.Fatal("missing factor kept ready/fresh authority", p, err)
	}
	accessExec(t, s, "INSERT INTO mfa_factors(user_id,secret_ciphertext,confirmed_at,last_step) VALUES('demo-user-admin',?,?,?)", ciphertext, confirmedAt, lastStep)
	accessExec(t, s, "UPDATE role_grants SET revoked_at=? WHERE user_id='demo-user-admin'", now)
	if _, err = s.AccountDetailsFor(ctx, admin, "demo-user-owner", 1, 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked role retained detail", err)
	}
	if _, err = s.ChangeAccountStatus(ctx, admin, "demo-user-owner", AccountStatusInput{accessInput(1), "SUSPEND"}); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked actor wrote", err)
	}
}

func TestRecoverableAdministratorGuardRequiresCurrentActiveVerifiedFactor(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	x := grantAppointment(t, s, admin, "demo-user-owner", "ADMINISTRATOR", 30)
	check := func(want bool) {
		t.Helper()
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		err = guardSuccessor(ctx, tx, "demo-user-admin")
		if (err == nil) != want {
			t.Fatal("successor readiness", want, err)
		}
	}
	check(false) // A password-activated appointment without an authenticator is not recoverable.
	_ = reviewLogin(t, s, "owner@demo.society")
	check(true)
	accessExec(t, s, "UPDATE users SET status='DISABLED' WHERE id='demo-user-owner'")
	check(false)
	accessExec(t, s, "UPDATE users SET status='ACTIVE' WHERE id='demo-user-owner'")
	accessExec(t, s, "UPDATE role_grants SET valid_from=?,valid_until=? WHERE id=?", time.Now().Add(-time.Hour).Unix(), time.Now().Add(-time.Second).Unix(), x.Grants[0].ID)
	check(false)
	accessExec(t, s, "UPDATE role_grants SET valid_until=? WHERE id=?", time.Now().Add(time.Hour).Unix(), x.Grants[0].ID)
	accessExec(t, s, "UPDATE role_grants SET revoked_at=? WHERE id=?", time.Now().Unix(), x.Grants[0].ID)
	check(false)
}
