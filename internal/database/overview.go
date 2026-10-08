package database

import (
	"context"
	"database/sql"
	"time"
)

// Overview contains metadata only. Each section is a separate authenticated
// snapshot so an unavailable source does not prevent reading other sections.
type Overview struct {
	Section     string           `json:"section"`
	AsOf        int64            `json:"as_of"`
	Day         string           `json:"day"`
	PeriodStart string           `json:"period_start"`
	Calendar    string           `json:"calendar"`
	Counts      map[string]int64 `json:"counts"`
	Items       []OverviewItem   `json:"items"`
}
type OverviewItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Home        string `json:"home"`
	Kind        string `json:"kind"`
	State       string `json:"state"`
	Priority    string `json:"priority"`
	Date        string `json:"date"`
	At          int64  `json:"at"`
	AmountPaise int64  `json:"amount_paise"`
}

func overviewCounts(ctx context.Context, tx *sql.Tx, out *Overview, names []string, query string, args ...any) error {
	values := make([]int64, len(names))
	pointers := make([]any, len(names))
	for i := range pointers {
		pointers[i] = &values[i]
	}
	if err := tx.QueryRowContext(ctx, query, args...).Scan(pointers...); err != nil {
		return err
	}
	for i, name := range names {
		out.Counts[name] = values[i]
	}
	return nil
}
func overviewItems(ctx context.Context, tx *sql.Tx, out *Overview, query string, args ...any) error {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var x OverviewItem
		if err = rows.Scan(&x.ID, &x.Title, &x.Home, &x.Kind, &x.State, &x.Priority, &x.Date, &x.At, &x.AmountPaise); err != nil {
			return err
		}
		out.Items = append(out.Items, x)
	}
	return rows.Err()
}

func overviewWindow(now time.Time) (string, string) {
	now = now.In(societyZone)
	return now.Format("2006-01-02"), now.AddDate(0, 0, -29).Format("2006-01-02")
}
func (s *Store) OverviewFor(ctx context.Context, token, section string) (Overview, error) {
	now := time.Now()
	day, start := overviewWindow(now)
	out := Overview{Section: section, AsOf: now.Unix(), Day: day, PeriodStart: start, Calendar: "Asia/Kolkata", Counts: map[string]int64{}, Items: []OverviewItem{}}
	if section != "finance" && section != "reviews" && section != "service" && section != "notices" && section != "documents" && section != "maintenance" && section != "upkeep" && section != "collections" && section != "incidents" && section != "fines" && section != "messages" && section != "statements" && section != "community" && section != "meetings" {
		return out, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), now)
	if err != nil {
		return out, err
	}
	switch section {
	case "meetings":
		err = overviewMeetings(ctx, tx, p, &out)
	case "community":
		err = overviewCommunity(ctx, tx, p, &out)
	case "statements":
		err = overviewStatements(ctx, tx, p, &out)
	case "messages":
		err = overviewMessages(ctx, tx, p, &out)
	case "fines":
		err = overviewFines(ctx, tx, p, &out)
	case "incidents":
		err = overviewIncidents(ctx, tx, p, &out)
	case "collections":
		err = overviewCollections(ctx, tx, p, &out)
	case "upkeep":
		err = overviewUpkeep(ctx, tx, p, &out)
	case "finance":
		err = overviewFinance(ctx, tx, p, &out)
	case "maintenance":
		err = overviewMaintenance(ctx, tx, p, &out)
	case "reviews":
		err = overviewReviews(ctx, tx, p, &out)
	case "service":
		err = overviewService(ctx, tx, p, &out)
	case "notices":
		err = overviewNotices(ctx, tx, p, &out)
	case "documents":
		err = overviewDocuments(ctx, tx, p, &out)
	}
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

