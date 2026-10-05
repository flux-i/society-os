package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type FundInput struct {
	OperationKey     string                 `json:"operation_key"`
	Title            string                 `json:"title"`
	Purpose          string                 `json:"purpose"`
	ContributionType string                 `json:"contribution_type"`
	StartDate        string                 `json:"start_date"`
	DueDate          string                 `json:"due_date"`
	Target           string                 `json:"target"`
	SourceReference  string                 `json:"source_reference"`
	Note             string                 `json:"note"`
	Lines            []MaintenanceLineInput `json:"lines"`
	Confirmed        bool                   `json:"confirmed"`
}
type FundAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type FundTotals struct {
	RequestedPaise   int64 `json:"requested_paise"`
	ActivePaise      int64 `json:"active_paise"`
	AllocatedPaise   int64 `json:"allocated_paise"`
	OutstandingPaise int64 `json:"outstanding_paise"`
	OverduePaise     int64 `json:"overdue_paise"`
	WaivedPaise      int64 `json:"waived_paise"`
	VoluntaryPaise   int64 `json:"voluntary_paise"`
	PendingReports   int   `json:"pending_reports"`
	PaidHomes        int   `json:"paid_homes"`
	PartialHomes     int   `json:"partial_homes"`
	UnpaidHomes      int   `json:"unpaid_homes"`
	ExemptHomes      int   `json:"exempt_homes"`
}
type FundCampaign struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Purpose          string `json:"purpose"`
	ContributionType string `json:"contribution_type"`
	StartDate        string `json:"start_date"`
	DueDate          string `json:"due_date"`
	TargetPaise      int64  `json:"target_paise"`
	SourceReference  string `json:"source_reference,omitempty"`
	Note             string `json:"note,omitempty"`
	State            string `json:"state"`
	Version          int    `json:"version"`
	AuthorID         string `json:"author_id,omitempty"`
	Author           string `json:"author,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	DecidedBy        string `json:"decided_by,omitempty"`
	DecidedAt        int64  `json:"decided_at"`
	DecisionReason   string `json:"decision_reason,omitempty"`
	Participants     int    `json:"participants"`
	FundTotals
}
type FundParticipant struct {
	FlatID           string `json:"flat_id"`
	Home             string `json:"home"`
	RequestedPaise   int64  `json:"requested_paise"`
	ActivePaise      int64  `json:"active_paise"`
	AllocatedPaise   int64  `json:"allocated_paise"`
	OutstandingPaise int64  `json:"outstanding_paise"`
	WaivedPaise      int64  `json:"waived_paise"`
	VoluntaryPaise   int64  `json:"voluntary_paise"`
	PendingReports   int    `json:"pending_reports"`
	EntryID          string `json:"entry_id"`
	OriginalEntryID  string `json:"original_entry_id"`
	Version          int    `json:"version"`
	Status           string `json:"status"`
}
type FundEvent struct {
	Action  string `json:"action"`
	Actor   string `json:"actor"`
	Reason  string `json:"reason"`
	Version int    `json:"version"`
	At      int64  `json:"at"`
}
type FundDetail struct {
	FundCampaign
	Lines      []FundParticipant `json:"lines"`
	LinePage   int               `json:"line_page"`
	PageSize   int               `json:"page_size"`
	Events     []FundEvent       `json:"events,omitempty"`
	EventTotal int               `json:"event_total,omitempty"`
	EventPage  int               `json:"event_page,omitempty"`
}
type FundPage struct {
	Items    []FundCampaign `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Homes    []RecordHome   `json:"homes"`
	Totals   FundTotals     `json:"totals"`
}

