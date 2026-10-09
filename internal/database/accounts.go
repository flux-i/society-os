package database

import (
	"context"
	"database/sql"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Invitation struct {
	ResidentID       string `json:"resident_id"`
	Email            string `json:"email"`
	Role             string `json:"role"`
	TermDays         int    `json:"term_days"`
	IdentityVerified bool   `json:"identity_verified"`
	Note             string `json:"note"`
}
type IssuedLink struct {
	Token     string `json:"token"`
	Purpose   string `json:"purpose"`
	ExpiresAt int64  `json:"expires_at"`
	Name      string `json:"name"`
}
type LinkDetails struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Purpose   string `json:"purpose"`
	ExpiresAt int64  `json:"expires_at"`
}
type Account struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	ResidentName string   `json:"resident_name"`
	State        string   `json:"state"`
	Roles        []string `json:"roles"`
	MFA          bool     `json:"mfa_enrolled"`
	ActiveHomes  int      `json:"active_homes"`
}
type AccountPage struct {
	Items    []Account `json:"items"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

func normalizeEmail(email string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(email))
	if len(value) > 254 || len(value) < 3 {
		return "", invalid("Enter a valid email address.")
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || strings.ContainsAny(value, "\r\n\t ") {
		return "", invalid("Enter a valid email address.")
	}
	return value, nil
}
func validPassword(password string) bool {
	if !utf8.ValidString(password) || len(password) > 256 || utf8.RuneCountInString(password) < 12 {
		return false
	}
	for _, r := range password {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func verifiedNote(verified bool, note string) error {
	if !verified || !validText(note, 10, 300) {
		return invalid("Confirm identity and relationship, and record a verification note of 10–300 characters.")
	}
	return nil
}
func requireAccountAdmin(p Principal) error {
	if err := requireFresh(p); err != nil {
		return err
	}
	if !p.CanManageAccounts {
		return ErrForbidden
	}
	return nil
}
func newAccountToken(ctx context.Context, tx *sql.Tx, actor, user, purpose, note string) (IssuedLink, error) {
	now := time.Now()
	expires := now.Add(time.Hour)
	if purpose == "PASSWORD_RESET" {
		expires = now.Add(30 * time.Minute)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE account_tokens SET revoked_at=? WHERE user_id=? AND consumed_at IS NULL AND revoked_at IS NULL", now.Unix(), user); err != nil {
		return IssuedLink{}, err
	}
	var version int
	var name string
	if err := tx.QueryRowContext(ctx, "SELECT auth_version,display_name FROM users WHERE id=?", user).Scan(&version, &name); err != nil {
		return IssuedLink{}, err
	}
	token := randomToken()
	if _, err := tx.ExecContext(ctx, "INSERT INTO account_tokens(id,user_id,purpose,token_hash,expires_at,auth_version,created_at,created_by,verification_note) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", randomToken(), user, purpose, TokenHash(token), expires.Unix(), version, now.Unix(), actor, note); err != nil {
		return IssuedLink{}, err
	}
	if err := accountAudit(ctx, tx, actor, user, purpose+"_LINK_ISSUED", note, map[string]any{"user_id": user, "expires_at": expires.Unix()}); err != nil {
		return IssuedLink{}, err
	}
	return IssuedLink{Token: token, Purpose: purpose, ExpiresAt: expires.Unix(), Name: name}, nil
}

func (s *Store) Invite(ctx context.Context, token string, input Invitation) (IssuedLink, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return IssuedLink{}, err
	}
	if err := verifiedNote(input.IdentityVerified, input.Note); err != nil {
		return IssuedLink{}, err
	}
	if input.Role != "RESIDENT" && input.Role != "COMMITTEE" && input.Role != "ADMINISTRATOR" {
		return IssuedLink{}, invalid("Choose resident, committee or administrator access.")
	}
	if input.Role != "RESIDENT" && (input.TermDays < 1 || input.TermDays > 365) {
		return IssuedLink{}, invalid("Choose an access term of 1–365 days.")
	}
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return IssuedLink{}, err
	}
	defer tx.Rollback()
	if err := requireAccountAdmin(p); err != nil {
		return IssuedLink{}, err
	}
	var name string
	err = tx.QueryRowContext(ctx, `SELECT r.full_name FROM residents r WHERE r.id=? AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.resident_id=r.id AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`, input.ResidentID, today(), today()).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return IssuedLink{}, invalid("Choose a person with a current relationship to a home.")
	}
	if err != nil {
		return IssuedLink{}, err
	}
	var duplicates int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE resident_id=? OR login=?", input.ResidentID, email).Scan(&duplicates); err != nil {
		return IssuedLink{}, err
	}
	if duplicates != 0 {
		return IssuedLink{}, invalid("This person or email already has an account. Use its invitation/recovery controls.")
	}
	id := randomToken()
	now := time.Now().Unix()
	// No login until activation. Identifier verification is the documented in-person/channel attestation.
	if _, err := tx.ExecContext(ctx, "INSERT INTO users(id,resident_id,login,display_name,password_hash,status,created_at) VALUES (?, ?, ?, ?, '', 'DISABLED', ?)", id, input.ResidentID, email, name, now); err != nil {
		return IssuedLink{}, err
	}
	if input.Role != "RESIDENT" {
		if _, err := tx.ExecContext(ctx, "INSERT INTO role_grants(id,user_id,role,valid_from,valid_until,revoked_at,granted_by) VALUES (?, ?, ?, ?, ?, NULL, ?)", randomToken(), id, input.Role, now, now+int64(input.TermDays)*86400, p.ID); err != nil {
			return IssuedLink{}, err
		}
	}
	link, err := newAccountToken(ctx, tx, p.ID, id, "INVITE", input.Note)
	if err != nil {
		return link, err
	}
	return link, tx.Commit()
}

func (s *Store) IssueAccountLink(ctx context.Context, token, userID, purpose string, verified bool, note string) (IssuedLink, error) {
	if purpose != "INVITE" && purpose != "PASSWORD_RESET" {
		return IssuedLink{}, ErrInvalid
	}
	if err := verifiedNote(verified, note); err != nil {
		return IssuedLink{}, err
	}
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return IssuedLink{}, err
	}
	defer tx.Rollback()
	if err := requireAccountAdmin(p); err != nil {
		return IssuedLink{}, err
	}
	var status string
	var suspended bool
	var verifiedAt *int64
	if err := tx.QueryRowContext(ctx, "SELECT status,verified_at,suspended_at IS NOT NULL FROM users WHERE id=?", userID).Scan(&status, &verifiedAt, &suspended); err != nil {
		return IssuedLink{}, err
	}
	if suspended {
		return IssuedLink{}, invalid("Resume this account before issuing an invitation or recovery link.")
	}
	if purpose == "INVITE" && verifiedAt != nil {
		return IssuedLink{}, invalid("This account is already activated.")
	}
	if purpose == "PASSWORD_RESET" && (verifiedAt == nil || status != "ACTIVE") {
		return IssuedLink{}, invalid("Password recovery is available only for an active account.")
	}
	if purpose == "INVITE" {
		var eligible int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u JOIN flat_memberships m ON m.resident_id=u.resident_id WHERE u.id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?)`, userID, today(), today()).Scan(&eligible); err != nil {
			return IssuedLink{}, err
		}
		if eligible == 0 {
			return IssuedLink{}, invalid("The person no longer has a current home relationship.")
		}
	}
	link, err := newAccountToken(ctx, tx, p.ID, userID, purpose, note)
	if err != nil {
		return link, err
	}
	return link, tx.Commit()
}

