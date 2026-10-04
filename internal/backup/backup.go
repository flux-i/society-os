package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"society.local/portal/internal/database"
)

type Manifest struct {
	FormatVersion      int             `json:"format_version"`
	CreatedAt          string          `json:"created_at"`
	ApplicationVersion string          `json:"application_version"`
	SchemaVersion      int             `json:"schema_version"`
	SQLiteVersion      string          `json:"sqlite_version"`
	DataKind           string          `json:"data_kind"`
	FixtureVersion     string          `json:"fixture_version"`
	SHA256             string          `json:"sha256"`
	Bytes              int64           `json:"bytes"`
	Counts             database.Counts `json:"counts"`
}

func digest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

func inspect(ctx context.Context, path string) (Manifest, error) {
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	uri.RawQuery = url.Values{"mode": {"ro"}, "_pragma": {"foreign_keys(ON)", "busy_timeout(5000)"}}.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return Manifest{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := (&database.Store{DB: db}).VerifySchema(ctx); err != nil {
		return Manifest{}, err
	}
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return Manifest{}, err
	}
	if integrity != "ok" {
		return Manifest{}, errors.New("snapshot failed integrity check")
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return Manifest{}, err
	}
	invalid := rows.Next()
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Manifest{}, err
	}
	if invalid {
		return Manifest{}, errors.New("snapshot has invalid foreign keys")
	}
	var result Manifest
	if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&result.SQLiteVersion); err != nil {
		return Manifest{}, err
	}
	if err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&result.SchemaVersion); err != nil {
		return Manifest{}, err
	}
	if result.SchemaVersion != database.SchemaVersion {
		return Manifest{}, errors.New("snapshot schema is not supported by this application")
	}
	if err := db.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = 'data_kind'").Scan(&result.DataKind); err != nil {
		return Manifest{}, err
	}
	if result.DataKind != "synthetic" {
		return Manifest{}, errors.New("local recovery milestone accepts synthetic data only")
	}
	if err := db.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = 'fixture_version'").Scan(&result.FixtureVersion); err != nil {
		return Manifest{}, err
	}
	result.Counts, err = database.CountRecords(ctx, db)
	return result, err
}

func Snapshot(ctx context.Context, store *database.Store, bundle, applicationVersion string) (Manifest, error) {
	if err := store.RequireDemo(ctx); err != nil {
		return Manifest{}, err
	}
	abs, err := filepath.Abs(bundle)
	if err != nil {
		return Manifest{}, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return Manifest{}, err
	}
	// A new private bundle prevents accidental overwrite of an older checkpoint.
	if err := os.Mkdir(abs, 0700); err != nil {
		return Manifest{}, fmt.Errorf("snapshot bundle must be new: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			os.RemoveAll(abs)
		}
	}()
	path := filepath.Join(abs, "society.db")
	if _, err := store.DB.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return Manifest{}, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		return Manifest{}, err
	}
	// Recovery never revives a bearer session. Strip sessions only from the private copy.
	snapshot, err := database.Open(ctx, path)
	if err != nil {
		return Manifest{}, err
	}
	tx, purgeErr := snapshot.DB.BeginTx(ctx, nil)
	if purgeErr == nil {
		for _, table := range []string{"sessions", "account_tokens", "mfa_recovery_codes"} {
			if _, purgeErr = tx.ExecContext(ctx, "DELETE FROM "+table); purgeErr != nil {
				break
			}
		}
		if purgeErr == nil {
			purgeErr = tx.Commit()
		} else {
			tx.Rollback()
		}
	}
	snapshotCloseErr := snapshot.Close()
	if purgeErr != nil {
		return Manifest{}, purgeErr
	}
	if snapshotCloseErr != nil {
		return Manifest{}, snapshotCloseErr
	}
	if err := syncPath(path); err != nil {
		return Manifest{}, err
	}
	result, err := inspect(ctx, path)
	if err != nil {
		return Manifest{}, err
	}
	result.FormatVersion = 1
	result.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	result.ApplicationVersion = applicationVersion
	result.SHA256, result.Bytes, err = digest(path)
	if err != nil {
		return Manifest{}, err
	}
	f, err := os.OpenFile(filepath.Join(abs, "manifest.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Manifest{}, err
	}
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(result)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return Manifest{}, err
	}
	if closeErr != nil {
		return Manifest{}, closeErr
	}
	if err := syncPath(abs); err != nil {
		return Manifest{}, err
	}
	if err := syncPath(filepath.Dir(abs)); err != nil {
		return Manifest{}, err
	}
	complete = true
	return result, nil
}

func Verify(ctx context.Context, bundle string) (Manifest, error) {
	abs, err := filepath.Abs(bundle)
	if err != nil {
		return Manifest{}, err
	}
	data, err := os.ReadFile(filepath.Join(abs, "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	var expected Manifest
	if err := json.Unmarshal(data, &expected); err != nil {
		return Manifest{}, err
	}
	if expected.FormatVersion != 1 {
		return Manifest{}, errors.New("unknown snapshot format")
	}
	path := filepath.Join(abs, "society.db")
	hash, bytes, err := digest(path)
	if err != nil {
		return Manifest{}, err
	}
	if hash != expected.SHA256 || bytes != expected.Bytes {
		return Manifest{}, errors.New("snapshot checksum or size mismatch")
	}
	actual, err := inspect(ctx, path)
	if err != nil {
		return Manifest{}, err
	}
	if actual.Counts != expected.Counts || actual.SchemaVersion != expected.SchemaVersion || actual.DataKind != expected.DataKind || actual.FixtureVersion != expected.FixtureVersion {
		return Manifest{}, errors.New("snapshot metadata does not match manifest")
	}
	return expected, nil
}

func Restore(ctx context.Context, bundle, destination string) (Manifest, error) {
	manifest, err := Verify(ctx, bundle)
	if err != nil {
		return Manifest{}, err
	}
	abs, err := filepath.Abs(destination)
	if err != nil {
		return Manifest{}, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return Manifest{}, err
	}
	// Never replace an active database or its WAL. Restoration is into a fresh file.
	for _, path := range []string{abs, abs + "-wal", abs + "-shm"} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return Manifest{}, errors.New("restore destination and sidecars must not exist")
		}
	}
	sourcePath, err := filepath.Abs(filepath.Join(bundle, "society.db"))
	if err != nil {
		return Manifest{}, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return Manifest{}, err
	}
	defer source.Close()
	target, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Manifest{}, err
	}
	complete := false
	defer func() {
		if !complete {
			os.Remove(abs)
		}
	}()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(target, h), source)
	if err == nil {
		err = target.Sync()
	}
	closeErr := target.Close()
	if err != nil {
		return Manifest{}, err
	}
	if closeErr != nil {
		return Manifest{}, closeErr
	}
	if n != manifest.Bytes || hex.EncodeToString(h.Sum(nil)) != manifest.SHA256 {
		return Manifest{}, errors.New("copied snapshot checksum mismatch")
	}
	actual, err := inspect(ctx, abs)
	if err != nil {
		return Manifest{}, err
	}
	if actual.Counts != manifest.Counts {
		return Manifest{}, errors.New("restored record counts differ")
	}
	if err := syncPath(filepath.Dir(abs)); err != nil {
		return Manifest{}, err
	}
	complete = true
	return manifest, nil
}

func syncPath(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
