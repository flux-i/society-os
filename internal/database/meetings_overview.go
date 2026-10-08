package database

import (
	"context"
	"database/sql"
)

func overviewMeetings(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	if e := communityReadPrincipal(ctx, tx, p); e != nil {
		return e
	}
	scope, args := communityVisibleScope(p)
	personal := `EXISTS(SELECT 1 FROM json_each(v.homes_json) mh JOIN flat_memberships mm ON mm.flat_id=json_extract(mh.value,'$.id') WHERE mm.resident_id=? AND mm.start_date<=? AND (mm.end_date IS NULL OR mm.end_date>?))`
	missing := `v.ack_required=1 AND ` + personal + ` AND NOT EXISTS(SELECT 1 FROM meeting_acknowledgements a WHERE a.resource_id=h.id AND a.publication_version=v.version AND a.resident_id=?)`
	base := ` FROM meeting_resources h JOIN meeting_versions v ON v.resource_id=h.id AND v.version=h.published_version WHERE v.action IN('AGENDA','MINUTES') AND ` + scope
	upcoming := `v.action='AGENDA' AND v.start_at>? AND v.start_at<=?`
	missingArgs := []any{p.ResidentID, today(), today(), p.ResidentID}
	countArgs := append([]any{out.AsOf, out.AsOf + 30*86400}, missingArgs...)
	countArgs = append(countArgs, args...)
	if e := overviewCounts(ctx, tx, out, []string{"upcoming", "acknowledgements_needed"}, "SELECT COUNT(CASE WHEN "+upcoming+" THEN 1 END),COUNT(CASE WHEN "+missing+" THEN 1 END)"+base, countArgs...); e != nil {
		return e
	}
	out.Counts["needs_your_decision"] = 0
	out.Counts["awaiting_other_reviewer"] = 0
	if p.CanReviewRequests {
		if e := overviewCounts(ctx, tx, out, []string{"needs_your_decision", "awaiting_other_reviewer"}, `SELECT COUNT(CASE WHEN v.submitted_by<>? THEN 1 END),COUNT(CASE WHEN v.submitted_by=? THEN 1 END) FROM meeting_resources h JOIN meeting_versions v ON v.resource_id=h.id AND v.version=h.pending_version`, p.ID, p.ID); e != nil {
			return e
		}
	}
	pub := `SELECT h.id,v.title,'','MEETING',CASE WHEN ` + missing + ` THEN 'ACKNOWLEDGEMENT' ELSE 'UPCOMING' END AS state,'NORMAL','',v.start_at,0` + base + ` AND ((` + upcoming + `) OR (` + missing + `))`
	itemArgs := append([]any{}, missingArgs...)
	itemArgs = append(itemArgs, args...)
	itemArgs = append(itemArgs, out.AsOf, out.AsOf+30*86400)
	itemArgs = append(itemArgs, missingArgs...)
	if p.CanReviewRequests {
		pub += ` UNION ALL SELECT h.id,v.title,'','MEETING','PENDING','NORMAL','',v.submitted_at,0 FROM meeting_resources h JOIN meeting_versions v ON v.resource_id=h.id AND v.version=h.pending_version WHERE v.submitted_by<>?`
		itemArgs = append(itemArgs, p.ID)
	}
	// Upcoming and personally outstanding describe one published row, while a
	// pending separate review retains its own deliberately private destination.
	var attention int64
	if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+pub+")", itemArgs...).Scan(&attention); e != nil {
		return e
	}
	out.Counts["attention_items"] = attention
	return overviewItems(ctx, tx, out, "SELECT * FROM ("+pub+") ORDER BY CASE state WHEN 'ACKNOWLEDGEMENT' THEN 0 WHEN 'PENDING' THEN 1 ELSE 2 END,8,1 LIMIT 4", itemArgs...)
}
