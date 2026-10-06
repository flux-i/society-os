package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// One global external identity retains one compatible original receipt across purposes.
type externalPaidClaim struct {
	ID, FlatID, PaymentDate, Payer, Method, Reference, Description string
	AmountPaise                                                    int64
}

func confirmExternalReceived(ctx context.Context, tx *sql.Tx, p Principal, report externalPaidClaim, in FundReportDecision) (Entry, error) {
	if !validText(in.VerificationSource, 5, 120) || !validText(in.PaymentIdentity, 3, 120) || (in.Mode != "NEW" && in.Mode != "LINK") || (in.Mode == "LINK" && (in.EntryID == "" || len(in.EntryID) > 100)) || (in.Mode == "NEW" && in.EntryID != "") {
		return Entry{}, invalid("Review the external source/payment identity and choose a new or already recorded receipt.")
	}
	fingerprint := TokenHash(strings.ToUpper(strings.TrimSpace(in.VerificationSource)) + "\x00" + strings.ToUpper(strings.TrimSpace(in.PaymentIdentity)))
	entryID := ""
	err := tx.QueryRowContext(ctx, "SELECT entry_id FROM verified_fund_payments WHERE fingerprint=?", fingerprint).Scan(&entryID)
	known := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Entry{}, err
	}
	if known && in.Mode == "LINK" && in.EntryID != entryID {
		return Entry{}, fmt.Errorf("%w: this verified payment already has a different receipt", ErrConflict)
	}
	if !known && in.Mode == "LINK" {
		entryID = in.EntryID
	}
	if !known && in.Mode == "NEW" {
		var exists bool
		// A compatible manual record must be linked, never recorded again.
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM entries e WHERE e.flat_id=? AND e.kind='RECEIVED' AND e.state='POSTED' AND e.amount_paise=? AND e.entry_date=? AND e.method=? AND upper(trim(e.reference))=upper(trim(?)) AND NOT EXISTS(SELECT 1 FROM entry_reversals v WHERE v.entry_id=e.id))`, report.FlatID, report.AmountPaise, report.PaymentDate, report.Method, report.Reference).Scan(&exists)
		if err != nil {
			return Entry{}, err
		}
		if exists && (report.Method != "CASH" || report.Reference != "") {
			return Entry{}, fmt.Errorf("%w: matching money is already recorded; reload and link its original receipt", ErrConflict)
		}
		home, err := permittedFinancialHome(ctx, tx, p, report.FlatID)
		if err != nil {
			return Entry{}, err
		}
		entryID = randomToken()
		now := time.Now()
		e := Entry{ID: entryID, FlatID: report.FlatID, Home: home, Kind: "RECEIVED", AmountPaise: report.AmountPaise, Date: report.PaymentDate, Description: report.Description, Payer: report.Payer, Method: report.Method, Reference: report.Reference, SourceNote: "Reviewed money already received; private verification retained in the report"}
		_, err = tx.ExecContext(ctx, `INSERT INTO entries(id,flat_id,kind,amount_paise,entry_date,description,payer,method,reference,source_note,state,created_by,created_at,posted_by,posted_at) VALUES(?,?,'RECEIVED',?,?,?,?,?,?,?,'POSTED',?,?,?,?)`, e.ID, e.FlatID, e.AmountPaise, e.Date, e.Description, e.Payer, e.Method, e.Reference, e.SourceNote, p.ID, now.Unix(), p.ID, now.Unix())
		if err != nil {
			return Entry{}, err
		}
		if err = issueReceipt(ctx, tx, p, e, now); err != nil {
			return Entry{}, err
		}
		if err = appendAudit(ctx, tx, p.ID, e.FlatID, "ENTRY_POSTED", "Externally verified payment report", map[string]any{"report_id": report.ID}, map[string]any{"entry_id": e.ID, "amount_paise": e.AmountPaise}); err != nil {
			return Entry{}, err
		}
	}
	entry, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", entryID))
	if err != nil {
		return Entry{}, err
	}
	if entry.FlatID != report.FlatID || entry.Kind != "RECEIVED" || entry.State != "POSTED" || entry.ReceiptID == "" || entry.AmountPaise != report.AmountPaise || entry.Date != report.PaymentDate || entry.Method != report.Method || strings.ToUpper(strings.TrimSpace(entry.Reference)) != strings.ToUpper(strings.TrimSpace(report.Reference)) {
		return Entry{}, invalid("The original receipt must match this home's reported amount, date, method and reference.")
	}
	if !known {
		var mapped bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM verified_fund_payments WHERE entry_id=?)", entryID).Scan(&mapped); err != nil {
			return Entry{}, err
		}
		if mapped {
			return Entry{}, fmt.Errorf("%w: this receipt already has a verified payment identity", ErrConflict)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO verified_fund_payments VALUES(?,?,?,?,?,?)", fingerprint, entryID, in.VerificationSource, in.PaymentIdentity, p.ID, time.Now().Unix()); err != nil {
			return Entry{}, err
		}
	}
	return entry, nil
}
