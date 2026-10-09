package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func TestBootstrapRequiresExplicitPrivateDestinationsAndPreservesBoundKeysOnRetry(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	root := t.TempDir()
	setup := database.WorkspaceSetup{FormatVersion: 1, SocietyKey: "rehearsal-society", SocietyName: "Sample local rehearsal", Mode: "FICTIONAL_REHEARSAL", BootstrapID: "setup-test", AdministratorName: "Sample Custodian", AdministratorEmail: "registry@example.test", IdentityVerified: true, VerificationNote: "Verified fictional local custodian and supplied identity", TermDays: 90}
	body, _ := json.Marshal(setup)
	setupPath := filepath.Join(root, "setup.json")
	passwordPath := filepath.Join(root, "password.txt")
	if err := os.WriteFile(setupPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passwordPath, []byte("Private-fictional-setup-password!\n"), 0600); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(root, "data", "society.db")
	mfa := filepath.Join(root, "keys", "mfa.key")
	message := filepath.Join(root, "keys", "message.key")
	base := []string{"bootstrap", "--db", db, "--setup-file", setupPath, "--password-file", passwordPath, "--mfa-key-file", mfa, "--message-key-file", message}
	for _, args := range [][]string{{"bootstrap"}, {"bootstrap", "--db", db, "--setup-file", setupPath, "--password-file", passwordPath}, {"bootstrap", "--db", db, "--setup-file", setupPath, "--password-file", passwordPath, "--mfa-key-file", mfa, "--message-key-file", mfa}} {
		if err := run(ctx, args, logger); err == nil {
			t.Fatal("implicit/overlapping destinations accepted")
		}
		if _, err := os.Stat(db); !os.IsNotExist(err) {
			t.Fatal("rejected command created database", err)
		}
	}
	if err := os.Chmod(passwordPath, 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, base, logger); err == nil {
		t.Fatal("public password file accepted")
	}
	if err := os.Chmod(passwordPath, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "password-link.txt")
	if err := os.Symlink(passwordPath, link); err != nil {
		t.Fatal(err)
	}
	linked := append([]string{}, base...)
	// Locate the value rather than assuming flag ordering in future edits.
	for i := range linked {
		if i > 0 && linked[i-1] == "--password-file" {
			linked[i] = link
		}
	}
	if err := run(ctx, linked, logger); err == nil {
		t.Fatal("symlink secret accepted")
	}
	if err := run(ctx, base, logger); err != nil {
		t.Fatal(err)
	}
	mfaBytes, err := os.ReadFile(mfa)
	if err != nil {
		t.Fatal(err)
	}
	messageBytes, err := os.ReadFile(message)
	if err != nil {
		t.Fatal(err)
	}
	if err = run(ctx, base, logger); err != nil {
		t.Fatal("safe setup retry", err)
	}
	nextMFA, _ := os.ReadFile(mfa)
	nextMessage, _ := os.ReadFile(message)
	if string(mfaBytes) != string(nextMFA) || string(messageBytes) != string(nextMessage) {
		t.Fatal("retry replaced keys")
	}
	s, err := database.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	var users, roles, demos int
	err = s.DB.QueryRow("SELECT (SELECT COUNT(*) FROM users),(SELECT COUNT(*) FROM role_grants),(SELECT COUNT(*) FROM users WHERE is_demo=1)").Scan(&users, &roles, &demos)
	s.Close()
	if err != nil || users != 1 || roles != 1 || demos != 0 {
		t.Fatal("bootstrap seeded extra identities", err)
	}
	if err = os.Rename(mfa, mfa+".retained"); err != nil {
		t.Fatal(err)
	}
	if err = run(ctx, base, logger); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatal("missing setup key recreated", err)
	}
	if _, err = os.Stat(mfa); !os.IsNotExist(err) {
		t.Fatal("missing key replaced")
	}
	if _, err = security.LoadKey(mfa, true); err != nil {
		t.Fatal(err)
	}
	if err = run(ctx, base, logger); err == nil {
		t.Fatal("changed setup key accepted")
	}
}

