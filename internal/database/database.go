package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"society.local/portal/internal/security"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

const SchemaVersion = 6

type Store struct {
	DB   *sql.DB
	Path string
	MFA  *security.Box
}

type Engine struct {
	Version     string `json:"version"`
	JournalMode string `json:"journal_mode"`
	Synchronous int    `json:"synchronous"`
	ForeignKeys int    `json:"foreign_keys"`
	BusyTimeout int    `json:"busy_timeout_ms"`
}

func Open(ctx context.Context, path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return nil, err
	}
	// Create the file with private permissions before SQLite opens it. Preserve
	// existing data; refuse symlinks and files with wider permissions.
	file, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err == nil {
		err = file.Close()
	} else if errors.Is(err, os.ErrExist) {
		info, statErr := os.Lstat(abs)
		if statErr != nil {
			return nil, statErr
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return nil, errors.New("database must be a regular private file (mode 0600)")
		}
		err = nil
	}
	if err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	params := url.Values{"mode": {"rwc"}, "_pragma": {
		"journal_mode(WAL)", "synchronous(FULL)", "foreign_keys(ON)", "busy_timeout(5000)",
	}}
	uri.RawQuery = params.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	store := &Store{DB: db, Path: abs}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	engine, err := store.Engine(ctx)
	if err == nil {
		err = validateEngine(engine)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.DB.Close() }

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func inspectEngine(ctx context.Context, q queryer) (Engine, error) {
	var engine Engine
	checks := []struct {
		query string
		value any
	}{
		{"SELECT sqlite_version()", &engine.Version},
		{"PRAGMA journal_mode", &engine.JournalMode},
		{"PRAGMA synchronous", &engine.Synchronous},
		{"PRAGMA foreign_keys", &engine.ForeignKeys},
		{"PRAGMA busy_timeout", &engine.BusyTimeout},
	}
	for _, check := range checks {
		if err := q.QueryRowContext(ctx, check.query).Scan(check.value); err != nil {
			return Engine{}, err
		}
	}
	return engine, nil
}

func (s *Store) Engine(ctx context.Context) (Engine, error) {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return Engine{}, err
	}
	defer conn.Close()
	return inspectEngine(ctx, conn)
}

func fixedSQLite(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	nums := make([]int, 3)
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return false
		}
		nums[i] = n
	}
	return nums[0] == 3 && (nums[1] > 51 || nums[1] == 51 && nums[2] >= 3 ||
		nums[1] == 50 && nums[2] >= 7 || nums[1] == 44 && nums[2] >= 6)
}

func validateEngine(engine Engine) error {
	if !fixedSQLite(engine.Version) {
		return fmt.Errorf("SQLite %s does not meet the documented WAL-reset fix requirement", engine.Version)
	}
	if engine.JournalMode != "wal" || engine.Synchronous != 2 || engine.ForeignKeys != 1 || engine.BusyTimeout != 5000 {
		return errors.New("SQLite connection settings do not match WAL/FULL/foreign-keys/busy-timeout requirements")
	}
	return nil
}

func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations
		(version INTEGER PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL) STRICT`); err != nil {
		return err
	}
	var newest int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&newest); err != nil {
		return err
	}
	if newest > SchemaVersion {
		return errors.New("database schema is newer than this application")
	}
	for version := 1; version <= SchemaVersion; version++ {
		body, err := migrationBody(version)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(body)
		checksum := hex.EncodeToString(hash[:])
		var recorded string
		err = tx.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = ?", version).Scan(&recorded)
		if err == nil {
			if recorded != checksum {
				return fmt.Errorf("migration %d checksum mismatch", version)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations VALUES (?, ?, ?)", version, checksum, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func migrationBody(version int) ([]byte, error) {
	paths, err := fs.Glob(migrations, fmt.Sprintf("migrations/%03d_*.sql", version))
	if err != nil {
		return nil, err
	}
	if len(paths) != 1 {
		return nil, fmt.Errorf("expected one migration for version %d", version)
	}
	return migrations.ReadFile(paths[0])
}

// VerifySchema checks migration provenance without mutating a restored database.
func (s *Store) VerifySchema(ctx context.Context) error {
	var count, latest int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&count, &latest); err != nil {
		return err
	}
	if latest != SchemaVersion || count != SchemaVersion {
		return errors.New("snapshot migration history is not supported")
	}
	for version := 1; version <= SchemaVersion; version++ {
		body, err := migrationBody(version)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(body)
		var recorded string
		if err := s.DB.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = ?", version).Scan(&recorded); err != nil {
			return err
		}
		if recorded != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("migration %d checksum mismatch", version)
		}
	}
	return nil
}

func (s *Store) Ready(ctx context.Context) error {
	var version int
	if err := s.DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		return err
	}
	if version != SchemaVersion {
		return errors.New("schema is not current")
	}
	return nil
}

func (s *Store) RequireDemo(ctx context.Context) error {
	var kind string
	if err := s.DB.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = 'data_kind'").Scan(&kind); err != nil {
		return errors.New("explicitly seeded synthetic database required")
	}
	if kind != "synthetic" {
		return errors.New("this foundation server accepts synthetic data only")
	}
	return nil
}
