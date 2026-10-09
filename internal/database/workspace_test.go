package database

import (
	"context"
	"encoding/base32"
	"errors"
	"path/filepath"
	"society.local/portal/internal/security"
	"strings"
	"testing"
	"time"
)

const workspacePassword = "Fictional-setup-password-2026!"

func workspaceSetup() WorkspaceSetup {
	return WorkspaceSetup{FormatVersion: 1, SocietyKey: "rehearsal-society", SocietyName: "The Neighbourhood Rehearsal", Mode: "FICTIONAL_REHEARSAL", BootstrapID: "setup-october-2026", AdministratorName: "Sample Registry Custodian", AdministratorEmail: "registry@example.test", IdentityVerified: true, VerificationNote: "Fictional custodian identity verified for local rehearsal", TermDays: 90}
}
func workspaceFixture(t *testing.T) (*Store, string) {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	if _, err = s.BootstrapWorkspace(ctx, workspaceSetup(), workspacePassword, s.MFA.Fingerprint(), InputDigest([]byte("different fictional private signing key"))); err != nil {
		t.Fatal(err)
	}
	token, p := loginTest(t, s, "registry@example.test", workspacePassword)
	if !p.MFAPending || p.IsDemo || p.CanManageRegistry || p.CanManageRecords {
		t.Fatal("workspace first login must require real MFA without finance", p)
	}
	if _, _, err = s.DemoVerificationCode(ctx, token); err == nil {
		t.Fatal("non-demo account obtained a preview verification code")
	}
	setup, err := s.SetupMFA(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ConfirmMFA(ctx, token, security.Code(secret, time.Now().Unix()/30, 6))
	if err != nil {
		t.Fatal(err)
	}
	if !result.User.CanManageRegistry || result.User.CanManageRecords || result.User.CanReadRecords || result.User.IsDemo || result.User.MFAPending {
		t.Fatal("MFA identity/finance separation", result.User)
	}
	return s, token
}

func TestWorkspaceBootstrapOneAdministratorRealMFASeparateFinanceAndExactRetry(t *testing.T) {
	s, token := workspaceFixture(t)
	ctx := context.Background()
	for _, query := range []string{"SELECT COUNT(*) FROM users", "SELECT COUNT(*) FROM role_grants", "SELECT COUNT(*) FROM workspace_setup", "SELECT COUNT(*) FROM audit_events WHERE action='WORKSPACE_BOOTSTRAPPED'"} {
		maintenanceCount(t, s, query, 1)
	}
	for _, query := range []string{"SELECT COUNT(*) FROM buildings", "SELECT COUNT(*) FROM flats", "SELECT COUNT(*) FROM residents", "SELECT COUNT(*) FROM users WHERE is_demo=1", "SELECT COUNT(*) FROM role_grants WHERE role IN ('TREASURER','AUDITOR','COMMITTEE')"} {
		maintenanceCount(t, s, query, 0)
	}
	// A setup retry cannot reset a later password, appointment or authenticator.
	changed := HashPassword("Later-fictional-password-2026!")
	accessExec(t, s, "UPDATE users SET password_hash=? WHERE login='registry@example.test'", changed)
	if _, err := s.BootstrapWorkspace(ctx, workspaceSetup(), workspacePassword, s.MFA.Fingerprint(), InputDigest([]byte("different fictional private signing key"))); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := s.DB.QueryRow("SELECT password_hash FROM users").Scan(&got); err != nil || got != changed {
		t.Fatal("retry reset password", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM mfa_factors", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='WORKSPACE_BOOTSTRAPPED'", 1)
	for _, mutate := range []func(*WorkspaceSetup){func(x *WorkspaceSetup) { x.SocietyName = "Different society" }, func(x *WorkspaceSetup) { x.BootstrapID = "different-operation" }, func(x *WorkspaceSetup) { x.AdministratorEmail = "different@example.test" }} {
		input := workspaceSetup()
		mutate(&input)
		if _, err := s.BootstrapWorkspace(ctx, input, workspacePassword, s.MFA.Fingerprint(), InputDigest([]byte("different fictional private signing key"))); !errors.Is(err, ErrConflict) {
			t.Fatal("changed setup replay accepted", err)
		}
	}
	if _, err := s.BootstrapWorkspace(ctx, workspaceSetup(), "Different-password-2026!", s.MFA.Fingerprint(), InputDigest([]byte("different fictional private signing key"))); !errors.Is(err, ErrConflict) {
		t.Fatal("changed password accepted", err)
	}
	if err := s.VerifyWorkspaceKeys(ctx, s.MFA.Fingerprint(), strings.Repeat("f", 64)); err == nil {
		t.Fatal("replacement key accepted")
	}
	if err := s.RequireDemo(ctx); err == nil {
		t.Fatal("workspace passed demo guard")
	}
	if err := s.SeedDemo(ctx); err == nil {
		t.Fatal("seed mixed demo rows")
	}
	if err := s.SeedDemoAccounts(ctx); err == nil {
		t.Fatal("seed mixed demo accounts")
	}
	if err := s.SeedDemoTreasury(ctx); err == nil {
		t.Fatal("seed mixed demo finance")
	}
	if _, err := s.RegistryImportStatus(ctx, token); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceBootstrapRefusesOccupiedDatabaseAndImmutableSetup(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	if _, err := s.BootstrapWorkspace(ctx, workspaceSetup(), workspacePassword, strings.Repeat("a", 64), strings.Repeat("b", 64)); err == nil {
		t.Fatal("bootstrap replaced existing registry")
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM flats", 118)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM workspace_setup", 0)
	workspace, _ := workspaceFixture(t)
	for _, query := range []string{"UPDATE workspace_setup SET society_name='changed'", "DELETE FROM workspace_setup"} {
		if _, err := workspace.DB.Exec(query); err == nil {
			t.Fatal("setup provenance changed")
		}
	}
}
