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