func TestBootstrapOccupiedDestinationKeepsItsSchemaAndCreatesNoKeys(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "existing.db")
	s, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("CREATE TABLE prior_business(id INTEGER PRIMARY KEY,note TEXT); INSERT INTO prior_business VALUES(1,'retained original business row')"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	setup := database.WorkspaceSetup{FormatVersion: 1, SocietyKey: "rehearsal-society", SocietyName: "Sample rehearsal", Mode: "FICTIONAL_REHEARSAL", BootstrapID: "setup", AdministratorName: "Sample Custodian", AdministratorEmail: "registry@example.test", IdentityVerified: true, VerificationNote: "Verified fictional setup identity for rejection coverage", TermDays: 90}
	body, _ := json.Marshal(setup)
	setupPath := filepath.Join(root, "setup.json")
	secretPath := filepath.Join(root, "password.txt")
	os.WriteFile(setupPath, body, 0600)
	os.WriteFile(secretPath, []byte("Private-fictional-password!"), 0600)
	mfa := filepath.Join(root, "keys", "mfa.key")
	message := filepath.Join(root, "keys", "message.key")
	args := []string{"bootstrap", "--db", path, "--setup-file", setupPath, "--password-file", secretPath, "--mfa-key-file", mfa, "--message-key-file", message}
	if err = run(ctx, args, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil || !strings.Contains(err.Error(), "empty database") {
		t.Fatal("occupied destination accepted", err)
	}
	s, err = database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var tables, rows int
	if err = s.DB.QueryRow("SELECT (SELECT COUNT(*) FROM sqlite_schema WHERE type='table'),(SELECT COUNT(*) FROM prior_business)").Scan(&tables, &rows); err != nil || tables != 1 || rows != 1 {
		t.Fatal("refused setup changed schema or data", tables, rows, err)
	}
	for _, key := range []string{mfa, message} {
		if _, err = os.Stat(key); !os.IsNotExist(err) {
			t.Fatal("refused setup created keys", err)
		}
	}
}

func TestWorkspaceServeRefusesDemoImplicitPathsAndReplacementKeysBeforeMFAEnrollment(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	root := t.TempDir()
	db := filepath.Join(root, "data", "workspace.db")
	s, err := database.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	mfa := filepath.Join(root, "keys", "mfa.key")
	message := filepath.Join(root, "keys", "messages.key")
	box, err := security.LoadKey(mfa, true)
	if err != nil {
		t.Fatal(err)
	}
	setup := database.WorkspaceSetup{FormatVersion: 1, SocietyKey: "local-test", SocietyName: "Sample local workspace", Mode: "LOCAL_WORKSPACE", BootstrapID: "setup-test", AdministratorName: "Sample Custodian", AdministratorEmail: "registry@example.test", IdentityVerified: true, VerificationNote: "Verified fictional local custodian for testing", TermDays: 90}
	if _, err = s.BootstrapWorkspace(ctx, setup, "Private-fictional-password!", box.Fingerprint(), strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	s.Close()
	web := filepath.Join(root, "web")
	os.Mkdir(web, 0700)
	os.WriteFile(filepath.Join(web, "index.html"), []byte("<title>Local QA</title>"), 0600)
	for _, args := range [][]string{{"serve", "--workspace", "--db", db, "--web-dir", web}, {"serve", "--demo", "--db", db, "--web-dir", web, "--mfa-key-file", mfa}, {"seed-demo", "--demo", "--db", db}, {"serve", "--workspace", "--db", db, "--web-dir", web, "--mfa-key-file", mfa, "--message-key-file", message}} {
		if err = run(ctx, args, logger); err == nil {
			t.Fatal("unsafe workspace startup accepted")
		}
	}
	if _, err = os.Stat(message); !os.IsNotExist(err) {
		t.Fatal("startup created missing bound message key")
	}
}
