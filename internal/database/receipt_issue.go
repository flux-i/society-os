package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Both manual posting and verified payment reporting use this one atomic issuer.
// A charge, draft, claim or allocation never receives a money receipt.
func issueReceipt(ctx context.Context, tx *sql.Tx, p Principal, e Entry, now time.Time) error {
	if e.Kind != "RECEIVED" {
		return nil
	}
	year := now.In(time.FixedZone("IST", 19800)).Format("2006")
	var seq int
	if err := tx.QueryRowContext(ctx, `INSERT INTO receipt_counter VALUES (?,1) ON CONFLICT(year) DO UPDATE SET next_number=next_number+1 RETURNING next_number`, year).Scan(&seq); err != nil {
		return err
	}
	snapshot := ReceiptSnapshot{Number: fmt.Sprintf("SOS-%s-%06d", year, seq), Home: e.Home, Payer: e.Payer, AmountPaise: e.AmountPaise, Date: e.Date, Description: e.Description, Method: e.Method, Reference: e.Reference, SourceNote: e.SourceNote, Operator: p.Name, IssuedAt: now.Unix()}
	blob, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	receiptID := randomToken()
	if _, err = tx.ExecContext(ctx, "INSERT INTO receipts VALUES(?,?,?,?,?)", receiptID, e.ID, snapshot.Number, string(blob), now.Unix()); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO receipt_jobs(receipt_id,state,available_at) VALUES(?,'PENDING',?)", receiptID, now.Unix())
	return err
}
