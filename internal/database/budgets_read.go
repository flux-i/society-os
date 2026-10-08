package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const budgetStateSQL = `CASE WHEN a.action='CLOSE' THEN 'CLOSED' WHEN h.approved_version IS NOT NULL THEN 'OPEN' ELSE 'UNAPPROVED' END`

func (s *Store) BudgetsFor(ctx context.Context, token, query, state string, page int) (BudgetPage, error) {
	out := BudgetPage{Items: []BudgetResource{}, Page: page, PageSize: 12, Counts: map[string]int64{}}
	query = strings.TrimSpace(query)
	if !validText(query, 0, 100) || page < 1 || page > 10000 || (state != "" && state != "OPEN" && state != "CLOSED" && state != "UNAPPROVED" && state != "PENDING") {
		return out, ErrInvalid
	}
	tx, p, e := s.beginBudgetRead(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	out.CanPrepare = p.CanManageRecords
	var open, closed, pending, unapproved int64
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(h.approved_version IS NOT NULL AND a.action='PLAN'),0),COALESCE(SUM(a.action='CLOSE'),0),COALESCE(SUM(h.pending_version IS NOT NULL),0),COALESCE(SUM(h.approved_version IS NULL),0) FROM budget_resources h LEFT JOIN budget_versions a ON a.resource_id=h.id AND a.version=h.approved_version`).Scan(&open, &closed, &pending, &unapproved)
	if e != nil {
		return out, e
	}
	out.Counts = map[string]int64{"open": open, "closed": closed, "pending": pending, "unapproved": unapproved}
	from := ` FROM budget_resources h JOIN budget_versions v ON v.resource_id=h.id AND v.version=h.latest_version LEFT JOIN budget_versions a ON a.resource_id=h.id AND a.version=h.approved_version WHERE 1=1`
	args := []any{}
	if query != "" {
		from += ` AND (instr(lower(json_extract(v.snapshot_json,'$.title')),?)>0 OR instr(lower(json_extract(v.snapshot_json,'$.source_reference')),?)>0)`
		args = append(args, strings.ToLower(query), strings.ToLower(query))
	}
	if state == "PENDING" {
		from += " AND h.pending_version IS NOT NULL"
	} else if state != "" {
		from += " AND (" + budgetStateSQL + ")=?"
		args = append(args, state)
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, "SELECT h.id"+from+" ORDER BY json_extract(v.snapshot_json,'$.period_start') DESC,h.created_at DESC,h.id LIMIT 12 OFFSET ?", append(args, (page-1)*12)...)
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
		h, err := budgetHeadIn(ctx, tx, id)
		if err != nil {
			return out, err
		}
		x, err := budgetResourceIn(ctx, tx, p, h)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	raw, e := json.Marshal(out)
	if e != nil {
		return out, e
	}
	out.CurrentKey = TokenHash(string(raw))
	return out, tx.Commit()
}
func (s *Store) BudgetFor(ctx context.Context, token, id string, eventPage int) (BudgetDetail, error) {
	out := BudgetDetail{Events: []BudgetEvent{}, EventPage: eventPage, PageSize: 12}
	if len(id) < 1 || len(id) > 100 || eventPage < 1 || eventPage > 10000 {
		return out, ErrInvalid
	}
	tx, p, e := s.beginBudgetRead(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	h, e := budgetHeadIn(ctx, tx, id)
	if e != nil {
		return out, e
	}
	out.BudgetResource, e = budgetResourceIn(ctx, tx, p, h)
	if e != nil {
		return out, e
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM budget_events WHERE resource_id=?", id).Scan(&out.EventTotal); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT e.version,e.proposal_version,e.action,u.display_name,e.reason,e.occurred_at,v.snapshot_json FROM budget_events e JOIN users u ON u.id=e.actor_id JOIN budget_versions v ON v.resource_id=e.resource_id AND v.version=e.proposal_version WHERE e.resource_id=? ORDER BY e.version DESC LIMIT 12 OFFSET ?`, id, (eventPage-1)*12)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var x BudgetEvent
		var raw string
		if e = rows.Scan(&x.Version, &x.ProposalVersion, &x.Action, &x.Actor, &x.Reason, &x.At, &raw); e != nil {
			rows.Close()
			return out, e
		}
		if e = json.Unmarshal([]byte(raw), &x.Snapshot); e != nil {
			rows.Close()
			return out, e
		}
		out.Events = append(out.Events, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	out.CurrentKey = TokenHash(fmt.Sprintf("%s:%d:%d", id, h.Version, out.EventTotal))
	return out, tx.Commit()
}
func (s *Store) BudgetComparisonFor(ctx context.Context, token, id string, expectedVersion int) (BudgetComparison, error) {
	out := BudgetComparison{}
	if len(id) < 1 || len(id) > 100 || expectedVersion < 0 || expectedVersion > 1000000 {
		return out, ErrInvalid
	}
	tx, _, e := s.beginBudgetRead(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	h, e := budgetHeadIn(ctx, tx, id)
	if e != nil {
		return out, e
	}
	if expectedVersion > 0 && h.Version != expectedVersion {
		return out, ErrConflict
	}
	if h.Approved == 0 {
		out.CurrentKey = TokenHash(fmt.Sprintf("unapproved:%s:%d", id, h.Version))
		out.CheckedAt = time.Now().Unix()
		return out, tx.Commit()
	}
	plan, e := budgetSnapshotIn(ctx, tx, id, h.Approved)
	if e != nil {
		return out, e
	}
	out.Available = true
	out.PeriodStart = plan.PeriodStart
	out.PeriodEnd = plan.PeriodEnd
	out.PlanVersion = plan.PlanVersion
	out.PlannedCollectionsPaise = plan.CollectionsPaise
	out.PlannedExpensesPaise = plan.ExpensesPaise
	e = tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(e.amount_paise),0),COALESCE(SUM(CASE WHEN r.entry_id IS NOT NULL THEN e.amount_paise ELSE 0 END),0) FROM entries e LEFT JOIN entry_reversals r ON r.entry_id=e.id WHERE e.kind='RECEIVED' AND e.state='POSTED' AND e.entry_date BETWEEN ? AND ?`, plan.PeriodStart, plan.PeriodEnd).Scan(&out.ReceiptCount, &out.OriginalCollectionsPaise, &out.ReversedCollectionsPaise)
	if e != nil {
		return out, e
	}
	var expenseSources int
	e = tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(h.approved_version IS NOT NULL AND v.action<>'VOID'),0),COALESCE(SUM(CASE WHEN h.approved_version IS NOT NULL THEN json_extract(v.snapshot_json,'$.amount_paise') ELSE 0 END),0),COALESCE(SUM(h.pending_version IS NOT NULL),0) FROM paid_expense_resources h LEFT JOIN paid_expense_versions v ON v.resource_id=h.id AND v.version=h.approved_version WHERE h.budget_id=?`, id).Scan(&expenseSources, &out.ExpenseCount, &out.RecordedExpensesPaise, &out.PendingExpenses)
	if e != nil {
		return out, e
	}
	if out.ReceiptCount > 10000 || expenseSources > 10000 {
		return out, invalid("This comparison exceeds the 10,000-source limit. No partial totals are displayed.")
	}
	out.CollectionsPaise = out.OriginalCollectionsPaise - out.ReversedCollectionsPaise
	out.CollectionVariancePaise = out.CollectionsPaise - plan.CollectionsPaise
	out.ExpenseHeadroomPaise = plan.ExpensesPaise - out.RecordedExpensesPaise
	out.DifferencePaise = out.CollectionsPaise - out.RecordedExpensesPaise
	var receipts, expenses string
	e = tx.QueryRowContext(ctx, `SELECT json_group_array(json_array(id,amount_paise,entry_date,reversed_at)) FROM(SELECT e.id,e.amount_paise,e.entry_date,COALESCE(r.created_at,0) reversed_at FROM entries e LEFT JOIN entry_reversals r ON r.entry_id=e.id WHERE e.kind='RECEIVED' AND e.state='POSTED' AND e.entry_date BETWEEN ? AND ? ORDER BY e.id)`, plan.PeriodStart, plan.PeriodEnd).Scan(&receipts)
	if e != nil {
		return out, e
	}
	e = tx.QueryRowContext(ctx, `SELECT json_group_array(json_array(id,version,approved_version,pending_version)) FROM(SELECT id,version,approved_version,pending_version FROM paid_expense_resources WHERE budget_id=? ORDER BY id)`, id).Scan(&expenses)
	if e != nil {
		return out, e
	}
	raw, e := json.Marshal(struct {
		ID                 string
		Version            int
		Comparison         BudgetComparison
		Receipts, Expenses string
	}{id, h.Version, out, receipts, expenses})
	if e != nil {
		return out, e
	}
	out.CurrentKey = TokenHash(string(raw))
	out.CheckedAt = time.Now().Unix()
	return out, tx.Commit()
}
