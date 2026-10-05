package database

import (
	"context"
	"database/sql"
)

func overviewCollections(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	if !p.CanReadRecords {
		return ErrForbidden
	}
	args := append(fundScopeArgs(p, ""), sql.Named("manage", p.CanManageRecords))
	if err := overviewCounts(ctx, tx, out, []string{"outstanding_paise", "overdue_paise", "confirmed_paise", "overdue_funds", "pending_review", "awaiting_other_reviewer"}, fundLinesCTE+`SELECT COALESCE(SUM(s.outstanding_paise),0),COALESCE(SUM(s.overdue_paise),0),COALESCE(SUM(s.allocated_paise+s.voluntary_paise),0),COUNT(CASE WHEN s.overdue_paise>0 THEN 1 END),COUNT(CASE WHEN c.state='PENDING' AND c.author_id<>:actor AND :manage=1 THEN 1 END),COUNT(CASE WHEN c.state='PENDING' AND c.author_id=:actor THEN 1 END) FROM fund_campaigns c JOIN fund_summary s ON s.campaign_id=c.id`, args...); err != nil {
		return err
	}
	if err := overviewCounts(ctx, tx, out, []string{"awaiting_verification", "needs_your_verification", "changes_requested"}, `SELECT COUNT(CASE WHEN r.state='PENDING' THEN 1 END),COUNT(CASE WHEN r.state='PENDING' AND r.author_id<>:actor AND :manage=1 THEN 1 END),COUNT(CASE WHEN r.state='NEEDS_INFO' AND r.author_id=:actor THEN 1 END) FROM fund_reports r WHERE `+reportScope, args...); err != nil {
		return err
	}
	if err := overviewCounts(ctx, tx, out, []string{"pending_exemptions"}, `SELECT COUNT(*) FROM fund_waivers WHERE state='PENDING' AND author_id<>:actor AND :manage=1`, args...); err != nil {
		return err
	}
	query := fundLinesCTE + `SELECT * FROM (
 SELECT c.id,c.title,'' home,'FUND' kind,CASE WHEN c.state='PENDING' THEN 'PENDING' ELSE 'OVERDUE' END state,'NORMAL' priority,c.due_date date,c.created_at at,s.outstanding_paise amount FROM fund_campaigns c JOIN fund_summary s ON s.campaign_id=c.id WHERE (c.state='PENDING' AND c.author_id<>:actor AND :manage=1) OR s.overdue_paise>0
 UNION ALL SELECT r.id,c.title,b.code||'-'||f.flat_number,'PAYMENT_REPORT',r.state,'NORMAL',r.payment_date,r.updated_at,r.amount_paise FROM fund_reports r JOIN fund_campaigns c ON c.id=r.campaign_id JOIN flats f ON f.id=r.flat_id JOIN buildings b ON b.id=f.building_id WHERE ` + reportScope + ` AND ((r.state='PENDING' AND r.author_id<>:actor AND :manage=1) OR (r.state='NEEDS_INFO' AND r.author_id=:actor))
 UNION ALL SELECT w.id,c.title,b.code||'-'||f.flat_number,'FUND_WAIVER',w.state,'NORMAL','',w.created_at,w.amount_paise FROM fund_waivers w JOIN fund_campaigns c ON c.id=w.campaign_id JOIN flats f ON f.id=w.flat_id JOIN buildings b ON b.id=f.building_id WHERE w.state='PENDING' AND w.author_id<>:actor AND :manage=1
 ) ORDER BY CASE WHEN state='NEEDS_INFO' THEN 0 WHEN state='PENDING' THEN 1 ELSE 2 END,at,id LIMIT 4`
	return overviewItems(ctx, tx, out, query, args...)
}
