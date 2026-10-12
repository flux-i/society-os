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

	"society.local/portal/internal/backup"
)

func TestOffsiteCustodyCommandRetainsPrivateIdentityButNeverServesOrPrintsIt(t *testing.T) {
	root := t.TempDir()
	identity := filepath.Join(root, "custody", "identity.txt")
	db := filepath.Join(root, "never-opened", "society.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	output, err := os.CreateTemp(root, "stdout-")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	previous := os.Stdout
	os.Stdout = output
	defer func() { os.Stdout = previous }()
	if err := run(context.Background(), []string{"offsite-keygen", "--out", identity, "--db", db}, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := output.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	public, err := io.ReadAll(output)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal(public, &result); err != nil || len(result) != 1 || !strings.HasPrefix(result["public_recipient"], "age1") || strings.Contains(string(public), "AGE-SECRET-KEY-") {
		t.Fatal("Key command did not restrict output to a public recipient", err)
	}
	private, err := os.ReadFile(identity)
	if err != nil || !strings.HasPrefix(string(private), "AGE-SECRET-KEY-") {
		t.Fatal("Independent private custody material was not retained", err)
	}
	stat, err := os.Lstat(identity)
	if err != nil || !stat.Mode().IsRegular() || stat.Mode().Perm() != 0600 {
		t.Fatal("Custody identity file was not private", err)
	}
	if _, err := backup.LoadCustodyIdentities(identity); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"offsite-keygen", "--out", identity}, logger); err == nil {
		t.Fatal("Key command overwrote a retained identity")
	}
	unchanged, err := os.ReadFile(identity)
	if err != nil || string(unchanged) != string(private) {
		t.Fatal("Refused overwrite damaged the original identity", err)
	}
	if err := run(context.Background(), []string{"serve", "--db", db, "--custody-identities-file", identity}, logger); err == nil || !strings.Contains(err.Error(), "refuses private") {
		t.Fatal("Running server accepted a decryption identity", err)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatal("Custody command unexpectedly opened a live database", err)
	}
}

func TestOffsiteArchiveFlagsDenyIncompleteOrProviderCoupledRequestsBeforeOpeningData(t *testing.T) {
	root := t.TempDir()
	db, identity, output := filepath.Join(root, "absent-source.db"), filepath.Join(root, "absent-private.key"), filepath.Join(root, "must-not-create")
	valid := []string{"offsite-archive", "--db", db, "--custody-identities-file", identity, "--backup-id", strings.Repeat("C", 43), "--out", output, "--reason", "Independent verification of the unchanged retained original", "--encrypted-file", filepath.Join(root, "absent.age")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, required := range []string{"db", "custody-identities-file", "backup-id", "out", "reason", "encrypted-file"} {
		t.Run("missing-"+required, func(t *testing.T) {
			var args []string
			for i := 0; i < len(valid); i++ {
				if valid[i] == "--"+required {
					i++
					continue
				}
				args = append(args, valid[i])
			}
			if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "offline archival requires") {
				t.Fatal("Incomplete command crossed the pre-I/O boundary", err)
			}
		})
	}
	for _, extra := range [][]string{{"--offsite-config-file", filepath.Join(root, "absent-provider.json")}, {"--object-version", "unsupported-version"}, {"--offsite-fixture"}} {
		args := append(append([]string(nil), valid...), extra...)
		if err := run(context.Background(), args, logger); err == nil || !strings.Contains(err.Error(), "offline archival requires") {
			t.Fatal("Custody command accepted upload/reconciliation configuration", err)
		}
	}
	for _, path := range []string{db, identity, output} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("Rejected command created or opened data", path, err)
		}
	}
}
