package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

var (
	ErrUnauthenticated = errors.New("sign in required")
	ErrForbidden       = errors.New("permission required")
	ErrConflict        = errors.New("record changed; reload before saving")
	ErrInvalid         = errors.New("invalid input")
	ErrMFARequired     = errors.New("second factor required")
	ErrReauthRequired  = errors.New("fresh authentication required")
	ErrVerification    = errors.New("verification code did not match")
	ErrThrottled       = errors.New("verification attempts temporarily limited")
	ErrLink            = errors.New("link is invalid or expired")
)

// These public credentials are exclusively for the loopback-only fictional fixture.
const DemoPassword = "Community-preview-2026!"
const SessionIdle = 30 * time.Minute
const SessionAbsolute = 8 * time.Hour

func randomToken() string { return base64.RawURLEncoding.EncodeToString(randomBytes(32)) }
func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
func TokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Fixed, bounded parameters: 64 MiB, three passes, two lanes. Calibrate on production hardware.
func HashPassword(password string) string {
	salt := randomBytes(16)
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return "$argon2id$v=19$m=65536,t=3,p=2$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key)
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=2" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != 32 {
		return false
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(key, want) == 1
}

type Principal struct {
	ID                  string   `json:"id"`
	ResidentID          string   `json:"-"`
	Name                string   `json:"name"`
	Roles               []string `json:"roles"`
	ScopeKey            string   `json:"scope_key"`
	CanReadRegistry     bool     `json:"can_read_registry"`
	CanManageAccounts   bool     `json:"can_manage_accounts"`
	CanManageRegistry   bool     `json:"can_manage_registry"`
	CanManageRecords    bool     `json:"can_manage_records"`
	CanReadAllRecords   bool     `json:"can_read_all_records"`
	CanReadRecords      bool     `json:"can_read_records"`
	CanReviewRequests   bool     `json:"can_review_requests"`
	CanHandleComplaints bool     `json:"can_handle_complaints"`
	CanManageDocuments  bool     `json:"can_manage_documents"`
	CanReadContacts     bool     `json:"can_read_contacts"`
	CanManageContacts   bool     `json:"can_manage_contacts"`
	CSRF                string   `json:"csrf_token"`
	MFARequired         bool     `json:"mfa_required"`
	MFAEnrolled         bool     `json:"mfa_enrolled"`
	MFAPending          bool     `json:"mfa_pending"`
	IsDemo              bool     `json:"is_demo"`
	Fresh               bool     `json:"fresh_authentication"`
	passwordAt          int64
	factorAt            int64
}

type identityReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func principal(ctx context.Context, q identityReader, hash string, now time.Time) (Principal, error) {
	var p Principal
	err := q.QueryRowContext(ctx, `SELECT u.id, COALESCE(u.resident_id, ''), u.display_name, s.csrf_token,
        u.is_demo, EXISTS(SELECT 1 FROM mfa_factors WHERE user_id = u.id),
        COALESCE(s.reauthenticated_at,0), COALESCE(s.mfa_verified_at,0)
        FROM sessions s JOIN users u ON u.id = s.user_id
        WHERE s.token_hash = ? AND u.status = 'ACTIVE' AND u.suspended_at IS NULL AND s.auth_version = u.auth_version
        AND u.verified_at IS NOT NULL AND s.expires_at > ? AND s.last_seen_at > ?`, hash, now.Unix(), now.Add(-SessionIdle).Unix()).Scan(&p.ID, &p.ResidentID, &p.Name, &p.CSRF, &p.IsDemo, &p.MFAEnrolled, &p.passwordAt, &p.factorAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrUnauthenticated
	}
	if err != nil {
		return p, err
	}
	p.Roles = []string{}
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT role FROM role_grants WHERE user_id = ?
        AND valid_from <= ? AND valid_until > ? AND revoked_at IS NULL ORDER BY role`, p.ID, now.Unix(), now.Unix())
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return p, err
		}
		p.Roles = append(p.Roles, role)
		if role == "TREASURER" {
			p.CanManageRecords = true
		}
		if role == "TREASURER" || role == "COMMITTEE" || role == "AUDITOR" {
			p.CanReadAllRecords = true
		}
		if role == "ADMINISTRATOR" {
			p.CanManageRegistry = true
			p.CanManageAccounts = true
		}
		if role == "ADMINISTRATOR" || role == "COMMITTEE" {
			p.CanReviewRequests = true
			p.CanHandleComplaints = true
			p.CanManageDocuments = true
		}
		if role == "ADMINISTRATOR" || role == "TREASURER" || role == "COMMITTEE" || role == "AUDITOR" {
			p.MFARequired = true
		}
		if role == "ADMINISTRATOR" || role == "COMMITTEE" || role == "TREASURER" {
			p.CanReadRegistry = true
		}
	}
	if err := rows.Err(); err != nil {
		return p, err
	}
	rows.Close()
	date := now.In(societyZone).Format("2006-01-02")
	p.CanReadRecords = p.CanReadAllRecords
	if !p.CanReadRecords && p.ResidentID != "" {
		err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flat_memberships WHERE resident_id = ? AND can_view_finances = 1 AND start_date <= ? AND (end_date IS NULL OR end_date > ?))`, p.ResidentID, date, date).Scan(&p.CanReadRecords)
		if err != nil {
			return p, err
		}
	}
	p.MFAPending = (p.MFARequired || p.MFAEnrolled || p.factorAt > 0) && (p.factorAt == 0 || !p.MFAEnrolled)
	p.Fresh = !p.MFAPending && p.passwordAt > now.Add(-5*time.Minute).Unix() && (!(p.MFARequired || p.MFAEnrolled) || p.factorAt > now.Add(-5*time.Minute).Unix())
	if p.MFAPending {
		p.CanReadRegistry = false
		p.CanManageRegistry = false
		p.CanManageAccounts = false
		p.CanManageRecords = false
		p.CanReadAllRecords = false
		p.CanReadRecords = false
		p.CanReviewRequests = false
		p.CanHandleComplaints = false
		p.CanManageDocuments = false
	}
	p.CanReadContacts = !p.MFAPending && (p.CanReviewRequests || p.ResidentID != "")
	p.CanManageContacts = !p.MFAPending && p.CanReviewRequests
	p.ScopeKey, err = principalScope(ctx, q, p, date)
	return p, err
}

