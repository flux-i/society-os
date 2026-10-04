package database

import (
	"context"
	"database/sql"
	"encoding/base32"
	"errors"
	"net/url"
	"strings"
	"time"

	"society.local/portal/internal/security"
)

func (s *Store) VerifyMFAKey(ctx context.Context) error {
	if s.MFA == nil {
		return errors.New("MFA encryption key is not configured")
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT user_id,secret_ciphertext FROM mfa_factors UNION ALL SELECT user_id,secret_ciphertext FROM mfa_pending")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var user string
		var encrypted []byte
		if err := rows.Scan(&user, &encrypted); err != nil {
			return err
		}
		secret, err := s.MFA.Open(user, encrypted)
		if err != nil {
			return err
		}
		if len(secret) != 20 {
			return errors.New("stored MFA secret has invalid size")
		}
	}
	return rows.Err()
}

func (s *Store) beginIdentityWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, Principal{}, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at = last_seen_at WHERE token_hash = ?", TokenHash(token)); err != nil {
		tx.Rollback()
		return nil, Principal{}, err
	}
	p, err := principal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func requireFresh(p Principal) error {
	if p.MFAPending {
		return ErrMFARequired
	}
	if !p.Fresh {
		return ErrReauthRequired
	}
	return nil
}
func accountAudit(ctx context.Context, tx *sql.Tx, actor, target, action, reason string, after any) error {
	return appendAudit(ctx, tx, actor, "", action, reason, map[string]any{"user_id": target}, after)
}

type MFASetup struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

func (s *Store) SetupMFA(ctx context.Context, token string) (MFASetup, error) {
	if s.MFA == nil {
		return MFASetup{}, errors.New("MFA encryption unavailable")
	}
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return MFASetup{}, err
	}
	defer tx.Rollback()
	if p.passwordAt <= time.Now().Add(-5*time.Minute).Unix() {
		return MFASetup{}, ErrReauthRequired
	}
	if p.MFAEnrolled {
		return MFASetup{}, invalid("Two-step verification is already enabled.")
	}
	now := time.Now()
	var secret []byte
	var encrypted []byte
	err = tx.QueryRowContext(ctx, "SELECT secret_ciphertext FROM mfa_pending WHERE session_hash = ? AND expires_at > ?", TokenHash(token), now.Unix()).Scan(&encrypted)
	if err == nil {
		secret, err = s.MFA.Open(p.ID, encrypted)
		if err != nil {
			return MFASetup{}, err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		secret = security.NewSecret()
		if _, err := tx.ExecContext(ctx, `INSERT INTO mfa_pending VALUES (?, ?, ?, ?) ON CONFLICT(session_hash)
            DO UPDATE SET secret_ciphertext = excluded.secret_ciphertext, expires_at = excluded.expires_at`, TokenHash(token), p.ID, s.MFA.Seal(p.ID, secret), now.Add(10*time.Minute).Unix()); err != nil {
			return MFASetup{}, err
		}
	} else {
		return MFASetup{}, err
	}
	var login string
	if err := tx.QueryRowContext(ctx, "SELECT login FROM users WHERE id = ?", p.ID).Scan(&login); err != nil {
		return MFASetup{}, err
	}
	text := security.SecretText(secret)
	uri := url.URL{Scheme: "otpauth", Host: "totp", Path: "/Society OS:" + login, RawQuery: url.Values{"secret": {text}, "issuer": {"Society OS"}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}.Encode()}
	return MFASetup{Secret: text, URI: uri.String()}, tx.Commit()
}

func normalizeRecovery(code string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "", "\n", "", "\r", "").Replace(strings.TrimSpace(code)))
}
func recoveryCode() string {
	value := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes(20))
	return value[:8] + "-" + value[8:16] + "-" + value[16:24] + "-" + value[24:]
}
func issueRecovery(ctx context.Context, tx *sql.Tx, user string) ([]string, error) {
	if _, err := tx.ExecContext(ctx, "DELETE FROM mfa_recovery_codes WHERE user_id = ?", user); err != nil {
		return nil, err
	}
	codes := make([]string, 10)
	for i := range codes {
		codes[i] = recoveryCode()
		if _, err := tx.ExecContext(ctx, "INSERT INTO mfa_recovery_codes(id,user_id,code_hash) VALUES (?, ?, ?)", randomToken(), user, TokenHash(normalizeRecovery(codes[i]))); err != nil {
			return nil, err
		}
	}
	return codes, nil
}

