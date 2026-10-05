package database

import (
	"context"
	"database/sql"
	"time"
)

func overviewUpkeep(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	if err := upkeepTaskCounts(ctx, tx, p, out.Counts); err != nil {
		return err
	}
	end := time.Now().In(societyZone).AddDate(0, 0, 30).Format("2006-01-02")
	visits := time.Now().In(societyZone).AddDate(0, 0, 7).Format("2006-01-02")
	for _, key := range []string{"amc_expired", "amc_expiring", "inspection_overdue", "inspection_upcoming"} {
		out.Counts[key] = 0
	}
	if p.CanHandleComplaints {
		if err := overviewCounts(ctx, tx, out, []string{"amc_expired", "amc_expiring", "inspection_overdue", "inspection_upcoming"}, `SELECT COUNT(CASE WHEN amc_end!='' AND amc_end<? THEN 1 END),COUNT(CASE WHEN amc_end BETWEEN ? AND ? THEN 1 END),COUNT(CASE WHEN inspection_date!='' AND inspection_date<? THEN 1 END),COUNT(CASE WHEN inspection_date BETWEEN ? AND ? THEN 1 END) FROM upkeep_register WHERE kind='ASSET' AND state='ACTIVE'`, out.Day, out.Day, end, out.Day, out.Day, visits); err != nil {
			return err
		}
	}
	scope, args := upkeepTaskScope(p)
	title, state, priority, due, visit := upkeepVisible("title", p), upkeepVisible("state", p), upkeepVisible("priority", p), upkeepVisible("due_date", p), upkeepVisible("visit_date", p)
	at := "t.published_at"
	attention := due + "<? OR " + visit + " BETWEEN ? AND ?"
	args = append(args, out.Day, out.Day, visits)
	if p.CanHandleComplaints {
		at = "t.updated_at"
		attention += " OR t.assigned_to IS NULL OR (t.state='READY_FOR_CHECK' AND t.ready_by!=?)"
		args = append(args, p.ID)
	}
	query := `SELECT t.id,` + title + ` title,'' home,'UPKEEP' kind,` + state + ` state,` + priority + ` priority,` + due + ` date,` + at + ` at,0 amount FROM upkeep_tasks t WHERE ` + scope + ` AND ` + state + ` NOT IN ('DONE','CANCELLED') AND (` + attention + ")"
	if p.CanHandleComplaints {
		query += ` UNION ALL SELECT id,name,'','ASSET',CASE WHEN amc_end<? THEN 'AMC_EXPIRED' ELSE 'AMC_EXPIRING' END,'NORMAL',amc_end,updated_at,0 FROM upkeep_register WHERE kind='ASSET' AND state='ACTIVE' AND amc_end!='' AND amc_end<=? UNION ALL SELECT id,name,'','ASSET',CASE WHEN inspection_date<? THEN 'INSPECTION_OVERDUE' ELSE 'INSPECTION_DUE' END,'NORMAL',inspection_date,updated_at,0 FROM upkeep_register WHERE kind='ASSET' AND state='ACTIVE' AND inspection_date!='' AND inspection_date<=?`
		args = append(args, out.Day, end, out.Day, visits)
	}
	return overviewItems(ctx, tx, out, "SELECT * FROM ("+query+") ORDER BY date,CASE priority WHEN 'URGENT' THEN 0 WHEN 'HIGH' THEN 1 ELSE 2 END,id,kind,state LIMIT 4", args...)
}
