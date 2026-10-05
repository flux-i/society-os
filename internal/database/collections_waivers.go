package database

import (
	"context"
	"time"
)

type FundWaiverInput struct {
	OperationKey       string `json:"operation_key"`
	CampaignID         string `json:"campaign_id"`
	FlatID             string `json:"flat_id"`
	ParticipantVersion int    `json:"participant_version"`
	Amount             string `json:"amount"`
	SourceReference    string `json:"source_reference"`
	Reason             string `json:"reason"`
	Confirmed          bool   `json:"confirmed"`
}
type FundWaiver struct {
	ID                 string      `json:"id"`
	CampaignID         string      `json:"campaign_id"`
	Campaign           string      `json:"campaign"`
	FlatID             string      `json:"flat_id"`
	Home               string      `json:"home"`
	ParticipantVersion int         `json:"participant_version"`
	AmountPaise        int64       `json:"amount_paise"`
	SourceReference    string      `json:"source_reference"`
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
	Events             []FundEvent `json:"events"`
	EventTotal         int         `json:"event_total"`
	EventPage          int         `json:"event_page"`
	PageSize           int         `json:"page_size"`
}
type FundWaiverPage struct {
	Items    []FundWaiver `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

func (s *Store) ProposeFundWaiver(ctx context.Context, token string, in FundWaiverInput) (string, error) {
	amount, err := ParseAmount(in.Amount)
	if err != nil {
		return "", err
	}
	if !in.Confirmed || in.CampaignID == "" || len(in.CampaignID) > 100 || in.FlatID == "" || len(in.FlatID) > 100 || in.ParticipantVersion < 1 || !validText(in.SourceReference, 5, 300) || !validText(in.Reason, 10, 300) {
		return "", invalid("Supply the explicit exemption source, amount, participant version and reviewed reason.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "FUND_WAIVER_SUBMIT", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	var remaining int64
	var version int
	err = tx.QueryRowContext(ctx, `SELECT e.amount_paise,l.version FROM fund_participants l JOIN fund_campaigns c ON c.id=l.campaign_id JOIN entries e ON e.id=l.current_entry_id WHERE l.campaign_id=? AND l.flat_id=? AND c.contribution_type='FIXED' AND c.state IN ('PUBLISHED','CLOSED') AND e.state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id)`, in.CampaignID, in.FlatID).Scan(&remaining, &version)
	if err != nil {
		return "", err
	}
	if version != in.ParticipantVersion {
		return "", ErrConflict
	}
	if amount > remaining {
		return "", invalid("An exemption cannot exceed the current supplied charge.")
	}
	id, now := randomToken(), time.Now().Unix()
	_, err = tx.ExecContext(ctx, `INSERT INTO fund_waivers(id,campaign_id,flat_id,participant_version,amount_paise,source_reference,reason,author_id,state,version,created_at) VALUES(?,?,?,?,?,?,?,?,'PENDING',1,?)`, id, in.CampaignID, in.FlatID, version, amount, in.SourceReference, in.Reason, p.ID, now)
	if err != nil {
		return "", err
	}
	if err = fundEvent(ctx, tx, p, in.CampaignID, "WAIVER", id, "SUBMITTED", in.Reason, 1, in); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, in.FlatID, "FUND_WAIVER_SUBMITTED", in.Reason, map[string]any{}, map[string]any{"waiver_id": id, "amount_paise": amount}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

const waiverSelect = `SELECT w.id,w.campaign_id,c.title,w.flat_id,b.code||'-'||f.flat_number,w.participant_version,w.amount_paise,w.source_reference,w.reason,w.author_id,u.display_name,w.state,w.version,w.created_at,COALESCE(v.display_name,''),COALESCE(w.reviewed_at,0),w.decision_reason,COALESCE(w.replacement_entry_id,'') FROM fund_waivers w JOIN fund_campaigns c ON c.id=w.campaign_id JOIN flats f ON f.id=w.flat_id JOIN buildings b ON b.id=f.building_id JOIN users u ON u.id=w.author_id LEFT JOIN users v ON v.id=w.reviewer_id `

func scanFundWaiver(row interface{ Scan(...any) error }) (FundWaiver, error) {
	var w FundWaiver
	err := row.Scan(&w.ID, &w.CampaignID, &w.Campaign, &w.FlatID, &w.Home, &w.ParticipantVersion, &w.AmountPaise, &w.SourceReference, &w.Reason, &w.AuthorID, &w.Author, &w.State, &w.Version, &w.CreatedAt, &w.Reviewer, &w.ReviewedAt, &w.DecisionReason, &w.ReplacementEntryID)
	return w, err
}
func (s *Store) FundWaiverFor(ctx context.Context, token, id string, eventPage int) (FundWaiver, error) {
	if id == "" || len(id) > 100 || !boundedMaintenancePage(eventPage) {
		return FundWaiver{}, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return FundWaiver{}, err
	}
	defer tx.Rollback()
	if !p.CanReadAllRecords {
		return FundWaiver{}, ErrForbidden
	}
	out, err := scanFundWaiver(tx.QueryRowContext(ctx, waiverSelect+"WHERE w.id=?", id))
	if err != nil {
		return out, err
	}
	out.PageSize = 20
	out.Events, out.EventTotal, out.EventPage, err = fundEvents(ctx, tx, "WAIVER", id, eventPage)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) FundWaiversFor(ctx context.Context, token, campaign, state string, page int) (FundWaiverPage, error) {
	out := FundWaiverPage{Items: []FundWaiver{}, PageSize: 12, Page: page}
	if len(campaign) > 100 || !boundedMaintenancePage(page) || (state != "" && state != "PENDING" && state != "APPROVED" && state != "DECLINED" && state != "WITHDRAWN") {
		return out, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if !p.CanReadAllRecords {
		return out, ErrForbidden
	}
	where := "WHERE (?='' OR w.campaign_id=?) AND (?='' OR w.state=?) "
	args := []any{campaign, campaign, state, state}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fund_waivers w "+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.Page = clampMaintenancePage(page, out.Total, out.PageSize)
	args = append(args, out.PageSize, (out.Page-1)*out.PageSize)
	rows, err := tx.QueryContext(ctx, waiverSelect+where+"ORDER BY w.created_at DESC,w.id LIMIT ? OFFSET ?", args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		w, e := scanFundWaiver(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, w)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) DecideFundWaiver(ctx context.Context, token, id string, in FundAction) (string, error) {
	if !in.Confirmed || in.Version < 1 || !validText(in.Reason, 10, 300) || (in.Action != "APPROVED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN") {
		return "", ErrInvalid
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	w, err := scanFundWaiver(tx.QueryRowContext(ctx, waiverSelect+"WHERE w.id=?", id))
	if err != nil {
		return "", err
	}
	if (in.Action == "WITHDRAWN" && p.ID != w.AuthorID) || (in.Action != "WITHDRAWN" && p.ID == w.AuthorID) {
		return "", ErrForbidden
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "FUND_WAIVER_DECIDE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	if w.State != "PENDING" || w.Version != in.Version {
		return "", ErrConflict
	}
	replacement := ""
	now := time.Now().Unix()
	if in.Action == "APPROVED" {
		var original string
		var version int
		var requested, waived int64
		err = tx.QueryRowContext(ctx, "SELECT COALESCE(current_entry_id,''),version,requested_paise,waived_paise FROM fund_participants WHERE campaign_id=? AND flat_id=?", w.CampaignID, w.FlatID).Scan(&original, &version, &requested, &waived)
		if err != nil {
			return "", err
		}
		if version != w.ParticipantVersion {
			return "", ErrConflict
		}
		e, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", original))
		if err != nil {
			return "", err
		}
		if e.State != "POSTED" || e.Kind != "CHARGE" || e.AmountPaise != requested-waived || w.AmountPaise > e.AmountPaise {
			return "", ErrConflict
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO entry_reversals VALUES(?,?,?,?)", e.ID, "Separately reviewed fund exemption: "+in.Reason, p.ID, now); err != nil {
			return "", err
		}
		if e.AmountPaise > w.AmountPaise {
			replacement = randomToken()
			_, err = tx.ExecContext(ctx, `INSERT INTO entries(id,flat_id,kind,amount_paise,entry_date,description,payer,method,reference,source_note,state,created_by,created_at,posted_by,posted_at) VALUES(?,?,'CHARGE',?,?,?,'','','','Separately reviewed fund exemption; original charge retained','POSTED',?,?,?,?)`, replacement, w.FlatID, e.AmountPaise-w.AmountPaise, e.Date, "Fund · "+w.Campaign, w.AuthorID, w.CreatedAt, p.ID, now)
			if err != nil {
				return "", err
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE fund_participants SET current_entry_id=?,waived_paise=waived_paise+?,version=version+1 WHERE campaign_id=? AND flat_id=? AND version=?", optionalID(replacement), w.AmountPaise, w.CampaignID, w.FlatID, version); err != nil {
			return "", err
		}
		if err = appendAudit(ctx, tx, p.ID, w.FlatID, "FUND_CHARGE_CORRECTED", in.Reason, map[string]any{"entry_id": original}, map[string]any{"waiver_id": id, "replacement_entry_id": replacement, "waived_paise": w.AmountPaise}); err != nil {
			return "", err
		}
	}
	_, err = tx.ExecContext(ctx, "UPDATE fund_waivers SET state=?,version=version+1,reviewer_id=?,reviewed_at=?,decision_reason=?,replacement_entry_id=? WHERE id=? AND version=?", in.Action, p.ID, now, in.Reason, optionalID(replacement), id, w.Version)
	if err != nil {
		return "", err
	}
	if err = fundEvent(ctx, tx, p, w.CampaignID, "WAIVER", id, in.Action, in.Reason, w.Version+1, in); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, w.FlatID, "FUND_WAIVER_"+in.Action, in.Reason, map[string]any{"waiver_id": id, "state": w.State}, map[string]any{"waiver_id": id, "state": in.Action}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