// Persistent per-user bounds prevent resetting attempts by signing in again.
func mfaBudget(ctx context.Context, tx *sql.Tx, user string, now time.Time) error {
	var attempts int
	var start int64
	if err := tx.QueryRowContext(ctx, "SELECT mfa_attempts,mfa_window_start FROM users WHERE id = ?", user).Scan(&attempts, &start); err != nil {
		return err
	}
	if start <= now.Add(-15*time.Minute).Unix() {
		if _, err := tx.ExecContext(ctx, "UPDATE users SET mfa_attempts=0,mfa_window_start=? WHERE id=?", now.Unix(), user); err != nil {
			return err
		}
		return nil
	}
	if attempts >= 5 {
		return ErrThrottled
	}
	return nil
}
func mfaFailure(ctx context.Context, tx *sql.Tx, user string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE users SET mfa_attempts=mfa_attempts+1 WHERE id=?", user); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return ErrVerification
}
func consumeFactor(ctx context.Context, tx *sql.Tx, box *security.Box, p Principal, code string, recovery bool, token string, now time.Time) error {
	if err := mfaBudget(ctx, tx, p.ID, now); err != nil {
		return err
	}
	if recovery {
		result, err := tx.ExecContext(ctx, `UPDATE mfa_recovery_codes SET consumed_at=? WHERE user_id=? AND code_hash=?
            AND consumed_at IS NULL AND (expires_at IS NULL OR expires_at>?) AND (session_hash IS NULL OR session_hash=?)`, now.Unix(), p.ID, TokenHash(normalizeRecovery(code)), now.Unix(), TokenHash(token))
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return mfaFailure(ctx, tx, p.ID)
		}
	} else {
		var encrypted []byte
		var last int64
		if err := tx.QueryRowContext(ctx, "SELECT secret_ciphertext,last_step FROM mfa_factors WHERE user_id=?", p.ID).Scan(&encrypted, &last); err != nil {
			return err
		}
		secret, err := box.Open(p.ID, encrypted)
		if err != nil {
			return err
		}
		step, valid := security.Match(secret, strings.TrimSpace(code), now, last)
		if !valid {
			return mfaFailure(ctx, tx, p.ID)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE mfa_factors SET last_step=? WHERE user_id=?", step, p.ID); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, "UPDATE users SET mfa_attempts=0,mfa_window_start=0 WHERE id=?", p.ID)
	return err
}

type MFAVerified struct {
	User          Principal `json:"user"`
	RecoveryCodes []string  `json:"recovery_codes,omitempty"`
}

func (s *Store) ConfirmMFA(ctx context.Context, token, code string) (MFAVerified, error) {
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return MFAVerified{}, err
	}
	defer tx.Rollback()
	now := time.Now()
	if p.passwordAt <= now.Add(-5*time.Minute).Unix() {
		return MFAVerified{}, ErrReauthRequired
	}
	if p.MFAEnrolled {
		return MFAVerified{}, invalid("Two-step verification is already enabled.")
	}
	if err := mfaBudget(ctx, tx, p.ID, now); err != nil {
		return MFAVerified{}, err
	}
	var encrypted []byte
	err = tx.QueryRowContext(ctx, "SELECT secret_ciphertext FROM mfa_pending WHERE session_hash=? AND user_id=? AND expires_at>?", TokenHash(token), p.ID, now.Unix()).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return MFAVerified{}, invalid("Start authenticator setup again.")
	}
	if err != nil {
		return MFAVerified{}, err
	}
	secret, err := s.MFA.Open(p.ID, encrypted)
	if err != nil {
		return MFAVerified{}, err
	}
	step, valid := security.Match(secret, strings.TrimSpace(code), now, -1)
	if !valid {
		return MFAVerified{}, mfaFailure(ctx, tx, p.ID)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO mfa_factors VALUES (?, ?, ?, ?)", p.ID, encrypted, now.Unix(), step); err != nil {
		return MFAVerified{}, err
	}
	codes, err := issueRecovery(ctx, tx, p.ID)
	if err != nil {
		return MFAVerified{}, err
	}
	// Existing devices must prove the newly enabled factor. Keep only this session.
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=? AND token_hash!=?", p.ID, TokenHash(token)); err != nil {
		return MFAVerified{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM mfa_pending WHERE user_id=?", p.ID); err != nil {
		return MFAVerified{}, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sessions SET mfa_verified_at=? WHERE token_hash=?", now.Unix(), TokenHash(token)); err != nil {
		return MFAVerified{}, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET mfa_attempts=0,mfa_window_start=0 WHERE id=?", p.ID); err != nil {
		return MFAVerified{}, err
	}
	if err := accountAudit(ctx, tx, p.ID, p.ID, "MFA_ENABLED", "Authenticator confirmed", map[string]any{"user_id": p.ID, "recovery_code_count": len(codes)}); err != nil {
		return MFAVerified{}, err
	}
	p, err = principal(ctx, tx, TokenHash(token), now)
	if err != nil {
		return MFAVerified{}, err
	}
	return MFAVerified{User: p, RecoveryCodes: codes}, tx.Commit()
}

