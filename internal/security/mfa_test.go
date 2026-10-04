package security

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRFC6238PublishedSHA1VectorsAndReplayWindow(t *testing.T) {
	secret := []byte("12345678901234567890")
	// RFC 6238 Appendix B, independent published expected values.
	for _, v := range []struct {
		second int64
		code   string
	}{{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"}, {1234567890, "89005924"}, {2000000000, "69279037"}, {20000000000, "65353130"}} {
		if got := Code(secret, v.second/30, 8); got != v.code {
			t.Fatalf("RFC vector at %d: %s", v.second, got)
		}
	}
	now := time.Unix(1234567890, 0)
	step := now.Unix() / 30
	for _, accepted := range []int64{step - 1, step, step + 1} {
		code := Code(secret, accepted, 6)
		matched, ok := Match(secret, code, now, -1)
		if !ok || matched != accepted {
			t.Fatal("valid drift code rejected")
		}
		if _, ok := Match(secret, code, now, accepted); ok {
			t.Fatal("same or older timestep reused")
		}
	}
	for _, code := range []string{"", "12345", "1234567", "abcdef", Code(secret, step-2, 6), Code(secret, step+2, 6)} {
		if _, ok := Match(secret, code, now, -1); ok {
			t.Fatal("invalid/outside window code accepted")
		}
	}
}
func TestEncryptedSecretsAreBoundToUserAndPrivateKeyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "mfa.key")
	box, err := LoadKey(path, true)
	if err != nil {
		t.Fatal(err)
	}
	secret := NewSecret()
	ciphertext := box.Seal("alice", secret)
	if bytes.Contains(ciphertext, secret) {
		t.Fatal("plaintext persisted")
	}
	same, err := LoadKey(path, false)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := same.Open("alice", ciphertext)
	if err != nil || !bytes.Equal(recovered, secret) {
		t.Fatal("key could not be reopened")
	}
	wrong, _ := NewBox(make([]byte, 32))
	if _, err := wrong.Open("alice", ciphertext); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := box.Open("bob", ciphertext); err == nil {
		t.Fatal("factor transplanted to another user")
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err := box.Open("alice", ciphertext); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKey(path, false); err == nil {
		t.Fatal("public key file accepted")
	}
	missing := filepath.Join(t.TempDir(), "absent.key")
	if _, err := LoadKey(missing, false); err == nil {
		t.Fatal("missing key silently regenerated")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("missing key created during recovery")
	}
}
