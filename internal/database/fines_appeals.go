package database

import (
	"context"
	"database/sql"
	"time"
)

type FineAppealInput struct {
	OperationKey string `json:"operation_key"`
	NoticeID     string `json:"notice_id"`
	Body         string `json:"body"`
	Confirmed    bool   `json:"confirmed"`
}
type FineAppeal struct {
	ID                 string      `json:"id"`
	FineID             string      `json:"fine_id"`
	Fine               string      `json:"fine"`
	Home               string      `json:"home"`
	AuthorID           string      `json:"author_id,omitempty"`
	Author             string      `json:"author,omitempty"`
	Body               string      `json:"body"`
	State              string      `json:"state"`
	Version            int         `json:"version"`
	CreatedAt          int64       `json:"created_at"`
	UpdatedAt          int64       `json:"updated_at"`
	Reviewer           string      `json:"reviewer,omitempty"`
	ReviewedAt         int64       `json:"reviewed_at,omitempty"`
	DecisionReason     string      `json:"decision_reason"`
	PolicyReference    string      `json:"policy_reference"`
	PauseUntil         string      `json:"pause_until"`
	CanDecide          bool        `json:"can_decide"`
	CanWithdraw        bool        `json:"can_withdraw"`
	CurrentFineVersion int         `json:"current_fine_version,omitempty"`
	Events             []FundEvent `json:"events,omitempty"`
	EventTotal         int         `json:"event_total,omitempty"`
	EventPage          int         `json:"event_page,omitempty"`
	PageSize           int         `json:"page_size"`
}
type FineAppealPage struct {
	Items    []FineAppeal `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

func (s *Store) CreateFineAppeal(ctx context.Context, token string, in FineAppealInput) (string, error) {
	if !in.Confirmed || len(in.NoticeID) < 1 || len(in.NoticeID) > 100 || !paragraph(in.Body, 10, 4000) {
		return "", ErrInvalid
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var fine, home, state string
	e = tx.QueryRowContext(ctx, "SELECT n.fine_id,n.flat_id,f.state FROM fine_notices n JOIN fines f ON f.id=n.fine_id WHERE n.id=?", in.NoticeID).Scan(&fine, &home, &state)
	if e != nil {
		return "", e
	}
	member, e := incidentHomeMember(ctx, tx, p, home)
	if e != nil {
		return "", e
	}
	if !member {
		return "", sql.ErrNoRows
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "FINE_APPEAL_CREATE", in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if state != "ISSUED" && state != "WAIVED" {
		return "", invalid("An appeal concerns an issued fine. Before issuance, respond to its household notice.")
	}
	var pending bool
	e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM fine_appeals WHERE fine_id=? AND author_id=? AND state IN('PENDING','PAUSED'))", fine, p.ID).Scan(&pending)
	if e != nil {
		return "", e
	}
	if pending {
		return "", invalid("Your existing appeal is still pending. Open it to follow its decision.")
	}
	id, now := randomToken(), time.Now().Unix()
	_, e = tx.ExecContext(ctx, `INSERT INTO fine_appeals(id,fine_id,author_id,body,state,version,created_at,updated_at) VALUES(?,?,?,?,'PENDING',1,?,?)`, id, fine, p.ID, in.Body, now, now)
	if e != nil {
		return "", e
	}
	if e = fineEvent(ctx, tx, p, fine, "APPEAL", id, "SUBMITTED", "A current household member supplied a private appeal", 1, in); e != nil {
		return "", e
	}
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", fine))
	if e != nil {
		return "", e
	}
	if e = fineTouch(ctx, tx, p, x, "APPEAL_SUBMITTED", "A supplied appeal awaits an independent decision", false, map[string]string{"appeal_id": id}); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}

const fineAppealSelect = `SELECT a.id,a.fine_id,f.title,b.code||'-'||h.flat_number,a.author_id,u.display_name,a.body,a.state,a.version,a.created_at,a.updated_at,COALESCE(v.display_name,''),COALESCE(a.reviewed_at,0),a.decision_reason,a.policy_reference,a.pause_until FROM fine_appeals a JOIN fines f ON f.id=a.fine_id JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id JOIN users u ON u.id=a.author_id LEFT JOIN users v ON v.id=a.reviewer_id `

func scanFineAppeal(row interface{ Scan(...any) error }) (FineAppeal, error) {
	var a FineAppeal
	e := row.Scan(&a.ID, &a.FineID, &a.Fine, &a.Home, &a.AuthorID, &a.Author, &a.Body, &a.State, &a.Version, &a.CreatedAt, &a.UpdatedAt, &a.Reviewer, &a.ReviewedAt, &a.DecisionReason, &a.PolicyReference, &a.PauseUntil)
	return a, e
}
func fineAppealScope(p Principal) (string, []any) {
	if p.CanReadAllRecords {
		return "1=1", nil
	}
	return `a.author_id=? AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.flat_id AND m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`, []any{p.ID, p.ResidentID, today(), today()}
}
func (s *Store) ActOnFineAppeal(ctx context.Context, token, id string, in FineChildAction) (string, error) {
	if len(id) < 1 || len(id) > 100 || !in.Confirmed || in.Version < 1 || !paragraph(in.Reason, 10, 2000) || (in.Action != "PAUSED" && in.Action != "RESOLVED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN") {
		return "", ErrInvalid
	}
	if in.Action == "WITHDRAWN" {
		if in.FineVersion != 0 || in.PolicyReference != "" || in.PauseUntil != "" {
			return "", ErrInvalid
		}
	} else if in.FineVersion < 1 || !validText(in.PolicyReference, 5, 300) || (in.Action == "PAUSED" && (!validMaintenanceDate(in.PauseUntil) || in.PauseUntil < today())) || (in.Action != "PAUSED" && in.PauseUntil != "") {
		return "", ErrInvalid
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	scope, args := fineAppealScope(p)
	a, e := scanFineAppeal(tx.QueryRowContext(ctx, fineAppealSelect+"WHERE a.id=? AND "+scope, append([]any{id}, args...)...))
	if e != nil {
		return "", e
	}
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", a.FineID))
	if e != nil {
		return "", e
	}
	if in.Action == "WITHDRAWN" {
		member, e := incidentHomeMember(ctx, tx, p, x.FlatID)
		if e != nil {
			return "", e
		}
		if a.AuthorID != p.ID || !member {
			return "", ErrForbidden
		}
	} else {
		if a.AuthorID == p.ID {
			return "", ErrForbidden
		}
		if e = fineEligible(ctx, tx, p, x); e != nil {
			return "", e
		}
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "FINE_APPEAL_ACTION:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if a.Version != in.Version || (in.Action != "WITHDRAWN" && x.Version != in.FineVersion) || (a.State != "PENDING" && a.State != "PAUSED") || (in.Action == "WITHDRAWN" && a.State != "PENDING") {
		return "", ErrConflict
	}
	now := time.Now().Unix()
	if in.Action == "PAUSED" {
		if x.PauseAppealID != "" && x.PauseAppealID != id {
			return "", invalid("Resolve the current supplied pause before pausing another appeal.")
		}
		_, e = tx.ExecContext(ctx, `UPDATE fines SET pause_until=?,pause_appeal_id=?,version=version+1,public_version=public_version+1,public_updated_at=?,updated_at=? WHERE id=?`, in.PauseUntil, id, now, now, x.ID)
	} else if x.PauseAppealID == id {
		_, e = tx.ExecContext(ctx, `UPDATE fines SET pause_until='',pause_appeal_id=NULL,version=version+1,public_version=public_version+1,public_updated_at=?,updated_at=? WHERE id=?`, now, now, x.ID)
	} else {
		e = fineTouch(ctx, tx, p, x, "APPEAL_"+in.Action, in.Reason, false, map[string]string{"appeal_id": id})
	}
	if e != nil {
		return "", e
	}
	if in.Action == "PAUSED" || x.PauseAppealID == id {
		if e = fineEvent(ctx, tx, p, x.ID, "FINE", x.ID, "APPEAL_"+in.Action, in.Reason, x.Version+1, map[string]string{"appeal_id": id, "pause_until": in.PauseUntil}); e != nil {
			return "", e
		}
	}
	_, e = tx.ExecContext(ctx, `UPDATE fine_appeals SET state=?,version=version+1,updated_at=?,reviewer_id=?,reviewed_at=?,decision_reason=?,policy_reference=?,pause_until=? WHERE id=?`, in.Action, now, p.ID, now, in.Reason, in.PolicyReference, in.PauseUntil, id)
	if e != nil {
		return "", e
	}
	if e = fineEvent(ctx, tx, p, x.ID, "APPEAL", id, in.Action, in.Reason, a.Version+1, in); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func fineAppealEnrich(ctx context.Context, tx *sql.Tx, p Principal, a *FineAppeal) error {
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", a.FineID))
	if e != nil {
		return e
	}
	member, e := incidentHomeMember(ctx, tx, p, x.FlatID)
	if e != nil {
		return e
	}
	a.CanWithdraw = member && a.AuthorID == p.ID && a.State == "PENDING"
	a.CanDecide = a.AuthorID != p.ID && fineEligible(ctx, tx, p, x) == nil
	if p.CanReadAllRecords {
		a.CurrentFineVersion = x.Version
	} else {
		a.AuthorID = ""
		a.Author = ""
		a.Reviewer = ""
	}
	return nil
}
func (s *Store) FineAppealFor(ctx context.Context, token, id string, page int) (FineAppeal, error) {
	out := FineAppeal{PageSize: 20}
	if len(id) < 1 || len(id) > 100 || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	scope, args := fineAppealScope(p)
	out, e = scanFineAppeal(tx.QueryRowContext(ctx, fineAppealSelect+"WHERE a.id=? AND "+scope, append([]any{id}, args...)...))
	if e != nil {
		return out, e
	}
	out.PageSize = 20
	if e = fineAppealEnrich(ctx, tx, p, &out); e != nil {
		return out, e
	}
	// Only the author and authorised financial readers receive this appeal's events.
	out.Events, out.EventTotal, out.EventPage, e = fineEvents(ctx, tx, "APPEAL", id, page)
	if e != nil {
		return out, e
	}
	if !p.CanReadAllRecords {
		for i := range out.Events {
			out.Events[i].Actor = ""
		}
	}
	return out, tx.Commit()
}
func (s *Store) FineAppealsFor(ctx context.Context, token, fine string, page int) (FineAppealPage, error) {
	out := FineAppealPage{Items: []FineAppeal{}, Page: page, PageSize: 12}
	if len(fine) > 100 || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	scope, args := fineAppealScope(p)
	if fine != "" {
		scope += " AND a.fine_id=?"
		args = append(args, fine)
	}
	e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM fine_appeals a JOIN fines f ON f.id=a.fine_id WHERE `+scope, args...).Scan(&out.Total)
	if e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	rows, e := tx.QueryContext(ctx, fineAppealSelect+"WHERE "+scope+" ORDER BY a.created_at DESC,a.id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		a, err := scanFineAppeal(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, a)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for i := range out.Items {
		if e = fineAppealEnrich(ctx, tx, p, &out.Items[i]); e != nil {
			return out, e
		}
	}
	return out, tx.Commit()
}