func overviewMaintenance(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	if !p.CanReadRecords {
		return ErrForbidden
	}
	query, args := maintenanceCycleQuery(p, "", "", "")
	counts := `SELECT COUNT(CASE WHEN state='PENDING' AND submitted_by<>? AND ?=1 THEN 1 END),
        COUNT(CASE WHEN state='PENDING' AND submitted_by=? THEN 1 END),
        COUNT(CASE WHEN state='PUBLISHED' THEN 1 END),
        COALESCE(SUM(active),0),COALESCE(SUM(allocated),0),COALESCE(SUM(outstanding),0),COALESCE(SUM(overdue),0),
        COUNT(CASE WHEN state='PUBLISHED' AND overdue>0 THEN 1 END) FROM (` + query + ")"
	// The outer SELECT parameters precede those inside the scoped subquery.
	if err := overviewCounts(ctx, tx, out, []string{"pending_review", "awaiting_other_reviewer", "published_periods", "active_paise", "allocated_paise", "outstanding_paise", "overdue_paise", "overdue_periods"}, counts, append([]any{p.ID, p.CanManageRecords, p.ID}, args...)...); err != nil {
		return err
	}
	items := `SELECT id,title,'','MAINTENANCE',CASE WHEN state='PENDING' THEN 'PENDING' ELSE 'OVERDUE' END,'NORMAL',due_date,submitted_at,outstanding
        FROM (` + query + `) WHERE (state='PENDING' AND submitted_by<>? AND ?=1) OR (state='PUBLISHED' AND overdue>0)
        ORDER BY CASE WHEN state='PENDING' THEN 0 ELSE 1 END,due_date,submitted_at,id LIMIT 4`
	return overviewItems(ctx, tx, out, items, append(args, p.ID, p.CanManageRecords)...)
}

func overviewFinance(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	if !p.CanReadRecords {
		return ErrForbidden
	}
	scope, args := recordScope(p)
	posted := `e.state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id)`
	query := `WITH balances AS (SELECT e.flat_id,SUM(CASE WHEN e.kind IN('CHARGE','OPENING_DEBIT') THEN e.amount_paise ELSE -e.amount_paise+COALESCE((SELECT SUM(fc.amount_paise) FROM live_fund_contributions fc WHERE fc.source_id=e.id),0) END) AS balance FROM entries e WHERE ` + scope + ` AND ` + posted + ` GROUP BY e.flat_id) SELECT COALESCE(SUM(balance),0),COALESCE(SUM(MAX(balance,0)),0),COALESCE(SUM(MAX(-balance,0)),0),COUNT(CASE WHEN balance>0 THEN 1 END) FROM balances`
	if err := overviewCounts(ctx, tx, out, []string{"balance_paise", "positive_balance_paise", "credit_balance_paise", "homes_with_balance"}, query, args...); err != nil {
		return err
	}
	if err := overviewCounts(ctx, tx, out, []string{"received_paise"}, `SELECT COALESCE(SUM(e.amount_paise),0) FROM entries e WHERE `+scope+` AND `+posted+` AND e.kind='RECEIVED' AND e.entry_date BETWEEN ? AND ?`, append(append([]any{}, args...), out.PeriodStart, out.Day)...); err != nil {
		return err
	}
	if err := overviewCounts(ctx, tx, out, []string{"voluntary_paise"}, "SELECT COALESCE(SUM((SELECT COALESCE(SUM(fc.amount_paise),0) FROM live_fund_contributions fc WHERE fc.source_id=e.id)),0) FROM entries e WHERE "+scope, args...); err != nil {
		return err
	}
	draft := `e.state='DRAFT' AND NOT EXISTS(SELECT 1 FROM draft_discards d WHERE d.entry_id=e.id)`
	if err := overviewCounts(ctx, tx, out, []string{"awaiting_confirmation"}, `SELECT COUNT(*) FROM entries e WHERE `+scope+` AND `+draft+` AND ?=1`, append(append([]any{}, args...), p.CanManageRecords)...); err != nil {
		return err
	}
	if err := overviewCounts(ctx, tx, out, []string{"receipt_pending", "receipt_failed"}, `SELECT COUNT(CASE WHEN j.state IN('PENDING','RUNNING') THEN 1 END),COUNT(CASE WHEN j.state='FAILED' THEN 1 END) FROM entries e JOIN receipts r ON r.entry_id=e.id JOIN receipt_jobs j ON j.receipt_id=r.id WHERE `+scope+` AND `+posted, args...); err != nil {
		return err
	}
	query = `SELECT e.id,e.description,b.code||'-'||f.flat_number,'ENTRY',CASE WHEN e.state='DRAFT' THEN 'DRAFT' ELSE j.state END,CASE WHEN j.state='FAILED' THEN 'HIGH' ELSE 'NORMAL' END,e.entry_date,e.created_at,e.amount_paise FROM entries e JOIN flats f ON f.id=e.flat_id JOIN buildings b ON b.id=f.building_id LEFT JOIN receipts r ON r.entry_id=e.id LEFT JOIN receipt_jobs j ON j.receipt_id=r.id WHERE ` + scope + ` AND ((` + draft + ` AND ?=1) OR (` + posted + ` AND j.state IN('PENDING','RUNNING','FAILED'))) ORDER BY (j.state='FAILED') DESC,e.created_at,e.id LIMIT 4`
	return overviewItems(ctx, tx, out, query, append(args, p.CanManageRecords)...)
}

