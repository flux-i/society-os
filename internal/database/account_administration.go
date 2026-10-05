package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type RoleAppointment struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	State       string `json:"state"`
	ValidFrom   int64  `json:"valid_from"`
	ValidUntil  int64  `json:"valid_until"`
	GrantedBy   string `json:"granted_by"`
	GrantedName string `json:"granted_name"`
	RevokedAt   int64  `json:"revoked_at"`
	RevokedBy   string `json:"revoked_by"`
	RevokedName string `json:"revoked_name"`
}
type AccessEvent struct {
	ID     int64  `json:"id"`
	Action string `json:"action"`
	Actor  string `json:"actor"`
	At     int64  `json:"at"`
	Reason string `json:"reason"`
}
type AccountDetails struct {
	Account     Account           `json:"account"`
	Version     int               `json:"version"`
	Grants      []RoleAppointment `json:"grants"`
	GrantTotal  int               `json:"grant_total"`
	GrantPage   int               `json:"grant_page"`
	Events      []AccessEvent     `json:"events"`
	EventTotal  int               `json:"event_total"`
	HistoryPage int               `json:"history_page"`
	PageSize    int               `json:"page_size"`
}
type AccessChange struct {
	Version   int    `json:"version"`
	Confirmed bool   `json:"confirmed"`
	Reason    string `json:"reason"`
}
type AppointmentInput struct {
	AccessChange
	Role     string `json:"role"`
	TermDays int    `json:"term_days"`
}
type AccountStatusInput struct {
	AccessChange
	Action string `json:"action"`
}
type AccessResult struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

func validAppointment(role string) bool {
	return role == "ADMINISTRATOR" || role == "COMMITTEE" || role == "TREASURER" || role == "AUDITOR"
}
func validateAccessChange(c AccessChange) error {
	if c.Version < 1 {
		return invalid("Reload this account before making a change.")
	}
	if !c.Confirmed || !validText(c.Reason, 10, 300) {
		return invalid("Confirm the verified authority and give a reason of 10–300 characters.")
	}
	return nil
}

func (s *Store) AccountDetailsFor(ctx context.Context, token, id string, grantPage, historyPage int) (AccountDetails, error) {
	if len(id) == 0 || len(id) > 100 || grantPage < 1 || grantPage > 10000 || historyPage < 1 || historyPage > 10000 {
		return AccountDetails{}, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return AccountDetails{}, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return AccountDetails{}, err
	}
	if !p.CanManageAccounts {
		return AccountDetails{}, ErrForbidden
	}
	result := AccountDetails{Grants: []RoleAppointment{}, Events: []AccessEvent{}, GrantPage: grantPage, HistoryPage: historyPage, PageSize: 20}
	a := &result.Account
	err = tx.QueryRowContext(ctx, `SELECT u.id,u.display_name,u.login,COALESCE(r.full_name,''),
        CASE WHEN u.suspended_at IS NOT NULL THEN 'SUSPENDED' WHEN u.verified_at IS NULL THEN 'PENDING' ELSE u.status END,
        EXISTS(SELECT 1 FROM mfa_factors WHERE user_id=u.id),
        (SELECT COUNT(DISTINCT flat_id) FROM flat_memberships m WHERE m.resident_id=u.resident_id AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?)),u.access_version
        FROM users u LEFT JOIN residents r ON r.id=u.resident_id WHERE u.id=?`, today(), today(), id).Scan(&a.ID, &a.Name, &a.Email, &a.ResidentName, &a.State, &a.MFA, &a.ActiveHomes, &result.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return result, sql.ErrNoRows
	}
	if err != nil {
		return result, err
	}
	a.Roles = []string{}
	now := time.Now().Unix()
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT role FROM role_grants WHERE user_id=? AND valid_from<=? AND valid_until>? AND revoked_at IS NULL ORDER BY role`, id, now, now)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var role string
		if err = rows.Scan(&role); err != nil {
			break
		}
		a.Roles = append(a.Roles, role)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return result, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM role_grants WHERE user_id=?", id).Scan(&result.GrantTotal); err != nil {
		return result, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT g.id,g.role,g.valid_from,g.valid_until,COALESCE(g.granted_by,''),COALESCE(u.display_name,'Bootstrap'),COALESCE(g.revoked_at,0),COALESCE(g.revoked_by,''),COALESCE(r.display_name,'')
        FROM role_grants g LEFT JOIN users u ON u.id=g.granted_by LEFT JOIN users r ON r.id=g.revoked_by WHERE g.user_id=?
        ORDER BY (g.revoked_at IS NULL AND g.valid_until>?) DESC,g.valid_from DESC,g.id DESC LIMIT 20 OFFSET ?`, id, now, (grantPage-1)*20)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var g RoleAppointment
		if err = rows.Scan(&g.ID, &g.Role, &g.ValidFrom, &g.ValidUntil, &g.GrantedBy, &g.GrantedName, &g.RevokedAt, &g.RevokedBy, &g.RevokedName); err != nil {
			break
		}
		g.State = "ACTIVE"
		if g.RevokedAt != 0 {
			g.State = "REVOKED"
		} else if g.ValidUntil <= now {
			g.State = "EXPIRED"
		} else if g.ValidFrom > now {
			g.State = "UPCOMING"
		}
		result.Grants = append(result.Grants, g)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return result, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE json_extract(before_json,'$.user_id')=?", id).Scan(&result.EventTotal); err != nil {
		return result, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT e.id,e.action,u.display_name,e.occurred_at,e.reason FROM audit_events e JOIN users u ON u.id=e.actor_user_id WHERE json_extract(e.before_json,'$.user_id')=? ORDER BY e.id DESC LIMIT 20 OFFSET ?`, id, (historyPage-1)*20)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var e AccessEvent
		if err = rows.Scan(&e.ID, &e.Action, &e.Actor, &e.At, &e.Reason); err != nil {
			break
		}
		result.Events = append(result.Events, e)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func (s *Store) beginAccessChange(ctx context.Context, token, id string, c AccessChange) (*sql.Tx, Principal, error) {
	if len(id) == 0 || len(id) > 100 {
		return nil, Principal{}, ErrInvalid
	}
	if err := validateAccessChange(c); err != nil {
		return nil, Principal{}, err
	}
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return nil, p, err
	}
	fail := func(err error) (*sql.Tx, Principal, error) { tx.Rollback(); return nil, p, err }
	if err = requireAccountAdmin(p); err != nil {
		return fail(err)
	}
	if p.ID == id {
		return fail(invalid("Ask a different administrator to change your own access."))
	}
	var version int
	if err = tx.QueryRowContext(ctx, "SELECT access_version FROM users WHERE id=?", id).Scan(&version); errors.Is(err, sql.ErrNoRows) {
		return fail(sql.ErrNoRows)
	} else if err != nil {
		return fail(err)
	}
	if version != c.Version {
		return fail(ErrConflict)
	}
	return tx, p, nil
}

func guardSuccessor(ctx context.Context, tx *sql.Tx, target string) error {
	var count int
	now := time.Now().Unix()
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u WHERE u.id<>? AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL
        AND EXISTS(SELECT 1 FROM mfa_factors WHERE user_id=u.id)
        AND EXISTS(SELECT 1 FROM role_grants WHERE user_id=u.id AND role='ADMINISTRATOR' AND revoked_at IS NULL AND valid_from<=? AND valid_until>?)`, target, now, now).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return invalid("Keep an active administrator with a confirmed authenticator. Complete the successor handover first.")
	}
	return nil
}
func revokeAccessCredentials(ctx context.Context, tx *sql.Tx, id string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE users SET auth_version=auth_version+1,access_version=access_version+1 WHERE id=?", id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE account_tokens SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL AND consumed_at IS NULL", time.Now().Unix(), id)
	return err
}