func (s *Store) VerifyMFA(ctx context.Context, token, code string, recovery bool) (Principal, error) {
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	now := time.Now()
	if p.passwordAt <= now.Add(-5*time.Minute).Unix() {
		return p, ErrReauthRequired
	}
	if !p.MFAEnrolled {
		return p, ErrMFARequired
	}
	if err := consumeFactor(ctx, tx, s.MFA, p, code, recovery, token, now); err != nil {
		return p, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sessions SET mfa_verified_at=? WHERE token_hash=?", now.Unix(), TokenHash(token)); err != nil {
		return p, err
	}
	if recovery {
		if err := accountAudit(ctx, tx, p.ID, p.ID, "MFA_RECOVERY_CODE_USED", "Single-use recovery verification", map[string]any{"user_id": p.ID, "preview": p.IsDemo}); err != nil {
			return p, err
		}
	}
	p, err = principal(ctx, tx, TokenHash(token), now)
	if err != nil {
		return p, err
	}
	return p, tx.Commit()
}

// Only publicly fictional identities in the loopback-only synthetic preview qualify.
// Setup uses a real TOTP. Later preview logins get a session-bound, expiring recovery code.
func (s *Store) DemoVerificationCode(ctx context.Context, token string) (string, bool, error) {
	if err := s.RequireDemo(ctx); err != nil {
		return "", false, ErrForbidden
	}
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if !p.IsDemo {
		return "", false, ErrForbidden
	}
	now := time.Now()
	if !p.MFAEnrolled {
		if p.passwordAt <= now.Add(-5*time.Minute).Unix() {
			return "", false, ErrReauthRequired
		}
		var encrypted []byte
		if err := tx.QueryRowContext(ctx, "SELECT secret_ciphertext FROM mfa_pending WHERE session_hash=? AND expires_at>?", TokenHash(token), now.Unix()).Scan(&encrypted); err != nil {
			return "", false, err
		}
		secret, err := s.MFA.Open(p.ID, encrypted)
		if err != nil {
			return "", false, err
		}
		return security.Code(secret, now.Unix()/30, 6), false, tx.Commit()
	}
	code := recoveryCode()
	if _, err := tx.ExecContext(ctx, "DELETE FROM mfa_recovery_codes WHERE session_hash=? OR expires_at<=?", TokenHash(token), now.Unix()); err != nil {
		return "", true, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO mfa_recovery_codes(id,user_id,code_hash,expires_at,session_hash) VALUES (?, ?, ?, ?, ?)", randomToken(), p.ID, TokenHash(normalizeRecovery(code)), now.Add(2*time.Minute).Unix(), TokenHash(token)); err != nil {
		return "", true, err
	}
	return code, true, tx.Commit()
}

func (s *Store) Reauthenticate(ctx context.Context, token, password, code string, recovery bool) (Principal, error) {
	// Bound password work in the HTTP guard; recheck the hash/version inside the transaction.
	p, err := s.CheckSession(ctx, token)
	if err != nil {
		return p, err
	}
	var hash string
	var version int
	if err := s.DB.QueryRowContext(ctx, "SELECT password_hash,auth_version FROM users WHERE id=?", p.ID).Scan(&hash, &version); err != nil {
		return p, err
	}
	if !verifyPassword(password, hash) {
		return p, invalid("Your current password didn’t match. Please try again.")
	}
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	var currentHash string
	var currentVersion int
	if err := tx.QueryRowContext(ctx, "SELECT password_hash,auth_version FROM users WHERE id=?", p.ID).Scan(&currentHash, &currentVersion); err != nil {
		return p, err
	}
	if currentHash != hash || currentVersion != version {
		return p, ErrUnauthenticated
	}
	now := time.Now()
	if p.MFAEnrolled {
		if err := consumeFactor(ctx, tx, s.MFA, p, code, recovery, token, now); err != nil {
			return p, err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sessions SET reauthenticated_at=?,mfa_verified_at=CASE WHEN ? THEN ? ELSE mfa_verified_at END WHERE token_hash=?", now.Unix(), p.MFAEnrolled, now.Unix(), TokenHash(token)); err != nil {
		return p, err
	}
	p, err = principal(ctx, tx, TokenHash(token), now)
	if err != nil {
		return p, err
	}
	return p, tx.Commit()
}

func (s *Store) RegenerateRecovery(ctx context.Context, token string) ([]string, error) {
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := requireFresh(p); err != nil {
		return nil, err
	}
	if !p.MFAEnrolled {
		return nil, ErrMFARequired
	}
	codes, err := issueRecovery(ctx, tx, p.ID)
	if err != nil {
		return nil, err
	}
	if err := accountAudit(ctx, tx, p.ID, p.ID, "MFA_RECOVERY_CODES_REPLACED", "Reauthenticated code replacement", map[string]any{"user_id": p.ID, "code_count": len(codes)}); err != nil {
		return nil, err
	}
	return codes, tx.Commit()
}
