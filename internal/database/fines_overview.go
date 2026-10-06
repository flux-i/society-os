package database

import (
	"context"
	"database/sql"
	"sort"
)

func overviewFines(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	if p.CanReadRecords {
		totals, err := fineTotalsIn(ctx, tx, p, "", "")
		if err != nil {
			return err
		}
		out.Counts["active_paise"] = totals.ActivePaise
		out.Counts["allocated_paise"] = totals.AllocatedPaise
		out.Counts["outstanding_paise"] = totals.OutstandingPaise
		out.Counts["overdue_paise"] = totals.OverduePaise
		out.Counts["paused"] = int64(totals.Paused)
		if p.CanReadAllRecords {
			out.Counts["needs_review"] = int64(totals.NeedsReview)
		}
	}
	args := []any{sql.Named("actor", p.ID), sql.Named("resident", p.ResidentID), sql.Named("day", out.Day), sql.Named("treasury", p.CanManageRecords)}
	member := `EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.flat_id AND m.resident_id=:resident AND m.start_date<=:day AND (m.end_date IS NULL OR m.end_date>:day))`
	eligible := `:treasury=1 AND f.author_id<>:actor AND c.reporter_id<>:actor`
	if err := overviewCounts(ctx, tx, out, []string{"pending_review"}, `SELECT COUNT(*) FROM fines f JOIN incidents c ON c.id=f.incident_id WHERE f.state IN('PENDING','NOTIFIED') AND `+eligible, args...); err != nil {
		return err
	}
	if err := overviewCounts(ctx, tx, out, []string{"responses_needed"}, `SELECT COUNT(*) FROM fine_notices n JOIN fines f ON f.id=n.fine_id WHERE f.state='NOTIFIED' AND `+member+` AND NOT EXISTS(SELECT 1 FROM fine_responses r WHERE r.notice_id=n.id AND r.author_id=:actor)`, args...); err != nil {
		return err
	}
	appealScope := `((a.author_id=:actor AND ` + member + `) OR (` + eligible + ` AND a.author_id<>:actor))`
	if err := overviewCounts(ctx, tx, out, []string{"active_appeals", "appeals_for_review"}, `SELECT COUNT(*),COUNT(CASE WHEN `+eligible+` AND a.author_id<>:actor THEN 1 END) FROM fine_appeals a JOIN fines f ON f.id=a.fine_id JOIN incidents c ON c.id=f.incident_id WHERE a.state IN('PENDING','PAUSED') AND `+appealScope, args...); err != nil {
		return err
	}
	if err := overviewCounts(ctx, tx, out, []string{"corrections_for_review"}, `SELECT COUNT(*) FROM fine_waivers w JOIN fines f ON f.id=w.fine_id JOIN incidents c ON c.id=f.incident_id WHERE w.state='PENDING' AND w.author_id<>:actor AND `+eligible, args...); err != nil {
		return err
	}
	if p.CanReadRecords {
		scope, params := fineReportScope(p)
		query := `SELECT COUNT(CASE WHEN r.state='PENDING' THEN 1 END),COUNT(CASE WHEN r.state='PENDING' AND r.author_id<>? AND ?=1 THEN 1 END),COUNT(CASE WHEN r.state='NEEDS_INFO' AND r.author_id=? THEN 1 END) FROM fine_reports r WHERE ` + scope
		if err := overviewCounts(ctx, tx, out, []string{"awaiting_verification", "needs_your_verification", "changes_requested"}, query, append([]any{p.ID, p.CanManageRecords, p.ID}, params...)...); err != nil {
			return err
		}
		items := `SELECT r.id,f.title,b.code||'-'||h.flat_number,'FINE_PAYMENT',r.state,'NORMAL',r.payment_date,r.updated_at,0 FROM fine_reports r JOIN fines f ON f.id=r.fine_id JOIN flats h ON h.id=r.flat_id JOIN buildings b ON b.id=h.building_id WHERE ` + scope + ` AND ((r.state='PENDING' AND r.author_id<>? AND ?=1) OR (r.state='NEEDS_INFO' AND r.author_id=?)) ORDER BY r.updated_at,r.id LIMIT 4`
		if err := overviewItems(ctx, tx, out, items, append(params, p.ID, p.CanManageRecords, p.ID)...); err != nil {
			return err
		}
	}
	// Public notices and an author's own appeals remain useful without ledger access.
	query := `SELECT id,title,home,kind,state,priority,date,at,amount FROM (` +
		`SELECT n.id id,n.title title,b.code||'-'||h.flat_number home,'FINE_NOTICE' kind,'RESPONSE_NEEDED' state,'NORMAL' priority,n.response_by date,n.created_at at,0 amount FROM fine_notices n JOIN fines f ON f.id=n.fine_id JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id WHERE f.state='NOTIFIED' AND ` + member + ` AND NOT EXISTS(SELECT 1 FROM fine_responses r WHERE r.notice_id=n.id AND r.author_id=:actor)
 UNION ALL SELECT a.id,f.title,b.code||'-'||h.flat_number,'FINE_APPEAL',a.state,'HIGH',a.pause_until,a.updated_at,0 FROM fine_appeals a JOIN fines f ON f.id=a.fine_id JOIN incidents c ON c.id=f.incident_id JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id WHERE a.state IN('PENDING','PAUSED') AND ` + appealScope + `
 UNION ALL SELECT w.id,f.title,b.code||'-'||h.flat_number,'FINE_CORRECTION','PENDING','HIGH','',w.created_at,0 FROM fine_waivers w JOIN fines f ON f.id=w.fine_id JOIN incidents c ON c.id=f.incident_id JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id WHERE w.state='PENDING' AND w.author_id<>:actor AND ` + eligible + `
 UNION ALL SELECT f.id,f.title,b.code||'-'||h.flat_number,'FINE',f.state,'NORMAL',f.response_by,f.updated_at,0 FROM fines f JOIN incidents c ON c.id=f.incident_id JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id WHERE f.state IN('PENDING','NOTIFIED') AND ` + eligible + `
 ) ORDER BY (priority='HIGH') DESC,at,id LIMIT 4`
	if err := overviewItems(ctx, tx, out, query, args...); err != nil {
		return err
	}
	if p.CanReadRecords {
		scope, params := fineReadScope(p)
		financial := `SELECT f.id,f.title,b.code||'-'||h.flat_number,'FINE',CASE WHEN ?=1 AND ` + fineReviewSQL + ` THEN 'NEEDS_REVIEW' WHEN f.pause_until!='' THEN 'PAUSED' ELSE 'OVERDUE' END,'NORMAL',f.due_date,CASE WHEN ?=1 THEN f.updated_at ELSE f.public_updated_at END,COALESCE(e.amount_paise,0)-` + fineAllocatedSQL + fineBalanceFrom + ` JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id WHERE ` + scope + ` AND ((f.state='ISSUED' AND COALESCE(e.amount_paise,0)-` + fineAllocatedSQL + `>0 AND (f.due_date<? OR f.pause_until!='')) OR (?=1 AND ` + fineReviewSQL + `)) ORDER BY f.due_date,f.id LIMIT 4`
		if err := overviewItems(ctx, tx, out, financial, append(append([]any{p.CanReadAllRecords, p.CanReadAllRecords}, params...), out.Day, p.CanReadAllRecords)...); err != nil {
			return err
		}
		count := `SELECT COUNT(*)` + fineBalanceFrom + ` WHERE ` + scope + ` AND f.state='ISSUED' AND COALESCE(e.amount_paise,0)-` + fineAllocatedSQL + `>0 AND f.due_date<? AND f.pause_until=''`
		if err := overviewCounts(ctx, tx, out, []string{"overdue_fines"}, count, append(params, out.Day)...); err != nil {
			return err
		}
	}
	sort.Slice(out.Items, func(i, j int) bool {
		a, b := out.Items[i], out.Items[j]
		if a.Priority != b.Priority {
			return a.Priority == "HIGH"
		}
		if a.At != b.At {
			return a.At < b.At
		}
		return a.ID < b.ID
	})
	if len(out.Items) > 4 {
		out.Items = out.Items[:4]
	}
	return nil
}