func overviewReviews(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	scope, args := reviewPrivateScope(p)
	query := `SELECT COUNT(CASE WHEN x.state='PENDING' AND x.submitted_by<>? AND ?=1 THEN 1 END),COUNT(CASE WHEN x.state='PENDING' AND x.submitted_by=? THEN 1 END),COUNT(CASE WHEN x.state='CHANGES_REQUESTED' AND x.submitted_by=? THEN 1 END) FROM review_requests x WHERE ` + scope
	if err := overviewCounts(ctx, tx, out, []string{"needs_your_decision", "awaiting_others", "changes_requested"}, query, append([]any{p.ID, p.CanReviewRequests, p.ID, p.ID}, args...)...); err != nil {
		return err
	}
	query = `SELECT x.id,x.title,COALESCE(b.code||'-'||f.flat_number,''),x.kind,x.state,'NORMAL','',x.updated_at,0 FROM review_requests x LEFT JOIN flats f ON f.id=x.flat_id LEFT JOIN buildings b ON b.id=f.building_id WHERE ` + scope + ` AND ((x.state='PENDING' AND x.submitted_by<>? AND ?=1) OR (x.state='CHANGES_REQUESTED' AND x.submitted_by=?)) ORDER BY (x.state='CHANGES_REQUESTED') DESC,x.updated_at,x.id LIMIT 4`
	return overviewItems(ctx, tx, out, query, append(args, p.ID, p.CanReviewRequests, p.ID)...)
}

func overviewService(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	scope, args := complaintScope(p)
	active := `c.status IN('OPEN','ACKNOWLEDGED','IN_PROGRESS','WAITING')`
	query := `SELECT COUNT(CASE WHEN ` + active + ` THEN 1 END),COUNT(CASE WHEN ` + active + ` AND c.priority='URGENT' THEN 1 END),COUNT(CASE WHEN ` + active + ` AND c.assigned_to IS NULL AND ?=1 THEN 1 END),COUNT(CASE WHEN c.status='RESOLVED' AND c.reported_by=? THEN 1 END) FROM complaints c WHERE ` + scope
	if err := overviewCounts(ctx, tx, out, []string{"active", "urgent", "unassigned", "needs_closure"}, query, append([]any{p.CanHandleComplaints, p.ID}, args...)...); err != nil {
		return err
	}
	updated := "c.public_updated_at"
	if p.CanHandleComplaints {
		updated = "c.updated_at"
	}
	query = `SELECT c.id,c.subject,b.code||'-'||f.flat_number,c.category,c.status,c.priority,'',` + updated + `,0 FROM complaints c JOIN flats f ON f.id=c.flat_id JOIN buildings b ON b.id=f.building_id WHERE ` + scope + ` AND (` + active + ` OR (c.status='RESOLVED' AND c.reported_by=?)) ORDER BY CASE c.priority WHEN 'URGENT' THEN 0 WHEN 'HIGH' THEN 1 ELSE 2 END,` + updated + `,c.id LIMIT 4`
	return overviewItems(ctx, tx, out, query, append(args, p.ID)...)
}