func (s *Store) GrantAppointment(ctx context.Context, token, id string, in AppointmentInput) (AccessResult, error) {
	if !validAppointment(in.Role) || in.TermDays < 1 || in.TermDays > 365 {
		return AccessResult{}, invalid("Choose an appointment and a term of 1–365 days.")
	}
	tx, p, err := s.beginAccessChange(ctx, token, id, in.AccessChange)
	if err != nil {
		return AccessResult{}, err
	}
	defer tx.Rollback()
	var eligible bool
	if err = tx.QueryRowContext(ctx, "SELECT status='ACTIVE' AND verified_at IS NOT NULL AND suspended_at IS NULL FROM users WHERE id=?", id).Scan(&eligible); err != nil {
		return AccessResult{}, err
	}
	if !eligible {
		return AccessResult{}, invalid("Activate or resume this account before granting an appointment.")
	}
	now := time.Now().Unix()
	var duplicate bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM role_grants WHERE user_id=? AND role=? AND revoked_at IS NULL AND valid_until>?)", id, in.Role, now).Scan(&duplicate); err != nil {
		return AccessResult{}, err
	}
	if duplicate {
		return AccessResult{}, invalid("This appointment already has an unexpired term. End it before creating a new term.")
	}
	grant := randomToken()
	until := now + int64(in.TermDays)*86400
	if _, err = tx.ExecContext(ctx, "INSERT INTO role_grants(id,user_id,role,valid_from,valid_until,granted_by) VALUES(?,?,?,?,?,?)", grant, id, in.Role, now, until, p.ID); err != nil {
		return AccessResult{}, err
	}
	if err = revokeAccessCredentials(ctx, tx, id); err != nil {
		return AccessResult{}, err
	}
	if err = appendAudit(ctx, tx, p.ID, "", "APPOINTMENT_GRANTED", in.Reason,
		map[string]any{"user_id": id, "version": in.Version, "grant_id": grant, "present": false},
		map[string]any{"user_id": id, "grant_id": grant, "role": in.Role, "valid_from": now, "valid_until": until, "version": in.Version + 1, "sessions_revoked": true, "present": true}); err != nil {
		return AccessResult{}, err
	}
	return AccessResult{ID: id, Version: in.Version + 1}, tx.Commit()
}

