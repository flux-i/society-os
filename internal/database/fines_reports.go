package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type FineReportInput struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	FineID       string `json:"fine_id"`
	Amount       string `json:"amount"`
	PaymentDate  string `json:"payment_date"`
	Payer        string `json:"payer"`
	Method       string `json:"method"`
	Reference    string `json:"reference"`
	Comment      string `json:"comment"`
	EvidenceID   string `json:"evidence_id"`
	Confirmed    bool   `json:"confirmed"`
}
type FineReport struct {
	ID                string      `json:"id"`
	FineID            string      `json:"fine_id"`
	Fine              string      `json:"fine"`
	FlatID            string      `json:"flat_id"`
	Home              string      `json:"home"`
	AuthorID          string      `json:"author_id"`
	Author            string      `json:"author"`
	AmountPaise       int64       `json:"amount_paise"`
	PaymentDate       string      `json:"payment_date"`
	Payer             string      `json:"payer"`
	Method            string      `json:"method"`
	Reference         string      `json:"reference"`
	Comment           string      `json:"comment"`
	EvidenceID        string      `json:"evidence_id"`
	State             string      `json:"state"`
	Version           int         `json:"version"`
	CreatedAt         int64       `json:"created_at"`
	UpdatedAt         int64       `json:"updated_at"`
	EntryID           string      `json:"entry_id"`
	AllocationID      string      `json:"allocation_id"`
	DuplicateReportID string      `json:"duplicate_report_id"`
	Reviewer          string      `json:"reviewer"`
	ReviewedAt        int64       `json:"reviewed_at"`
	DecisionReason    string      `json:"decision_reason"`
	ReceiptID         string      `json:"receipt_id"`
	Receipt           string      `json:"receipt"`
	CurrentState      string      `json:"current_state"`
	CanDecide         bool        `json:"can_decide"`
	Events            []FundEvent `json:"events,omitempty"`
	EventTotal        int         `json:"event_total,omitempty"`
	EventPage         int         `json:"event_page,omitempty"`
	PageSize          int         `json:"page_size"`
}
type FineReportPage struct {
	Items    []FineReport     `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Counts   map[string]int64 `json:"counts"`
}
type FineConfirmOptions struct {
	Entries          []FundReceiptChoice `json:"entries"`
	OutstandingPaise int64               `json:"outstanding_paise"`
	ChargeID         string              `json:"charge_id"`
	Total            int                 `json:"total"`
	Page             int                 `json:"page"`
	PageSize         int                 `json:"page_size"`
}
type FineEvidenceChoice struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
}
type FineEvidencePage struct {
	Items    []FineEvidenceChoice `json:"items"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

const fineReportSelect = `SELECT r.id,r.fine_id,f.title,r.flat_id,b.code||'-'||h.flat_number,r.author_id,u.display_name,r.amount_paise,r.payment_date,r.payer,r.method,r.reference,r.comment,COALESCE(r.evidence_id,''),r.state,r.version,r.created_at,r.updated_at,COALESCE(r.entry_id,''),COALESCE(r.allocation_id,''),COALESCE(r.duplicate_report_id,''),COALESCE(v.display_name,''),COALESCE(r.reviewed_at,0),r.decision_reason,COALESCE(rc.id,''),COALESCE(rc.number,''),CASE WHEN EXISTS(SELECT 1 FROM entry_reversals x WHERE x.entry_id=r.entry_id) THEN 'REVERSED' ELSE COALESCE(e.state,'') END FROM fine_reports r JOIN fines f ON f.id=r.fine_id JOIN flats h ON h.id=r.flat_id JOIN buildings b ON b.id=h.building_id JOIN users u ON u.id=r.author_id LEFT JOIN users v ON v.id=r.reviewer_id LEFT JOIN entries e ON e.id=r.entry_id LEFT JOIN receipts rc ON rc.entry_id=e.id `

func scanFineReport(row interface{ Scan(...any) error }) (FineReport, error) {
	var r FineReport
	e := row.Scan(&r.ID, &r.FineID, &r.Fine, &r.FlatID, &r.Home, &r.AuthorID, &r.Author, &r.AmountPaise, &r.PaymentDate, &r.Payer, &r.Method, &r.Reference, &r.Comment, &r.EvidenceID, &r.State, &r.Version, &r.CreatedAt, &r.UpdatedAt, &r.EntryID, &r.AllocationID, &r.DuplicateReportID, &r.Reviewer, &r.ReviewedAt, &r.DecisionReason, &r.ReceiptID, &r.Receipt, &r.CurrentState)
	return r, e
}
func fineReportScope(p Principal) (string, []any) {
	if p.CanReadAllRecords {
		return "1=1", nil
	}
	return `r.author_id=? AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=r.flat_id AND m.resident_id=? AND m.can_view_finances=1 AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`, []any{p.ID, p.ResidentID, today(), today()}
}
func fineReportIn(ctx context.Context, tx *sql.Tx, p Principal, id string) (FineReport, error) {
	if !p.CanReadRecords {
		return FineReport{}, ErrForbidden
	}
	scope, args := fineReportScope(p)
	r, e := scanFineReport(tx.QueryRowContext(ctx, fineReportSelect+"WHERE r.id=? AND "+scope, append([]any{id}, args...)...))
	r.CanDecide = p.CanManageRecords && p.Fresh && r.AuthorID != p.ID
	return r, e
}
func (s *Store) SaveFineReport(ctx context.Context, token, id string, in FineReportInput) (string, error) {
	if len(id) > 100 || len(in.FineID) < 1 || len(in.FineID) > 100 || (id == "" && in.Version != 0) || (id != "" && in.Version < 1) {
		return "", ErrInvalid
	}
	tx, p, e := s.beginFundReportWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", in.FineID))
	if e != nil {
		return "", e
	}
	if !fineFinanceAllowed(ctx, tx, p, x.FlatID) {
		return "", sql.ErrNoRows
	}
	if x.State != "ISSUED" && x.State != "WAIVED" {
		return "", ErrConflict
	}
	amount, e := validateFundReport(FundReportInput{CampaignID: in.FineID, FlatID: x.FlatID, Amount: in.Amount, PaymentDate: in.PaymentDate, Payer: in.Payer, Method: in.Method, Reference: in.Reference, Comment: in.Comment, EvidenceID: in.EvidenceID, Confirmed: in.Confirmed})
	if e != nil {
		return "", e
	}
	if e = fundEvidenceAllowed(ctx, tx, p, x.FlatID, in.EvidenceID); e != nil {
		return "", e
	}
	var previous FineReport
	if id != "" {
		previous, e = fineReportIn(ctx, tx, p, id)
		if e != nil {
			return "", e
		}
		if previous.AuthorID != p.ID {
			return "", sql.ErrNoRows
		}
		if previous.FineID != in.FineID {
			return "", ErrInvalid
		}
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "FINE_REPORT_SAVE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	version, action, now := 1, "REPORTED", time.Now().Unix()
	if id == "" {
		id = randomToken()
		_, e = tx.ExecContext(ctx, `INSERT INTO fine_reports(id,fine_id,flat_id,author_id,amount_paise,payment_date,payer,method,reference,comment,evidence_id,state,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,'PENDING',1,?,?)`, id, x.ID, x.FlatID, p.ID, amount, in.PaymentDate, in.Payer, in.Method, in.Reference, in.Comment, optionalID(in.EvidenceID), now, now)
	} else {
		if previous.Version != in.Version || (previous.State != "NEEDS_INFO" && previous.State != "REJECTED") {
			return "", ErrConflict
		}
		version = previous.Version + 1
		action = "REVISED"
		_, e = tx.ExecContext(ctx, `UPDATE fine_reports SET amount_paise=?,payment_date=?,payer=?,method=?,reference=?,comment=?,evidence_id=?,state='PENDING',version=version+1,updated_at=?,reviewer_id=NULL,reviewed_at=NULL,decision_reason='' WHERE id=?`, amount, in.PaymentDate, in.Payer, in.Method, in.Reference, in.Comment, optionalID(in.EvidenceID), now, id)
	}
	if e != nil {
		return "", e
	}
	if e = fineEvent(ctx, tx, p, x.ID, "REPORT", id, action, "Reported money already paid; no receipt issued", version, in); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) DecideFineReport(ctx context.Context, token, id string, in FundReportDecision) (string, error) {
	if len(id) < 1 || len(id) > 100 || !in.Confirmed || in.Version < 1 || !validText(in.Reason, 10, 300) || (in.Action != "CONFIRMED" && in.Action != "NEEDS_INFO" && in.Action != "REJECTED" && in.Action != "WITHDRAWN") {
		return "", ErrInvalid
	}
	tx, p, e := s.beginFundReportWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	r, e := fineReportIn(ctx, tx, p, id)
	if e != nil {
		return "", e
	}
	if in.Action == "WITHDRAWN" {
		if r.AuthorID != p.ID {
			return "", ErrForbidden
		}
	} else {
		if !p.CanManageRecords || r.AuthorID == p.ID {
			return "", ErrForbidden
		}
		if !p.Fresh {
			return "", ErrReauthRequired
		}
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "FINE_REPORT_ACTION:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if r.Version != in.Version || (r.State != "PENDING" && !(in.Action == "WITHDRAWN" && (r.State == "NEEDS_INFO" || r.State == "REJECTED"))) {
		return "", ErrConflict
	}
	entry, allocation, duplicate, state := "", "", "", in.Action
	if in.Action == "CONFIRMED" {
		amount, e := zeroableFundAmount(in.AllocationAmount)
		if e != nil {
			return "", e
		}
		if amount > r.AmountPaise {
			return "", ErrInvalid
		}
		paid, e := confirmExternalReceived(ctx, tx, p, externalPaidClaim{ID: r.ID, FlatID: r.FlatID, AmountPaise: r.AmountPaise, PaymentDate: r.PaymentDate, Payer: r.Payer, Method: r.Method, Reference: r.Reference, Description: "Fine payment · " + r.Fine}, in)
		if e != nil {
			return "", e
		}
		entry = paid.ID
		e = tx.QueryRowContext(ctx, "SELECT id FROM fine_reports WHERE fine_id=? AND entry_id=? AND state='CONFIRMED' ORDER BY reviewed_at,id LIMIT 1", r.FineID, entry).Scan(&duplicate)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return "", e
		}
		if duplicate != "" {
			state = "DUPLICATE"
		} else if amount > 0 {
			var charge string
			e = tx.QueryRowContext(ctx, "SELECT COALESCE(current_entry_id,'') FROM fines WHERE id=? AND state IN('ISSUED','WAIVED')", r.FineID).Scan(&charge)
			if e != nil {
				return "", e
			}
			if charge == "" {
				return "", invalid("This corrected fine has no live charge. Confirm the payment as available credit.")
			}
			var available, outstanding int64
			e = tx.QueryRowContext(ctx, `SELECT e.amount_paise-COALESCE((SELECT SUM(amount_paise) FROM live_credit_uses WHERE source_id=e.id),0) FROM entries e WHERE e.id=?`, entry).Scan(&available)
			if e != nil {
				return "", e
			}
			e = tx.QueryRowContext(ctx, `SELECT e.amount_paise-COALESCE((SELECT SUM(amount_paise) FROM live_entry_allocations WHERE charge_id=e.id),0) FROM entries e WHERE e.id=? AND e.state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id)`, charge).Scan(&outstanding)
			if e != nil {
				return "", e
			}
			if amount > available || amount > outstanding {
				return "", fmt.Errorf("%w: usable credit or the fine's outstanding amount changed", ErrConflict)
			}
			allocation = randomToken()
			_, e = tx.ExecContext(ctx, "INSERT INTO entry_allocations VALUES(?,?,?,?,?,?,?)", allocation, entry, charge, amount, p.ID, in.Reason, time.Now().Unix())
			if e != nil {
				return "", e
			}
			if e = appendAudit(ctx, tx, p.ID, r.FlatID, "FINE_CREDIT_ALLOCATED", in.Reason, map[string]string{"source_id": entry}, map[string]any{"allocation_id": allocation, "amount_paise": amount}); e != nil {
				return "", e
			}
		}
	} else if in.VerificationSource != "" || in.PaymentIdentity != "" || in.Mode != "" || in.EntryID != "" || in.AllocationAmount != "" {
		return "", ErrInvalid
	}
	now := time.Now().Unix()
	_, e = tx.ExecContext(ctx, `UPDATE fine_reports SET state=?,version=version+1,updated_at=?,entry_id=?,allocation_id=?,duplicate_report_id=?,reviewer_id=?,reviewed_at=?,decision_reason=? WHERE id=?`, state, now, optionalID(entry), optionalID(allocation), optionalID(duplicate), p.ID, now, in.Reason, id)
	if e != nil {
		return "", e
	}
	if e = fineEvent(ctx, tx, p, r.FineID, "REPORT", id, state, in.Reason, r.Version+1, in); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) FineReportFor(ctx context.Context, token, id string, page int) (FineReport, error) {
	out := FineReport{PageSize: 20}
	if len(id) < 1 || len(id) > 100 || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	out, e = fineReportIn(ctx, tx, p, id)
	if e != nil {
		return out, e
	}
	out.PageSize = 20
	out.Events, out.EventTotal, out.EventPage, e = fineEvents(ctx, tx, "REPORT", id, page)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func (s *Store) FineReportsFor(ctx context.Context, token, fine, state string, page int) (FineReportPage, error) {
	out := FineReportPage{Items: []FineReport{}, Page: page, PageSize: 12, Counts: map[string]int64{}}
	if len(fine) > 100 || !boundedMaintenancePage(page) || (state != "" && state != "PENDING" && state != "NEEDS_INFO" && state != "REJECTED" && state != "WITHDRAWN" && state != "CONFIRMED" && state != "DUPLICATE") {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if !p.CanReadRecords {
		return out, ErrForbidden
	}
	scope, args := fineReportScope(p)
	if fine != "" {
		scope += " AND r.fine_id=?"
		args = append(args, fine)
	}
	rows, e := tx.QueryContext(ctx, "SELECT r.state,COUNT(*) FROM fine_reports r WHERE "+scope+" GROUP BY r.state", args...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var k string
		var count int64
		if e = rows.Scan(&k, &count); e != nil {
			rows.Close()
			return out, e
		}
		out.Counts[k] = count
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if state != "" {
		scope += " AND r.state=?"
		args = append(args, state)
	}
	e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fine_reports r WHERE "+scope, args...).Scan(&out.Total)
	if e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	rows, e = tx.QueryContext(ctx, fineReportSelect+"WHERE "+scope+" ORDER BY r.updated_at DESC,r.id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		r, err := scanFineReport(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		r.CanDecide = p.CanManageRecords && p.Fresh && r.AuthorID != p.ID
		out.Items = append(out.Items, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
