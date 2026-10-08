package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

func budgetEvent(ctx context.Context, tx *sql.Tx, p Principal, id string, version, proposal int, action, reason string) error {
	_, e := tx.ExecContext(ctx, "INSERT INTO budget_events VALUES(?,?,?,?,?,?,?)", id, version, proposal, action, p.ID, reason, time.Now().Unix())
	if e != nil {
		return e
	}
	return appendAudit(ctx, tx, p.ID, "", "BUDGET_"+action, "Budget decision recorded", map[string]any{}, map[string]any{"id": id, "version": version, "proposal_version": proposal})
}
func (s *Store) ProposeBudget(ctx context.Context, token, id string, input BudgetInput) (string, error) {
	in, collections, expenses, e := normaliseBudget(input)
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
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "BUDGET_PROPOSE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	h := budgetHead{}
	version := 1
	event := "PROPOSED"
	var x BudgetSnapshot
	if id == "" {
		if in.Version != 0 || in.Action != "PLAN" {
			return "", ErrInvalid
		}
		id = randomToken()
	} else {
		h, e = budgetHeadIn(ctx, tx, id)
		if e != nil {
			return "", e
		}
		if h.Version != in.Version {
			return "", ErrConflict
		}
		version = h.Version + 1
		if h.Pending > 0 {
			pending, err := budgetSnapshotIn(ctx, tx, id, h.Pending)
			if err != nil {
				return "", err
			}
			if pending.SubmittedBy != p.ID {
				return "", ErrForbidden
			}
			event = "REVISED"
		}
		if h.Approved > 0 {
			x, e = budgetSnapshotIn(ctx, tx, id, h.Approved)
			if e != nil {
				return "", e
			}
			if x.Action == "CLOSE" {
				return "", invalid("A closed budget retains its history. Prepare another period for new work.")
			}
		}
	}
	if in.Action == "PLAN" {
		if h.Approved > 0 && (in.PeriodStart != x.PeriodStart || in.PeriodEnd != x.PeriodEnd) {
			return "", invalid("An approved budget retains its original period boundaries.")
		}
		x = BudgetSnapshot{Action: "PLAN", Title: in.Title, PeriodStart: in.PeriodStart, PeriodEnd: in.PeriodEnd, SourceReference: in.SourceReference, CollectionsPaise: collections, ExpensesPaise: expenses}
	} else {
		if h.Approved == 0 {
			return "", ErrConflict
		}
		x.Action = "CLOSE"
	}
	if x.Action == "PLAN" {
		x.PlanVersion = version
	}
	x.Version = version
	x.SubmittedBy = p.ID
	x.SubmittedAt = time.Now().Unix()
	x.Reason = in.Reason
	if h.ID == "" {
		if _, e = tx.ExecContext(ctx, "INSERT INTO budget_resources VALUES(?,1,1,1,NULL,'PENDING',?,?)", id, p.ID, x.SubmittedAt); e != nil {
			return "", e
		}
	}
	raw, e := json.Marshal(x)
	if e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO budget_versions VALUES(?,?,?,?,?,?,?)", id, version, x.Action, string(raw), p.ID, x.SubmittedAt, in.Reason); e != nil {
		return "", e
	}
	if e = budgetEvent(ctx, tx, p, id, version, version, event, in.Reason); e != nil {
		return "", e
	}
	if h.ID != "" {
		if _, e = tx.ExecContext(ctx, "UPDATE budget_resources SET version=?,latest_version=?,pending_version=?,decision='PENDING' WHERE id=? AND version=?", version, version, version, id, in.Version); e != nil {
			return "", e
		}
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) DecideBudget(ctx context.Context, token, id string, in BudgetAction) (string, error) {
	if !validBudgetDecision(in) || len(id) < 1 || len(id) > 100 {
		return "", invalid("Confirm the exact budget proposal and a separate decision.")
	}
	tx, p, e := s.beginBudgetWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "BUDGET_DECIDE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	h, e := budgetHeadIn(ctx, tx, id)
	if e != nil {
		return "", e
	}
	if h.Version != in.Version || h.Pending == 0 {
		return "", ErrConflict
	}
	x, e := budgetSnapshotIn(ctx, tx, id, h.Pending)
	if e != nil {
		return "", e
	}
	if (in.Action == "CANCELLED" && x.SubmittedBy != p.ID) || (in.Action != "CANCELLED" && x.SubmittedBy == p.ID) {
		return "", ErrForbidden
	}
	if in.Action == "APPROVED" && x.Action == "PLAN" {
		var overlap bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM budget_resources h JOIN budget_versions v ON v.resource_id=h.id AND v.version=h.approved_version
   WHERE h.id<>? AND v.action='PLAN' AND json_extract(v.snapshot_json,'$.period_start')<=? AND json_extract(v.snapshot_json,'$.period_end')>=?)`, id, x.PeriodEnd, x.PeriodStart).Scan(&overlap)
		if e != nil {
			return "", e
		}
		if overlap {
			return "", invalid("This period overlaps another open approved budget. Review or close that period before approving another.")
		}
	}
	version := h.Version + 1
	if e = budgetEvent(ctx, tx, p, id, version, h.Pending, in.Action, in.Reason); e != nil {
		return "", e
	}
	approved := optionalCommunityVersion(h.Approved)
	if in.Action == "APPROVED" {
		approved = h.Pending
	}
	if _, e = tx.ExecContext(ctx, "UPDATE budget_resources SET version=?,pending_version=NULL,approved_version=?,decision=? WHERE id=? AND version=?", version, approved, in.Action, id, in.Version); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