func inspectAccountLink(ctx context.Context, q identityReader, token string) (LinkDetails, string, string, error) {
	if len(token) != 43 {
		return LinkDetails{}, "", "", ErrLink
	}
	var result LinkDetails
	var user, id string
	err := q.QueryRowContext(ctx, `SELECT u.display_name,u.login,t.purpose,t.expires_at,u.id,t.id FROM account_tokens t JOIN users u ON u.id=t.user_id
        WHERE t.token_hash=? AND t.consumed_at IS NULL AND t.revoked_at IS NULL AND t.expires_at>?
        AND u.suspended_at IS NULL AND t.auth_version=u.auth_version AND ((t.purpose='INVITE' AND u.verified_at IS NULL
            AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.resident_id=u.resident_id AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?)))
        OR (t.purpose='PASSWORD_RESET' AND u.verified_at IS NOT NULL AND u.status='ACTIVE'))`, TokenHash(token), time.Now().Unix(), today(), today()).Scan(&result.Name, &result.Email, &result.Purpose, &result.ExpiresAt, &user, &id)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrLink
	}
	return result, user, id, err
}
func (s *Store) InspectAccountLink(ctx context.Context, token string) (LinkDetails, error) {
	result, _, _, err := inspectAccountLink(ctx, s.DB, token)
	return result, err
}

