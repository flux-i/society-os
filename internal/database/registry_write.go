package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type RegistryChange struct {
	Version int    `json:"version"`
	Reason  string `json:"reason"`
}
type OccupancyChange struct {
	RegistryChange
	Status string `json:"status"`
}
type AddMembership struct {
	RegistryChange
	ResidentID     string `json:"resident_id"`
	Name           string `json:"name"`
	Relationship   string `json:"relationship"`
	StartDate      string `json:"start_date"`
	PrimaryContact bool   `json:"primary_contact"`
}
type EndMembership struct {
	RegistryChange
	EndDate string `json:"end_date"`
}

func validText(value string, min, max int) bool {
	n := utf8.RuneCountInString(value)
	if !utf8.ValidString(value) || n < min || n > max || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value && value >= "1900-01-01" && value <= today()
}
func validateChange(c RegistryChange) error {
	if c.Version < 1 {
		return invalid("Reload this home before saving.")
	}
	if !validText(c.Reason, 5, 300) {
		return invalid("Give a reason of 5–300 characters.")
	}
	return nil
}

// Acquire the write reservation before reading identity/version, avoiding WAL snapshot upgrades.
// Permissions, version check, registry mutation and audit append share this transaction.
func (s *Store) beginRegistryWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, Principal{}, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at = last_seen_at WHERE token_hash = ?", TokenHash(token)); err != nil {
		tx.Rollback()
		return nil, Principal{}, err
	}
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err == nil && !p.CanManageRegistry {
		err = ErrForbidden
	}
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func bumpFlat(ctx context.Context, tx *sql.Tx, id string, version int) error {
	result, err := tx.ExecContext(ctx, "UPDATE flats SET version = version + 1 WHERE id = ? AND version = ?", id, version)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		var found int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM flats WHERE id = ?", id).Scan(&found); err != nil {
			return err
		}
		return ErrConflict
	}
	return nil
}
func appendAudit(ctx context.Context, tx *sql.Tx, actor, flatID, action, reason string, before, after any) error {
	b, err := json.Marshal(before)
	if err != nil {
		return err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return err
	}
	var flat any
	if flatID != "" {
		flat = flatID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(actor_user_id, flat_id, action, occurred_at, reason, before_json, after_json)
        VALUES (?, ?, ?, ?, ?, ?, ?)`, actor, flat, action, time.Now().Unix(), reason, string(b), string(a))
	return err
}

func (s *Store) ChangeOccupancy(ctx context.Context, token, flatID string, change OccupancyChange) error {
	if err := validateChange(change.RegistryChange); err != nil {
		return err
	}
	if change.Status != "OWNER_OCCUPIED" && change.Status != "RENTED" && change.Status != "VACANT" {
		return invalid("Choose a valid occupancy status.")
	}
	tx, p, err := s.beginRegistryWrite(ctx, token)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := bumpFlat(ctx, tx, flatID, change.Version); err != nil {
		return err
	}
	var old string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM flats WHERE id = ?", flatID).Scan(&old); err != nil {
		return err
	}
	if old == change.Status {
		return invalid("This home already has that occupancy status.")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE flats SET status = ? WHERE id = ?", change.Status, flatID); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, p.ID, flatID, "OCCUPANCY_CHANGED", change.Reason, map[string]any{"status": old, "version": change.Version}, map[string]any{"status": change.Status, "version": change.Version + 1}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AddMembership(ctx context.Context, token, flatID string, change AddMembership) error {
	if err := validateChange(change.RegistryChange); err != nil {
		return err
	}
	if (change.ResidentID == "") == (change.Name == "") {
		return invalid("Choose an existing person or enter a new name.")
	}
	if change.Name != "" && !validText(change.Name, 2, 120) {
		return invalid("Enter a name of 2–120 characters.")
	}
	if !validDate(change.StartDate) {
		return invalid("Enter a valid start date on or before today.")
	}
	if change.Relationship != "OWNER" && change.Relationship != "TENANT" && change.Relationship != "FAMILY" && change.Relationship != "AUTHORIZED_OCCUPANT" {
		return invalid("Choose a valid relationship.")
	}
	tx, p, err := s.beginRegistryWrite(ctx, token)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := bumpFlat(ctx, tx, flatID, change.Version); err != nil {
		return err
	}
	residentID := change.ResidentID
	if residentID == "" {
		residentID = "person-" + randomToken()
		if _, err := tx.ExecContext(ctx, "INSERT INTO residents VALUES (?, ?)", residentID, change.Name); err != nil {
			return err
		}
	} else {
		if err := tx.QueryRowContext(ctx, "SELECT full_name FROM residents WHERE id = ?", residentID).Scan(&change.Name); errors.Is(err, sql.ErrNoRows) {
			return invalid("Choose a person in the registry.")
		} else if err != nil {
			return err
		}
	}
	// No overlapping intervals for the same person/relationship in a home.
	var overlap int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM flat_memberships WHERE flat_id = ? AND resident_id = ? AND relationship = ?
        AND (end_date IS NULL OR end_date > ?)`, flatID, residentID, change.Relationship, change.StartDate).Scan(&overlap); err != nil {
		return err
	}
	if overlap != 0 {
		return invalid("This person already has an overlapping relationship in this home.")
	}
	if change.PrimaryContact {
		if _, err := tx.ExecContext(ctx, "UPDATE flat_memberships SET is_primary_contact = 0 WHERE flat_id = ? AND is_primary_contact = 1", flatID); err != nil {
			return err
		}
	}
	id := "membership-" + randomToken()
	if _, err := tx.ExecContext(ctx, `INSERT INTO flat_memberships VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`, id, flatID, residentID, change.Relationship, change.StartDate, change.PrimaryContact, false); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, p.ID, flatID, "MEMBERSHIP_ADDED", change.Reason, map[string]any{"version": change.Version}, map[string]any{"membership_id": id, "resident_id": residentID, "name": change.Name, "relationship": change.Relationship, "start_date": change.StartDate, "primary_contact": change.PrimaryContact, "can_view_finances": false, "version": change.Version + 1}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) EndMembership(ctx context.Context, token, flatID, membershipID string, change EndMembership) error {
	if err := validateChange(change.RegistryChange); err != nil {
		return err
	}
	if !validDate(change.EndDate) {
		return invalid("Enter a valid end date on or before today.")
	}
	tx, p, err := s.beginRegistryWrite(ctx, token)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := bumpFlat(ctx, tx, flatID, change.Version); err != nil {
		return err
	}
	var residentID, start, name, relationship string
	var end *string
	var primaryContact bool
	if err := tx.QueryRowContext(ctx, `SELECT m.resident_id, m.start_date, m.end_date, m.is_primary_contact, r.full_name, m.relationship
        FROM flat_memberships m JOIN residents r ON r.id = m.resident_id WHERE m.id = ? AND m.flat_id = ?`, membershipID, flatID).Scan(&residentID, &start, &end, &primaryContact, &name, &relationship); err != nil {
		return err
	}
	if end != nil || change.EndDate <= start {
		return invalid("Choose an open relationship and an end date after its start date.")
	}
	// Backdating must not introduce overlap with another relationship.
	if _, err := tx.ExecContext(ctx, "UPDATE flat_memberships SET end_date = ?, is_primary_contact = 0 WHERE id = ?", change.EndDate, membershipID); err != nil {
		return err
	}
	if primaryContact {
		if _, err := tx.ExecContext(ctx, `UPDATE flat_memberships SET is_primary_contact = 1 WHERE id = (
            SELECT id FROM flat_memberships WHERE flat_id = ? AND end_date IS NULL AND start_date <= ?
            ORDER BY relationship != 'OWNER', start_date, id LIMIT 1)`, flatID, today()); err != nil {
			return err
		}
	}
	if err := appendAudit(ctx, tx, p.ID, flatID, "MEMBERSHIP_ENDED", change.Reason, map[string]any{"membership_id": membershipID, "resident_id": residentID, "name": name, "relationship": relationship, "primary_contact": primaryContact, "version": change.Version}, map[string]any{"membership_id": membershipID, "end_date": change.EndDate, "version": change.Version + 1}); err != nil {
		return err
	}
	return tx.Commit()
}

