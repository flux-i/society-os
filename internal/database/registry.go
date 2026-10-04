package database

import (
	"context"
	"database/sql"
	"time"
	_ "time/tzdata"
)

type Counts struct {
	Buildings   int `json:"buildings"`
	Flats       int `json:"flats"`
	Residents   int `json:"residents"`
	Memberships int `json:"memberships"`
}

func CountRecords(ctx context.Context, db *sql.DB) (Counts, error) {
	var c Counts
	err := db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM buildings),
		(SELECT COUNT(*) FROM flats), (SELECT COUNT(*) FROM residents),
		(SELECT COUNT(*) FROM flat_memberships)`).Scan(&c.Buildings, &c.Flats, &c.Residents, &c.Memberships)
	return c, err
}

type BuildingSummary struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Flats         int    `json:"flats"`
	OwnerOccupied int    `json:"owner_occupied"`
	Rented        int    `json:"rented"`
	Vacant        int    `json:"vacant"`
	Owners        int    `json:"owners"`
	Tenants       int    `json:"tenants"`
}

type CommunityStats struct {
	Owners        int `json:"owners"`
	Tenants       int `json:"tenants"`
	Occupied      int `json:"occupied"`
	Vacant        int `json:"vacant"`
	OwnerOccupied int `json:"owner_occupied"`
	Rented        int `json:"rented"`
}

type Summary struct {
	Counts         Counts            `json:"counts"`
	Community      CommunityStats    `json:"community"`
	Buildings      []BuildingSummary `json:"buildings"`
	DataKind       string            `json:"data_kind"`
	FixtureVersion string            `json:"fixture_version"`
}

func (s *Store) Summary(ctx context.Context) (Summary, error) {
	return s.summary(ctx, "", false)
}

func (s *Store) SummaryFor(ctx context.Context, token string) (Summary, error) {
	return s.summary(ctx, token, true)
}

func (s *Store) summary(ctx context.Context, token string, authenticated bool) (Summary, error) {
	// A read transaction keeps counts and wing totals on the same snapshot.
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Summary{}, err
	}
	defer tx.Rollback()
	if authenticated {
		p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
		if err != nil {
			return Summary{}, err
		}
		if !p.CanReadRegistry {
			return Summary{}, ErrForbidden
		}
	}
	var result Summary
	date := today()
	if err := tx.QueryRowContext(ctx, `SELECT
        COUNT(DISTINCT CASE WHEN relationship = 'OWNER' THEN resident_id END),
        COUNT(DISTINCT CASE WHEN relationship = 'TENANT' THEN resident_id END)
        FROM flat_memberships WHERE start_date <= ? AND (end_date IS NULL OR end_date > ?)`, date, date).Scan(&result.Community.Owners, &result.Community.Tenants); err != nil {
		return Summary{}, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM buildings),
		(SELECT COUNT(*) FROM flats), (SELECT COUNT(*) FROM residents),
		(SELECT COUNT(*) FROM flat_memberships)`).Scan(&result.Counts.Buildings, &result.Counts.Flats, &result.Counts.Residents, &result.Counts.Memberships); err != nil {
		return Summary{}, err
	}
	if err := tx.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = 'data_kind'").Scan(&result.DataKind); err != nil {
		return Summary{}, err
	}
	if err := tx.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = 'fixture_version'").Scan(&result.FixtureVersion); err != nil {
		return Summary{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT b.id, b.code, b.name, COUNT(f.id),
		COALESCE(SUM(f.status = 'OWNER_OCCUPIED'), 0), COALESCE(SUM(f.status = 'RENTED'), 0), COALESCE(SUM(f.status = 'VACANT'), 0),
        (SELECT COUNT(DISTINCT m.resident_id) FROM flat_memberships m JOIN flats mf ON mf.id = m.flat_id
         WHERE mf.building_id = b.id AND m.relationship = 'OWNER' AND m.start_date <= ? AND (m.end_date IS NULL OR m.end_date > ?)),
        (SELECT COUNT(DISTINCT m.resident_id) FROM flat_memberships m JOIN flats mf ON mf.id = m.flat_id
         WHERE mf.building_id = b.id AND m.relationship = 'TENANT' AND m.start_date <= ? AND (m.end_date IS NULL OR m.end_date > ?))
		FROM buildings b LEFT JOIN flats f ON f.building_id = b.id GROUP BY b.id ORDER BY b.code`, date, date, date, date)
	if err != nil {
		return Summary{}, err
	}
	result.Buildings = []BuildingSummary{}
	for rows.Next() {
		var b BuildingSummary
		if err := rows.Scan(&b.ID, &b.Code, &b.Name, &b.Flats, &b.OwnerOccupied, &b.Rented, &b.Vacant, &b.Owners, &b.Tenants); err != nil {
			rows.Close()
			return Summary{}, err
		}
		result.Buildings = append(result.Buildings, b)
		result.Community.OwnerOccupied += b.OwnerOccupied
		result.Community.Rented += b.Rented
		result.Community.Vacant += b.Vacant
		result.Community.Occupied += b.OwnerOccupied + b.Rented
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Summary{}, err
	}
	return result, tx.Commit()
}

type Flat struct {
	ID             string `json:"id"`
	BuildingCode   string `json:"building_code"`
	Number         string `json:"number"`
	Floor          int    `json:"floor"`
	Status         string `json:"status"`
	PrimaryContact string `json:"primary_contact"`
	Version        int    `json:"version"`
}

type FlatFilter struct {
	Query    string
	Building string
	Status   string
	Page     int
	PageSize int
}

type FlatPage struct {
	Items    []Flat `json:"items"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

var societyZone, _ = time.LoadLocation("Asia/Kolkata")

func today() string { return time.Now().In(societyZone).Format("2006-01-02") }

func (s *Store) Flats(ctx context.Context, filter FlatFilter) (FlatPage, error) {
	return s.flats(ctx, filter, "", false)
}

func (s *Store) FlatsFor(ctx context.Context, filter FlatFilter, token string) (FlatPage, error) {
	return s.flats(ctx, filter, token, true)
}

func (s *Store) flats(ctx context.Context, filter FlatFilter, token string, authenticated bool) (FlatPage, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return FlatPage{}, err
	}
	defer tx.Rollback()
	var p Principal
	if authenticated {
		p, err = fullPrincipal(ctx, tx, TokenHash(token), time.Now())
		if err != nil {
			return FlatPage{}, err
		}
	}
	where := ` FROM flats f JOIN buildings b ON b.id = f.building_id
		WHERE (? = '' OR b.code = ?) AND (? = '' OR f.status = ?)
		AND (? = '' OR instr(lower(b.code || '-' || f.flat_number), lower(?)) > 0
		OR EXISTS (SELECT 1 FROM flat_memberships m JOIN residents r ON r.id = m.resident_id
			WHERE m.flat_id = f.id AND m.start_date <= ? AND (m.end_date IS NULL OR m.end_date > ?)
			AND instr(lower(r.full_name), lower(?)) > 0))`
	date := today()
	args := []any{filter.Building, filter.Building, filter.Status, filter.Status, filter.Query, filter.Query, date, date, filter.Query}
	if authenticated && !p.CanReadRegistry {
		where += ` AND EXISTS (SELECT 1 FROM flat_memberships scope WHERE scope.flat_id = f.id AND scope.resident_id = ?
            AND scope.start_date <= ? AND (scope.end_date IS NULL OR scope.end_date > ?))`
		args = append(args, p.ResidentID, date, date)
	}
	result := FlatPage{Items: []Flat{}, Page: filter.Page, PageSize: filter.PageSize}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*)"+where, args...).Scan(&result.Total); err != nil {
		return FlatPage{}, err
	}
	query := `SELECT f.id, b.code, f.flat_number, f.floor, f.status, f.version,
		COALESCE((SELECT r.full_name FROM flat_memberships m JOIN residents r ON r.id = m.resident_id
			WHERE m.flat_id = f.id AND m.is_primary_contact = 1 AND m.start_date <= ?
			AND (m.end_date IS NULL OR m.end_date > ?) ORDER BY m.id LIMIT 1), '')` + where +
		` ORDER BY b.code, f.floor, f.flat_number LIMIT ? OFFSET ?`
	queryArgs := append([]any{date, date}, args...)
	queryArgs = append(queryArgs, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := tx.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return FlatPage{}, err
	}
	for rows.Next() {
		var f Flat
		if err := rows.Scan(&f.ID, &f.BuildingCode, &f.Number, &f.Floor, &f.Status, &f.Version, &f.PrimaryContact); err != nil {
			rows.Close()
			return FlatPage{}, err
		}
		result.Items = append(result.Items, f)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return FlatPage{}, err
	}
	return result, tx.Commit()
}

