package database

import (
	"context"
	"strings"
)

// Scope, all counts, amounts and the bounded page use one current-auth read
// transaction. No private proposal is included in a resident's totals.
func maintenanceLinesQuery(p Principal, home string) (string, []any) {
	args := []any{}
	scope := "1=1"
	if !p.CanReadAllRecords {
		scope = `EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=l.flat_id AND m.resident_id=? AND m.can_view_finances=1 AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
		args = append(args, p.ResidentID, today(), today())
	}
	if home != "" {
		scope += " AND l.flat_id=?"
		args = append(args, home)
	}
	return `WITH used AS (SELECT charge_id,SUM(amount_paise) allocated FROM live_entry_allocations GROUP BY charge_id),
        visible_lines AS (SELECT l.*, b.code||'-'||f.flat_number home,
            CASE WHEN e.id IS NULL THEN 'PROPOSED' WHEN v.entry_id IS NOT NULL THEN 'REVERSED' ELSE 'POSTED' END line_state,
            CASE WHEN e.id IS NOT NULL AND v.entry_id IS NULL THEN l.amount_paise ELSE 0 END active,
            COALESCE(a.allocated,0) allocated,
            CASE WHEN e.id IS NOT NULL AND v.entry_id IS NULL THEN l.amount_paise-COALESCE(a.allocated,0) ELSE 0 END outstanding
            FROM maintenance_lines l JOIN flats f ON f.id=l.flat_id JOIN buildings b ON b.id=f.building_id
            LEFT JOIN entries e ON e.id=l.entry_id LEFT JOIN entry_reversals v ON v.entry_id=e.id
            LEFT JOIN used a ON a.charge_id=e.id WHERE ` + scope + ") ", args
}

func maintenanceCycleQuery(p Principal, home, query, state string) (string, []any) {
	cte, args := maintenanceLinesQuery(p, home)
	args = append(args, today())
	where := "1=1"
	if !p.CanReadAllRecords {
		where += " AND c.state='PUBLISHED'"
	}
	if query != "" {
		where += " AND instr(lower(c.title),?)>0"
		args = append(args, strings.ToLower(strings.TrimSpace(query)))
	}
	if state != "" {
		where += " AND c.state=?"
		args = append(args, state)
	}
	return cte + `SELECT c.id,c.title,c.period_start,c.period_end,c.due_date,c.source_reference,c.note,c.state,c.version,
        c.submitted_by,u.display_name,c.submitted_at,COALESCE(du.display_name,''),COALESCE(c.decided_at,0),c.decision_reason,
        f.participants,f.requested requested,f.active active,f.allocated allocated,f.outstanding outstanding,
        CASE WHEN c.state='PUBLISHED' AND c.due_date<? THEN f.outstanding ELSE 0 END overdue,f.reversed reversed
        FROM maintenance_cycles c JOIN users u ON u.id=c.submitted_by LEFT JOIN users du ON du.id=c.decided_by
        JOIN (SELECT cycle_id,COUNT(*) participants,SUM(amount_paise) requested,SUM(active) active,SUM(allocated) allocated,
            SUM(outstanding) outstanding,SUM(CASE WHEN line_state='REVERSED' THEN amount_paise ELSE 0 END) reversed
            FROM visible_lines GROUP BY cycle_id) f ON f.cycle_id=c.id WHERE ` + where, args
}

func scanMaintenanceCycle(row interface{ Scan(...any) error }, p Principal) (MaintenanceCycle, error) {
	var c MaintenanceCycle
	err := row.Scan(&c.ID, &c.Title, &c.PeriodStart, &c.PeriodEnd, &c.DueDate, &c.SourceReference, &c.Note, &c.State, &c.Version,
		&c.AuthorID, &c.Author, &c.SubmittedAt, &c.DecidedBy, &c.DecidedAt, &c.DecisionReason, &c.Participants,
		&c.RequestedPaise, &c.ActivePaise, &c.AllocatedPaise, &c.OutstandingPaise, &c.OverduePaise, &c.ReversedPaise)
	if !p.CanReadAllRecords {
		c.SourceReference, c.Note, c.AuthorID, c.Author, c.DecidedBy, c.DecisionReason = "", "", "", "", "", ""
	}
	return c, err
}

func validMaintenanceFilter(query, home, state string) bool {
	return validText(query, 0, 100) && len(home) <= 100 && (state == "" || state == "PENDING" || state == "PUBLISHED" || state == "DECLINED" || state == "WITHDRAWN")
}

func (s *Store) MaintenanceCyclesFor(ctx context.Context, token, query, home, state string, page int) (MaintenancePage, error) {
	result := MaintenancePage{Items: []MaintenanceCycle{}, PageSize: 12}
	if !validMaintenanceFilter(query, home, state) || !boundedMaintenancePage(page) {
		return result, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	result.Homes, err = financialHomes(ctx, tx, p)
	if err != nil {
		return result, err
	}
	queryText, args := maintenanceCycleQuery(p, home, query, state)
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM(requested),0),COALESCE(SUM(active),0),COALESCE(SUM(allocated),0),COALESCE(SUM(outstanding),0),COALESCE(SUM(overdue),0),COALESCE(SUM(reversed),0) FROM ("+
		queryText+")", args...).
		Scan(&result.Total, &result.Totals.RequestedPaise, &result.Totals.ActivePaise, &result.Totals.AllocatedPaise, &result.Totals.OutstandingPaise, &result.Totals.OverduePaise, &result.Totals.ReversedPaise)
	if err != nil {
		return result, err
	}
	result.Page = clampMaintenancePage(page, result.Total, result.PageSize)
	pageArgs := append(append([]any{}, args...), result.PageSize, (result.Page-1)*result.PageSize)
	rows, err := tx.QueryContext(ctx, queryText+" ORDER BY CASE WHEN c.state='PENDING' THEN 0 ELSE 1 END,c.period_start DESC,c.submitted_at DESC,c.id LIMIT ? OFFSET ?", pageArgs...)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		c, e := scanMaintenanceCycle(rows, p)
		if e != nil {
			rows.Close()
			return result, e
		}
		result.Items = append(result.Items, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func (s *Store) MaintenanceCycleFor(ctx context.Context, token, id string, linePage, eventPage int) (MaintenanceDetails, error) {
	result := MaintenanceDetails{Lines: []MaintenanceLine{}, PageSize: 20}
	if len(id) < 1 || len(id) > 100 || !boundedMaintenancePage(linePage) || !boundedMaintenancePage(eventPage) {
		return result, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	queryText, args := maintenanceCycleQuery(p, "", "", "")
	result.MaintenanceCycle, err = scanMaintenanceCycle(tx.QueryRowContext(ctx, queryText+" AND c.id=?", append(args, id)...), p)
	if err != nil {
		return result, err
	}
	result.LinePage = clampMaintenancePage(linePage, result.Participants, result.PageSize)
	cte, args := maintenanceLinesQuery(p, "")
	rows, err := tx.QueryContext(ctx, cte+"SELECT flat_id,home,COALESCE(entry_id,''),line_state,amount_paise,allocated,outstanding FROM visible_lines WHERE cycle_id=? ORDER BY home LIMIT ? OFFSET ?", append(args, id, result.PageSize, (result.LinePage-1)*result.PageSize)...)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var l MaintenanceLine
		if err = rows.Scan(&l.FlatID, &l.Home, &l.EntryID, &l.State, &l.AmountPaise, &l.AllocatedPaise, &l.OutstandingPaise); err != nil {
			rows.Close()
			return result, err
		}
		result.Lines = append(result.Lines, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if p.CanReadAllRecords {
		result.Events = []MaintenanceEvent{}
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM maintenance_events WHERE cycle_id=?", id).Scan(&result.EventTotal); err != nil {
			return result, err
		}
		result.EventPage = clampMaintenancePage(eventPage, result.EventTotal, result.PageSize)
		rows, err = tx.QueryContext(ctx, "SELECT e.action,e.version,u.display_name,e.reason,e.occurred_at FROM maintenance_events e JOIN users u ON u.id=e.actor_id WHERE e.cycle_id=? ORDER BY e.id DESC LIMIT ? OFFSET ?", id, result.PageSize, (result.EventPage-1)*result.PageSize)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var event MaintenanceEvent
			if err = rows.Scan(&event.Action, &event.Version, &event.Actor, &event.Reason, &event.At); err != nil {
				rows.Close()
				return result, err
			}
			result.Events = append(result.Events, event)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, err
		}
	}
	return result, tx.Commit()
}

const statementQuery = `WITH used_source AS (SELECT source_id,SUM(amount_paise) allocated FROM live_entry_allocations GROUP BY source_id),
    used_charge AS (SELECT charge_id,SUM(amount_paise) allocated FROM live_entry_allocations GROUP BY charge_id),
    active_statement AS (SELECT e.id,e.kind,e.description,e.entry_date,e.amount_paise,
        CASE WHEN e.kind IN ('CHARGE','OPENING_DEBIT') THEN COALESCE(c.allocated,0) ELSE COALESCE(s.allocated,0) END allocated,
        COALESCE(r.number,'') receipt,COALESCE(l.cycle_id,'') cycle_id,COALESCE(m.due_date,'') due_date
        FROM entries e LEFT JOIN used_source s ON s.source_id=e.id LEFT JOIN used_charge c ON c.charge_id=e.id
        LEFT JOIN receipts r ON r.entry_id=e.id LEFT JOIN maintenance_lines l ON l.entry_id=e.id
        LEFT JOIN maintenance_cycles m ON m.id=l.cycle_id AND m.state='PUBLISHED'
        WHERE e.flat_id=? AND e.state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals v WHERE v.entry_id=e.id)) `

func (s *Store) HomeStatementFor(ctx context.Context, token, home string, chargePage, creditPage, allocationPage int) (HomeStatement, error) {
	result := HomeStatement{FlatID: home, PageSize: 20, Charges: []StatementEntry{}, Credits: []StatementEntry{}, Allocations: []AllocationRecord{}}
	if len(home) < 1 || len(home) > 100 || !boundedMaintenancePage(chargePage) || !boundedMaintenancePage(creditPage) || !boundedMaintenancePage(allocationPage) {
		return result, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	result.Home, err = permittedFinancialHome(ctx, tx, p, home)
	if err != nil {
		return result, err
	}
	err = tx.QueryRowContext(ctx, statementQuery+`SELECT
        COALESCE(SUM(CASE WHEN kind IN ('CHARGE','OPENING_DEBIT') THEN amount_paise ELSE 0 END),0),
        COALESCE(SUM(CASE WHEN kind IN ('RECEIVED','OPENING_CREDIT') THEN amount_paise ELSE 0 END),0),
        COALESCE(SUM(CASE WHEN kind IN ('CHARGE','OPENING_DEBIT') THEN allocated ELSE 0 END),0),
        COALESCE(SUM(CASE WHEN kind IN ('CHARGE','OPENING_DEBIT') THEN amount_paise-allocated ELSE 0 END),0),
        COALESCE(SUM(CASE WHEN kind IN ('RECEIVED','OPENING_CREDIT') THEN amount_paise-allocated ELSE 0 END),0),
        COALESCE(SUM(CASE WHEN kind IN ('CHARGE','OPENING_DEBIT') AND due_date!='' AND due_date<? THEN amount_paise-allocated ELSE 0 END),0),
        COALESCE(SUM(kind IN ('CHARGE','OPENING_DEBIT')),0),COALESCE(SUM(kind IN ('RECEIVED','OPENING_CREDIT')),0)
        FROM active_statement`, home, today()).Scan(&result.DebitPaise, &result.CreditPaise, &result.AllocatedPaise, &result.OutstandingPaise, &result.UnallocatedPaise, &result.OverduePaise, &result.ChargeTotal, &result.CreditTotal)
	if err != nil {
		return result, err
	}
	result.ChargePage = clampMaintenancePage(chargePage, result.ChargeTotal, result.PageSize)
	result.CreditPage = clampMaintenancePage(creditPage, result.CreditTotal, result.PageSize)
	for _, list := range []struct {
		kind   string
		page   int
		target *[]StatementEntry
	}{
		{"'CHARGE','OPENING_DEBIT'", result.ChargePage, &result.Charges}, {"'RECEIVED','OPENING_CREDIT'", result.CreditPage, &result.Credits}} {
		rows, e := tx.QueryContext(ctx, statementQuery+"SELECT id,kind,description,entry_date,amount_paise,allocated,amount_paise-allocated,receipt,cycle_id,due_date FROM active_statement WHERE kind IN ("+list.kind+") ORDER BY entry_date DESC,id LIMIT ? OFFSET ?", home, result.PageSize, (list.page-1)*result.PageSize)
		if e != nil {
			return result, e
		}
		for rows.Next() {
			var entry StatementEntry
			if e = rows.Scan(&entry.ID, &entry.Kind, &entry.Description, &entry.Date, &entry.AmountPaise, &entry.AllocatedPaise, &entry.RemainingPaise, &entry.Receipt, &entry.CycleID, &entry.DueDate); e != nil {
				rows.Close()
				return result, e
			}
			*list.target = append(*list.target, entry)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return result, e
		}
	}
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM entry_allocations a JOIN entries e ON e.id=a.source_id WHERE e.flat_id=?", home).Scan(&result.AllocationTotal)
	if err != nil {
		return result, err
	}
	result.AllocationPage = clampMaintenancePage(allocationPage, result.AllocationTotal, result.PageSize)
	rows, err := tx.QueryContext(ctx, `SELECT a.id,a.source_id,a.charge_id,a.amount_paise,COALESCE(r.number,''),e.kind,c.description,
        CASE WHEN v.allocation_id IS NOT NULL THEN 'CORRECTED'
             WHEN EXISTS(SELECT 1 FROM entry_reversals x WHERE x.entry_id=a.source_id) THEN 'SOURCE_REVERSED'
             WHEN EXISTS(SELECT 1 FROM entry_reversals x WHERE x.entry_id=a.charge_id) THEN 'CHARGE_REVERSED' ELSE 'ACTIVE' END,
        a.reason,u.display_name,a.created_at,COALESCE(v.reason,''),COALESCE(vu.display_name,''),COALESCE(v.created_at,0)
        FROM entry_allocations a JOIN entries e ON e.id=a.source_id JOIN entries c ON c.id=a.charge_id JOIN users u ON u.id=a.actor_id
        LEFT JOIN receipts r ON r.entry_id=e.id LEFT JOIN allocation_reversals v ON v.allocation_id=a.id LEFT JOIN users vu ON vu.id=v.actor_id
        WHERE e.flat_id=? ORDER BY a.created_at DESC,a.id DESC LIMIT ? OFFSET ?`, home, result.PageSize, (result.AllocationPage-1)*result.PageSize)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var allocation AllocationRecord
		if err = rows.Scan(&allocation.ID, &allocation.SourceID, &allocation.ChargeID, &allocation.AmountPaise, &allocation.Receipt, &allocation.SourceKind, &allocation.Description, &allocation.State, &allocation.Reason, &allocation.Actor, &allocation.CreatedAt, &allocation.CorrectionReason, &allocation.CorrectedBy, &allocation.CorrectedAt); err != nil {
			rows.Close()
			return result, err
		}
		if !p.CanReadAllRecords {
			allocation.Reason, allocation.Actor, allocation.CorrectionReason, allocation.CorrectedBy = "", "", "", ""
		}
		result.Allocations = append(result.Allocations, allocation)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}
