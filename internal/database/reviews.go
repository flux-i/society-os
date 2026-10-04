package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

type ReviewInput struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	FlatID       string `json:"flat_id"`
	Audience     string `json:"audience"`
	BuildingCode string `json:"building_code"`
	Estimate     string `json:"estimate"`
}
type ReviewAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type Review struct {
	ID            string        `json:"id"`
	Kind          string        `json:"kind"`
	Title         string        `json:"title"`
	Body          string        `json:"body"`
	FlatID        string        `json:"flat_id"`
	Home          string        `json:"home"`
	Audience      string        `json:"audience"`
	BuildingCode  string        `json:"building_code"`
	EstimatePaise int64         `json:"estimate_paise"`
	State         string        `json:"state"`
	AuthorID      string        `json:"author_id"`
	Author        string        `json:"author"`
	SubmittedAt   int64         `json:"submitted_at"`
	UpdatedAt     int64         `json:"updated_at"`
	Version       int           `json:"version"`
	Events        []ReviewEvent `json:"events,omitempty"`
}
type ReviewEvent struct {
	Action  string `json:"action"`
	Reason  string `json:"reason"`
	Actor   string `json:"actor"`
	At      int64  `json:"at"`
	Version int    `json:"version"`
}
type ReviewPage struct {
	Items    []Review     `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Homes    []RecordHome `json:"homes"`
}

const reviewSelect = `SELECT x.id,x.kind,x.title,x.body,COALESCE(x.flat_id,''),COALESCE(b.code||'-'||f.flat_number,''),x.audience,x.building_code,x.estimate_paise,x.state,x.submitted_by,u.display_name,x.submitted_at,x.updated_at,x.version FROM review_requests x JOIN users u ON u.id=x.submitted_by LEFT JOIN flats f ON f.id=x.flat_id LEFT JOIN buildings b ON b.id=f.building_id`

func scanReview(row interface{ Scan(...any) error }) (Review, error) {
	var x Review
	err := row.Scan(&x.ID, &x.Kind, &x.Title, &x.Body, &x.FlatID, &x.Home, &x.Audience, &x.BuildingCode, &x.EstimatePaise, &x.State, &x.AuthorID, &x.Author, &x.SubmittedAt, &x.UpdatedAt, &x.Version)
	return x, err
}
func paragraph(value string, min, max int) bool {
	return utf8.RuneCountInString(value) <= max && validText(strings.ReplaceAll(value, "\n", " "), min, max)
}
func validateReview(in ReviewInput) (int64, error) {
	if !validText(in.Title, 5, 120) || !paragraph(in.Body, 10, 4000) {
		return 0, invalid("Use a title of 5–120 characters and a description of 10–4000 characters.")
	}
	if in.Kind != "NOTICE" && in.Kind != "MAINTENANCE" && in.Kind != "REGISTRY_CHANGE" && in.Kind != "EXPENSE" {
		return 0, invalid("Choose a supported request type.")
	}
	if in.Kind == "NOTICE" {
		if in.FlatID != "" || in.Estimate != "" {
			return 0, invalid("Notices do not contain a home or expense estimate.")
		}
		if in.Audience != "ALL_RESIDENTS" && in.Audience != "OWNERS_ONLY" && in.Audience != "TENANTS_ONLY" && in.Audience != "COMMITTEE_ONLY" && in.Audience != "BUILDING" {
			return 0, invalid("Select the notice audience before submitting.")
		}
		if in.Audience == "BUILDING" {
			if in.BuildingCode != "A" && in.BuildingCode != "B" && in.BuildingCode != "C" {
				return 0, invalid("Choose a wing for this notice.")
			}
		} else if in.BuildingCode != "" {
			return 0, invalid("A wing target applies only to a wing notice.")
		}
	} else if in.Audience != "" || in.BuildingCode != "" {
		return 0, invalid("Only notices may select a public audience.")
	}
	if in.Kind != "EXPENSE" && in.Estimate != "" {
		return 0, invalid("An estimate applies only to an expense proposal.")
	}
	if in.Estimate != "" {
		return ParseAmount(in.Estimate)
	}
	return 0, nil
}
func (s *Store) beginReviewWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at=last_seen_at WHERE token_hash=?", TokenHash(token)); err != nil {
		tx.Rollback()
		return nil, Principal{}, err
	}
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func reviewHomes(ctx context.Context, q identityReader, p Principal) ([]RecordHome, error) {
	query := `SELECT f.id,b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id`
	args := []any{}
	if !p.CanReviewRequests {
		query += ` WHERE EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
		args = []any{p.ResidentID, today(), today()}
	}
	rows, err := q.QueryContext(ctx, query+" ORDER BY b.code,f.floor,f.flat_number", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	homes := []RecordHome{}
	for rows.Next() {
		var h RecordHome
		if err = rows.Scan(&h.ID, &h.Label); err != nil {
			return nil, err
		}
		homes = append(homes, h)
	}
	return homes, rows.Err()
}
func canSubmitReview(ctx context.Context, q identityReader, p Principal, flatID string) error {
	homes, err := reviewHomes(ctx, q, p)
	if err != nil {
		return err
	}
	if !p.CanReviewRequests && len(homes) == 0 {
		return ErrForbidden
	}
	if flatID != "" {
		for _, h := range homes {
			if h.ID == flatID {
				return nil
			}
		}
		return sql.ErrNoRows
	}
	return nil
}
func appendReviewEvent(ctx context.Context, tx *sql.Tx, p Principal, x Review, action, reason string, before any) error {
	copy := x
	copy.Events = nil
	blob, err := json.Marshal(copy)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO review_events(request_id,actor_id,action,reason,occurred_at,version,snapshot_json) VALUES(?,?,?,?,?,?,?)`, x.ID, p.ID, action, reason, time.Now().Unix(), x.Version, string(blob)); err != nil {
		return err
	}
	return appendAudit(ctx, tx, p.ID, x.FlatID, "REVIEW_"+action, reason, before, copy)
}
func (s *Store) SubmitReview(ctx context.Context, token, id string, in ReviewInput) (string, error) {
	estimate, err := validateReview(in)
	if err != nil {
		return "", err
	}
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = canSubmitReview(ctx, tx, p, in.FlatID); err != nil {
		return "", err
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "REVIEW_SUBMIT:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	now := time.Now().Unix()
	var flat any
	if in.FlatID != "" {
		flat = in.FlatID
	}
	action := "SUBMITTED"
	var before any = map[string]any{}
	if id == "" {
		if in.Version != 0 {
			return "", ErrInvalid
		}
		id = randomToken()
		_, err = tx.ExecContext(ctx, `INSERT INTO review_requests VALUES(?,?,?,?,?,?,?,?,'PENDING',?,?,?,1)`, id, in.Kind, in.Title, in.Body, flat, in.Audience, in.BuildingCode, estimate, p.ID, now, now)
	} else {
		old, e := scanReview(tx.QueryRowContext(ctx, reviewSelect+" WHERE x.id=? AND x.submitted_by=?", id, p.ID))
		if e != nil {
			return "", e
		}
		if old.State != "CHANGES_REQUESTED" || old.Version != in.Version || old.Kind != in.Kind {
			return "", ErrConflict
		}
		before = old
		action = "RESUBMITTED"
		_, err = tx.ExecContext(ctx, `UPDATE review_requests SET title=?,body=?,flat_id=?,audience=?,building_code=?,estimate_paise=?,state='PENDING',updated_at=?,version=version+1 WHERE id=? AND version=?`, in.Title, in.Body, flat, in.Audience, in.BuildingCode, estimate, now, id, in.Version)
	}
	if err != nil {
		return "", err
	}
	x, err := scanReview(tx.QueryRowContext(ctx, reviewSelect+" WHERE x.id=?", id))
	if err != nil {
		return "", err
	}
	if err = appendReviewEvent(ctx, tx, p, x, action, "Submitted for separate review", before); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func reviewPrivateScope(p Principal) (string, []any) {
	if p.CanReviewRequests {
		return "1=1", nil
	}
	return "x.submitted_by=?", []any{p.ID}
}
func noticeScope(p Principal) (string, []any) {
	if p.CanReviewRequests {
		return "x.kind='NOTICE' AND x.state='APPROVED'", nil
	}
	return `x.kind='NOTICE' AND x.state='APPROVED' AND EXISTS(SELECT 1 FROM flat_memberships m JOIN flats nf ON nf.id=m.flat_id JOIN buildings nb ON nb.id=nf.building_id WHERE m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?) AND (x.audience='ALL_RESIDENTS' OR (x.audience='OWNERS_ONLY' AND m.relationship='OWNER') OR (x.audience='TENANTS_ONLY' AND m.relationship='TENANT') OR (x.audience='BUILDING' AND x.building_code=nb.code)))`, []any{p.ResidentID, today(), today()}
}
func (s *Store) ReviewFor(ctx context.Context, token, id string, notice bool) (Review, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return Review{}, err
	}
	scope, args := reviewPrivateScope(p)
	if notice {
		scope, args = noticeScope(p)
	}
	x, err := scanReview(tx.QueryRowContext(ctx, reviewSelect+" WHERE x.id=? AND "+scope, append([]any{id}, args...)...))
	if err != nil {
		return x, err
	}
	if !notice {
		rows, e := tx.QueryContext(ctx, `SELECT e.action,e.reason,u.display_name,e.occurred_at,e.version FROM review_events e JOIN users u ON u.id=e.actor_id WHERE request_id=? ORDER BY e.version`, id)
		if e != nil {
			return x, e
		}
		x.Events = []ReviewEvent{}
		for rows.Next() {
			var event ReviewEvent
			if e = rows.Scan(&event.Action, &event.Reason, &event.Actor, &event.At, &event.Version); e != nil {
				rows.Close()
				return x, e
			}
			x.Events = append(x.Events, event)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return x, e
		}
	}
	return x, tx.Commit()
}
func (s *Store) ReviewsFor(ctx context.Context, token, query, state string, page int, notices bool) (ReviewPage, error) {
	out := ReviewPage{Items: []Review{}, Homes: []RecordHome{}, Page: page, PageSize: 12}
	if page < 1 || page > 10000 || !validText(query, 0, 100) {
		return out, ErrInvalid
	}
	if state != "" && state != "PENDING" && state != "CHANGES_REQUESTED" && state != "APPROVED" && state != "REJECTED" && state != "WITHDRAWN" && state != "ARCHIVED" {
		return out, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return out, err
	}
	scope, args := reviewPrivateScope(p)
	if notices {
		scope, args = noticeScope(p)
	} else {
		out.Homes, err = reviewHomes(ctx, tx, p)
		if err != nil {
			return out, err
		}
	}
	if query != "" {
		scope += " AND instr(lower(x.title||' '||x.body),lower(?))>0"
		args = append(args, query)
	}
	if state != "" {
		scope += " AND x.state=?"
		args = append(args, state)
	}
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM review_requests x WHERE "+scope, args...).Scan(&out.Total)
	if err != nil {
		return out, err
	}
	maxPage := (out.Total + 11) / 12
	if maxPage < 1 {
		maxPage = 1
	}
	if out.Page > maxPage {
		out.Page = maxPage
	}
	rows, err := tx.QueryContext(ctx, reviewSelect+" WHERE "+scope+" ORDER BY x.updated_at DESC,x.rowid DESC LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		x, e := scanReview(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) DecideReview(ctx context.Context, token, id string, in ReviewAction) (string, error) {
	if in.Version < 1 || !in.Confirmed || !validText(in.Reason, 5, 300) {
		return "", invalid("Review the item, confirm your decision and give a reason of 5–300 characters.")
	}
	if in.Decision != "APPROVED" && in.Decision != "REJECTED" && in.Decision != "CHANGES_REQUESTED" && in.Decision != "WITHDRAWN" && in.Decision != "ARCHIVED" {
		return "", ErrInvalid
	}
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if in.Decision != "WITHDRAWN" && !p.CanReviewRequests {
		return "", ErrForbidden
	}
	if in.Decision != "WITHDRAWN" && !p.Fresh {
		return "", ErrReauthRequired
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "REVIEW_DECIDE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	scope, args := reviewPrivateScope(p)
	x, err := scanReview(tx.QueryRowContext(ctx, reviewSelect+" WHERE x.id=? AND "+scope, append([]any{id}, args...)...))
	if err != nil {
		return "", err
	}
	if x.Version != in.Version {
		return "", ErrConflict
	}
	if in.Decision == "WITHDRAWN" {
		if x.AuthorID != p.ID {
			return "", ErrForbidden
		}
		if x.State != "PENDING" && x.State != "CHANGES_REQUESTED" {
			return "", ErrConflict
		}
	} else if in.Decision == "ARCHIVED" {
		if x.Kind != "NOTICE" || x.State != "APPROVED" {
			return "", ErrConflict
		}
	} else {
		if x.AuthorID == p.ID {
			return "", ErrForbidden
		}
		if x.State != "PENDING" {
			return "", ErrConflict
		}
	}
	before := x
	x.State = in.Decision
	x.Version++
	x.UpdatedAt = time.Now().Unix()
	_, err = tx.ExecContext(ctx, "UPDATE review_requests SET state=?,version=?,updated_at=? WHERE id=? AND version=?", x.State, x.Version, x.UpdatedAt, id, in.Version)
	if err != nil {
		return "", err
	}
	if err = appendReviewEvent(ctx, tx, p, x, in.Decision, in.Reason, before); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