func (s *Store) RevokeAppointment(ctx context.Context, token, id, grant string, in AccessChange) (AccessResult, error) {
	if len(grant) == 0 || len(grant) > 100 {
		return AccessResult{}, ErrInvalid
	}
	tx, p, err := s.beginAccessChange(ctx, token, id, in)
	if err != nil {
		return AccessResult{}, err
	}
	defer tx.Rollback()
	var role string
	var validFrom int64
	now := time.Now().Unix()
	err = tx.QueryRowContext(ctx, "SELECT role,valid_from FROM role_grants WHERE id=? AND user_id=? AND revoked_at IS NULL AND valid_until>?", grant, id, now).Scan(&role, &validFrom)
	if errors.Is(err, sql.ErrNoRows) {
		return AccessResult{}, ErrConflict
	}
	if err != nil {
		return AccessResult{}, err
	}
	if role == "ADMINISTRATOR" {
		if err = guardSuccessor(ctx, tx, id); err != nil {
			return AccessResult{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE role_grants SET revoked_at=?,revoked_by=? WHERE id=?", now, p.ID, grant); err != nil {
		return AccessResult{}, err
	}
	if err = revokeAccessCredentials(ctx, tx, id); err != nil {
		return AccessResult{}, err
	}
	previousState := "ACTIVE"
	if validFrom > now {
		previousState = "UPCOMING"
	}
	if err = appendAudit(ctx, tx, p.ID, "", "APPOINTMENT_REVOKED", in.Reason,
		map[string]any{"user_id": id, "version": in.Version, "grant_id": grant, "role": role, "state": previousState, "revoked_at": nil},
		map[string]any{"user_id": id, "grant_id": grant, "role": role, "state": "REVOKED", "revoked_at": now, "revoked_by": p.ID, "version": in.Version + 1, "sessions_revoked": true}); err != nil {
		return AccessResult{}, err
	}
	return AccessResult{ID: id, Version: in.Version + 1}, tx.Commit()
}

func (s *Store) ChangeAccountStatus(ctx context.Context, token, id string, in AccountStatusInput) (AccessResult, error) {
	if in.Action != "SUSPEND" && in.Action != "RESUME" {
		return AccessResult{}, ErrInvalid
	}
	tx, p, err := s.beginAccessChange(ctx, token, id, in.AccessChange)
	if err != nil {
		return AccessResult{}, err
	}
	defer tx.Rollback()
	var suspended, verified bool
	var previousState string
	if err = tx.QueryRowContext(ctx, "SELECT suspended_at IS NOT NULL,verified_at IS NOT NULL,status FROM users WHERE id=?", id).Scan(&suspended, &verified, &previousState); err != nil {
		return AccessResult{}, err
	}
	if suspended != (in.Action == "RESUME") {
		return AccessResult{}, ErrConflict
	}
	now := time.Now().Unix()
	action := "ACCOUNT_RESUMED"
	if suspended {
		previousState = "SUSPENDED"
	} else if !verified {
		previousState = "PENDING"
	}
	nextState := "SUSPENDED"
	var appointmentsEnded int64
	if in.Action == "SUSPEND" {
		var administrator bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM role_grants WHERE user_id=? AND role='ADMINISTRATOR' AND revoked_at IS NULL AND valid_from<=? AND valid_until>?)", id, now, now).Scan(&administrator); err != nil {
			return AccessResult{}, err
		}
		if administrator {
			if err = guardSuccessor(ctx, tx, id); err != nil {
				return AccessResult{}, err
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE users SET status='DISABLED',suspended_at=? WHERE id=?", now, id); err != nil {
			return AccessResult{}, err
		}
		result, writeErr := tx.ExecContext(ctx, "UPDATE role_grants SET revoked_at=?,revoked_by=? WHERE user_id=? AND revoked_at IS NULL", now, p.ID, id)
		if writeErr != nil {
			return AccessResult{}, writeErr
		}
		if appointmentsEnded, err = result.RowsAffected(); err != nil {
			return AccessResult{}, err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM mfa_recovery_codes WHERE user_id=?", id); err != nil {
			return AccessResult{}, err
		}
		action = "ACCOUNT_SUSPENDED"
	} else {
		status := "DISABLED"
		nextState = "PENDING"
		if verified {
			status = "ACTIVE"
			nextState = "ACTIVE"
		}
		if _, err = tx.ExecContext(ctx, "UPDATE users SET status=?,suspended_at=NULL WHERE id=?", status, id); err != nil {
			return AccessResult{}, err
		}
	}
	if err = revokeAccessCredentials(ctx, tx, id); err != nil {
		return AccessResult{}, err
	}
	if err = appendAudit(ctx, tx, p.ID, "", action, in.Reason,
		map[string]any{"user_id": id, "version": in.Version, "state": previousState},
		map[string]any{"user_id": id, "version": in.Version + 1, "state": nextState, "appointments_ended": appointmentsEnded, "sessions_revoked": true, "appointments_restored": false, "mfa_preserved": true}); err != nil {
		return AccessResult{}, err
	}
	return AccessResult{ID: id, Version: in.Version + 1}, tx.Commit()
}
