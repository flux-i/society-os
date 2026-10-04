package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func TestServeRefusesMissingOrWrongMFAKeyWhenFactorsExist(t *testing.T) {
	root := t.TempDir()
	db := filepath.Join(root, "db", "society.db")
	web := filepath.Join(root, "web")
	if err := os.Mkdir(web, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("<title>Local QA</title>"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := run(ctx, []string{"seed-demo", "--demo", "--db", db}, logger); err != nil {
		t.Fatal(err)
	}
	s, err := database.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	token, _, err := s.Login(ctx, "admin@demo.society", database.DemoPassword, database.HashPassword("dummy"))
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
	s.Close()
	missing := filepath.Join(root, "keys", "missing.key")
	args := []string{"serve", "--demo", "--db", db, "--web-dir", web, "--mfa-key-file", missing}
	if err := run(ctx, args, logger); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatal("missing key did not fail closed", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("startup regenerated lost key")
	}
	if _, err := security.LoadKey(missing, true); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, args, logger); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatal("wrong key did not fail closed", err)
	}
}
