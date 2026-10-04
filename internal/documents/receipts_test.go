package documents

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"society.local/portal/internal/database"
)

func TestReceiptPDFUnicodeAmountPrivateFilesIntegrityAndTraversal(t *testing.T) {
	receipt := database.ReceiptSnapshot{Number: "SOS-2026-000001", Home: "A-101", Payer: "Demo Owner A-101", AmountPaise: 40001, Date: "2026-01-01", Description: "Fictional maintenance already paid", Method: "BANK_TRANSFER", Reference: "DEMO-101", Operator: "Demo Registry Officer", IssuedAt: 1767225600}
	pdf, err := Render(receipt)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("PDF render", err)
	}
	root := filepath.Join(t.TempDir(), "documents")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, err := store.Put(pdf)
	if err != nil {
		t.Fatal(err)
	}
	read, err := store.Read(hash)
	if err != nil || !bytes.Equal(read, pdf) {
		t.Fatal("private read", err)
	}
	for _, path := range []string{root, filepath.Join(root, hash+".pdf")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0600)
		if info.IsDir() {
			want = 0700
		}
		if info.Mode().Perm() != want {
			t.Fatal("public private file", info.Mode())
		}
	}
	for _, value := range []string{"../society.db", "../../keys/mfa.key", "", strings.Repeat("g", 64)} {
		if _, err := store.Read(value); err == nil {
			t.Fatal("accepted traversal", value)
		}
	}
	if err = os.WriteFile(filepath.Join(root, hash+".pdf"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Read(hash); err == nil {
		t.Fatal("tampered file accepted")
	}
	// Regeneration replaces only the derived blob, preserving the immutable receipt identity.
	if _, err = store.Put(pdf); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Read(hash); err != nil {
		t.Fatal(err)
	}
	receipt.Payer = strings.Repeat("W", 120)
	receipt.Description = strings.Repeat("W", 300)
	receipt.Reference = strings.Repeat("W", 120)
	receipt.Operator = strings.Repeat("W", 120)
	long, err := Render(receipt)
	if err != nil || len(long) == 0 {
		t.Fatal("maximum-length fields require safe wrapping/pages", err)
	}
}
