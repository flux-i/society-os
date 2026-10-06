package database

import (
	"context"
	"database/sql"
)

func overviewIncidents(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	can, e := incidentParticipant(ctx, tx, p)
	if e != nil {
		return e
	}
	args := []any{sql.Named("actor", p.ID), sql.Named("resident", p.ResidentID), sql.Named("operator", p.CanHandleComplaints), sql.Named("can", can), sql.Named("day", today())}
	if e = overviewCounts(ctx, tx, out, []string{"needs_review", "information_needed"}, `SELECT COUNT(CASE WHEN :operator=1 AND reporter_id<>:actor AND state IN('REPORTED','NEEDS_INFO','UNDER_REVIEW') THEN 1 END),COUNT(CASE WHEN reporter_id=:actor AND state='NEEDS_INFO' AND :can=1 THEN 1 END) FROM incidents`, args...); e != nil {
		return e
	}
	if e = overviewCounts(ctx, tx, out, []string{"pending_rules"}, `SELECT COUNT(*) FROM society_rules WHERE state='PENDING' AND author_id<>:actor AND :operator=1`, args...); e != nil {
		return e
	}
	noticeScope := `c.notice_id=n.id AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=n.flat_id AND m.resident_id=:resident AND m.start_date<=:day AND (m.end_date IS NULL OR m.end_date>:day)) AND NOT EXISTS(SELECT 1 FROM incident_responses s WHERE s.notice_id=n.id AND s.actor_id=:actor)`
	if e = overviewCounts(ctx, tx, out, []string{"responses_needed"}, `SELECT COUNT(*) FROM incident_notices n JOIN incidents c ON c.id=n.incident_id WHERE `+noticeScope, args...); e != nil {
		return e
	}
	query := `SELECT * FROM (
 SELECT c.id,r.title,b.code||'-'||f.flat_number home,CASE WHEN c.reporter_id=:actor AND c.state='NEEDS_INFO' THEN 'INCIDENT_CLARIFICATION' ELSE 'INCIDENT' END kind,c.state,'NORMAL' priority,c.incident_date date,CASE WHEN :operator=1 THEN c.updated_at ELSE c.reporter_updated_at END at,0 amount FROM incidents c JOIN society_rules r ON r.id=c.rule_id JOIN flats f ON f.id=c.flat_id JOIN buildings b ON b.id=f.building_id WHERE (:operator=1 AND c.reporter_id<>:actor AND c.state IN('REPORTED','NEEDS_INFO','UNDER_REVIEW')) OR (c.reporter_id=:actor AND c.state='NEEDS_INFO' AND :can=1)
 UNION ALL SELECT id,title,'','RULE','PENDING','NORMAL',effective_from,created_at,0 FROM society_rules WHERE state='PENDING' AND author_id<>:actor AND :operator=1
 UNION ALL SELECT n.id,n.title,b.code||'-'||f.flat_number,'INCIDENT_NOTICE','RESPONSE_NEEDED','NORMAL',n.response_by,n.created_at,0 FROM incident_notices n JOIN incidents c ON c.id=n.incident_id JOIN flats f ON f.id=n.flat_id JOIN buildings b ON b.id=f.building_id WHERE ` + noticeScope + `
 ) ORDER BY CASE WHEN state='NEEDS_INFO' THEN 0 WHEN state='RESPONSE_NEEDED' THEN 1 WHEN state='PENDING' THEN 2 ELSE 3 END,at,id LIMIT 4`
	return overviewItems(ctx, tx, out, query, args...)
}
