package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
)

func (s *Store) FineEvidenceFor(ctx context.Context, token, fine, q string, page int) (FineEvidencePage, error) {
	out := FineEvidencePage{Items: []FineEvidenceChoice{}, Page: page, PageSize: 12}
	if len(fine) < 1 || len(fine) > 100 || incidentReadInput(q, "", page) != nil {
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
	var home string
	e = tx.QueryRowContext(ctx, "SELECT flat_id FROM fines WHERE id=? AND state IN('ISSUED','WAIVED')", fine).Scan(&home)
	if e != nil {
		return out, e
	}
	if !fineFinanceAllowed(ctx, tx, p, home) {
		return out, sql.ErrNoRows
	}
	from := ` FROM library_documents x JOIN document_groups g ON g.id=x.group_id WHERE x.uploaded_by=? AND x.flat_id=? AND x.category='PAYMENT_EVIDENCE' AND x.visibility='FLAT_SPECIFIC' AND x.validation_status='AVAILABLE' AND x.review_state IN('PENDING','APPROVED') AND g.archived=0 AND (x.title LIKE ? ESCAPE '\' OR x.original_filename LIKE ? ESCAPE '\')`
	args := []any{p.ID, home, fineSearchPattern(q), fineSearchPattern(q)}
	e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&out.Total)
	if e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	rows, e := tx.QueryContext(ctx, "SELECT x.id,x.title,x.original_filename"+from+" ORDER BY x.created_at DESC,x.id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var x FineEvidenceChoice
		if e = rows.Scan(&x.ID, &x.Title, &x.Filename); e != nil {
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	return out, tx.Commit()
}
func (s *Store) FineConfirmOptionsFor(ctx context.Context, token, id string, page int) (FineConfirmOptions, error) {
	out := FineConfirmOptions{Entries: []FundReceiptChoice{}, Page: page, PageSize: 12}
	if len(id) < 1 || len(id) > 100 || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if !p.CanManageRecords {
		return out, ErrForbidden
	}
	r, e := fineReportIn(ctx, tx, p, id)
	if e != nil {
		return out, e
	}
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", r.FineID))
	if e != nil {
		return out, e
	}
	if e = fineEnrich(ctx, tx, p, &x); e != nil {
		return out, e
	}
	out.ChargeID = x.CurrentEntryID
	out.OutstandingPaise = x.OutstandingPaise
	from := ` FROM entries e JOIN receipts rc ON rc.entry_id=e.id LEFT JOIN verified_fund_payments v ON v.entry_id=e.id WHERE e.flat_id=? AND e.kind='RECEIVED' AND e.state='POSTED' AND e.amount_paise=? AND e.entry_date=? AND e.method=? AND upper(trim(e.reference))=upper(trim(?)) AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id)`
	args := []any{r.FlatID, r.AmountPaise, r.PaymentDate, r.Method, r.Reference}
	e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&out.Total)
	if e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	rows, e := tx.QueryContext(ctx, `SELECT e.id,e.kind,e.description,e.entry_date,e.amount_paise,COALESCE((SELECT SUM(amount_paise) FROM live_credit_uses WHERE source_id=e.id),0),e.amount_paise-COALESCE((SELECT SUM(amount_paise) FROM live_credit_uses WHERE source_id=e.id),0),rc.number,COALESCE(v.verification_source,''),COALESCE(v.payment_identity,'')`+from+" ORDER BY e.created_at DESC,e.id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var x FundReceiptChoice
		if e = rows.Scan(&x.ID, &x.Kind, &x.Description, &x.Date, &x.AmountPaise, &x.AllocatedPaise, &x.RemainingPaise, &x.Receipt, &x.VerificationSource, &x.PaymentIdentity); e != nil {
			return out, e
		}
		out.Entries = append(out.Entries, x)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	return out, tx.Commit()
}
func (s *Store) FineReportEvidenceFor(ctx context.Context, token, id string) (LibraryDocument, []byte, error) {
	if len(id) < 1 || len(id) > 100 {
		return LibraryDocument{}, nil, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return LibraryDocument{}, nil, e
	}
	defer tx.Rollback()
	r, e := fineReportIn(ctx, tx, p, id)
	if e != nil {
		return LibraryDocument{}, nil, e
	}
	if r.EvidenceID == "" {
		return LibraryDocument{}, nil, sql.ErrNoRows
	}
	x, e := scanDocument(tx.QueryRowContext(ctx, librarySelect+` WHERE x.id=? AND x.flat_id=? AND x.uploaded_by=? AND x.category='PAYMENT_EVIDENCE' AND x.validation_status='AVAILABLE' AND x.review_state IN('PENDING','APPROVED') AND g.archived=0`, r.EvidenceID, r.FlatID, r.AuthorID))
	if e != nil {
		return LibraryDocument{}, nil, e
	}
	var data []byte
	e = tx.QueryRowContext(ctx, "SELECT original_bytes FROM local_document_objects WHERE document_id=?", x.ID).Scan(&data)
	if e != nil {
		return x, nil, e
	}
	sum := sha256.Sum256(data)
	if fmt.Sprintf("%x", sum) != x.SHA256 {
		return x, nil, fmt.Errorf("evidence original checksum is unavailable")
	}
	return x, data, tx.Commit()
}
