package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

type HouseholdFinancePerson struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Relationships []string `json:"relationships"`
	Visible       bool     `json:"visible"`
	AllVisible    bool     `json:"all_visible"`
	IsActor       bool     `json:"is_actor"`
}

type HouseholdFinancePage struct {
	FlatID   string                   `json:"flat_id"`
	Home     string                   `json:"home"`
	Version  int                      `json:"version"`
	Items    []HouseholdFinancePerson `json:"items"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type HouseholdFinanceInput struct {
	OperationKey string `json:"operation_key"`
	PersonID     string `json:"person_id"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Confirmed    bool   `json:"confirmed"`
	Note         string `json:"note"`
}

type HouseholdFinanceMembership struct {
	ID           string `json:"id"`
	Relationship string `json:"relationship"`
	Visible      bool   `json:"visible"`
}

type HouseholdFinanceAction struct {
	ID         string                       `json:"id"`
	FlatID     string                       `json:"flat_id"`
	Home       string                       `json:"home"`
	PersonID   string                       `json:"person_id"`
	PersonName string                       `json:"person_name"`
	Action     string                       `json:"action"`
	Visible    bool                         `json:"visible"`
	Version    int                          `json:"version"`
	Before     []HouseholdFinanceMembership `json:"before"`
	After      []HouseholdFinanceMembership `json:"after"`
	Note       string                       `json:"note"`
	Actor      string                       `json:"actor"`
	CreatedAt  int64                        `json:"created_at"`
}

// Personal financial visibility is managed by Treasury, independently of the
// registry and of a person's separate, time-limited staff appointments.
func (s *Store) HouseholdFinanceFor(ctx context.Context, token, flatID, query string, page int) (HouseholdFinancePage, error) {
	if !validText(query, 0, 100) || page < 1 || page > 400 {
		return HouseholdFinancePage{}, invalid("Use a short person search and a valid page.")
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return HouseholdFinancePage{}, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return HouseholdFinancePage{}, err
	}
	if !p.CanManageRecords {
		return HouseholdFinancePage{}, ErrForbidden
	}
	result := HouseholdFinancePage{FlatID: flatID, Items: []HouseholdFinancePerson{}, Page: page, PageSize: 24}
	if err = tx.QueryRowContext(ctx, `SELECT b.code||'-'||f.flat_number,f.version FROM flats f JOIN buildings b ON b.id=f.building_id WHERE f.id=?`, flatID).Scan(&result.Home, &result.Version); err != nil {
		return HouseholdFinancePage{}, err
	}
	day := today()
	where := ` FROM flat_memberships m JOIN residents r ON r.id=m.resident_id WHERE m.flat_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?) AND instr(lower(r.full_name),lower(?))>0`
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT r.id)`+where, flatID, day, day, query).Scan(&result.Total); err != nil {
		return HouseholdFinancePage{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT r.id,r.full_name,group_concat(DISTINCT m.relationship),MAX(m.can_view_finances),MIN(m.can_view_finances)`+where+` GROUP BY r.id,r.full_name ORDER BY lower(r.full_name),r.id LIMIT 24 OFFSET ?`, flatID, day, day, query, (page-1)*24)
	if err != nil {
		return HouseholdFinancePage{}, err
	}
	for rows.Next() {
		var person HouseholdFinancePerson
		var relationships string
		if err = rows.Scan(&person.ID, &person.Name, &relationships, &person.Visible, &person.AllVisible); err != nil {
			rows.Close()
			return HouseholdFinancePage{}, err
		}
		person.Relationships = strings.Split(relationships, ",")
		sort.Strings(person.Relationships)
		person.IsActor = person.ID == p.ResidentID
		result.Items = append(result.Items, person)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return HouseholdFinancePage{}, err
	}
	return result, tx.Commit()
}

func householdFinanceSaved(ctx context.Context, tx *sql.Tx, actor, flatID, key string) (HouseholdFinanceAction, string, error) {
	var result HouseholdFinanceAction
	var digest, blob string
	if err := tx.QueryRowContext(ctx, `SELECT request_digest,result_json FROM household_finance_actions WHERE actor_id=? AND flat_id=? AND operation_key=?`, actor, flatID, key).Scan(&digest, &blob); err != nil {
		return result, "", err
	}
	if err := json.Unmarshal([]byte(blob), &result); err != nil {
		return result, "", err
	}
	return result, digest, nil
}

