package database

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"society.local/portal/internal/security"
)

func TestSchemaTwoUpgradePreservesRegistryAndRevokesPasswordOnlySessions(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "previous.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.DB.Exec("CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT"); err != nil {
		t.Fatal(err)
	}
	for version := 1; version <= 2; version++ {
		body, err := migrationBody(version)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		if _, err := s.DB.Exec("INSERT INTO schema_migrations VALUES(?,?,?)", version, hex.EncodeToString(sum[:]), "2026-10-01"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if _, err := s.DB.Exec("INSERT INTO users VALUES('demo-user-admin',NULL,'admin@demo.society','Demo Registry Officer',?,'ACTIVE',1,?)", HashPassword(DemoPassword), now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("INSERT INTO role_grants VALUES('grant','demo-user-admin','ADMINISTRATOR',?,?,NULL,NULL)", now-60, now+86400); err != nil {
		t.Fatal(err)
	}
	oldToken := randomToken()
	if _, err := s.DB.Exec("INSERT INTO sessions VALUES(?,'demo-user-admin',1,?,?,?,?)", TokenHash(oldToken), randomToken(), now, now, now+3600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("INSERT INTO audit_events(actor_user_id,action,flat_id,occurred_at,reason,before_json,after_json) VALUES('demo-user-admin','OCCUPANCY_CHANGED','demo-flat-A-101',?,'Previous release audit','{}','{}')", now); err != nil {
		t.Fatal(err)
	}
	before, err := CountRecords(ctx, s.DB)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := CountRecords(ctx, s.DB)
	if err != nil || before != after {
		t.Fatal("upgrade changed registry", err)
	}
	if err := s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckSession(ctx, oldToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("upgrade retained password-only session")
	}
	_, p := loginTest(t, s, "admin@demo.society", DemoPassword)
	if !p.MFAPending || !p.IsDemo || p.CanManageRegistry {
		t.Fatal("upgraded administrator bypassed MFA")
	}
	var reason string
	if err := s.DB.QueryRow("SELECT reason FROM audit_events WHERE id=1").Scan(&reason); err != nil || reason != "Previous release audit" {
		t.Fatal("upgrade lost audit", err)
	}
}

func loginTest(t *testing.T, s *Store, email, password string) (string, Principal) {
	t.Helper()
	token, p, err := s.Login(context.Background(), email, password, HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	return token, p
}
func invitationTest(t *testing.T, s *Store, token, email, role string) IssuedLink {
	t.Helper()
	link, err := s.Invite(context.Background(), token, Invitation{ResidentID: "demo-owner-B-101", Email: email, Role: role, TermDays: 90, IdentityVerified: true, Note: "Verified fictional identity and email in person"})
	if err != nil {
		t.Fatal(err)
	}
	return link
}
func TestPrivilegedMFAEnrollmentReplaySingleUseAndPersistentBudget(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	s.MFA, _ = security.NewBox(make([]byte, 32))
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	token, p := loginTest(t, s, "admin@demo.society", DemoPassword)
	if !p.MFAPending || p.CanReadRegistry || p.CanManageRegistry {
		t.Fatal("password alone gave privileged access")
	}
	if _, err := s.FlatsFor(ctx, FlatFilter{Page: 1, PageSize: 12}, token); !errors.Is(err, ErrMFARequired) {
		t.Fatal("registry read bypassed MFA", err)
	}
	other, _ := loginTest(t, s, "admin@demo.society", DemoPassword)
	setup, err := s.SetupMFA(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
	if err != nil {
		t.Fatal(err)
	}
	code := security.Code(secret, time.Now().Unix()/30, 6)
	result, err := s.ConfirmMFA(ctx, token, code)
	if err != nil {
		t.Fatal(err)
	}
	if result.User.MFAPending || !result.User.Fresh || !result.User.CanManageRegistry || len(result.RecoveryCodes) != 10 {
		t.Fatal("enrollment incomplete")
	}
	if _, err := s.CheckSession(ctx, other); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("enrollment did not revoke another device")
	}
	token2, _ := loginTest(t, s, "admin@demo.society", DemoPassword)
	if _, err := s.VerifyMFA(ctx, token2, code, false); !errors.Is(err, ErrVerification) {
		t.Fatal("enrollment code replay succeeded", err)
	}
	token3, _ := loginTest(t, s, "admin@demo.society", DemoPassword)
	var wg sync.WaitGroup
	out := make(chan error, 2)
	for _, tok := range []string{token2, token3} {
		wg.Add(1)
		go func(tok string) {
			defer wg.Done()
			_, err := s.VerifyMFA(ctx, tok, result.RecoveryCodes[0], true)
			out <- err
		}(tok)
	}
	wg.Wait()
	close(out)
	success, failed := 0, 0
	for err := range out {
		if err == nil {
			success++
		} else if errors.Is(err, ErrVerification) {
			failed++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || failed != 1 {
		t.Fatal("recovery code was not consumed atomically")
	}
	newCodes, err := s.RegenerateRecovery(ctx, token)
	if err != nil || len(newCodes) != 10 {
		t.Fatal(err)
	}
	if _, err := s.VerifyMFA(ctx, token2, result.RecoveryCodes[1], true); !errors.Is(err, ErrVerification) {
		t.Fatal("replaced code accepted")
	}
	// The budget is shared by all sessions, including new password logins.
	if _, err := s.VerifyMFA(ctx, token2, newCodes[0], true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := s.VerifyMFA(ctx, token2, "invalid", true); !errors.Is(err, ErrVerification) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	newest, _ := loginTest(t, s, "admin@demo.society", DemoPassword)
	if _, err := s.VerifyMFA(ctx, newest, newCodes[1], true); !errors.Is(err, ErrThrottled) {
		t.Fatal("new login reset MFA attempt budget", err)
	}
	var events string
	rows, err := s.DB.Query("SELECT before_json||after_json FROM audit_events")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		rows.Scan(&value)
		events += value
	}
	for _, private := range []string{setup.Secret, code, token, result.RecoveryCodes[0], newCodes[0], DemoPassword} {
		if strings.Contains(events, private) {
			t.Fatal("credential leaked to audit")
		}
	}
}
func TestInvitationActivationAndConcurrentConsumption(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	link := invitationTest(t, s, admin, "neighbour@example.test", "RESIDENT")
	if _, _, err := s.Login(ctx, "neighbour@example.test", DemoPassword, HashPassword("dummy")); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("unverified invited account signed in")
	}
	details, err := s.InspectAccountLink(ctx, link.Token)
	if err != nil || details.Name != "Demo Owner B-101" {
		t.Fatal("link inspection", err)
	}
	out := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); out <- s.CompleteAccountLink(ctx, link.Token, "A memorable new password!") }()
	}
	wg.Wait()
	close(out)
	ok, used := 0, 0
	for err := range out {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrLink) {
			used++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || used != 1 {
		t.Fatal("activation consumed twice")
	}
	if _, err := s.InspectAccountLink(ctx, link.Token); !errors.Is(err, ErrLink) {
		t.Fatal("consumed invitation inspected")
	}
	token, p := loginTest(t, s, details.Email, "A memorable new password!")
	if p.IsDemo || p.MFAPending || p.CanReadRegistry || p.CanManageRegistry {
		t.Fatal("resident gained extra privilege")
	}
	page, err := s.FlatsFor(ctx, FlatFilter{Page: 1, PageSize: 12}, token)
	if err != nil || page.Total != 1 || page.Items[0].BuildingCode != "B" || page.Items[0].Number != "101" {
		t.Fatal("invited resident scope", err)
	}
	if _, _, err := s.DemoVerificationCode(ctx, token); !errors.Is(err, ErrForbidden) {
		t.Fatal("invited account could use demo bypass")
	}
	if _, err := s.Invite(ctx, admin, Invitation{ResidentID: "demo-owner-B-101", Email: "duplicate@example.test", Role: "RESIDENT", IdentityVerified: true, Note: "Verified fictional identity again"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("duplicate person account")
	}
}
func TestInvitationEligibilityRevocationExpiryAndFreshAuth(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	for _, bad := range []Invitation{
		{ResidentID: "demo-owner-B-101", Email: "x@example.test", Role: "TREASURER", IdentityVerified: true, Note: "Verified fictional identity in person"},
		{ResidentID: "demo-former-tenant-A-103", Email: "x@example.test", Role: "RESIDENT", IdentityVerified: true, Note: "Verified fictional identity in person"},
		{ResidentID: "demo-owner-B-101", Email: "x@example.test", Role: "ADMINISTRATOR", TermDays: 366, IdentityVerified: true, Note: "Verified fictional identity in person"},
		{ResidentID: "demo-owner-B-101", Email: "x@example.test", Role: "RESIDENT", Note: "Verified fictional identity in person"},
	} {
		if _, err := s.Invite(ctx, admin, bad); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid invitation accepted", err)
		}
	}
	link := invitationTest(t, s, admin, "new-admin@example.test", "ADMINISTRATOR")
	var id string
	s.DB.QueryRow("SELECT id FROM users WHERE login=?", "new-admin@example.test").Scan(&id)
	replacement, err := s.IssueAccountLink(ctx, admin, id, "INVITE", true, "Reconfirmed fictional person in person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InspectAccountLink(ctx, link.Token); !errors.Is(err, ErrLink) {
		t.Fatal("old invitation still valid")
	}
	s.DB.Exec("UPDATE account_tokens SET expires_at=0 WHERE token_hash=?", TokenHash(replacement.Token))
	if err := s.CompleteAccountLink(ctx, replacement.Token, "A memorable new password!"); !errors.Is(err, ErrLink) {
		t.Fatal("expired invitation accepted")
	}
	replacement, err = s.IssueAccountLink(ctx, admin, id, "INVITE", true, "Reconfirmed fictional person in person")
	if err != nil {
		t.Fatal(err)
	}
	s.DB.Exec("UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-B-101'", today())
	if err := s.CompleteAccountLink(ctx, replacement.Token, "A memorable new password!"); !errors.Is(err, ErrLink) {
		t.Fatal("ended member activated")
	}
	s.DB.Exec("UPDATE sessions SET reauthenticated_at=? WHERE token_hash=?", time.Now().Add(-6*time.Minute).Unix(), TokenHash(admin))
	if _, err := s.IssueAccountLink(ctx, admin, "demo-user-owner", "PASSWORD_RESET", true, "Reconfirmed fictional identity in person"); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("stale admin issued reset", err)
	}
	code, isRecovery, err := s.DemoVerificationCode(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reauthenticate(ctx, admin, DemoPassword, code, isRecovery); err != nil {
		t.Fatal(err)
	}
	if _, err := s.IssueAccountLink(ctx, admin, "demo-user-owner", "PASSWORD_RESET", true, "Reconfirmed fictional identity in person"); err != nil {
		t.Fatal(err)
	}
}
func TestPasswordResetKeepsMFARevokesSessionsAndOfflineRecovery(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	link := invitationTest(t, s, admin, "new-admin@example.test", "ADMINISTRATOR")
	if err := s.CompleteAccountLink(ctx, link.Token, "A memorable new password!"); err != nil {
		t.Fatal(err)
	}
	token, p := loginTest(t, s, "new-admin@example.test", "A memorable new password!")
	if !p.MFAPending || !p.MFARequired || p.IsDemo {
		t.Fatal("invited administrator not gated")
	}
	setup, err := s.SetupMFA(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
	factor, err := s.ConfirmMFA(ctx, token, security.Code(secret, time.Now().Unix()/30, 6))
	if err != nil {
		t.Fatal(err)
	}
	other, _ := loginTest(t, s, "new-admin@example.test", "A memorable new password!")
	reset, err := s.IssueAccountLink(ctx, admin, p.ID, "PASSWORD_RESET", true, "Verified synthetic administrator again")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteAccountLink(ctx, reset.Token, "A different new password!"); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{token, other} {
		if _, err := s.CheckSession(ctx, tok); !errors.Is(err, ErrUnauthenticated) {
			t.Fatal("reset left session active")
		}
	}
	token, p = loginTest(t, s, "new-admin@example.test", "A different new password!")
	if !p.MFAEnrolled || !p.MFAPending {
		t.Fatal("reset removed MFA")
	}
	if _, err := s.VerifyMFA(ctx, token, factor.RecoveryCodes[0], true); err != nil {
		t.Fatal("reset lost valid real recovery code", err)
	}
	if err := s.OfflineMFARecovery(ctx, "new-admin@example.test", "Custodian One", "Custodian One", "Lost fictional authenticator"); !errors.Is(err, ErrInvalid) {
		t.Fatal("one custodian could reset MFA")
	}
	if err := s.OfflineMFARecovery(ctx, "new-admin@example.test", "Custodian One", "Custodian Two", "Lost fictional authenticator"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckSession(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("offline recovery left sessions active")
	}
	token, p = loginTest(t, s, "new-admin@example.test", "A different new password!")
	if p.MFAEnrolled || !p.MFAPending || p.CanManageRegistry {
		t.Fatal("lost-factor recovery bypassed reenrollment")
	}
	if _, _, err := s.DemoVerificationCode(ctx, token); !errors.Is(err, ErrForbidden) {
		t.Fatal("non-demo MFA shortcut allowed")
	}
	if _, err := s.VerifyMFA(ctx, token, factor.RecoveryCodes[1], true); !errors.Is(err, ErrMFARequired) {
		t.Fatal("offline reset kept factor")
	}
}
