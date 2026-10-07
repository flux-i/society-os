package database

import (
	"context"
	"database/sql"
)

func overviewCommunity(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	if e := communityReadPrincipal(ctx, tx, p); e != nil {
		return e
	}
	scope, args := communityVisibleScope(p)
	active := `SELECT h.id,v.title,v.service,v.start_at,v.estimated_end FROM community_resources h JOIN community_versions v ON v.resource_id=h.id AND v.version=h.published_version WHERE v.kind='INTERRUPTION' AND v.action='PUBLISH' AND v.start_at<=? AND ` + scope
	activeArgs := append([]any{out.AsOf}, args...)
	if e := overviewCounts(ctx, tx, out, []string{"active", "update_needed"}, "SELECT COUNT(*),COUNT(CASE WHEN estimated_end>0 AND estimated_end<=? THEN 1 END) FROM ("+active+")", append([]any{out.AsOf}, activeArgs...)...); e != nil {
		return e
	}
	out.Counts["needs_your_decision"] = 0
	out.Counts["awaiting_other_reviewer"] = 0
	if p.CanReviewRequests {
		if e := overviewCounts(ctx, tx, out, []string{"needs_your_decision", "awaiting_other_reviewer"}, `SELECT COUNT(CASE WHEN v.submitted_by<>? THEN 1 END),COUNT(CASE WHEN v.submitted_by=? THEN 1 END) FROM community_resources h JOIN community_versions v ON v.resource_id=h.id AND v.version=h.pending_version`, p.ID, p.ID); e != nil {
			return e
		}
	}
	query := `SELECT id,title,'',service,CASE WHEN estimated_end>0 AND estimated_end<=? THEN 'UPDATE_NEEDED' ELSE 'ACTIVE' END AS state,'HIGH','',start_at,0 FROM (` + active + `)`
	itemArgs := append([]any{out.AsOf}, activeArgs...)
	if p.CanReviewRequests {
		query += ` UNION ALL SELECT h.id,v.title,'',v.kind,'PENDING','NORMAL','',v.submitted_at,0 FROM community_resources h JOIN community_versions v ON v.resource_id=h.id AND v.version=h.pending_version WHERE v.submitted_by<>?`
		itemArgs = append(itemArgs, p.ID)
	}
	return overviewItems(ctx, tx, out, "SELECT * FROM ("+query+") ORDER BY (state='PENDING'),8,1 LIMIT 4", itemArgs...)
}
