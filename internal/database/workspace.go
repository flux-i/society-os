package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var sourceIdentity = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$`)

type WorkspaceSetup struct {
	FormatVersion      int    `json:"format_version"`
	SocietyKey         string `json:"society_key"`
	SocietyName        string `json:"society_name"`
	Mode               string `json:"mode"`
	BootstrapID        string `json:"bootstrap_id"`
	AdministratorName  string `json:"administrator_name"`
	AdministratorEmail string `json:"administrator_email"`
	IdentityVerified   bool   `json:"identity_verified"`
	VerificationNote   string `json:"verification_note"`
	TermDays           int    `json:"term_days"`
}

// Public sign-in configuration contains no account list, credentials or registry.
type WorkspaceInfo struct {
	Mode string `json:"mode"`
	Name string `json:"name"`
}

func strictJSON(body []byte, value any) error {
	if !utf8.Valid(body) {
		return ErrInvalid
	}
	// encoding/json otherwise accepts duplicate object keys and null scalar
	// fields. Reject ambiguous source documents before mapping their values.
	scan := json.NewDecoder(bytes.NewReader(body))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return ErrInvalid
		}
		token, err := scan.Token()
		if err != nil {
			return ErrInvalid
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{':
				seen := map[string]bool{}
				for scan.More() {
					key, err := scan.Token()
					name, ok := key.(string)
					if err != nil || !ok || seen[name] {
						return ErrInvalid
					}
					seen[name] = true
					if err = walk(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for scan.More() {
					if err = walk(depth + 1); err != nil {
						return err
					}
				}
			default:
				return ErrInvalid
			}
			if _, err = scan.Token(); err != nil {
				return ErrInvalid
			}
		}
		return nil
	}
	if err := walk(0); err != nil {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return ErrInvalid
	}
	return nil
}

func InputDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func ParseWorkspaceSetup(body []byte) (WorkspaceSetup, error) {
	var input WorkspaceSetup
	if len(body) > 16384 || strictJSON(body, &input) != nil {
		return input, invalid("Use a valid version-1 setup JSON file, at most 16 KiB.")
	}
	if err := input.validate(); err != nil {
		return input, err
	}
	return input, nil
}

func (input WorkspaceSetup) validate() error {
	if input.FormatVersion != 1 || !sourceIdentity.MatchString(input.SocietyKey) || !sourceIdentity.MatchString(input.BootstrapID) || !validText(input.SocietyName, 2, 120) || !validText(input.AdministratorName, 2, 120) || (input.Mode != "FICTIONAL_REHEARSAL" && input.Mode != "LOCAL_WORKSPACE") || input.TermDays < 1 || input.TermDays > 365 {
		return invalid("Check setup version, society/operation keys, names, mode and the 1–365 day administrator term.")
	}
	email, err := normalizeEmail(input.AdministratorEmail)
	if err != nil || email != input.AdministratorEmail {
		return invalid("Supply a normalized administrator email address.")
	}
	return verifiedNote(input.IdentityVerified, input.VerificationNote)
}

func workspaceInfo(ctx context.Context, q identityReader) (WorkspaceInfo, error) {
	var result WorkspaceInfo
	var exists bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='workspace_setup')").Scan(&exists)
	if err != nil {
		return result, err
	}
	err = sql.ErrNoRows
	if exists {
		err = q.QueryRowContext(ctx, "SELECT mode,society_name FROM workspace_setup WHERE id=1").Scan(&result.Mode, &result.Name)
	}
	if errors.Is(err, sql.ErrNoRows) {
		var kind, fixture string
		err = q.QueryRowContext(ctx, "SELECT (SELECT value FROM app_metadata WHERE key='data_kind'),(SELECT value FROM app_metadata WHERE key='fixture_version')").Scan(&kind, &fixture)
		if err == nil && kind == "synthetic" && fixture == FixtureVersion {
			return WorkspaceInfo{Mode: "DEMO", Name: "Society OS preview"}, nil
		}
		return WorkspaceInfo{}, errors.New("workspace has not been configured")
	}
	return result, err
}

func (s *Store) WorkspaceInfo(ctx context.Context) (WorkspaceInfo, error) {
	return workspaceInfo(ctx, s.DB)
}

func (s *Store) RequireWorkspace(ctx context.Context) error {
	var mode, kind string
	if err := s.DB.QueryRowContext(ctx, "SELECT mode,(SELECT value FROM app_metadata WHERE key='data_kind') FROM workspace_setup WHERE id=1").Scan(&mode, &kind); err != nil {
		return errors.New("an explicitly configured workspace is required")
	}
	if (mode == "FICTIONAL_REHEARSAL" && kind == "synthetic") || (mode == "LOCAL_WORKSPACE" && kind == "local-workspace") {
		return nil
	}
	return errors.New("workspace mode and data provenance do not match")
}

func (s *Store) RequireRecoverableWorkspace(ctx context.Context) error {
	if err := s.RequireDemo(ctx); err == nil {
		return nil
	}
	return s.RequireWorkspace(ctx)
}

func (s *Store) VerifyWorkspaceKeys(ctx context.Context, mfa, message string) error {
	var expectedMFA, expectedMessage string
	err := s.DB.QueryRowContext(ctx, "SELECT mfa_key_fingerprint,message_key_fingerprint FROM workspace_setup WHERE id=1").Scan(&expectedMFA, &expectedMessage)
	if err != nil {
		return errors.New("workspace setup is unavailable")
	}
	if mfa != expectedMFA || message != expectedMessage {
		return errors.New("restore the separately held workspace keys; replacement is refused")
	}
	return nil
}

// Called before migrations or key creation by the local bootstrap command.
// An accidental occupied destination must keep its original schema as well as
// its business rows. BootstrapWorkspace repeats the check under its write lock.
func (s *Store) CheckBootstrapTarget(ctx context.Context) error {
	var exists, configured bool
	if err := s.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='workspace_setup')").Scan(&exists); err != nil {
		return err
	}
	if exists {
		if err := s.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM workspace_setup)").Scan(&configured); err != nil {
			return err
		}
		if configured {
			return nil
		}
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name!='schema_migrations'")
	if err != nil {
		return err
	}
	names := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, name := range names {
		var count int
		if err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+strings.ReplaceAll(name, `"`, `""`)+`"`).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return errors.New("bootstrap requires an empty database; existing data and schema are preserved")
		}
	}
	return nil
}

// Bootstrap is local-custodian-only, atomic, and cannot modify an existing setup.
func (s *Store) BootstrapWorkspace(ctx context.Context, input WorkspaceSetup, password, mfa, message string) (WorkspaceInfo, error) {
	if err := input.validate(); err != nil {
		return WorkspaceInfo{}, err
	}
	if !validPassword(password) {
		return WorkspaceInfo{}, invalid("Use a password of at least 12 characters (maximum 256 bytes).")
	}
	if len(mfa) != 64 || len(message) != 64 || mfa == message {
		return WorkspaceInfo{}, invalid("Use two separate valid private keys.")
	}
	body, _ := json.Marshal(input)
	digest := InputDigest(body)
	// Hash before taking SQLite's writer reservation; retain the original hash for
	// exact setup retry even after the account later changes its own password.
	hash := HashPassword(password)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return WorkspaceInfo{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE app_metadata SET value=value WHERE key='data_kind'"); err != nil {
		return WorkspaceInfo{}, err
	}
	var oldDigest, oldPassword, oldMFA, oldMessage string
	err = tx.QueryRowContext(ctx, "SELECT input_digest,bootstrap_password_hash,mfa_key_fingerprint,message_key_fingerprint FROM workspace_setup WHERE id=1").Scan(&oldDigest, &oldPassword, &oldMFA, &oldMessage)
	if err == nil {
		if digest != oldDigest || mfa != oldMFA || message != oldMessage || !verifyPassword(password, oldPassword) {
			return WorkspaceInfo{}, ErrConflict
		}
		return WorkspaceInfo{Mode: input.Mode, Name: input.SocietyName}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return WorkspaceInfo{}, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name!='schema_migrations'")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	var tables []string
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			rows.Close()
			return WorkspaceInfo{}, err
		}
		tables = append(tables, table)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return WorkspaceInfo{}, err
	}
	for _, table := range tables {
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+strings.ReplaceAll(table, `"`, `""`)+`"`).Scan(&count); err != nil {
			return WorkspaceInfo{}, err
		}
		if count != 0 {
			return WorkspaceInfo{}, errors.New("bootstrap requires an empty database; existing business data is preserved")
		}
	}
	now := time.Now().Unix()
	id := "workspace-admin-" + randomToken()
	if _, err = tx.ExecContext(ctx, `INSERT INTO users(id,login,display_name,password_hash,status,created_at,verified_at,password_changed_at,is_demo) VALUES (?,?,?,?,'ACTIVE',?,?,?,0)`, id, input.AdministratorEmail, input.AdministratorName, hash, now, now, now); err != nil {
		return WorkspaceInfo{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO role_grants(id,user_id,role,valid_from,valid_until,granted_by) VALUES (?,?,'ADMINISTRATOR',?,?,?)`, randomToken(), id, now, now+int64(input.TermDays)*86400, id); err != nil {
		return WorkspaceInfo{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_setup VALUES (1,?,?,?,?,?,?,?,?,?,?)`, input.SocietyKey, input.SocietyName, input.Mode, input.BootstrapID, digest, id, hash, mfa, message, now); err != nil {
		return WorkspaceInfo{}, err
	}
	kind := "local-workspace"
	if input.Mode == "FICTIONAL_REHEARSAL" {
		kind = "synthetic"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO app_metadata(key,value) VALUES ('data_kind',?),('fixture_version','supplied_registry_v1')`, kind); err != nil {
		return WorkspaceInfo{}, err
	}
	if err = accountAudit(ctx, tx, id, id, "WORKSPACE_BOOTSTRAPPED", input.VerificationNote, map[string]any{"society_key": input.SocietyKey, "mode": input.Mode, "bootstrap_id": input.BootstrapID, "input_digest": digest, "administrator_term_days": input.TermDays}); err != nil {
		return WorkspaceInfo{}, err
	}
	return WorkspaceInfo{Mode: input.Mode, Name: input.SocietyName}, tx.Commit()
}
