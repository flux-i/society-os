package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"filippo.io/age"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func custodyPair(t *testing.T) ([]age.Recipient, []*age.X25519Identity) {
	t.Helper()
	var recipients []age.Recipient
	var identities []*age.X25519Identity
	for range 2 {
		key, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatal(err)
		}
		identities = append(identities, key)
		recipients = append(recipients, key.Recipient())
	}
	return recipients, identities
}

func persistentSnapshotRows(t *testing.T, path string) map[string][]string {
	t.Helper()
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	uri.RawQuery = "mode=ro"
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tables, err := db.Query("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for tables.Next() {
		var name string
		if err := tables.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := tables.Close(); err != nil {
		t.Fatal(err)
	}
	result := map[string][]string{}
	for _, name := range names {
		rows, err := db.Query(`SELECT * FROM "` + strings.ReplaceAll(name, `"`, `""`) + `"`)
		if err != nil {
			t.Fatal(err)
		}
		columns, _ := rows.Columns()
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range targets {
			targets[i] = &values[i]
		}
		result[name] = []string{}
		for rows.Next() {
			if err := rows.Scan(targets...); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			result[name] = append(result[name], string(encoded))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		sort.Strings(result[name])
	}
	return result
}

func TestEncryptedPopulatedSnapshotRestoresWithEitherIndependentCustodyIdentity(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	token := upkeepRecoveryLogin(t, s, "admin@demo.society")
	create := func(kind, amount, key string) string {
		in := database.EntryInput{OperationKey: key, FlatID: "demo-flat-A-101", Kind: kind, Amount: amount, Date: "2026-10-09", Description: "Independently supplied recovery record"}
		if kind == "RECEIVED" {
			in.Payer, in.Method, in.Reference = "Fictional owner", "BANK_TRANSFER", "FICTIONAL-RECOVERY-1"
		}
		id, err := s.CreateEntry(ctx, token, in)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.PostEntry(ctx, token, id, database.EntryAction{OperationKey: key + "-post", Confirmed: true}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	create("OPENING_DEBIT", "1000.00", "encrypted-opening-001")
	create("CHARGE", "500.01", "encrypted-charge-001")
	paid := create("RECEIVED", "400.00", "encrypted-received-001")
	original, err := s.EntryFor(ctx, token, paid)
	if err != nil || original.ReceiptNumber == "" || original.AmountPaise != 40000 {
		t.Fatal("independent original receipt", err)
	}
	if _, err := s.ReverseEntry(ctx, token, paid, database.EntryAction{OperationKey: "encrypted-correction-001", Confirmed: true, Reason: "Verified fictional correction; original receipt preserved"}); err != nil {
		t.Fatal(err)
	}
	ledger, err := s.EntriesFor(ctx, token, "demo-flat-A-101", "", "", false, 1)
	if err != nil || ledger.BalancePaise != 150001 {
		t.Fatal("independent corrected balance", ledger.BalancePaise, err)
	}
	bundle := filepath.Join(root, "snapshot")
	start := time.Now().Add(-time.Second).Unix()
	if _, err := Snapshot(ctx, s, bundle, "0.33.0-dev"); err != nil {
		t.Fatal(err)
	}
	recipients, identities := custodyPair(t)
	encrypted := filepath.Join(root, "original.tar.gz.age")
	info := ArchiveInfo{FormatVersion: 1, CapturedAt: start, JobID: "fictional-recovery-original", ConfigHash: strings.Repeat("b", 64)}
	hash, size, err := EncryptArchive(ctx, bundle, encrypted, info, recipients)
	if err != nil || size <= 0 {
		t.Fatal(err)
	}
	ciphertext, _ := os.ReadFile(encrypted)
	if bytes.Contains(ciphertext, []byte("SQLite format")) || bytes.Contains(ciphertext, []byte("Fictional owner")) || !bytes.HasPrefix(ciphertext, []byte("age-encryption.org/v1\n")) {
		t.Fatal("standard encrypted envelope or plaintext isolation")
	}
	want := persistentSnapshotRows(t, filepath.Join(bundle, "society.db"))
	if len(want) != 117 { // 116 persistent business tables plus migration provenance.
		t.Fatal("complete current snapshot inventory", len(want))
	}
	for i, identity := range identities {
		extracted := filepath.Join(root, "custodian-"+string(rune('a'+i)))
		got, manifest, err := DecryptArchive(ctx, encrypted, hash, extracted, []age.Identity{identity})
		if err != nil || got.CapturedAt != start || got.JobID != info.JobID || manifest.SchemaVersion != database.SchemaVersion {
			t.Fatal("independent custodian decrypt", i, got, err)
		}
		target := filepath.Join(root, "restored-"+string(rune('a'+i))+".db")
		if _, err := Restore(ctx, extracted, target); err != nil {
			t.Fatal(err)
		}
		assertQuarantinedOriginals(t, extracted, target)
		restored, err := database.Open(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		restored.MFA = s.MFA
		if _, err := restored.CheckSession(ctx, token); !errors.Is(err, database.ErrUnauthenticated) {
			t.Fatal("old bearer revived", err)
		}
		fresh := upkeepRecoveryLogin(t, restored, "admin@demo.society")
		if _, err := restored.EntryFor(ctx, fresh, paid); !errors.Is(err, database.ErrRecoveryRequired) {
			t.Fatal("Copied Treasury authority read a private receipt before review", err)
		}
		var number, state string
		var amount, balance int64
		if err := restored.DB.QueryRow("SELECT r.number,e.amount_paise,CASE WHEN EXISTS(SELECT 1 FROM entry_reversals WHERE entry_id=e.id) THEN 'REVERSED' ELSE e.state END FROM entries e JOIN receipts r ON r.entry_id=e.id WHERE e.id=?", paid).Scan(&number, &amount, &state); err != nil || number != original.ReceiptNumber || amount != 40000 || state != "REVERSED" {
			t.Fatal("Retained original receipt or linked correction changed", err)
		}
		if err := restored.DB.QueryRow("SELECT SUM(CASE WHEN kind='RECEIVED' THEN -amount_paise ELSE amount_paise END) FROM entries WHERE state='POSTED' AND id NOT IN(SELECT entry_id FROM entry_reversals)").Scan(&balance); err != nil || balance != 150001 {
			t.Fatal("Recovered independently expected balance changed", balance, err)
		}
		restored.Close()
	}
}

func TestEncryptedRecoveryRejectsWrongChecksumKeyCorruptionTruncationAndExistingDestination(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	bundle := filepath.Join(root, "source")
	Snapshot(ctx, s, bundle, "0.30.0-dev")
	recipients, identities := custodyPair(t)
	path := filepath.Join(root, "valid.age")
	hash, _, err := EncryptArchive(ctx, bundle, path, ArchiveInfo{FormatVersion: 1, CapturedAt: time.Now().Unix(), JobID: "fixture", ConfigHash: strings.Repeat("c", 64)}, recipients)
	if err != nil {
		t.Fatal(err)
	}
	wrong, _ := age.GenerateX25519Identity()
	for name, identity := range map[string]age.Identity{"wrong-key": wrong, "wrong-checksum": identities[0]} {
		checksum := hash
		if name == "wrong-checksum" {
			checksum = strings.Repeat("f", 64)
		}
		destination := filepath.Join(root, name)
		if _, _, err := DecryptArchive(ctx, path, checksum, destination, []age.Identity{identity}); err == nil {
			t.Fatal("invalid independent input accepted", name)
		}
		if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("failed plaintext destination retained", name)
		}
	}
	original, _ := os.ReadFile(path)
	for _, name := range []string{"corrupt", "truncated", "appended"} {
		data := append([]byte(nil), original...)
		if name == "corrupt" {
			data[len(data)-1] ^= 1
		} else if name == "truncated" {
			data = data[:len(data)-10]
		} else {
			data = append(data, []byte("untrusted trailing bytes")...)
		}
		altered := filepath.Join(root, name+".age")
		os.WriteFile(altered, data, 0600)
		sum := sha256.Sum256(data)
		dest := filepath.Join(root, name)
		if _, _, err := DecryptArchive(ctx, altered, hex.EncodeToString(sum[:]), dest, []age.Identity{identities[0]}); err == nil {
			t.Fatal("malformed authenticated envelope accepted", name)
		}
		if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("partial plaintext retained", name)
		}
	}
	existing := filepath.Join(root, "existing")
	os.Mkdir(existing, 0700)
	os.WriteFile(filepath.Join(existing, "preserve.txt"), []byte("preserve unrelated checkpoint"), 0600)
	if _, _, err := DecryptArchive(ctx, path, hash, existing, []age.Identity{identities[0]}); err == nil {
		t.Fatal("existing destination replaced")
	}
	if data, _ := os.ReadFile(filepath.Join(existing, "preserve.txt")); string(data) != "preserve unrelated checkpoint" {
		t.Fatal("existing destination changed")
	}
}

func TestEncryptedArchiveRejectsTraversalLinksDuplicatesOversizedHeadersAndTrailingPayload(t *testing.T) {
	recipients, identities := custodyPair(t)
	for _, name := range []string{"traversal", "link", "duplicate", "oversized", "trailing"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "malicious.age")
			f, _ := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			encrypted, _ := age.Encrypt(f, recipients...)
			compressed := gzip.NewWriter(encrypted)
			archive := tar.NewWriter(compressed)
			header := &tar.Header{Name: "archive.json", Typeflag: tar.TypeReg, Size: 2, Mode: 0600}
			archive.WriteHeader(header)
			archive.Write([]byte("{}"))
			bad := &tar.Header{Name: "society.db", Typeflag: tar.TypeReg, Size: 2, Mode: 0600}
			switch name {
			case "traversal":
				bad.Name = "../escape"
			case "link":
				bad.Typeflag, bad.Linkname, bad.Size = tar.TypeSymlink, "../../escape", 0
			case "duplicate":
				bad.Name = "archive.json"
			case "oversized":
				bad.Size = MaxSnapshotBytes + 1
			}
			archive.WriteHeader(bad)
			if bad.Size == 2 {
				archive.Write([]byte("{}"))
			}
			archive.Close()
			if name == "trailing" {
				io.WriteString(compressed, "untrusted appended compressed payload")
			}
			compressed.Close()
			encrypted.Close()
			f.Close()
			hash, _, _ := digest(path)
			dest := filepath.Join(root, "extracted")
			if _, _, err := DecryptArchive(context.Background(), path, hash, dest, []age.Identity{identities[0]}); err == nil {
				t.Fatal("malicious archive accepted", name)
			}
			if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed extraction retained", name)
			}
			if _, err := os.Stat(filepath.Join(root, "escape")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("archive escaped destination")
			}
		})
	}
}