type Person struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Store) PeopleFor(ctx context.Context, token, query string) ([]Person, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return nil, err
	}
	if !p.CanManageRegistry {
		return nil, ErrForbidden
	}
	rows, err := tx.QueryContext(ctx, "SELECT id, full_name FROM residents WHERE instr(lower(full_name), lower(?)) > 0 ORDER BY full_name, id LIMIT 30", query)
	if err != nil {
		return nil, err
	}
	result := []Person{}
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return result, tx.Commit()
}

type AuditEvent struct {
	ID         int64           `json:"id"`
	Actor      string          `json:"actor"`
	Action     string          `json:"action"`
	OccurredAt int64           `json:"occurred_at"`
	Reason     string          `json:"reason"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
}

func (s *Store) ActivityFor(ctx context.Context, token, flatID string) ([]AuditEvent, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return nil, err
	}
	if !p.CanManageRegistry {
		return nil, ErrForbidden
	}
	var found int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM flats WHERE id = ?", flatID).Scan(&found); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.id, u.display_name, a.action, a.occurred_at, a.reason, a.before_json, a.after_json
        FROM audit_events a JOIN users u ON u.id = a.actor_user_id WHERE a.flat_id = ? AND a.action IN ('OCCUPANCY_CHANGED','MEMBERSHIP_ADDED','MEMBERSHIP_ENDED') ORDER BY a.id DESC LIMIT 30`, flatID)
	if err != nil {
		return nil, err
	}
	result := []AuditEvent{}
	for rows.Next() {
		var a AuditEvent
		var before, after string
		if err := rows.Scan(&a.ID, &a.Actor, &a.Action, &a.OccurredAt, &a.Reason, &before, &after); err != nil {
			rows.Close()
			return nil, err
		}
		a.Before, a.After = json.RawMessage(before), json.RawMessage(after)
		result = append(result, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return result, tx.Commit()
}
