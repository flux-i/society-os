package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *Store) PaidExpensesFor(ctx context.Context, token, budget, query, state string, page int) (PaidExpensePage, error) {
	out := PaidExpensePage{Items: []PaidExpenseResource{}, Page: page, PageSize: 12, Counts: map[string]int64{}}
	query = strings.TrimSpace(query)
	if len(budget) < 1 || len(budget) > 100 || !validText(query, 0, 100) || page < 1 || page > 10000 || (state != "" && state != "PENDING" && state != "CONFIRMED" && state != "VOID" && state != "UNCONFIRMED") {
		return out, ErrInvalid
	}
	tx, p, e := s.beginBudgetRead(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	bh, e := budgetHeadIn(ctx, tx, budget)
	if e != nil {
		return out, e
	}
	var pending, confirmed, void, unconfirmed int64
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(h.pending_version IS NOT NULL),0),COALESCE(SUM(h.approved_version IS NOT NULL AND a.action<>'VOID'),0),COALESCE(SUM(a.action='VOID'),0),COALESCE(SUM(h.approved_version IS NULL),0) FROM paid_expense_resources h LEFT JOIN paid_expense_versions a ON a.resource_id=h.id AND a.version=h.approved_version WHERE h.budget_id=?`, budget).Scan(&pending, &confirmed, &void, &unconfirmed)
	if e != nil {
		return out, e
	}
	out.Counts = map[string]int64{"pending": pending, "confirmed": confirmed, "void": void, "unconfirmed": unconfirmed}
	from := ` FROM paid_expense_resources h JOIN paid_expense_versions v ON v.resource_id=h.id AND v.version=h.latest_version LEFT JOIN paid_expense_versions a ON a.resource_id=h.id AND a.version=h.approved_version WHERE h.budget_id=?`
	args := []any{budget}
	if query != "" {
		from += ` AND EXISTS(SELECT 1 FROM paid_expense_versions s WHERE s.resource_id=h.id AND (instr(lower(json_extract(s.snapshot_json,'$.payee')),?)>0 OR instr(lower(json_extract(s.snapshot_json,'$.category')),?)>0 OR instr(lower(json_extract(s.snapshot_json,'$.reference')),?)>0))`
		q := strings.ToLower(query)
		args = append(args, q, q, q)
	}
	switch state {
	case "PENDING":
		from += " AND h.pending_version IS NOT NULL"
	case "CONFIRMED":
		from += " AND h.approved_version IS NOT NULL AND a.action<>'VOID'"
	case "VOID":
		from += " AND a.action='VOID'"
	case "UNCONFIRMED":
		from += " AND h.approved_version IS NULL"
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, "SELECT h.id"+from+" ORDER BY h.created_at DESC,h.id LIMIT 12 OFFSET ?", append(args, (page-1)*12)...)
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
		h, b, err := expenseHeadIn(ctx, tx, id)
		if err != nil {
			return out, err
		}
		x, err := expenseResourceIn(ctx, tx, p, h, b)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	raw, e := json.Marshal(struct {
		BudgetVersion int
		Page          PaidExpensePage
	}{bh.Version, out})
	if e != nil {
		return out, e
	}
	out.CurrentKey = TokenHash(string(raw))
	return out, tx.Commit()
}
func (s *Store) PaidExpenseFor(ctx context.Context, token, id string, eventPage int) (PaidExpenseDetail, error) {
	out := PaidExpenseDetail{Events: []PaidExpenseEvent{}, EventPage: eventPage, PageSize: 12}
	if len(id) < 1 || len(id) > 100 || eventPage < 1 || eventPage > 10000 {
		return out, ErrInvalid
	}
	tx, p, e := s.beginBudgetRead(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	h, budget, e := expenseHeadIn(ctx, tx, id)
	if e != nil {
		return out, e
	}
	out.PaidExpenseResource, e = expenseResourceIn(ctx, tx, p, h, budget)
	if e != nil {
		return out, e
	}
	bh, e := budgetHeadIn(ctx, tx, budget)
	if e != nil {
		return out, e
	}
	plan, e := budgetSnapshotIn(ctx, tx, budget, bh.Latest)
	if e != nil {
		return out, e
	}
	if bh.Approved > 0 {
		plan, e = budgetSnapshotIn(ctx, tx, budget, bh.Approved)
		if e != nil {
			return out, e
		}
	}
	out.BudgetTitle = plan.Title
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM paid_expense_events WHERE resource_id=?", id).Scan(&out.EventTotal); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT e.version,e.proposal_version,e.action,u.display_name,e.reason,e.occurred_at,v.snapshot_json FROM paid_expense_events e JOIN users u ON u.id=e.actor_id JOIN paid_expense_versions v ON v.resource_id=e.resource_id AND v.version=e.proposal_version WHERE e.resource_id=? ORDER BY e.version DESC LIMIT 12 OFFSET ?`, id, (eventPage-1)*12)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var x PaidExpenseEvent
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
	out.CurrentKey = TokenHash(fmt.Sprintf("%s:%d:%d:%d", id, h.Version, bh.Version, out.EventTotal))
	return out, tx.Commit()
}
