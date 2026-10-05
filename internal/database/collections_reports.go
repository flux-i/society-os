package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type FundReportInput struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	CampaignID   string `json:"campaign_id"`
	FlatID       string `json:"flat_id"`
	Amount       string `json:"amount"`
	PaymentDate  string `json:"payment_date"`
	Payer        string `json:"payer"`
	Method       string `json:"method"`
	Reference    string `json:"reference"`
	Comment      string `json:"comment"`
	EvidenceID   string `json:"evidence_id"`
	Confirmed    bool   `json:"confirmed"`
}
type FundReportDecision struct {
	OperationKey       string `json:"operation_key"`
	Version            int    `json:"version"`
	Action             string `json:"action"`
	Reason             string `json:"reason"`
	VerificationSource string `json:"verification_source"`
	PaymentIdentity    string `json:"payment_identity"`
	Mode               string `json:"mode"`
	EntryID            string `json:"entry_id"`
	AllocationAmount   string `json:"allocation_amount"`
	Confirmed          bool   `json:"confirmed"`
}
type FundReport struct {
	ID                string      `json:"id"`
	CampaignID        string      `json:"campaign_id"`
	Campaign          string      `json:"campaign"`
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
	Events            []FundEvent `json:"events"`
	EventTotal        int         `json:"event_total"`
	EventPage         int         `json:"event_page"`
	PageSize          int         `json:"page_size"`
}
type FundReportPage struct {
	Items    []FundReport     `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Counts   map[string]int64 `json:"counts"`
	Homes    []RecordHome     `json:"homes"`
}
type FundReceiptChoice struct {
	StatementEntry
	VerificationSource string `json:"verification_source,omitempty"`
	PaymentIdentity    string `json:"payment_identity,omitempty"`
}
type FundConfirmOptions struct {
	Entries          []FundReceiptChoice `json:"entries"`
	ChargeID         string              `json:"charge_id"`
	OutstandingPaise int64               `json:"outstanding_paise"`
	ContributionType string              `json:"contribution_type"`
	CampaignState    string              `json:"campaign_state"`
}

func (s *Store) beginFundReportWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at=last_seen_at WHERE token_hash=?", TokenHash(token)); err != nil {
		tx.Rollback()
		return nil, Principal{}, err
	}
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err == nil && !p.CanReadRecords {
		err = ErrForbidden
	}
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func validateFundReport(in FundReportInput) (int64, error) {
	amount, err := ParseAmount(in.Amount)
	if err != nil {
		return 0, err
	}
	if !in.Confirmed || in.CampaignID == "" || len(in.CampaignID) > 100 || in.FlatID == "" || len(in.FlatID) > 100 || len(in.EvidenceID) > 100 || !validDate(in.PaymentDate) || !validText(in.Payer, 2, 120) || !validText(in.Reference, 0, 120) || !paragraph(in.Comment, 0, 2000) || (in.Method != "CASH" && in.Method != "BANK_TRANSFER" && in.Method != "CHEQUE" && in.Method != "UPI") || (in.Method != "CASH" && in.Reference == "") {
		return 0, invalid("Supply the already-paid amount, date, payer, method and reference, then review the claim.")
	}
	return amount, nil
}
func fundEvidenceAllowed(ctx context.Context, tx *sql.Tx, p Principal, home, id string) error {
	if id == "" {
		return nil
	}
	var allowed bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM library_documents x JOIN document_groups g ON g.id=x.group_id WHERE x.id=? AND x.uploaded_by=? AND x.flat_id=? AND x.category='PAYMENT_EVIDENCE' AND x.visibility='FLAT_SPECIFIC' AND x.validation_status='AVAILABLE' AND x.review_state IN ('PENDING','APPROVED') AND g.archived=0)`, id, p.ID, home).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return invalid("Select your validated payment evidence for this home. Unchecked or removed originals cannot support a report.")
	}
	return nil
}
func (s *Store) SaveFundReport(ctx context.Context, token, id string, in FundReportInput) (string, error) {
	amount, err := validateFundReport(in)
	if err != nil {
		return "", err
	}
	tx, p, err := s.beginFundReportWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = permittedFinancialHome(ctx, tx, p, in.FlatID); err != nil {
		return "", err
	}
	var campaignState string
	err = tx.QueryRowContext(ctx, "SELECT c.state FROM fund_campaigns c JOIN fund_participants l ON l.campaign_id=c.id WHERE c.id=? AND l.flat_id=?", in.CampaignID, in.FlatID).Scan(&campaignState)
	if err != nil {
		return "", err
	}

	if id != "" && campaignState != "PUBLISHED" && campaignState != "CLOSED" {
		return "", ErrConflict
	}
	if err = fundEvidenceAllowed(ctx, tx, p, in.FlatID, in.EvidenceID); err != nil {
		return "", err
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "FUND_REPORT_SAVE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	if id == "" && campaignState != "PUBLISHED" {
		return "", invalid("New reports require an open published campaign.")
	}
	now, version, action := time.Now().Unix(), 1, "REPORTED"
	if id == "" {
		if in.Version != 0 {
			return "", ErrInvalid
		}
		id = randomToken()
		_, err = tx.ExecContext(ctx, `INSERT INTO fund_reports(id,campaign_id,flat_id,author_id,amount_paise,payment_date,payer,method,reference,comment,evidence_id,state,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,'PENDING',1,?,?)`, id, in.CampaignID, in.FlatID, p.ID, amount, in.PaymentDate, in.Payer, in.Method, in.Reference, in.Comment, optionalID(in.EvidenceID), now, now)
	} else {
		var author, oldCampaign, home, state string
		var oldVersion int
		err = tx.QueryRowContext(ctx, "SELECT author_id,campaign_id,flat_id,state,version FROM fund_reports WHERE id=?", id).Scan(&author, &oldCampaign, &home, &state, &oldVersion)
		if err != nil {
			return "", err
		}
		if author != p.ID {
			return "", sql.ErrNoRows
		}
		if oldCampaign != in.CampaignID || home != in.FlatID {
			return "", ErrInvalid
		}
		if (state != "NEEDS_INFO" && state != "REJECTED") || oldVersion != in.Version {
			return "", ErrConflict
		}
		version, action = oldVersion+1, "REVISED"
		_, err = tx.ExecContext(ctx, `UPDATE fund_reports SET amount_paise=?,payment_date=?,payer=?,method=?,reference=?,comment=?,evidence_id=?,state='PENDING',version=version+1,updated_at=?,reviewer_id=NULL,reviewed_at=NULL,decision_reason='' WHERE id=? AND version=?`, amount, in.PaymentDate, in.Payer, in.Method, in.Reference, in.Comment, optionalID(in.EvidenceID), now, id, oldVersion)
	}
	if err != nil {
		return "", err
	}
	if err = fundEvent(ctx, tx, p, in.CampaignID, "REPORT", id, action, "Submitted already-paid claim; no receipt issued", version, in); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, in.FlatID, "FUND_PAYMENT_"+action, "Submitted claim for externally paid money", map[string]any{}, map[string]any{"campaign_id": in.CampaignID, "report_id": id, "amount_paise": amount}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func zeroableFundAmount(value string) (int64, error) {
	if value == "0" || value == "0.00" {
		return 0, nil
	}
	return ParseAmount(value)
}
func (s *Store) DecideFundReport(ctx context.Context, token, id string, in FundReportDecision) (string, error) {
	if !in.Confirmed || in.Version < 1 || !validText(in.Reason, 10, 300) || (in.Action != "CONFIRMED" && in.Action != "NEEDS_INFO" && in.Action != "REJECTED" && in.Action != "WITHDRAWN") {
		return "", invalid("Choose a report decision, review it and record a reason.")
	}
	tx, p, err := s.beginFundReportWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	report, err := fundReportIn(ctx, tx, p, id)
	if err != nil {
		return "", err
	}
	if in.Action == "WITHDRAWN" {
		if report.AuthorID != p.ID {
			return "", ErrForbidden
		}
	} else if !p.CanManageRecords || report.AuthorID == p.ID {
		return "", ErrForbidden
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "FUND_REPORT_DECIDE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	if report.Version != in.Version || (report.State != "PENDING" && !(in.Action == "WITHDRAWN" && (report.State == "NEEDS_INFO" || report.State == "REJECTED"))) {
		return "", ErrConflict
	}
	entry, allocation, duplicate, next := "", "", "", in.Action
	if in.Action == "CONFIRMED" {
		entry, allocation, duplicate, err = confirmFundReport(ctx, tx, p, report, in)
		if err != nil {
			return "", err
		}
		if duplicate != "" {
			next = "DUPLICATE"
		}
	} else if in.VerificationSource != "" || in.PaymentIdentity != "" || in.Mode != "" || in.EntryID != "" || in.AllocationAmount != "" {
		return "", invalid("Payment verification fields apply only to confirmation.")
	}
	now := time.Now().Unix()
	_, err = tx.ExecContext(ctx, "UPDATE fund_reports SET state=?,version=version+1,updated_at=?,entry_id=?,allocation_id=?,duplicate_report_id=?,reviewer_id=?,reviewed_at=?,decision_reason=? WHERE id=? AND version=?", next, now, optionalID(entry), optionalID(allocation), optionalID(duplicate), p.ID, now, in.Reason, id, report.Version)
	if err != nil {
		return "", err
	}
	if err = fundEvent(ctx, tx, p, report.CampaignID, "REPORT", id, next, in.Reason, report.Version+1, in); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, report.FlatID, "FUND_REPORT_"+next, in.Reason, map[string]any{"report_id": id, "state": report.State}, map[string]any{"report_id": id, "entry_id": entry, "state": next}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func confirmFundReport(ctx context.Context, tx *sql.Tx, p Principal, report FundReport, in FundReportDecision) (string, string, string, error) {
	if !validText(in.VerificationSource, 5, 120) || !validText(in.PaymentIdentity, 3, 120) || (in.Mode != "NEW" && in.Mode != "LINK") || (in.Mode == "LINK" && (in.EntryID == "" || len(in.EntryID) > 100)) || (in.Mode == "NEW" && in.EntryID != "") {
		return "", "", "", invalid("Review the external source/payment identity and choose a new or already recorded receipt.")
	}
	amount, err := zeroableFundAmount(in.AllocationAmount)
	if err != nil {
		return "", "", "", err
	}
	if amount > report.AmountPaise {
		return "", "", "", invalid("A fund allocation cannot exceed the payment.")
	}
	fingerprint := TokenHash(strings.ToUpper(strings.TrimSpace(in.VerificationSource)) + "\x00" + strings.ToUpper(strings.TrimSpace(in.PaymentIdentity)))
	entryID := ""
	err = tx.QueryRowContext(ctx, "SELECT entry_id FROM verified_fund_payments WHERE fingerprint=?", fingerprint).Scan(&entryID)
	known := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", "", "", err
	}
	if known && in.Mode == "LINK" && in.EntryID != entryID {
		return "", "", "", fmt.Errorf("%w: this verified payment already has a different receipt", ErrConflict)
	}
	if !known && in.Mode == "LINK" {
		entryID = in.EntryID
	}
	if !known && in.Mode == "NEW" {
		var exists bool
		// A compatible manual record must be linked, never recorded again.
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM entries e WHERE e.flat_id=? AND e.kind='RECEIVED' AND e.state='POSTED' AND e.amount_paise=? AND e.entry_date=? AND e.method=? AND upper(trim(e.reference))=upper(trim(?)) AND NOT EXISTS(SELECT 1 FROM entry_reversals v WHERE v.entry_id=e.id))`, report.FlatID, report.AmountPaise, report.PaymentDate, report.Method, report.Reference).Scan(&exists)
		if err != nil {
			return "", "", "", err
		}
		if exists && (report.Method != "CASH" || report.Reference != "") {
			return "", "", "", fmt.Errorf("%w: matching money is already recorded; reload and link its original receipt", ErrConflict)
		}
		home, err := permittedFinancialHome(ctx, tx, p, report.FlatID)
		if err != nil {
			return "", "", "", err
		}
		entryID = randomToken()
		now := time.Now()
		e := Entry{ID: entryID, FlatID: report.FlatID, Home: home, Kind: "RECEIVED", AmountPaise: report.AmountPaise, Date: report.PaymentDate, Description: "Fund contribution · " + report.Campaign, Payer: report.Payer, Method: report.Method, Reference: report.Reference, SourceNote: "Reviewed money already received; private verification retained in the report"}
		_, err = tx.ExecContext(ctx, `INSERT INTO entries(id,flat_id,kind,amount_paise,entry_date,description,payer,method,reference,source_note,state,created_by,created_at,posted_by,posted_at) VALUES(?,?,'RECEIVED',?,?,?,?,?,?,?,'POSTED',?,?,?,?)`, e.ID, e.FlatID, e.AmountPaise, e.Date, e.Description, e.Payer, e.Method, e.Reference, e.SourceNote, p.ID, now.Unix(), p.ID, now.Unix())
		if err != nil {
			return "", "", "", err
		}
		if err = issueReceipt(ctx, tx, p, e, now); err != nil {
			return "", "", "", err
		}
		if err = appendAudit(ctx, tx, p.ID, e.FlatID, "ENTRY_POSTED", "Externally verified payment report", map[string]any{"report_id": report.ID}, map[string]any{"entry_id": e.ID, "amount_paise": e.AmountPaise}); err != nil {
			return "", "", "", err
		}
	}
	entry, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", entryID))
	if err != nil {
		return "", "", "", err
	}
	if entry.FlatID != report.FlatID || entry.Kind != "RECEIVED" || entry.State != "POSTED" || entry.ReceiptID == "" || entry.AmountPaise != report.AmountPaise || entry.Date != report.PaymentDate || entry.Method != report.Method || strings.ToUpper(strings.TrimSpace(entry.Reference)) != strings.ToUpper(strings.TrimSpace(report.Reference)) {
		return "", "", "", invalid("The original receipt must match this home's reported amount, date, method and reference.")
	}
	if !known {
		var mapped bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM verified_fund_payments WHERE entry_id=?)", entryID).Scan(&mapped); err != nil {
			return "", "", "", err
		}
		if mapped {
			return "", "", "", fmt.Errorf("%w: this receipt already has a verified payment identity", ErrConflict)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO verified_fund_payments VALUES(?,?,?,?,?,?)", fingerprint, entryID, in.VerificationSource, in.PaymentIdentity, p.ID, time.Now().Unix()); err != nil {
			return "", "", "", err
		}
	}
	duplicate := ""
	err = tx.QueryRowContext(ctx, "SELECT id FROM fund_reports WHERE campaign_id=? AND flat_id=? AND entry_id=? AND state='CONFIRMED' ORDER BY reviewed_at,id LIMIT 1", report.CampaignID, report.FlatID, entryID).Scan(&duplicate)
	if err == nil {
		return entryID, "", duplicate, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", "", err
	}
	var kind, chargeID string
	var requested, waived int64
	if err = tx.QueryRowContext(ctx, "SELECT c.contribution_type,COALESCE(l.current_entry_id,''),l.requested_paise,l.waived_paise FROM fund_campaigns c JOIN fund_participants l ON l.campaign_id=c.id WHERE c.id=? AND l.flat_id=? AND c.state IN ('PUBLISHED','CLOSED')", report.CampaignID, report.FlatID).Scan(&kind, &chargeID, &requested, &waived); err != nil {
		return "", "", "", err
	}
	var used int64
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount_paise),0) FROM live_credit_uses WHERE source_id=?", entryID).Scan(&used); err != nil {
		return "", "", "", err
	}
	if amount > entry.AmountPaise-used {
		return "", "", "", fmt.Errorf("%w: this payment's usable credit changed; reload before allocating", ErrConflict)
	}
	allocation := ""
	if amount > 0 {
		if kind == "FIXED" {
			if chargeID == "" {
				return "", "", "", invalid("An exempt home has no live campaign charge. Record the payment as available credit.")
			}
			charge, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", chargeID))
			if err != nil {
				return "", "", "", err
			}
			if charge.State != "POSTED" {
				return "", "", "", ErrConflict
			}
			var allocated int64
			if err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount_paise),0) FROM live_entry_allocations WHERE charge_id=?", chargeID).Scan(&allocated); err != nil {
				return "", "", "", err
			}
			if amount > charge.AmountPaise-allocated {
				return "", "", "", fmt.Errorf("%w: the campaign's outstanding amount changed; reload before allocating", ErrConflict)
			}
			allocation = randomToken()
			if _, err = tx.ExecContext(ctx, "INSERT INTO entry_allocations VALUES(?,?,?,?,?,?,?)", allocation, entryID, chargeID, amount, p.ID, in.Reason, time.Now().Unix()); err != nil {
				return "", "", "", err
			}
		} else {
			contributionID := randomToken()
			if _, err = tx.ExecContext(ctx, "INSERT INTO fund_contributions VALUES(?,?,?,?,?,?,?,?)", contributionID, report.CampaignID, report.FlatID, entryID, amount, p.ID, in.Reason, time.Now().Unix()); err != nil {
				return "", "", "", err
			}
			if err = fundEvent(ctx, tx, p, report.CampaignID, "CONTRIBUTION", contributionID, "ATTRIBUTED", in.Reason, 1, map[string]any{"source_id": entryID, "amount_paise": amount}); err != nil {
				return "", "", "", err
			}
			if err = appendAudit(ctx, tx, p.ID, report.FlatID, "FUND_CREDIT_ATTRIBUTED", in.Reason, map[string]any{"source_id": entryID}, map[string]any{"contribution_id": contributionID, "amount_paise": amount}); err != nil {
				return "", "", "", err
			}
		}
	}
	return entryID, allocation, "", nil
}
