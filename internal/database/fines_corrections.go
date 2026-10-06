package database

import (
	"context"
	"time"
)

type FineWaiverInput struct {
	OperationKey    string `json:"operation_key"`
	FineID          string `json:"fine_id"`
	ChargeVersion   int    `json:"charge_version"`
	Kind            string `json:"kind"`
	Amount          string `json:"amount"`
	PolicyReference string `json:"policy_reference"`
	Reason          string `json:"reason"`
	Confirmed       bool   `json:"confirmed"`
}
type FineChildAction struct {
	OperationKey    string `json:"operation_key"`
	Version         int    `json:"version"`
	FineVersion     int    `json:"fine_version"`
	Action          string `json:"action"`
	Reason          string `json:"reason"`
	PolicyReference string `json:"policy_reference"`
	PauseUntil      string `json:"pause_until"`
	Confirmed       bool   `json:"confirmed"`
}
type FineWaiver struct {
	ID                 string      `json:"id"`
	FineID             string      `json:"fine_id"`
	Fine               string      `json:"fine"`
	Home               string      `json:"home"`
	ChargeVersion      int         `json:"charge_version"`
	AmountPaise        int64       `json:"amount_paise"`
	Kind               string      `json:"kind"`
	PolicyReference    string      `json:"policy_reference"`
	Reason             string      `json:"reason"`
	AuthorID           string      `json:"author_id"`
	Author             string      `json:"author"`
	State              string      `json:"state"`
	Version            int         `json:"version"`
	CreatedAt          int64       `json:"created_at"`
	Reviewer           string      `json:"reviewer"`
	ReviewedAt         int64       `json:"reviewed_at"`
	DecisionReason     string      `json:"decision_reason"`
	ReplacementEntryID string      `json:"replacement_entry_id"`
	CanDecide          bool        `json:"can_decide"`
	CurrentFineVersion int         `json:"current_fine_version"`
	Events             []FundEvent `json:"events"`
	EventTotal         int         `json:"event_total"`
	EventPage          int         `json:"event_page"`
	PageSize           int         `json:"page_size"`
}
type FineWaiverPage struct {
	Items    []FineWaiver `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

func (s *Store) ProposeFineWaiver(ctx context.Context, token string, in FineWaiverInput) (string, error) {
	amount, e := ParseAmount(in.Amount)
	if e != nil {
		return "", e
	}
	if !in.Confirmed || len(in.FineID) < 1 || len(in.FineID) > 100 || in.ChargeVersion < 1 || (in.Kind != "WAIVER" && in.Kind != "REVERSAL") || !validText(in.PolicyReference, 5, 300) || !paragraph(in.Reason, 10, 2000) {
		return "", ErrInvalid
	}
	tx, p, e := s.beginFineWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", in.FineID))
	if e != nil {
		return "", e
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "FINE_WAIVER_CREATE", in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if x.State != "ISSUED" || x.ChargeVersion != in.ChargeVersion || x.CurrentEntryID == "" {
		return "", ErrConflict
	}
	entry, e := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", x.CurrentEntryID))
	if e != nil {
		return "", e
	}
	if entry.State != "POSTED" || amount > entry.AmountPaise || (in.Kind == "REVERSAL" && amount != entry.AmountPaise) {
		return "", invalid("A waiver is at most the live charge; a reversal removes its full remaining amount.")
	}
	id, now := randomToken(), time.Now().Unix()
	_, e = tx.ExecContext(ctx, `INSERT INTO fine_waivers(id,fine_id,charge_version,amount_paise,kind,policy_reference,reason,author_id,state,version,created_at) VALUES(?,?,?,?,?,?,?,?,'PENDING',1,?)`, id, x.ID, x.ChargeVersion, amount, in.Kind, in.PolicyReference, in.Reason, p.ID, now)
	if e != nil {
		return "", e
	}
	if e = fineEvent(ctx, tx, p, x.ID, "WAIVER", id, "PROPOSED", in.Reason, 1, in); e != nil {
		return "", e
	}
	if e = fineTouch(ctx, tx, p, x, "CORRECTION_PROPOSED", "A supplied correction awaits separate review", false, map[string]string{"waiver_id": id}); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}

const fineWaiverSelect = `SELECT w.id,w.fine_id,f.title,b.code||'-'||h.flat_number,w.charge_version,w.amount_paise,w.kind,w.policy_reference,w.reason,w.author_id,u.display_name,w.state,w.version,w.created_at,COALESCE(v.display_name,''),COALESCE(w.reviewed_at,0),w.decision_reason,COALESCE(w.replacement_entry_id,'') FROM fine_waivers w JOIN fines f ON f.id=w.fine_id JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id JOIN users u ON u.id=w.author_id LEFT JOIN users v ON v.id=w.reviewer_id `

func scanFineWaiver(row interface{ Scan(...any) error }) (FineWaiver, error) {
	var w FineWaiver
	e := row.Scan(&w.ID, &w.FineID, &w.Fine, &w.Home, &w.ChargeVersion, &w.AmountPaise, &w.Kind, &w.PolicyReference, &w.Reason, &w.AuthorID, &w.Author, &w.State, &w.Version, &w.CreatedAt, &w.Reviewer, &w.ReviewedAt, &w.DecisionReason, &w.ReplacementEntryID)
	return w, e
}
func (s *Store) DecideFineWaiver(ctx context.Context, token, id string, in FineChildAction) (string, error) {
	if len(id) < 1 || len(id) > 100 || !in.Confirmed || in.Version < 1 || in.FineVersion < 1 || !paragraph(in.Reason, 10, 2000) || in.PolicyReference != "" || in.PauseUntil != "" || (in.Action != "APPROVED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN") {
		return "", ErrInvalid
	}
	tx, p, e := s.beginFineWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	w, e := scanFineWaiver(tx.QueryRowContext(ctx, fineWaiverSelect+"WHERE w.id=?", id))
	if e != nil {
		return "", e
	}
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", w.FineID))
	if e != nil {
		return "", e
	}
	if in.Action == "WITHDRAWN" {
		if w.AuthorID != p.ID {
			return "", ErrForbidden
		}
	} else {
		if w.AuthorID == p.ID {
			return "", ErrForbidden
		}
		if e = fineEligible(ctx, tx, p, x); e != nil {
			return "", e
		}
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "FINE_WAIVER_ACTION:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if w.State != "PENDING" || w.Version != in.Version || x.Version != in.FineVersion {
		return "", ErrConflict
	}
	now := time.Now().Unix()
	replacement := ""
	if in.Action == "APPROVED" {
		if x.State != "ISSUED" || x.ChargeVersion != w.ChargeVersion || x.CurrentEntryID == "" {
			return "", ErrConflict
		}
		entry, e := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", x.CurrentEntryID))
		if e != nil {
			return "", e
		}
		if entry.State != "POSTED" || w.AmountPaise > entry.AmountPaise {
			return "", ErrConflict
		}
		state := "WAIVED"
		if entry.AmountPaise > w.AmountPaise {
			state = "ISSUED"
			replacement = randomToken()
			_, e = tx.ExecContext(ctx, `INSERT INTO entries(id,flat_id,kind,amount_paise,entry_date,description,payer,method,reference,source_note,state,created_by,created_at,posted_by,posted_at) VALUES(?,?,'CHARGE',?,?,?,'','','','Separately reviewed fine correction; original retained','POSTED',?,?,?,?)`, replacement, x.FlatID, entry.AmountPaise-w.AmountPaise, entry.Date, "Fine · "+x.Title, w.AuthorID, w.CreatedAt, p.ID, now)
			if e != nil {
				return "", e
			}
		}
		// The database guard requires the completed independent decision before
		// it permits a reversal. Both records and the replacement remain atomic.
		_, e = tx.ExecContext(ctx, `UPDATE fine_waivers SET state='APPROVED',version=version+1,reviewer_id=?,reviewed_at=?,decision_reason=?,replacement_entry_id=? WHERE id=?`, p.ID, now, in.Reason, optionalID(replacement), id)
		if e != nil {
			return "", e
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO entry_reversals VALUES(?,?,?,?)", entry.ID, "Separately reviewed fine correction: "+in.Reason, p.ID, now)
		if e != nil {
			return "", e
		}
		_, e = tx.ExecContext(ctx, `UPDATE fines SET state=?,current_entry_id=?,waived_paise=waived_paise+?,charge_version=charge_version+1,version=version+1,public_version=public_version+1,public_updated_at=?,updated_at=? WHERE id=?`, state, optionalID(replacement), w.AmountPaise, now, now, x.ID)
		if e != nil {
			return "", e
		}
		if e = fineEvent(ctx, tx, p, x.ID, "FINE", x.ID, "CORRECTION_APPROVED", in.Reason, x.Version+1, map[string]any{"waiver_id": id, "original_entry_id": entry.ID, "replacement_entry_id": replacement, "amount_paise": w.AmountPaise}); e != nil {
			return "", e
		}
	} else {
		if e = fineTouch(ctx, tx, p, x, "CORRECTION_"+in.Action, in.Reason, false, map[string]string{"waiver_id": id}); e != nil {
			return "", e
		}
		_, e = tx.ExecContext(ctx, `UPDATE fine_waivers SET state=?,version=version+1,reviewer_id=?,reviewed_at=?,decision_reason=? WHERE id=?`, in.Action, p.ID, now, in.Reason, id)
		if e != nil {
			return "", e
		}
	}
	if e = fineEvent(ctx, tx, p, x.ID, "WAIVER", id, in.Action, in.Reason, w.Version+1, in); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) FineWaiverFor(ctx context.Context, token, id string, page int) (FineWaiver, error) {
	out := FineWaiver{PageSize: 20}
	if len(id) < 1 || len(id) > 100 || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if !p.CanReadAllRecords {
		return out, ErrForbidden
	}
	out, e = scanFineWaiver(tx.QueryRowContext(ctx, fineWaiverSelect+"WHERE w.id=?", id))
	if e != nil {
		return out, e
	}
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", out.FineID))
	if e != nil {
		return out, e
	}
	out.CanDecide = out.AuthorID != p.ID && fineEligible(ctx, tx, p, x) == nil
	out.CurrentFineVersion = x.Version
	out.PageSize = 20
	out.Events, out.EventTotal, out.EventPage, e = fineEvents(ctx, tx, "WAIVER", id, page)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func (s *Store) FineWaiversFor(ctx context.Context, token, fine string, page int) (FineWaiverPage, error) {
	out := FineWaiverPage{Items: []FineWaiver{}, Page: page, PageSize: 12}
	if len(fine) > 100 || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if !p.CanReadAllRecords {
		return out, ErrForbidden
	}
	where := "1=1"
	args := []any{}
	if fine != "" {
		where = "w.fine_id=?"
		args = append(args, fine)
	}
	e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fine_waivers w WHERE "+where, args...).Scan(&out.Total)
	if e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	rows, e := tx.QueryContext(ctx, fineWaiverSelect+"WHERE "+where+" ORDER BY w.created_at DESC,w.id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		w, err := scanFineWaiver(rows)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, w)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	return out, tx.Commit()
}
