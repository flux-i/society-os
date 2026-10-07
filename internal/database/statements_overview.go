package database

import (
	"context"
	"database/sql"
)

func overviewStatements(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	if p.MFAPending {
		return ErrForbidden
	}
	scope, args := statementScope(p)
	// Count publication visibility using the exact download scope, including
	// a retained publication while an internal replacement is being reviewed.
	if err := overviewCounts(ctx, tx, out, []string{"published"}, `SELECT COUNT(*) FROM statement_files x JOIN statement_groups g ON g.id=x.group_id JOIN statement_publications sp ON sp.id=g.current_publication_id AND sp.file_id=x.id AND sp.state='PUBLISHED' WHERE `+scope, args...); err != nil {
		return err
	}
	if !statementStaff(p) {
		from := ` FROM statement_files x JOIN statement_groups g ON g.id=x.group_id JOIN statement_publications sp ON sp.id=g.current_publication_id AND sp.file_id=x.id AND sp.state='PUBLISHED' WHERE ` + scope + ` AND date(sp.reviewed_at,'unixepoch','+5 hours','+30 minutes')>=:recent`
		values := append(args, sql.Named("recent", out.PeriodStart))
		if err := overviewCounts(ctx, tx, out, []string{"attention_items"}, `SELECT COUNT(*)`+from, values...); err != nil {
			return err
		}
		return overviewItems(ctx, tx, out, `SELECT x.id,x.title,'',x.kind,'PUBLISHED','NORMAL',x.period_end,sp.reviewed_at,0`+from+` ORDER BY sp.reviewed_at DESC,sp.id LIMIT 4`, values...)
	}
	if err := overviewCounts(ctx, tx, out, []string{"originals_checking", "pending_review", "awaiting_other_reviewer", "upload_attention"}, `SELECT COUNT(CASE WHEN validation IN('PENDING','VALIDATING') AND uploaded_at>0 THEN 1 END),COUNT(CASE WHEN validation='AVAILABLE' AND uploaded_by<>? THEN 1 END),COUNT(CASE WHEN validation='AVAILABLE' AND uploaded_by=? THEN 1 END),COUNT(CASE WHEN validation IN('REJECTED','ABANDONED') OR uploaded_at=0 THEN 1 END) FROM statement_files WHERE state='PENDING'`, p.ID, p.ID); err != nil {
		return err
	}
	if err := overviewCounts(ctx, tx, out, []string{"publication_review", "publication_awaiting_other"}, `SELECT COUNT(CASE WHEN proposed_by<>? THEN 1 END),COUNT(CASE WHEN proposed_by=? THEN 1 END) FROM statement_publications WHERE state='PENDING'`, p.ID, p.ID); err != nil {
		return err
	}
	from := ` FROM (
 SELECT x.id,x.title,x.kind,x.period_end,CASE WHEN x.validation='AVAILABLE' THEN 'READY_REVIEW' ELSE 'UPLOAD_ATTENTION' END state,x.created_at at,CASE WHEN x.validation='AVAILABLE' THEN 1 ELSE 0 END priority FROM statement_files x WHERE x.state='PENDING' AND ((x.validation='AVAILABLE' AND x.uploaded_by<>?) OR ((x.validation IN('REJECTED','ABANDONED') OR x.uploaded_at=0) AND x.uploaded_by=?))
 UNION ALL SELECT f.id,f.title,f.kind,f.period_end,'PUBLICATION_REVIEW',sp.proposed_at,2 FROM statement_publications sp JOIN statement_files f ON f.id=sp.file_id WHERE sp.state='PENDING' AND sp.proposed_by<>?)`
	if err := overviewCounts(ctx, tx, out, []string{"attention_items"}, `SELECT COUNT(*)`+from, p.ID, p.ID, p.ID); err != nil {
		return err
	}
	return overviewItems(ctx, tx, out, `SELECT id,title,'',kind,state,'NORMAL',period_end,at,0`+from+` ORDER BY priority,at,id LIMIT 4`, p.ID, p.ID, p.ID)
}

// The finance appointment permits this deliberate audience chooser. It does
// not expose contact destinations or grant the audience private home finance.
func (s *Store) StatementTargetsFor(ctx context.Context, token, target, search string, page int) (MessageTargetPage, error) {
	out := MessageTargetPage{Items: []RecordHome{}, Page: page, PageSize: 12}
	if (target != "HOMES" && target != "PEOPLE") || len(search) > 100 || page < 1 || page > 10000 {
		return out, ErrInvalid
	}
	tx, p, err := beginStatementRead(ctx, s, token)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if !statementStaff(p) {
		return out, ErrForbidden
	}
	selectSQL := `SELECT DISTINCT r.id,r.full_name FROM residents r JOIN flat_memberships m ON m.resident_id=r.id WHERE m.start_date<=:day AND (m.end_date IS NULL OR m.end_date>:day) AND r.full_name LIKE :search`
	if target == "HOMES" {
		selectSQL = `SELECT f.id,b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id WHERE b.code||'-'||f.flat_number LIKE :search AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.start_date<=:day AND (m.end_date IS NULL OR m.end_date>:day))`
	}
	args := []any{sql.Named("day", today()), sql.Named("search", "%"+search+"%")}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+selectSQL+`)`, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, selectSQL+` ORDER BY 2,1 LIMIT 12 OFFSET :offset`, append(args, sql.Named("offset", (page-1)*12))...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item RecordHome
		if err = rows.Scan(&item.ID, &item.Label); err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