func (s *Store) HouseholdFinanceActionFor(ctx context.Context, token, flatID, key string) (HouseholdFinanceAction, error) {
	if !operationPattern.MatchString(key) {
		return HouseholdFinanceAction{}, invalid("Use the original action identity.")
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return HouseholdFinanceAction{}, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return HouseholdFinanceAction{}, err
	}
	if !p.CanManageRecords {
		return HouseholdFinanceAction{}, ErrForbidden
	}
	result, _, err := householdFinanceSaved(ctx, tx, p.ID, flatID, key)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func (s *Store) ChangeHouseholdFinance(ctx context.Context, token, flatID string, in HouseholdFinanceInput) (HouseholdFinanceAction, error) {
	if !operationPattern.MatchString(in.OperationKey) || !validText(in.PersonID, 1, 120) || in.Version < 1 || (in.Action != "GRANT" && in.Action != "REVOKE") {
		return HouseholdFinanceAction{}, invalid("Choose a current person, action and home version with an original action identity.")
	}
	if err := verifiedNote(in.Confirmed, in.Note); err != nil {
		return HouseholdFinanceAction{}, err
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return HouseholdFinanceAction{}, err
	}
	defer tx.Rollback()
	if err = requireFresh(p); err != nil {
		return HouseholdFinanceAction{}, err
	}
	if in.Action == "GRANT" && in.PersonID == p.ResidentID {
		return HouseholdFinanceAction{}, invalid("Another finance officer must verify your personal household access.")
	}
	request, err := json.Marshal(struct {
		FlatID string                `json:"flat_id"`
		Input  HouseholdFinanceInput `json:"input"`
	}{flatID, in})
	if err != nil {
		return HouseholdFinanceAction{}, err
	}
	digest := TokenHash("HOUSEHOLD_FINANCE" + string(request))
	previous, oldDigest, err := householdFinanceSaved(ctx, tx, p.ID, flatID, in.OperationKey)
	if err == nil {
		if oldDigest != digest {
			return HouseholdFinanceAction{}, ErrConflict
		}
		return previous, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HouseholdFinanceAction{}, err
	}
	// A key reused for another home remains a conflict within this action domain.
	var reused int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM household_finance_actions WHERE actor_id=? AND operation_key=?`, p.ID, in.OperationKey).Scan(&reused); err != nil {
		return HouseholdFinanceAction{}, err
	}
	if reused != 0 {
		return HouseholdFinanceAction{}, ErrConflict
	}
	result := HouseholdFinanceAction{ID: randomToken(), FlatID: flatID, PersonID: in.PersonID, Action: in.Action, Visible: in.Action == "GRANT", Version: in.Version + 1, Before: []HouseholdFinanceMembership{}, After: []HouseholdFinanceMembership{}, Note: in.Note, Actor: p.Name, CreatedAt: time.Now().Unix()}
	var version int
	if err = tx.QueryRowContext(ctx, `SELECT b.code||'-'||f.flat_number,f.version FROM flats f JOIN buildings b ON b.id=f.building_id WHERE f.id=?`, flatID).Scan(&result.Home, &version); err != nil {
		return HouseholdFinanceAction{}, err
	}
	if version != in.Version {
		return HouseholdFinanceAction{}, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT full_name FROM residents WHERE id=?`, in.PersonID).Scan(&result.PersonName); err != nil {
		return HouseholdFinanceAction{}, err
	}
	day := today()
	rows, err := tx.QueryContext(ctx, `SELECT id,relationship,can_view_finances FROM flat_memberships WHERE flat_id=? AND resident_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?) ORDER BY id`, flatID, in.PersonID, day, day)
	if err != nil {
		return HouseholdFinanceAction{}, err
	}
	changed := false
	for rows.Next() {
		var member HouseholdFinanceMembership
		if err = rows.Scan(&member.ID, &member.Relationship, &member.Visible); err != nil {
			rows.Close()
			return HouseholdFinanceAction{}, err
		}
		result.Before = append(result.Before, member)
		changed = changed || member.Visible != result.Visible
		member.Visible = result.Visible
		result.After = append(result.After, member)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return HouseholdFinanceAction{}, err
	}
	if len(result.Before) == 0 {
		return HouseholdFinanceAction{}, sql.ErrNoRows
	}
	if !changed {
		return HouseholdFinanceAction{}, invalid("This person's current relationships already have that financial visibility.")
	}
	if err = bumpFlat(ctx, tx, flatID, in.Version); err != nil {
		return HouseholdFinanceAction{}, err
	}
	for _, member := range result.After {
		if _, err = tx.ExecContext(ctx, `UPDATE flat_memberships SET can_view_finances=? WHERE id=?`, member.Visible, member.ID); err != nil {
			return HouseholdFinanceAction{}, err
		}
	}
	if err = appendAudit(ctx, tx, p.ID, flatID, "HOUSEHOLD_FINANCE_"+in.Action, in.Note, map[string]any{"person_id": in.PersonID, "version": in.Version, "memberships": result.Before}, map[string]any{"action_id": result.ID, "person_id": in.PersonID, "version": result.Version, "memberships": result.After}); err != nil {
		return HouseholdFinanceAction{}, err
	}
	blob, err := json.Marshal(result)
	if err != nil {
		return HouseholdFinanceAction{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO household_finance_actions VALUES(?,?,?,?,?,?,?,?)`, result.ID, p.ID, in.OperationKey, flatID, in.PersonID, digest, string(blob), result.CreatedAt); err != nil {
		return HouseholdFinanceAction{}, err
	}
	return result, tx.Commit()
}
