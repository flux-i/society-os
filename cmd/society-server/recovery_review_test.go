package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineReviewFlagsDenyServingCustodyAndMissingInputsBeforeOpeningData(t *testing.T) {
	root := t.TempDir()
	db := filepath.Join(root, "must-not-create", "society.db")
	nomination := filepath.Join(root, "absent-private-nomination.json")
	passwords := filepath.Join(root, "absent-private-passwords.json")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	valid := []string{"recover-nominate", "--db", db, "--nomination-file", nomination, "--reviewer-passwords-file", passwords}
	for _, required := range []string{"db", "nomination-file", "reviewer-passwords-file"} {
		args := []string{}
		for i := 0; i < len(valid); i++ {
			if valid[i] == "--"+required {
				i++
				continue
			}
			args = append(args, valid[i])
		}
		if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "recovery review requires") {
			t.Fatal("Incomplete review request crossed pre-I/O denial", required, err)
		}
	}
	for _, extra := range [][]string{{"--demo"}, {"--workspace"}, {"--addr", "127.0.0.1:8080"}, {"--mfa-key-file", "absent.key"}, {"--offsite-config-file", "absent-provider.json"}, {"--custody-identities-file", "absent-private.key"}, {"--out", "new-handover"}} {
		args := append(append([]string{}, valid...), extra...)
		if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "refuse serving") {
			t.Fatal("Offline review accepted an unrelated provider/serving flag", extra[0], err)
		}
	}
	if err := run(context.Background(), []string{"serve", "--demo", "--db", db, "--nomination-file", nomination}, logger); err == nil || !strings.Contains(err.Error(), "exclusively offline") {
		t.Fatal("Serving consumed nomination/private handover inputs", err)
	}
	if err := run(context.Background(), []string{"recover-revoke", "--db", db}, logger); err == nil || !strings.Contains(err.Error(), "recovery review requires") {
		t.Fatal("Incomplete revocation opened a database", err)
	}
	for _, path := range []string{db, nomination, passwords, filepath.Dir(db)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("Denied review opened or created a private path", path, err)
		}
	}
}
