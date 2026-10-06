package database

import (
	"context"
	"database/sql"
)

const fineReviewSQL = `(f.state IN('ISSUED','WAIVED') AND (i.state!='SUBSTANTIATED' OR i.flat_id!=f.flat_id OR i.rule_id!=f.rule_id OR i.incident_date!=json_extract(f.source_json,'$.incident_date') OR COALESCE((SELECT MAX(id) FROM incident_events WHERE incident_id=i.id AND action='SUBSTANTIATED'),0)!=json_extract(f.source_json,'$.outcome_event_id') OR COALESCE(i.notice_id,'')!=f.issued_incident_notice_id OR (SELECT COUNT(*) FROM fine_responses WHERE notice_id=f.notice_id)+(SELECT COUNT(*) FROM incident_responses WHERE notice_id=i.notice_id)!=f.issued_response_count))`
const fineBalanceFrom = ` FROM fines f JOIN incidents i ON i.id=f.incident_id LEFT JOIN entries e ON e.id=f.current_entry_id AND e.state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id) `
const fineAllocatedSQL = `COALESCE((SELECT SUM(amount_paise) FROM live_entry_allocations WHERE charge_id=e.id),0)`

func fineFilter(p Principal, q, state string) (string, []any) {
	scope, args := fineReadScope(p)
	scope += " AND (f.title LIKE ? ESCAPE '\\' OR b.code||'-'||h.flat_number LIKE ? ESCAPE '\\')"
	args = append(args, fineSearchPattern(q), fineSearchPattern(q))
	if state != "" {
		scope += " AND f.state=?"
		args = append(args, state)
	}
	return scope, args
}
func fineTotalsIn(ctx context.Context, tx *sql.Tx, p Principal, q, state string) (FineTotals, error) {
	out := FineTotals{}
	scope, args := fineReadScope(p)
	from := fineBalanceFrom + ` JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id WHERE ` + scope + ` AND (f.title LIKE ? ESCAPE '\' OR b.code||'-'||h.flat_number LIKE ? ESCAPE '\')`
	args = append(args, fineSearchPattern(q), fineSearchPattern(q))
	if state != "" {
		from += " AND f.state=?"
		args = append(args, state)
	}
	query := `SELECT COALESCE(SUM(COALESCE(e.amount_paise,0)),0),COALESCE(SUM(` + fineAllocatedSQL + `),0),COALESCE(SUM(COALESCE(e.amount_paise,0)-` + fineAllocatedSQL + `),0),COALESCE(SUM(CASE WHEN f.due_date<? AND f.pause_until='' THEN COALESCE(e.amount_paise,0)-` + fineAllocatedSQL + ` ELSE 0 END),0),COUNT(CASE WHEN f.state='PENDING' THEN 1 END),COUNT(CASE WHEN f.state='NOTIFIED' THEN 1 END),COUNT(CASE WHEN f.pause_until!='' THEN 1 END),COUNT(CASE WHEN ` + fineReviewSQL + ` AND ?=1 THEN 1 END)` + from
	e := tx.QueryRowContext(ctx, query, append([]any{today(), p.CanReadAllRecords}, args...)...).Scan(&out.ActivePaise, &out.AllocatedPaise, &out.OutstandingPaise, &out.OverduePaise, &out.Pending, &out.Notified, &out.Paused, &out.NeedsReview)
	return out, e
}
func (s *Store) FinesFor(ctx context.Context, token, q, state string, page int) (FinePage, error) {
	out := FinePage{Items: []Fine{}, Page: page, PageSize: 12}
	if e := fineReadInput(q, state, page); e != nil {
		return out, e
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if !p.CanReadRecords {
		return out, ErrForbidden
	}
	scope, args := fineFilter(p, q, state)
	e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM fines f JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id WHERE `+scope, args...).Scan(&out.Total)
	if e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	updated := "f.public_updated_at"
	if p.CanReadAllRecords {
		updated = "f.updated_at"
	}
	rows, e := tx.QueryContext(ctx, fineSelect+"WHERE "+scope+" ORDER BY "+updated+" DESC,f.id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		x, err := scanFine(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for i := range out.Items {
		if e = fineEnrich(ctx, tx, p, &out.Items[i]); e != nil {
			return out, e
		}
		out.Items[i] = fineProjection(out.Items[i], p)
	}
	out.Totals, e = fineTotalsIn(ctx, tx, p, q, state)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func (s *Store) FineFor(ctx context.Context, token, id string, eventPage, responsePage int) (FineDetail, error) {
	out := FineDetail{PageSize: 20}
	if id == "" || len(id) > 100 || !boundedMaintenancePage(eventPage) || !boundedMaintenancePage(responsePage) {
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
	scope, args := fineReadScope(p)
	out.Fine, e = scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=? AND "+scope, append([]any{id}, args...)...))
	if e != nil {
		return out, e
	}
	if e = fineEnrich(ctx, tx, p, &out.Fine); e != nil {
		return out, e
	}
	if p.CanReadAllRecords {
		out.Events, out.EventTotal, out.EventPage, e = fineEvents(ctx, tx, "FINE", id, eventPage)
		if e != nil {
			return out, e
		}
	}
	if out.NoticeID != "" {
		n, e := fineNoticeIn(ctx, tx, p, out.NoticeID, responsePage)
		if e != nil {
			return out, e
		}
		out.Notice = &n
	}
	out.Fine = fineProjection(out.Fine, p)
	return out, tx.Commit()
}

const fineNoticeSelect = `SELECT n.id,n.fine_id,n.flat_id,b.code||'-'||h.flat_number,n.title,n.body,n.amount_paise,n.policy_reference,n.due_date,n.response_by,n.created_at,n.version,f.state FROM fine_notices n JOIN fines f ON f.id=n.fine_id JOIN flats h ON h.id=n.flat_id JOIN buildings b ON b.id=h.building_id `

func scanFineNotice(row interface{ Scan(...any) error }) (FineNotice, error) {
	var n FineNotice
	e := row.Scan(&n.ID, &n.FineID, &n.FlatID, &n.Home, &n.Title, &n.Body, &n.AmountPaise, &n.PolicyReference, &n.DueDate, &n.ResponseBy, &n.CreatedAt, &n.Version, &n.State)
	return n, e
}
func fineNoticeIn(ctx context.Context, tx *sql.Tx, p Principal, id string, page int) (FineNotice, error) {
	scope, args := fineNoticeScope(p)
	n, e := scanFineNotice(tx.QueryRowContext(ctx, fineNoticeSelect+"WHERE n.id=? AND "+scope, append([]any{id}, args...)...))
	if e != nil {
		return n, e
	}
	n.PageSize = 20
	n.Responses = []IncidentResponse{}
	member, e := incidentHomeMember(ctx, tx, p, n.FlatID)
	if e != nil {
		return n, e
	}
	n.CanRespond = member && n.State != "WITHDRAWN" && n.State != "DECLINED"
	n.CanAppeal = member && (n.State == "ISSUED" || n.State == "WAIVED")
	n.CanReadFinance = fineFinanceAllowed(ctx, tx, p, n.FlatID) && (n.State == "ISSUED" || n.State == "WAIVED")
	where := "r.notice_id=?"
	params := []any{id}
	if !p.CanReadAllRecords {
		where += " AND r.author_id=?"
		params = append(params, p.ID)
	}
	e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fine_responses r WHERE "+where, params...).Scan(&n.ResponseTotal)
	if e != nil {
		return n, e
	}
	n.ResponsePage = incidentPageBound(page, n.ResponseTotal, 20)
	rows, e := tx.QueryContext(ctx, `SELECT r.id,u.display_name,r.body,r.created_at FROM fine_responses r JOIN users u ON u.id=r.author_id WHERE `+where+" ORDER BY r.created_at,r.id LIMIT 20 OFFSET ?", append(params, (n.ResponsePage-1)*20)...)
	if e != nil {
		return n, e
	}
	defer rows.Close()
	for rows.Next() {
		var r IncidentResponse
		if e = rows.Scan(&r.ID, &r.Actor, &r.Body, &r.CreatedAt); e != nil {
			return n, e
		}
		if !p.CanReadAllRecords {
			r.Actor = ""
		}
		n.Responses = append(n.Responses, r)
	}
	return n, rows.Err()
}
func (s *Store) FineNoticeFor(ctx context.Context, token, id string, page int) (FineNotice, error) {
	if id == "" || len(id) > 100 || !boundedMaintenancePage(page) {
		return FineNotice{}, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return FineNotice{}, e
	}
	defer tx.Rollback()
	n, e := fineNoticeIn(ctx, tx, p, id, page)
	if e != nil {
		return n, e
	}
	return n, tx.Commit()
}
func (s *Store) FineNoticesFor(ctx context.Context, token, q string, page int) (FineNoticePage, error) {
	out := FineNoticePage{Items: []FineNotice{}, Page: page, PageSize: 12}
	if e := incidentReadInput(q, "", page); e != nil {
		return out, e
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	scope, args := fineNoticeScope(p)
	scope += ` AND (n.title LIKE ? ESCAPE '\' OR b.code||'-'||h.flat_number LIKE ? ESCAPE '\')`
	args = append(args, fineSearchPattern(q), fineSearchPattern(q))
	from := ` FROM fine_notices n JOIN fines f ON f.id=n.fine_id JOIN flats h ON h.id=n.flat_id JOIN buildings b ON b.id=h.building_id WHERE ` + scope
	e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&out.Total)
	if e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	rows, e := tx.QueryContext(ctx, "SELECT n.id"+from+" ORDER BY n.created_at DESC,n.id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		n, e := fineNoticeIn(ctx, tx, p, id, 1)
		if e != nil {
			return out, e
		}
		n.Responses = nil
		n.ResponseTotal = 0
		out.Items = append(out.Items, n)
	}
	return out, tx.Commit()
}