type Member struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Relationship   string  `json:"relationship"`
	StartDate      string  `json:"start_date"`
	EndDate        *string `json:"end_date"`
	MembershipID   string  `json:"membership_id"`
	PrimaryContact bool    `json:"primary_contact"`
	Active         bool    `json:"active"`
}

type FlatDetail struct {
	Flat
	Members []Member `json:"members"`
}

func (s *Store) Flat(ctx context.Context, id string) (FlatDetail, error) {
	return s.flat(ctx, id, "", false)
}

func (s *Store) FlatFor(ctx context.Context, id, token string) (FlatDetail, error) {
	return s.flat(ctx, id, token, true)
}

func (s *Store) flat(ctx context.Context, id, token string, authenticated bool) (FlatDetail, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return FlatDetail{}, err
	}
	defer tx.Rollback()
	var p Principal
	if authenticated {
		p, err = fullPrincipal(ctx, tx, TokenHash(token), time.Now())
		if err != nil {
			return FlatDetail{}, err
		}
	}
	var result FlatDetail
	result.Members = []Member{}
	date := today()
	scope := ""
	args := []any{id}
	if authenticated && !p.CanReadRegistry {
		scope = ` AND EXISTS (SELECT 1 FROM flat_memberships m WHERE m.flat_id = f.id AND m.resident_id = ?
            AND m.start_date <= ? AND (m.end_date IS NULL OR m.end_date > ?))`
		args = append(args, p.ResidentID, date, date)
	}
	err = tx.QueryRowContext(ctx, `SELECT f.id, b.code, f.flat_number, f.floor, f.status, f.version
		FROM flats f JOIN buildings b ON f.building_id = b.id WHERE f.id = ?`+scope, args...).Scan(&result.ID, &result.BuildingCode, &result.Number, &result.Floor, &result.Status, &result.Version)
	if err != nil {
		return FlatDetail{}, err
	}
	memberScope := ""
	memberArgs := []any{date, date, id}
	if authenticated && !p.CanReadRegistry {
		memberScope = ` AND m.start_date <= ? AND (m.end_date IS NULL OR m.end_date > ?)`
		memberArgs = append(memberArgs, date, date)
	}
	rows, err := tx.QueryContext(ctx, `SELECT r.id, r.full_name, m.relationship, m.start_date, m.end_date, m.id, m.is_primary_contact,
        m.start_date <= ? AND (m.end_date IS NULL OR m.end_date > ?)
		FROM flat_memberships m JOIN residents r ON r.id = m.resident_id
		WHERE m.flat_id = ?`+memberScope+` ORDER BY m.relationship, r.full_name`, memberArgs...)
	if err != nil {
		return FlatDetail{}, err
	}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Relationship, &m.StartDate, &m.EndDate, &m.MembershipID, &m.PrimaryContact, &m.Active); err != nil {
			rows.Close()
			return FlatDetail{}, err
		}
		result.Members = append(result.Members, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return FlatDetail{}, err
	}
	return result, tx.Commit()
}
