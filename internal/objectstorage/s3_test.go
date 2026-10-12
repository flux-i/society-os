package objectstorage_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/objectstorage"
	"society.local/portal/internal/testfixtures"
)

func object(t *testing.T) (string, string, int64) {
	t.Helper()
	body := []byte("independently supplied fictional encrypted object bytes")
	path := filepath.Join(t.TempDir(), "original.age")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return path, hex.EncodeToString(sum[:]), int64(len(body))
}

func TestSignedCreateOnlyOriginalAndSeparateExactVersionCustody(t *testing.T) {
	fixture := testfixtures.NewS3(t)
	writer, err := objectstorage.New(fixture.Config("writer"), true)
	if err != nil {
		t.Fatal(err)
	}
	path, hash, bytes := object(t)
	key := "recovery/society/2026/10/09/original.tar.gz.age"
	ack, err := writer.Put(context.Background(), key, path, hash, bytes, time.Now())
	if err != nil || ack.SHA256 != hash || ack.Bytes != bytes || ack.Version != "fixture-version-1" {
		t.Fatal("signed checksum/version acknowledgement", ack, err)
	}
	if _, err = writer.Put(context.Background(), key, path, hash, bytes, time.Now()); err == nil {
		t.Fatal("existing key was overwritten")
	} else {
		var failure *objectstorage.UploadError
		if !errors.As(err, &failure) || !failure.Uncertain {
			t.Fatal("conflict counted as confirmed or silently retryable", err)
		}
	}
	if err = writer.GetVersion(context.Background(), key, ack.Version, filepath.Join(t.TempDir(), "writer-read.age"), hash, bytes, time.Now()); err == nil {
		t.Fatal("writer obtained custody read authority")
	}
	reader, _ := objectstorage.New(fixture.Config("reader"), true)
	destination := filepath.Join(t.TempDir(), "custody-original.age")
	if err = reader.GetVersion(context.Background(), key, ack.Version, destination, hash, bytes, time.Now()); err != nil {
		t.Fatal("exact independently signed custody retrieval", err)
	}
	original, _ := os.ReadFile(path)
	retrieved, _ := os.ReadFile(destination)
	if string(original) != string(retrieved) {
		t.Fatal("retrieved original bytes changed")
	}
	if _, err = reader.Put(context.Background(), "recovery/society/reader-write.age", path, hash, bytes, time.Now()); err == nil {
		t.Fatal("custody read principal obtained upload authority")
	}
	if err = reader.GetVersion(context.Background(), key, ack.Version, destination, hash, bytes, time.Now()); err == nil {
		t.Fatal("existing custody destination overwritten")
	}
}

func TestUnverifiableResponsesStayUncertainWithoutAutomaticRetry(t *testing.T) {
	for _, mode := range []string{"LOST_REPLY", "MISSING_CHECKSUM", "WRONG_CHECKSUM", "MISSING_VERSION", "NULL_VERSION", "SERVICE_ERROR"} {
		t.Run(mode, func(t *testing.T) {
			fixture := testfixtures.NewS3(t)
			fixture.SetMode(mode)
			writer, _ := objectstorage.New(fixture.Config("writer"), true)
			path, hash, bytes := object(t)
			_, err := writer.Put(context.Background(), "recovery/society/original.age", path, hash, bytes, time.Now())
			var failure *objectstorage.UploadError
			if !errors.As(err, &failure) || !failure.Uncertain || failure.Code != "UPLOAD_UNCERTAIN" {
				t.Fatal("unverifiable response accepted", err)
			}
			puts, _ := fixture.Counts()
			if puts != 1 {
				t.Fatal("automatic network retry", puts)
			}
		})
	}
}

func TestRejectedCredentialsRedirectsAndLocalInputNeverBecomeAcknowledgements(t *testing.T) {
	for _, mode := range []string{"DENIED", "REJECTED", "REDIRECT"} {
		t.Run(mode, func(t *testing.T) {
			fixture := testfixtures.NewS3(t)
			fixture.SetMode(mode)
			writer, _ := objectstorage.New(fixture.Config("writer"), true)
			path, hash, bytes := object(t)
			_, err := writer.Put(context.Background(), "recovery/society/original.age", path, hash, bytes, time.Now())
			var failure *objectstorage.UploadError
			if !errors.As(err, &failure) || failure.Uncertain {
				t.Fatal("explicit rejection incorrectly acknowledged", err)
			}
			puts, gets := fixture.Counts()
			if puts != 1 || gets != 0 {
				t.Fatal("unexpected retry/redirect", puts, gets)
			}
		})
	}
	fixture := testfixtures.NewS3(t)
	writer, _ := objectstorage.New(fixture.Config("writer"), true)
	path, hash, bytes := object(t)
	for _, key := range []string{"recovery/other/original.age", "recovery/society/../escape.age"} {
		if _, err := writer.Put(context.Background(), key, path, hash, bytes, time.Now()); err == nil {
			t.Fatal("invalid private prefix accepted")
		}
	}
	if _, err := writer.Put(context.Background(), "recovery/society/original.age", path, strings.Repeat("a", 64), bytes, time.Now()); err == nil {
		t.Fatal("changed original uploaded")
	}
	if puts, _ := fixture.Counts(); puts != 0 {
		t.Fatal("invalid local input transmitted")
	}
}

func TestIndependentCustodyRejectsChangedBytesAndVersion(t *testing.T) {
	fixture := testfixtures.NewS3(t)
	writer, _ := objectstorage.New(fixture.Config("writer"), true)
	reader, _ := objectstorage.New(fixture.Config("reader"), true)
	path, hash, bytes := object(t)
	key := "recovery/society/original.age"
	ack, err := writer.Put(context.Background(), key, path, hash, bytes, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"CORRUPT_GET", "WRONG_GET_VERSION"} {
		fixture.SetMode(mode)
		dest := filepath.Join(t.TempDir(), "changed.age")
		if err := reader.GetVersion(context.Background(), key, ack.Version, dest, hash, bytes, time.Now()); err == nil {
			t.Fatal("changed custody object accepted", mode)
		}
		if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("failed custody download retained as usable", mode)
		}
	}
}

func TestStorageEndpointAndPrivateConfigurationAreStrict(t *testing.T) {
	fixture := testfixtures.NewS3(t)
	config := fixture.Config("writer")
	if _, err := objectstorage.New(config, false); err == nil {
		t.Fatal("fixture enabled implicitly")
	}
	for _, endpoint := range []string{"http://localhost:9999", "http://192.0.2.1:9999", "https://127.0.0.1:9999", "http://127.0.0.1:9999?secret=unused"} {
		config.Endpoint = endpoint
		if _, err := objectstorage.New(config, true); err == nil {
			t.Fatal("invalid fixture endpoint", endpoint)
		}
	}
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{"mode":"FIXTURE","secret_extra":"never echo this"}`), 0600)
	var parsed objectstorage.Config
	if err := objectstorage.DecodePrivate(path, &parsed); err == nil || strings.Contains(err.Error(), "never echo this") {
		t.Fatal("unsupported or exposed configuration", err)
	}
	os.Chmod(path, 0644)
	if _, err := objectstorage.ReadPrivate(path, 32768); err == nil {
		t.Fatal("public configuration read")
	}
}
