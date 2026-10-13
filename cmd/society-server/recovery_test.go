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

func TestOfflineRecoveryFlagsRejectIncompleteOrProviderCoupledRequestsBeforeOpeningData(t *testing.T) {
	root := t.TempDir()
	identity, cipher, destination := filepath.Join(root, "absent-private.key"), filepath.Join(root, "absent.age"), filepath.Join(root, "must-not-create", "society.db")
	valid := []string{"recover-restore", "--custody-identities-file", identity, "--encrypted-file", cipher, "--expected-sha256", strings.Repeat("a", 64), "--out", destination}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, required := range []string{"custody-identities-file", "encrypted-file", "expected-sha256", "out"} {
		t.Run("missing-"+required, func(t *testing.T) {
			var args []string
			for index := 0; index < len(valid); index++ {
				if valid[index] == "--"+required {
					index++
					continue
				}
				args = append(args, valid[index])
			}
			if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "offline recovery requires") {
				t.Fatal("Incomplete recovery crossed pre-I/O denial", err)
			}
		})
	}
	for _, extra := range [][]string{{"--db", filepath.Join(root, "active.db")}, {"--offsite-config-file", "absent-provider.json"}, {"--object-version", "supplied-version"}, {"--offsite-fixture"}, {"--snapshot", "ordinary-copy"}, {"--demo"}} {
		args := append(append([]string{}, valid...), extra...)
		if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "offline recovery requires") {
			t.Fatal("Provider/active-workspace flags crossed offline boundary", extra[0], err)
		}
	}
	for _, path := range []string{identity, cipher, destination, filepath.Join(root, "active.db")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("Refused command opened or created sensitive data", path, err)
		}
	}
}
