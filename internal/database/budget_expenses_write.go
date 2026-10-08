package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

func paidExpenseEvent(ctx context.Context, tx *sql.Tx, p Principal, id string, version, proposal int, action, reason string) error {
	_, e := tx.ExecContext(ctx, "INSERT INTO paid_expense_events VALUES(?,?,?,?,?,?,?)", id, version, proposal, action, p.ID, reason, time.Now().Unix())
	if e != nil {
		return e
	}
	return appendAudit(ctx, tx, p.ID, "", "PAID_EXPENSE_"+action, "Already-paid expense decision recorded", map[string]any{}, map[string]any{"id": id, "version": version, "proposal_version": proposal})
}
func reserveExpenseIdentity(ctx context.Context, tx *sql.Tx, x PaidExpenseSnapshot, id string) error {
	key := expenseIdentity(x.Method, x.Reference)
	var owner string
	e := tx.QueryRowContext(ctx, "SELECT expense_id FROM paid_expense_identities WHERE identity_hash=?", key).Scan(&owner)
	if e == nil {
		if owner != id {
			return ErrDuplicatePaidExpense
		}
		return nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO paid_expense_identities VALUES(?,?,?,?)", key, id, x.Version, time.Now().Unix())
	return e
}
func (s *Store) ProposePaidExpense(ctx context.Context, token, id string, input PaidExpenseInput) (string, error) {
	in, amount, e := normaliseExpense(input)
	if e != nil {
		return "", e
	}
	if len(id) > 100 {
		return "", ErrInvalid
	}
	tx, p, e := s.beginBudgetWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "PAID_EXPENSE_PROPOSE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	h := budgetHead{}
	version := 1
	event := "PROPOSED"
	budget := in.BudgetID
	var x, latest PaidExpenseSnapshot
	if id == "" {
		if in.Version != 0 || in.Action != "PAID" {
			return "", ErrInvalid
		}
		id = randomToken()
	} else {
		h, budget, e = expenseHeadIn(ctx, tx, id)
		if e != nil {
			return "", e
		}
		if h.Version != in.Version {
			return "", ErrConflict
		}
		version = h.Version + 1
		latest, e = expenseSnapshotIn(ctx, tx, id, h.Latest)
		if e != nil {
			return "", e
		}
		if h.Pending > 0 {
			if latest.SubmittedBy != p.ID {
				return "", ErrForbidden
			}
			event = "REVISED"
		}
		if h.Approved > 0 {
			x, e = expenseSnapshotIn(ctx, tx, id, h.Approved)
			if e != nil {
				return "", e
			}
		}
	}
	bh, e := budgetHeadIn(ctx, tx, budget)
	if e != nil {
		return "", e
	}
	if bh.Approved == 0 {
		return "", invalid("Choose a separately approved budget before recording paid expenses.")
	}
	current, e := budgetSnapshotIn(ctx, tx, budget, bh.Approved)
	if e != nil {
		return "", e
	}
	if h.ID == "" && current.Action != "PLAN" {
		return "", invalid("This budget is closed. Its existing records and linked corrections remain available.")
	}
	previous := h.Approved
	budgetVersion := bh.Approved
	if h.ID != "" {
		budgetVersion = latest.BudgetVersion
	}
	if h.Approved > 0 {
		budgetVersion = x.BudgetVersion
	}
	if in.Action == "VOID" {
		if h.Approved == 0 || x.Action == "VOID" {
			return "", ErrConflict
		}
		x.Action = "VOID"
		x.AmountPaise = 0
	} else {
		if in.BudgetID != budget {
			return "", invalid("An expense retains its original budget.")
		}
		if (h.Approved == 0 && in.Action != "PAID") || (h.Approved > 0 && in.Action != "CORRECTION") {
			return "", invalid("An accepted expense changes only through a linked correction.")
		}
		if in.PaidDate < current.PeriodStart || in.PaidDate > current.PeriodEnd {
			return "", invalid("The paid date must belong to this budget's original inclusive period.")
		}
		x = PaidExpenseSnapshot{Action: in.Action, BudgetID: budget, BudgetVersion: budgetVersion, AmountPaise: amount, PaidDate: in.PaidDate, Payee: in.Payee, Category: in.Category, Method: in.Method, Reference: in.Reference, SourceNote: in.SourceNote}
	}
	x.Version = version
	x.PreviousVersion = previous
	x.SubmittedBy = p.ID
	x.SubmittedAt = time.Now().Unix()
	x.Reason = in.Reason
	if h.ID == "" {
		var count int
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM paid_expense_resources WHERE budget_id=?", budget).Scan(&count); e != nil {
			return "", e
		}
		if count >= 10000 {
			return "", invalid("A budget retains at most 10,000 expense records.")
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO paid_expense_resources VALUES(?,?,1,1,1,NULL,'PENDING',?,?)", id, budget, p.ID, x.SubmittedAt); e != nil {
			return "", e
		}
	}
	raw, e := json.Marshal(x)
	if e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO paid_expense_versions VALUES(?,?,?,?,?,?,?,?,?,?)", id, version, budget, x.BudgetVersion, optionalCommunityVersion(previous), x.Action, string(raw), p.ID, x.SubmittedAt, in.Reason); e != nil {
		return "", e
	}
	if e = reserveExpenseIdentity(ctx, tx, x, id); e != nil {
		return "", e
	}
	if e = paidExpenseEvent(ctx, tx, p, id, version, version, event, in.Reason); e != nil {
		return "", e
	}
	if h.ID != "" {
		if _, e = tx.ExecContext(ctx, "UPDATE paid_expense_resources SET version=?,latest_version=?,pending_version=?,decision='PENDING' WHERE id=? AND version=?", version, version, version, id, in.Version); e != nil {
			return "", e
		}
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) DecidePaidExpense(ctx context.Context, token, id string, in BudgetAction) (string, error) {
	if !validBudgetDecision(in) || len(id) < 1 || len(id) > 100 {
		return "", invalid("Confirm the exact expense proposal and a separate decision.")
	}
	tx, p, e := s.beginBudgetWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "PAID_EXPENSE_DECIDE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	h, budget, e := expenseHeadIn(ctx, tx, id)
	if e != nil {
		return "", e
	}
	if h.Version != in.Version || h.Pending == 0 {
		return "", ErrConflict
	}
	x, e := expenseSnapshotIn(ctx, tx, id, h.Pending)
	if e != nil {
		return "", e
	}
	if (in.Action == "CANCELLED" && x.SubmittedBy != p.ID) || (in.Action != "CANCELLED" && x.SubmittedBy == p.ID) {
		return "", ErrForbidden
	}
	// A correction may only replace the still-effective accepted version.
	if x.PreviousVersion != h.Approved {
		return "", ErrConflict
	}
	bh, e := budgetHeadIn(ctx, tx, budget)
	if e != nil {
		return "", e
	}
	if bh.Approved == 0 {
		return "", ErrConflict
	}
	version := h.Version + 1
	if e = paidExpenseEvent(ctx, tx, p, id, version, h.Pending, in.Action, in.Reason); e != nil {
		return "", e
	}
	approved := optionalCommunityVersion(h.Approved)
	if in.Action == "APPROVED" {
		approved = h.Pending
	}
	if _, e = tx.ExecContext(ctx, "UPDATE paid_expense_resources SET version=?,pending_version=NULL,approved_version=?,decision=? WHERE id=? AND version=?", version, approved, in.Action, id, in.Version); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
