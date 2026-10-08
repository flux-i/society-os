package database

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBudgetAndPaidExpenseDatabaseGuardsRetainOriginalsAndSeparateDecisionSources(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	budget := budgetPropose(t, s, a, "", budgetProposal())
	budgetApprove(t, s, b, budget)
	expense := expensePropose(t, s, a, "", expenseProposal(budget, "32.51", "IMMUTABLE-PAID-VOUCHER"))
	expenseApprove(t, s, b, expense)
	for _, item := range []struct{ query, id string }{
		{"UPDATE budget_versions SET action='CLOSE' WHERE resource_id=?", budget}, {"DELETE FROM budget_versions WHERE resource_id=?", budget},
		{"UPDATE budget_events SET reason='Changed original decision' WHERE resource_id=?", budget}, {"DELETE FROM budget_events WHERE resource_id=?", budget},
		{"DELETE FROM budget_resources WHERE id=?", budget}, {"UPDATE budget_resources SET version=version+1 WHERE id=?", budget},
		{"UPDATE paid_expense_versions SET snapshot_json='{}' WHERE resource_id=?", expense}, {"DELETE FROM paid_expense_versions WHERE resource_id=?", expense},
		{"UPDATE paid_expense_events SET reason='Changed original decision' WHERE resource_id=?", expense}, {"DELETE FROM paid_expense_events WHERE resource_id=?", expense},
		{"DELETE FROM paid_expense_resources WHERE id=?", expense}, {"UPDATE paid_expense_resources SET version=version+1 WHERE id=?", expense},
		{"UPDATE paid_expense_identities SET source_version=99 WHERE expense_id=?", expense}, {"DELETE FROM paid_expense_identities WHERE expense_id=?", expense},
	} {
		if _, e := s.DB.ExecContext(ctx, item.query, item.id); e == nil {
			t.Fatal("immutable original/head/event guard absent", item.query)
		}
	}
	plan := *budgetDetail(t, s, a, budget).Approved
	plan.Version = 3
	plan.PlanVersion = 3
	plan.PeriodStart = "2026-01-02"
	raw, e := json.Marshal(plan)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.ExecContext(ctx, "INSERT INTO budget_versions VALUES(?,?,?,?,?,?,?)", budget, 3, "PLAN", string(raw), plan.SubmittedBy, plan.SubmittedAt, plan.Reason); e == nil {
		t.Fatal("raw original period moved after approval")
	}
	if _, e = s.DB.ExecContext(ctx, "INSERT INTO budget_events VALUES(?,?,?,?,?,?,?)", budget, 3, 1, "APPROVED", plan.SubmittedBy, plan.Reason, plan.SubmittedAt); e == nil {
		t.Fatal("raw self budget approval bypassed the separate-review guard")
	}
	paid := *expenseDetail(t, s, a, expense).Approved
	paid.Version = 3
	paid.Action = "CORRECTION"
	paid.PreviousVersion = 1
	raw, e = json.Marshal(paid)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.ExecContext(ctx, "INSERT INTO paid_expense_versions VALUES(?,?,?,?,?,?,json_remove(?,'$.amount_paise'),?,?,?)", expense, 3, budget, paid.BudgetVersion, 1, "CORRECTION", string(raw), paid.SubmittedBy, paid.SubmittedAt, paid.Reason); e == nil {
		t.Fatal("missing integer paise passed a nullable JSON CHECK")
	}
	if _, e = s.DB.ExecContext(ctx, "INSERT INTO paid_expense_events VALUES(?,?,?,?,?,?,?)", expense, 3, 1, "APPROVED", paid.SubmittedBy, paid.Reason, paid.SubmittedAt); e == nil {
		t.Fatal("raw self paid-expense approval bypassed the separate-review guard")
	}
	current := budgetComparison(t, s, a, budget)
	if current.RecordedExpensesPaise != 3251 || current.DifferencePaise != -3251 {
		t.Fatal("failed raw operations changed effective money", current)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM budget_versions", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM budget_events", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_versions", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_events", 2)
}