func overviewNotices(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	scope, args := noticeScope(p)
	start, _ := time.ParseInLocation("2006-01-02", out.PeriodStart, societyZone)
	query := `SELECT COUNT(*),COUNT(CASE WHEN x.updated_at BETWEEN ? AND ? THEN 1 END) FROM review_requests x WHERE ` + scope
	if err := overviewCounts(ctx, tx, out, []string{"total", "recent"}, query, append([]any{start.Unix(), out.AsOf}, args...)...); err != nil {
		return err
	}
	return overviewItems(ctx, tx, out, `SELECT x.id,x.title,'','NOTICE',x.audience,'NORMAL','',x.updated_at,0 FROM review_requests x WHERE `+scope+` ORDER BY x.updated_at DESC,x.id LIMIT 4`, args...)
}

func overviewDocuments(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	scope, args := documentScope(p)
	args = append(args, sql.Named("now", out.AsOf), sql.Named("end", time.Unix(out.AsOf, 0).In(societyZone).AddDate(0, 0, 30).Format("2006-01-02")))
	review := `(g.archived=0 AND x.review_state='PENDING' AND x.validation_status='AVAILABLE' AND x.uploaded_by<>:actor AND ((x.category IN ` + financialDocumentCategories + ` AND :review_finance=1) OR (x.category NOT IN ` + financialDocumentCategories + ` AND :review_docs=1)))`
	upload := `(x.review_state='PENDING' AND x.uploaded_by=:actor AND (x.validation_status IN('REJECTED','ABANDONED') OR (x.validation_status='PENDING' AND x.uploaded_at IS NULL AND x.expires_at>:now)))`
	deadline := `(x.review_state='APPROVED' AND g.current_document_id=x.id AND g.archived=0 AND x.category IN('CONTRACT','AMC') AND x.expiry_date IS NOT NULL)`
	query := `SELECT COUNT(CASE WHEN ` + review + ` THEN 1 END),COUNT(CASE WHEN ` + upload + ` THEN 1 END),COUNT(CASE WHEN ` + deadline + ` AND x.expiry_date BETWEEN :day AND :end THEN 1 END),COUNT(CASE WHEN ` + deadline + ` AND x.expiry_date<:day THEN 1 END) FROM library_documents x JOIN document_groups g ON g.id=x.group_id WHERE ` + scope
	if err := overviewCounts(ctx, tx, out, []string{"ready_review", "upload_attention", "expiring", "expired"}, query, args...); err != nil {
		return err
	}
	query = `SELECT x.id,x.title,COALESCE(b.code||'-'||f.flat_number,''),x.category,CASE WHEN ` + upload + ` THEN 'UPLOAD_ATTENTION' WHEN ` + review + ` THEN 'READY_REVIEW' WHEN x.expiry_date<:day THEN 'EXPIRED' ELSE 'EXPIRING' END,'NORMAL',COALESCE(x.expiry_date,''),x.created_at,0 FROM library_documents x JOIN document_groups g ON g.id=x.group_id LEFT JOIN flats f ON f.id=x.flat_id LEFT JOIN buildings b ON b.id=f.building_id WHERE ` + scope + ` AND (` + upload + ` OR ` + review + ` OR (` + deadline + ` AND x.expiry_date<=:end)) ORDER BY CASE WHEN ` + upload + ` THEN 0 WHEN ` + review + ` THEN 1 ELSE 2 END,COALESCE(x.expiry_date,'9999'),x.created_at,x.id LIMIT 4`
	return overviewItems(ctx, tx, out, query, args...)
}
