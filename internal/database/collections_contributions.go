package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type FundAttributionInput struct {
	OperationKey string `json:"operation_key"`
	CampaignID   string `json:"campaign_id"`
	FlatID       string `json:"flat_id"`
	SourceID     string `json:"source_id"`
	Amount       string `json:"amount"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type FundContribution struct {
	ID               string `json:"id"`
	CampaignID       string `json:"campaign_id"`
	Campaign         string `json:"campaign"`
	FlatID           string `json:"flat_id"`
	Home             string `json:"home"`
	SourceID         string `json:"source_id"`
	ReceiptID        string `json:"receipt_id"`
	Receipt          string `json:"receipt"`
	AmountPaise      int64  `json:"amount_paise"`
	State            string `json:"state"`
	Actor            string `json:"actor,omitempty"`
	Reason           string `json:"reason,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	CorrectionReason string `json:"correction_reason,omitempty"`
	CorrectedBy      string `json:"corrected_by,omitempty"`
	CorrectedAt      int64  `json:"corrected_at"`
}
type FundContributionPage struct {
	Items    []FundContribution `json:"items"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

func (s *Store) AttributeFundCredit(ctx context.Context, token string, in FundAttributionInput) (string, error) {
	amount, err := ParseAmount(in.Amount)
	if err != nil {
		return "", err
	}
	if !in.Confirmed || in.CampaignID == "" || len(in.CampaignID) > 100 || in.FlatID == "" || len(in.FlatID) > 100 || in.SourceID == "" || len(in.SourceID) > 100 || !validText(in.Reason, 10, 300) {
		return "", invalid("Select a confirmed same-home receipt, review its available credit and supply a purpose reason.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "FUND_ATTRIBUTE", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	var allowed bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM fund_campaigns c JOIN fund_participants l ON l.campaign_id=c.id WHERE c.id=? AND l.flat_id=? AND c.contribution_type='VOLUNTARY' AND c.state IN ('PUBLISHED','CLOSED'))`, in.CampaignID, in.FlatID).Scan(&allowed); err != nil {
		return "", err
	}
	if !allowed {
		return "", invalid("Choose a published voluntary fund and its participating home.")
	}
	e, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", in.SourceID))
	if err != nil {
		return "", err
	}
	if e.FlatID != in.FlatID || e.Kind != "RECEIVED" || e.State != "POSTED" || e.ReceiptID == "" {
		return "", ErrInvalid
	}
	var used int64
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount_paise),0) FROM live_credit_uses WHERE source_id=?", e.ID).Scan(&used); err != nil {
		return "", err
	}
	if amount > e.AmountPaise-used {
		return "", fmt.Errorf("%w: usable receipt credit changed; reload before attributing", ErrConflict)
	}
	id := randomToken()
	if _, err = tx.ExecContext(ctx, "INSERT INTO fund_contributions VALUES(?,?,?,?,?,?,?,?)", id, in.CampaignID, in.FlatID, e.ID, amount, p.ID, in.Reason, time.Now().Unix()); err != nil {
		return "", err
	}
	if err = fundEvent(ctx, tx, p, in.CampaignID, "CONTRIBUTION", id, "ATTRIBUTED", in.Reason, 1, in); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, in.FlatID, "FUND_CREDIT_ATTRIBUTED", in.Reason, map[string]any{"source_id": e.ID}, map[string]any{"contribution_id": id, "amount_paise": amount}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) CorrectFundContribution(ctx context.Context, token, id string, in AllocationCorrection) (string, error) {
	if !in.Confirmed || !validText(in.Reason, 10, 300) || id == "" || len(id) > 100 {
		return "", ErrInvalid
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "FUND_ATTRIBUTE_CORRECT:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	var campaign, home string
	var corrected bool
	if err = tx.QueryRowContext(ctx, "SELECT campaign_id,flat_id,EXISTS(SELECT 1 FROM fund_contribution_reversals r WHERE r.contribution_id=c.id) FROM fund_contributions c WHERE c.id=?", id).Scan(&campaign, &home, &corrected); err != nil {
		return "", err
	}
	if corrected {
		return "", ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO fund_contribution_reversals VALUES(?,?,?,?)", id, p.ID, in.Reason, time.Now().Unix()); err != nil {
		return "", err
	}
	if err = fundEvent(ctx, tx, p, campaign, "CONTRIBUTION", id, "CORRECTED", in.Reason, 2, in); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, home, "FUND_ATTRIBUTION_CORRECTED", in.Reason, map[string]any{"contribution_id": id}, map[string]any{"contribution_id": id, "corrected": true}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) FundContributionsFor(ctx context.Context, token, campaign, home string, page int) (FundContributionPage, error) {
	out := FundContributionPage{Items: []FundContribution{}, PageSize: 20, Page: page}
	if len(campaign) > 100 || len(home) > 100 || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if home != "" {
		if _, err = permittedFinancialHome(ctx, tx, p, home); err != nil {
			return out, err
		}
	}
	args := append(fundScopeArgs(p, home), sql.Named("campaign", campaign))
	base := `FROM fund_contributions fc JOIN fund_campaigns c ON c.id=fc.campaign_id JOIN fund_participants l ON l.campaign_id=fc.campaign_id AND l.flat_id=fc.flat_id JOIN flats f ON f.id=fc.flat_id JOIN buildings b ON b.id=f.building_id JOIN entries e ON e.id=fc.source_id JOIN receipts rc ON rc.entry_id=e.id LEFT JOIN fund_contribution_reversals r ON r.contribution_id=fc.id JOIN users u ON u.id=fc.actor_id LEFT JOIN users v ON v.id=r.actor_id `
	where := `WHERE ` + fundHomeScope + ` AND c.state IN ('PUBLISHED','CLOSED') AND (:campaign='' OR c.id=:campaign) AND (:home='' OR fc.flat_id=:home) `
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) "+base+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.Page = clampMaintenancePage(page, out.Total, out.PageSize)
	args = append(args, sql.Named("limit", out.PageSize), sql.Named("offset", (out.Page-1)*out.PageSize))
	rows, err := tx.QueryContext(ctx, `SELECT fc.id,fc.campaign_id,c.title,fc.flat_id,b.code||'-'||f.flat_number,fc.source_id,rc.id,rc.number,fc.amount_paise,CASE WHEN r.contribution_id IS NOT NULL THEN 'CORRECTED' WHEN EXISTS(SELECT 1 FROM entry_reversals x WHERE x.entry_id=e.id) THEN 'SOURCE_REVERSED' ELSE 'ACTIVE' END,u.display_name,fc.reason,fc.created_at,COALESCE(r.reason,''),COALESCE(v.display_name,''),COALESCE(r.created_at,0) `+base+where+`ORDER BY fc.created_at DESC,fc.id LIMIT :limit OFFSET :offset`, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var c FundContribution
		if err = rows.Scan(&c.ID, &c.CampaignID, &c.Campaign, &c.FlatID, &c.Home, &c.SourceID, &c.ReceiptID, &c.Receipt, &c.AmountPaise, &c.State, &c.Actor, &c.Reason, &c.CreatedAt, &c.CorrectionReason, &c.CorrectedBy, &c.CorrectedAt); err != nil {
			rows.Close()
			return out, err
		}
		if !p.CanReadAllRecords {
			c.Actor, c.Reason, c.CorrectionReason, c.CorrectedBy = "", "", "", ""
		}
		out.Items = append(out.Items, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
