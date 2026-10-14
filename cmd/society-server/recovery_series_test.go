package main

import (
	"context"
	"crypto/sha256"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seriesCLIInputs(root, command string) []string {
	args := []string{command, "--db", filepath.Join(root, "existing-private.db"), "--series-config-file", filepath.Join(root, "private-storage.json")}
	switch command {
	case "recover-series-reserve":
		return append(args, "--series-request-file", filepath.Join(root, "private-request.json"))
	case "recover-series-retry":
		return append(args, "--series-retry-file", filepath.Join(root, "private-retry.json"))
	default:
		return append(args, "--series-id", "independent-retained-intent-17", "--object-version", "independent-original-version", "--custody-identities-file", filepath.Join(root, "separate-custodian.key"))
	}
}

func TestOfflineSeriesCommandsReachExplicitExistingDatabaseGuard(t *testing.T) {
	root := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, command := range []string{"recover-series-reserve", "recover-series-retry", "recover-series-reconcile"} {
		if err := run(context.Background(), seriesCLIInputs(root, command), logger); err == nil || !strings.Contains(err.Error(), "existing regular private database") {
			t.Fatal("Registered offline command did not reach the existing-file guard", command, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("A missing offline database or custody input was created", err)
	}
}

func TestOfflineSeriesFlagsDenyServingCustodyAndIncompleteInputsBeforeIO(t *testing.T) {
	root := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, command := range []string{"recover-series-reserve", "recover-series-retry", "recover-series-reconcile"} {
		valid := seriesCLIInputs(root, command)
		for i := 1; i < len(valid); i += 2 {
			args := append(append([]string{}, valid[:i]...), valid[i+2:]...)
			if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "requires an explicit existing") {
				t.Fatal("Incomplete series command crossed pre-I/O denial", command, valid[i], err)
			}
		}
		for _, extra := range [][]string{{"--demo"}, {"--workspace"}, {"--addr", "127.0.0.1:8080"}, {"--mfa-key-file", "absent.key"}, {"--offsite-config-file", "absent-provider.json"}, {"--out", "new-output"}, {"--reason", "unrelated recovery action"}, {"--nomination-file", "absent-nomination.json"}} {
			args := append(append([]string{}, valid...), extra...)
			if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "refuse serving") {
				t.Fatal("Offline series accepted unrelated authority/provider flags", command, extra[0], err)
			}
		}
		if command != "recover-series-reconcile" {
			args := append(append([]string{}, valid...), "--custody-identities-file", "absent-private-identity.key")
			if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "refuse serving") {
				t.Fatal("A write command consumed private decryption custody", command, err)
			}
		}
	}
	for _, command := range []string{"serve", "inspect", "snapshot", "recover-nominate"} {
		for _, flag := range []string{"series-request-file", "series-retry-file", "series-config-file", "series-id", "series-fixture"} {
			args := []string{command, "--" + flag}
			if flag != "series-fixture" {
				args = append(args, "absent-private-input")
			}
			if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "exclusively offline") {
				t.Fatal("Another command consumed series custody flags", command, flag, err)
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("Denied flags created a database or private input", err)
	}
}

func TestOfflineSeriesStrictPrivateInputFailsBeforeDatabaseOpen(t *testing.T) {
	root := t.TempDir()
	db := filepath.Join(root, "existing-private.db")
	original := []byte("independently retained original; must never be opened as SQLite")
	if err := os.WriteFile(db, original, 0600); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, command := range []string{"recover-series-reserve", "recover-series-retry"} {
		args := seriesCLIInputs(root, command)
		input := args[len(args)-1]
		for _, body := range []string{`{}`, `{"format_version":1,"unsupported_secret":"PRIVATE-NOT-FOR-LOGS"}`, strings.Repeat(" ", 8193)} {
			if err := os.WriteFile(input, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			err := run(context.Background(), args, logger)
			if err == nil || strings.Contains(err.Error(), "unknown command") || strings.Contains(err.Error(), "PRIVATE-NOT-FOR-LOGS") || strings.Contains(err.Error(), "SQLite") {
				t.Fatal("Invalid private input reached data access or disclosed its contents", command, err)
			}
			contents, e := os.ReadFile(db)
			if e != nil || sha256.Sum256(contents) != sha256.Sum256(original) {
				t.Fatal("Invalid private input modified the retained source", e)
			}
		}
		if err := os.Chmod(input, 0644); err != nil {
			t.Fatal(err)
		}
		if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "private") {
			t.Fatal("World-readable custody input was consumed", err)
		}
		if err := os.Remove(input); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(db, input); err != nil {
			t.Fatal(err)
		}
		if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "private") {
			t.Fatal("Symlinked custody input was consumed", err)
		}
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(db + suffix); !os.IsNotExist(err) {
			t.Fatal("Rejected private input opened a database sidecar", suffix, err)
		}
	}
}
