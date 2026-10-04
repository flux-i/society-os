package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func setup(t *testing.T) (*database.Store, string) {
	t.Helper()
	root := t.TempDir()
	s, err := database.Open(context.Background(), filepath.Join(root, "live", "society.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedDemo(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, root
}

func TestRecoveryPreservesIdentityAndAuditButNeverRevivesSessions(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	token, _, err := s.Login(ctx, "admin@demo.society", database.DemoPassword, database.HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	if _, err := s.SetupMFA(ctx, token); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.DemoVerificationCode(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	mfa, err := s.ConfirmMFA(ctx, token, code)
	if err != nil {
		t.Fatal(err)
	}
	link, err := s.IssueAccountLink(ctx, token, "demo-user-owner", "PASSWORD_RESET", true, "Verified synthetic owner in person")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeOccupancy(ctx, token, "demo-flat-A-101", database.OccupancyChange{RegistryChange: database.RegistryChange{Version: 1, Reason: "Checkpointed registry change"}, Status: "VACANT"}); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "authenticated-checkpoint")
	if _, err := Snapshot(ctx, s, bundle, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckSession(ctx, token); err != nil {
		t.Fatal("snapshot logged out the live user")
	}
	target := filepath.Join(root, "recovered.db")
	if _, err := Restore(ctx, bundle, target); err != nil {
		t.Fatal(err)
	}
	recovered, err := database.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	recovered.MFA = s.MFA
	if err := recovered.VerifyMFAKey(ctx); err != nil {
		t.Fatal("encrypted factor did not survive", err)
	}
	for _, table := range []string{"sessions", "account_tokens", "mfa_recovery_codes", "mfa_pending"} {
		var n int
		if err := recovered.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatal("restored temporary credentials", table, n, err)
		}
	}
	if _, err := recovered.InspectAccountLink(ctx, link.Token); !errors.Is(err, database.ErrLink) {
		t.Fatal("reset link revived")
	}
	if _, err := s.InspectAccountLink(ctx, link.Token); err != nil {
		t.Fatal("snapshot revoked live link", err)
	}
	if _, err := recovered.CheckSession(ctx, token); !errors.Is(err, database.ErrUnauthenticated) {
		t.Fatal("restored database revived a session")
	}
	newToken, _, err := recovered.Login(ctx, "admin@demo.society", database.DemoPassword, database.HashPassword("dummy"))
	if err != nil {
		t.Fatal("identity was lost in recovery")
	}
	if _, err := recovered.VerifyMFA(ctx, newToken, mfa.RecoveryCodes[0], true); !errors.Is(err, database.ErrVerification) {
		t.Fatal("saved recovery code revived")
	}
	code, isRecovery, err := recovered.DemoVerificationCode(ctx, newToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recovered.VerifyMFA(ctx, newToken, code, isRecovery); err != nil {
		t.Fatal(err)
	}
	events, err := recovered.ActivityFor(ctx, newToken, "demo-flat-A-101")
	if err != nil || len(events) != 1 || events[0].Reason != "Checkpointed registry change" {
		t.Fatal("audited registry write was lost")
	}
}

func TestSnapshotRestoresCommittedWALAndNotLaterChanges(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if _, err := s.DB.Exec("UPDATE residents SET full_name = 'Demo Checkpoint Owner' WHERE id = 'demo-owner-A-101'"); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "checkpoint")
	manifest, err := Snapshot(ctx, s, bundle, "test")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Counts.Flats != 118 || manifest.SHA256 == "" {
		t.Fatalf("invalid checkpoint manifest: %+v", manifest)
	}
	if _, err := s.DB.Exec("UPDATE residents SET full_name = 'Demo Later Owner' WHERE id = 'demo-owner-A-101'"); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "restored", "society.db")
	recovered, err := Restore(ctx, bundle, destination)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Counts != manifest.Counts {
		t.Fatal("restored counts differ")
	}
	restored, err := database.Open(ctx, destination)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err := restored.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := restored.DB.QueryRow("SELECT full_name FROM residents WHERE id = 'demo-owner-A-101'").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Demo Checkpoint Owner" {
		t.Fatalf("snapshot lost committed WAL or included a later write: %s", name)
	}
	if err := restored.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(ctx, bundle, destination); err == nil {
		t.Fatal("active destination overwritten")
	}
	if _, err := Snapshot(ctx, s, bundle, "test"); err == nil {
		t.Fatal("existing checkpoint overwritten")
	}
}

func TestCorruptedSnapshotRejectedBeforeRestore(t *testing.T) {
	s, root := setup(t)
	bundle := filepath.Join(root, "checkpoint")
	if _, err := Snapshot(context.Background(), s, bundle, "test"); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(bundle, "society.db"), os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("damaged"), 100); err != nil {
		t.Fatal(err)
	}
	f.Close()
	destination := filepath.Join(root, "should-not-exist.db")
	if _, err := Restore(context.Background(), bundle, destination); err == nil {
		t.Fatal("corrupted snapshot restored")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("failed verification created a destination")
	}
}

func TestRestoreRefusesExistingSidecar(t *testing.T) {
	s, root := setup(t)
	bundle := filepath.Join(root, "checkpoint")
	if _, err := Snapshot(context.Background(), s, bundle, "test"); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "target.db")
	if err := os.WriteFile(destination+"-wal", []byte("existing sidecar"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(context.Background(), bundle, destination); err == nil {
		t.Fatal("existing WAL sidecar ignored")
	}
}
