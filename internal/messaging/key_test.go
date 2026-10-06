package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
)

func TestSigningKeyCustodyMissingExistingHistoryPermissionsAndSymlinkRejection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "private", "messages.key")
	if _, err := LoadKey(path, false); err == nil {
		t.Fatal("missing key silently replaced")
	}
	key, err := LoadKey(path, true)
	if err != nil || len(key) != 32 {
		t.Fatal(err)
	}
	again, err := LoadKey(path, true)
	if err != nil || !bytes.Equal(key, again) {
		t.Fatal("startup rotated signing key", err)
	}
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadKey(path, false); err == nil {
		t.Fatal("public key file accepted")
	}
	if err = os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "private", "link.key")
	if err = os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadKey(link, true); err == nil {
		t.Fatal("symbolic key file accepted")
	}
	if err = os.WriteFile(path, []byte(strings.Repeat("a", 129)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadKey(path, false); err == nil {
		t.Fatal("oversized key accepted")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadKey(path, false); err == nil {
		t.Fatal("missing held key regenerated")
	}
}
func TestCallbackSignatureBoundsTimestampAndStrictJSONBeforeAnyDatabaseAccess(t *testing.T) {
	e, err := New(&database.Store{}, bytes.Repeat([]byte{42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	valid, _ := json.Marshal(database.MessageCallback{EventID: "callback-valid-12345", ProviderID: "simulation-test", State: "READ", At: now.Unix()})
	for _, tc := range []struct {
		payload   []byte
		signature string
		expected  error
	}{
		{valid, strings.Repeat("0", 64), database.ErrForbidden},
		{valid, "invalid", database.ErrForbidden},
		{[]byte(strings.Repeat(" ", 8193)), strings.Repeat("0", 64), database.ErrForbidden},
		{append(append([]byte{}, valid...), []byte("{}")...), "", database.ErrInvalid},
		{[]byte(`{"event_id":"callback-valid-12345","provider_id":"simulation-test","state":"READ","at":0}`), "", database.ErrForbidden},
		{[]byte(`{"event_id":"callback-valid-12345","provider_id":"simulation-test","state":"READ","at":1,"destination":"private@example.test"}`), "", database.ErrInvalid},
	} {
		sig := tc.signature
		if sig == "" {
			sig = e.Signature(tc.payload)
		}
		if err = e.Callback(context.Background(), tc.payload, sig, now); !errors.Is(err, tc.expected) {
			t.Fatal(err, tc.expected)
		}
	}
	if _, err = New(&database.Store{}, make([]byte, 31)); err == nil {
		t.Fatal("short signing key accepted")
	}
}

func TestSigningKeyFingerprintSurvivesStartupAndRejectsWrongOrMissingCustody(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "society.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	first, _ := New(store, bytes.Repeat([]byte{7}, 32))
	wrong, _ := New(store, bytes.Repeat([]byte{8}, 32))
	if err = first.VerifyKey(ctx, false); err == nil {
		t.Fatal("missing binding silently accepted")
	}
	if err = first.VerifyKey(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err = first.VerifyKey(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err = wrong.VerifyKey(ctx, true); err == nil {
		t.Fatal("wrong key replaced the database fingerprint")
	}
	if err = first.VerifyKey(ctx, false); err != nil {
		t.Fatal("correct key no longer accepted", err)
	}
}