// An opaque cache boundary, not an authorisation credential. Include the current
// membership set even when another home or appointment keeps a permission true.
// Callers use the same transaction as the principal's current authority checks.
func principalScope(ctx context.Context, q identityReader, p Principal, date string) (string, error) {
	type membership struct {
		ID, Home, Relationship string
		Finance                bool
	}
	members := []membership{}
	if p.ResidentID != "" {
		rows, err := q.QueryContext(ctx, `SELECT id,flat_id,relationship,can_view_finances FROM flat_memberships
            WHERE resident_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?) ORDER BY id`, p.ResidentID, date, date)
		if err != nil {
			return "", err
		}
		defer rows.Close()
		for rows.Next() {
			var member membership
			if err := rows.Scan(&member.ID, &member.Home, &member.Relationship, &member.Finance); err != nil {
				return "", err
			}
			members = append(members, member)
		}
		if err := rows.Err(); err != nil {
			return "", err
		}
	}
	body, err := json.Marshal(struct {
		UserID  string
		Roles   []string
		Members []membership
	}{p.ID, p.Roles, members})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func fullPrincipal(ctx context.Context, q identityReader, hash string, now time.Time) (Principal, error) {
	p, err := principal(ctx, q, hash, now)
	if err == nil && p.MFAPending {
		err = ErrMFARequired
	}
	return p, err
}

func (s *Store) Authenticate(ctx context.Context, token string) (Principal, error) {
	return s.authenticate(ctx, token, true)
}

func (s *Store) CheckSession(ctx context.Context, token string) (Principal, error) {
	return s.authenticate(ctx, token, false)
}

func (s *Store) authenticate(ctx context.Context, token string, touch bool) (Principal, error) {
	if len(token) != 43 {
		return Principal{}, ErrUnauthenticated
	}
	now := time.Now()
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Principal{}, err
	}
	defer tx.Rollback()
	p, err := principal(ctx, tx, TokenHash(token), now)
	if err != nil {
		return p, err
	}
	if err := tx.Commit(); err != nil {
		return p, err
	}
	// Write at most once per minute, with an independent absolute session ceiling.
	if touch {
		if _, err := s.DB.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE token_hash = ? AND last_seen_at < ? AND last_seen_at > ? AND expires_at > ?`, now.Unix(), TokenHash(token), now.Add(-time.Minute).Unix(), now.Add(-SessionIdle).Unix(), now.Unix()); err != nil {
			return p, err
		}
	}
	return p, nil
}

func (s *Store) Login(ctx context.Context, login, password, dummyHash string) (string, Principal, error) {
	var id, encoded string
	var version int
	err := s.DB.QueryRowContext(ctx, `SELECT id, password_hash, auth_version FROM users WHERE login = ? AND status = 'ACTIVE' AND suspended_at IS NULL AND verified_at IS NOT NULL`, strings.ToLower(strings.TrimSpace(login))).Scan(&id, &encoded, &version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", Principal{}, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		encoded = dummyHash
	}
	valid := verifyPassword(password, encoded)
	if !valid || id == "" {
		return "", Principal{}, ErrUnauthenticated
	}
	now, token := time.Now(), randomToken()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", Principal{}, err
	}
	defer tx.Rollback()
	// Recheck the password/version/account in the insertion, after expensive password work.
	result, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,auth_version,csrf_token,created_at,last_seen_at,expires_at,reauthenticated_at)
        SELECT ?, id, auth_version, ?, ?, ?, ?, ? FROM users
        WHERE id = ? AND password_hash = ? AND auth_version = ? AND status = 'ACTIVE' AND suspended_at IS NULL AND verified_at IS NOT NULL`,
		TokenHash(token), randomToken(), now.Unix(), now.Unix(), now.Add(SessionAbsolute).Unix(), now.Unix(), id, encoded, version)
	if err != nil {
		return "", Principal{}, err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return "", Principal{}, ErrUnauthenticated
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ? OR last_seen_at <= ?", now.Unix(), now.Add(-SessionIdle).Unix()); err != nil {
		return "", Principal{}, err
	}
	// Bound session accumulation per identity while keeping recent devices usable.
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash NOT IN
        (SELECT token_hash FROM sessions WHERE user_id = ? ORDER BY created_at DESC, rowid DESC LIMIT 5)`, id, id); err != nil {
		return "", Principal{}, err
	}
	p, err := principal(ctx, tx, TokenHash(token), now)
	if err != nil {
		return "", p, err
	}
	return token, p, tx.Commit()
}

func (s *Store) Logout(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", TokenHash(token))
	return err
}

func (s *Store) SeedDemoAccounts(ctx context.Context) error {
	if err := s.RequireDemo(ctx); err != nil {
		return err
	}
	var marker string
	err := s.DB.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = 'demo_accounts'").Scan(&marker)
	if err == nil {
		if marker != "v1" {
			return errors.New("unknown demo identity fixture")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	hash := HashPassword(DemoPassword)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	accounts := []struct{ login, name, resident, role string }{
		{"admin@demo.society", "Demo Registry Officer", "", "ADMINISTRATOR"},
		{"committee@demo.society", "Demo Committee Member", "", "COMMITTEE"},
		{"owner@demo.society", "Demo Owner A-101", "demo-owner-A-101", ""},
		{"tenant@demo.society", "Demo Tenant A-103", "demo-tenant-A-103", ""},
	}
	now := time.Now().Unix()
	for _, a := range accounts {
		id := "demo-user-" + strings.Split(a.login, "@")[0]
		var resident any
		if a.resident != "" {
			resident = a.resident
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO users(id, resident_id, login, display_name, password_hash, created_at,verified_at,password_changed_at,is_demo) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)`, id, resident, a.login, a.name, hash, now, now, now); err != nil {
			return err
		}
		if a.role != "" {
			if _, err := tx.ExecContext(ctx, "INSERT INTO role_grants(id,user_id,role,valid_from,valid_until,revoked_at,granted_by) VALUES (?, ?, ?, ?, ?, NULL, ?)", "demo-grant-"+id, id, a.role, now, now+int64((365*24*time.Hour)/time.Second), "demo-user-admin"); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events(actor_user_id, action, occurred_at, reason, before_json, after_json)
        VALUES ('demo-user-admin', 'DEMO_IDENTITY_BOOTSTRAP', ?, 'Fictional local accounts', '{}', '{"accounts":4}')`, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO app_metadata VALUES ('demo_accounts', 'v1')"); err != nil {
		return err
	}
	return tx.Commit()
}

func invalid(message string) error { return fmt.Errorf("%w: %s", ErrInvalid, message) }