func (s *Store) CompleteAccountLink(ctx context.Context, token, password string) error {
	if !validPassword(password) {
		return invalid("Use a password of at least 12 characters (maximum 256 bytes).")
	}
	if _, _, _, err := inspectAccountLink(ctx, s.DB, token); err != nil {
		return err
	}
	hash := HashPassword(password)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE account_tokens SET expires_at=expires_at WHERE token_hash=?", TokenHash(token)); err != nil {
		return err
	}
	info, user, id, err := inspectAccountLink(ctx, tx, token)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, "UPDATE users SET password_hash=?,password_changed_at=?,auth_version=auth_version+1,status='ACTIVE',verified_at=COALESCE(verified_at,?) WHERE id=?", hash, now, now, user); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE account_tokens SET consumed_at=? WHERE id=?", now, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE account_tokens SET revoked_at=? WHERE user_id=? AND id!=? AND consumed_at IS NULL AND revoked_at IS NULL", now, user, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", user); err != nil {
		return err
	}
	if err := accountAudit(ctx, tx, user, user, info.Purpose+"_COMPLETED", "Single-use verified account link", map[string]any{"user_id": user, "sessions_revoked": true, "mfa_preserved": true}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ChangePassword(ctx context.Context, token, current, newPassword string) error {
	if !validPassword(newPassword) {
		return invalid("Use a password of at least 12 characters (maximum 256 bytes).")
	}
	p, err := s.CheckSession(ctx, token)
	if err != nil {
		return err
	}
	if err := requireFresh(p); err != nil {
		return err
	}
	var oldHash string
	if err := s.DB.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE id=?", p.ID).Scan(&oldHash); err != nil {
		return err
	}
	if !verifyPassword(current, oldHash) {
		return invalid("Your current password didn’t match. Please try again.")
	}
	newHash := HashPassword(newPassword)
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireFresh(p); err != nil {
		return err
	}
	var actual string
	if err := tx.QueryRowContext(ctx, "SELECT password_hash FROM users WHERE id=?", p.ID).Scan(&actual); err != nil {
		return err
	}
	if actual != oldHash {
		return ErrUnauthenticated
	}
	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, "UPDATE users SET password_hash=?,password_changed_at=?,auth_version=auth_version+1 WHERE id=?", newHash, now, p.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", p.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE account_tokens SET revoked_at=? WHERE user_id=? AND consumed_at IS NULL AND revoked_at IS NULL", now, p.ID); err != nil {
		return err
	}
	if err := accountAudit(ctx, tx, p.ID, p.ID, "PASSWORD_CHANGED", "Reauthenticated password change", map[string]any{"user_id": p.ID, "sessions_revoked": true}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AccountsFor(ctx context.Context, token, query string, page, pageSize int) (AccountPage, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return AccountPage{}, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return AccountPage{}, err
	}
	if !p.CanManageAccounts {
		return AccountPage{}, ErrForbidden
	}
	where := ` FROM users u LEFT JOIN residents r ON r.id=u.resident_id WHERE instr(lower(u.display_name || ' ' || u.login),lower(?))>0`
	result := AccountPage{Items: []Account{}, Page: page, PageSize: pageSize}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*)"+where, query).Scan(&result.Total); err != nil {
		return result, err
	}
	now := time.Now().Unix()
	rows, err := tx.QueryContext(ctx, `SELECT u.id,u.display_name,u.login,COALESCE(r.full_name,''),
        CASE WHEN u.suspended_at IS NOT NULL THEN 'SUSPENDED' WHEN u.verified_at IS NULL THEN 'PENDING' ELSE u.status END,
        EXISTS(SELECT 1 FROM mfa_factors mf WHERE mf.user_id=u.id),
        (SELECT COUNT(DISTINCT flat_id) FROM flat_memberships m WHERE m.resident_id=u.resident_id AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?)),
        COALESCE((SELECT group_concat(role,',') FROM (SELECT DISTINCT role FROM role_grants WHERE user_id=u.id AND valid_from<=? AND valid_until>? AND revoked_at IS NULL ORDER BY role)), '')`+where+` ORDER BY u.created_at,u.id LIMIT ? OFFSET ?`, today(), today(), now, now, query, pageSize, (page-1)*pageSize)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var a Account
		var roles string
		if err := rows.Scan(&a.ID, &a.Name, &a.Email, &a.ResidentName, &a.State, &a.MFA, &a.ActiveHomes, &roles); err != nil {
			rows.Close()
			return result, err
		}
		a.Roles = []string{}
		if roles != "" {
			a.Roles = strings.Split(roles, ",")
		}
		result.Items = append(result.Items, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

// The offline, local-only lost-factor path records both named custodians. It never returns a secret.
func (s *Store) OfflineMFARecovery(ctx context.Context, email, one, two, reason string) error {
	if err := s.RequireRecoverableWorkspace(ctx); err != nil {
		return err
	}
	if !validText(one, 2, 100) || !validText(two, 2, 100) || strings.EqualFold(one, two) || !validText(reason, 10, 300) {
		return invalid("Record two distinct custodians and a recovery reason.")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE users SET auth_version=auth_version WHERE login=?", strings.ToLower(strings.TrimSpace(email))); err != nil {
		return err
	}
	var user string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM users WHERE login=? AND status='ACTIVE' AND verified_at IS NOT NULL", strings.ToLower(strings.TrimSpace(email))).Scan(&user); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", user); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM mfa_recovery_codes WHERE user_id=?", user); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM mfa_factors WHERE user_id=?", user); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET auth_version=auth_version+1,mfa_attempts=0,mfa_window_start=0 WHERE id=?", user); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE account_tokens SET revoked_at=? WHERE user_id=? AND consumed_at IS NULL AND revoked_at IS NULL", time.Now().Unix(), user); err != nil {
		return err
	}
	if err := accountAudit(ctx, tx, user, user, "OFFLINE_MFA_RECOVERY", reason, map[string]any{"user_id": user, "custodian_one": one, "custodian_two": two, "operator": "offline maintenance", "sessions_revoked": true}); err != nil {
		return err
	}
	return tx.Commit()
}