func fundEvent(ctx context.Context, tx *sql.Tx, p Principal, campaign, kind, id, action, reason string, version int, snapshot any) error {
	blob, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO fund_events(campaign_id,subject_type,subject_id,version,action,actor_id,reason,snapshot_json,occurred_at) VALUES(?,?,?,?,?,?,?,?,?)", campaign, kind, id, version, action, p.ID, reason, string(blob), time.Now().Unix())
	return err
}
func validateFund(in FundInput) ([]int64, int64, error) {
	if !in.Confirmed || !validText(in.Title, 5, 120) || !paragraph(in.Purpose, 10, 2000) || !validText(in.SourceReference, 5, 300) || !paragraph(in.Note, 0, 2000) || !validMaintenanceDate(in.StartDate) || !validMaintenanceDate(in.DueDate) || in.DueDate < in.StartDate || (in.ContributionType != "FIXED" && in.ContributionType != "VOLUNTARY") {
		return nil, 0, invalid("Supply the approved purpose, type, dates, source and reviewed participants.")
	}
	if len(in.Lines) < 1 || len(in.Lines) > 500 {
		return nil, 0, invalid("Select between one and 500 distinct participating homes.")
	}
	amounts, seen := make([]int64, len(in.Lines)), map[string]bool{}
	for i, line := range in.Lines {
		if line.FlatID == "" || len(line.FlatID) > 100 || seen[line.FlatID] {
			return nil, 0, invalid("Include each participating home once.")
		}
		seen[line.FlatID] = true
		if in.ContributionType == "VOLUNTARY" {
			if line.Amount != "" {
				return nil, 0, invalid("Voluntary participation has no requested debt.")
			}
		} else {
			amount, err := ParseAmount(line.Amount)
			if err != nil {
				return nil, 0, err
			}
			amounts[i] = amount
		}
	}
	var target int64
	if in.Target != "" {
		var err error
		target, err = ParseAmount(in.Target)
		if err != nil {
			return nil, 0, err
		}
	}
	return amounts, target, nil
}
func (s *Store) CreateFundCampaign(ctx context.Context, token string, in FundInput) (string, error) {
	amounts, target, err := validateFund(in)
	if err != nil {
		return "", err
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "FUND_SUBMIT", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	id, now := randomToken(), time.Now().Unix()
	_, err = tx.ExecContext(ctx, `INSERT INTO fund_campaigns(id,title,purpose,contribution_type,start_date,due_date,target_paise,source_reference,note,state,version,author_id,created_at) VALUES(?,?,?,?,?,?,?,?,?,'PENDING',1,?,?)`, id, in.Title, in.Purpose, in.ContributionType, in.StartDate, in.DueDate, target, in.SourceReference, in.Note, p.ID, now)
	if err != nil {
		return "", err
	}
	for i, line := range in.Lines {
		var exists bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM flats WHERE id=?)", line.FlatID).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return "", invalid("A participant is unavailable. Reload the homes.")
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO fund_participants(campaign_id,flat_id,requested_paise) VALUES(?,?,?)", id, line.FlatID, amounts[i]); err != nil {
			return "", err
		}
	}
	if err = fundEvent(ctx, tx, p, id, "CAMPAIGN", id, "SUBMITTED", in.SourceReference, 1, in); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, "", "FUND_SUBMITTED", in.SourceReference, map[string]any{}, map[string]any{"campaign_id": id, "participants": len(in.Lines)}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) DecideFundCampaign(ctx context.Context, token, id string, in FundAction) (string, error) {
	if !in.Confirmed || in.Version < 1 || !validText(in.Reason, 10, 300) || (in.Action != "PUBLISHED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN" && in.Action != "CLOSED" && in.Action != "REOPEN") {
		return "", invalid("Review this decision and supply a reason of 10–300 characters.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "FUND_DECIDE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	var author, state, title, date, kind string
	var version int
	var created int64
	err = tx.QueryRowContext(ctx, "SELECT author_id,state,version,title,start_date,contribution_type,created_at FROM fund_campaigns WHERE id=?", id).Scan(&author, &state, &version, &title, &date, &kind, &created)
	if err != nil {
		return "", err
	}
	if version != in.Version {
		return "", ErrConflict
	}
	next := in.Action
	if in.Action == "REOPEN" {
		next = "PUBLISHED"
		if state != "CLOSED" {
			return "", ErrConflict
		}
	} else if in.Action == "CLOSED" {
		if state != "PUBLISHED" {
			return "", ErrConflict
		}
	} else {
		if state != "PENDING" {
			return "", ErrConflict
		}
		if (in.Action == "WITHDRAWN" && p.ID != author) || (in.Action != "WITHDRAWN" && p.ID == author) {
			return "", ErrForbidden
		}
	}
	now := time.Now().Unix()
	if state == "PENDING" && next == "PUBLISHED" && kind == "FIXED" {
		rows, e := tx.QueryContext(ctx, "SELECT flat_id,requested_paise FROM fund_participants WHERE campaign_id=? ORDER BY flat_id", id)
		if e != nil {
			return "", e
		}
		type charge struct {
			home   string
			amount int64
		}
		lines := []charge{}
		for rows.Next() {
			var c charge
			if e = rows.Scan(&c.home, &c.amount); e != nil {
				rows.Close()
				return "", e
			}
			lines = append(lines, c)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return "", e
		}
		for _, c := range lines {
			entry := randomToken()
			_, e = tx.ExecContext(ctx, `INSERT INTO entries(id,flat_id,kind,amount_paise,entry_date,description,payer,method,reference,source_note,state,created_by,created_at,posted_by,posted_at) VALUES(?,?,'CHARGE',?,?,?,'','','','Separately reviewed supplied fund contribution','POSTED',?,?,?,?)`, entry, c.home, c.amount, date, "Fund · "+title, author, created, p.ID, now)
			if e != nil {
				return "", e
			}
			if _, e = tx.ExecContext(ctx, "UPDATE fund_participants SET original_entry_id=?,current_entry_id=? WHERE campaign_id=? AND flat_id=?", entry, entry, id, c.home); e != nil {
				return "", e
			}
			if e = appendAudit(ctx, tx, p.ID, c.home, "ENTRY_POSTED", "Separately reviewed supplied fund charge", map[string]any{"campaign_id": id}, map[string]any{"campaign_id": id, "entry_id": entry, "amount_paise": c.amount}); e != nil {
				return "", e
			}
		}
	}
	if state == "PENDING" {
		_, err = tx.ExecContext(ctx, "UPDATE fund_campaigns SET state=?,version=version+1,decided_by=?,decided_at=?,decision_reason=? WHERE id=? AND version=?", next, p.ID, now, in.Reason, id, version)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE fund_campaigns SET state=?,version=version+1 WHERE id=? AND version=?", next, id, version)
	}
	if err != nil {
		return "", err
	}
	if err = fundEvent(ctx, tx, p, id, "CAMPAIGN", id, in.Action, in.Reason, version+1, in); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, "", "FUND_"+in.Action, in.Reason, map[string]any{"campaign_id": id, "state": state}, map[string]any{"campaign_id": id, "state": next}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
