package database

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestBudgetAndExpensePagesKeepIndependentCountsAndOriginalVersionHistory(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	ids := []string{}
	for i := 0; i < 13; i++ {
		in := budgetProposal()
		in.Title = fmt.Sprintf("PRIVATE budget %02d", i)
		in.PeriodStart = fmt.Sprintf("2025-01-%02d", i+1)
		in.PeriodEnd = in.PeriodStart
		ids = append(ids, budgetPropose(t, s, a, "", in))
	}
	one, e := s.BudgetsFor(ctx, a, "", "", 1)
	if e != nil || one.Total != 13 || len(one.Items) != 12 || one.Counts["pending"] != 13 {
		t.Fatal(one, e)
	}
	two, e := s.BudgetsFor(ctx, a, "", "", 2)
	if e != nil || two.Total != 13 || len(two.Items) != 1 || two.Counts["pending"] != 13 {
		t.Fatal(two, e)
	}
	filtered, e := s.BudgetsFor(ctx, a, "budget 12", "", 1)
	if e != nil || filtered.Total != 1 || filtered.Counts["pending"] != 13 || filtered.Items[0].ID != ids[12] {
		t.Fatal("filtered display changed full counts", filtered, e)
	}
	budget := budgetPropose(t, s, a, "", budgetProposal())
	budgetApprove(t, s, b, budget)
	expenses := []string{}
	for i := 0; i < 13; i++ {
		in := expenseProposal(budget, "0.01", fmt.Sprintf("PAGE-EXP-%02d", i))
		expenses = append(expenses, expensePropose(t, s, a, "", in))
	}
	expenseOne, e := s.PaidExpensesFor(ctx, a, budget, "", "", 1)
	if e != nil || expenseOne.Total != 13 || len(expenseOne.Items) != 12 || expenseOne.Counts["pending"] != 13 {
		t.Fatal(expenseOne, e)
	}
	expenseTwo, e := s.PaidExpensesFor(ctx, a, budget, "", "", 2)
	if e != nil || len(expenseTwo.Items) != 1 || expenseTwo.Total != 13 {
		t.Fatal(expenseTwo, e)
	}
	expenseFilter, e := s.PaidExpensesFor(ctx, a, budget, "PAGE-EXP-12", "", 1)
	if e != nil || expenseFilter.Total != 1 || expenseFilter.Counts["pending"] != 13 || expenseFilter.Items[0].ID != expenses[12] {
		t.Fatal(expenseFilter, e)
	}
	for version := 2; version <= 25; version++ {
		in := budgetProposal()
		in.Version = version - 1
		in.Title = fmt.Sprintf("PRIVATE pending revision %02d", version)
		budgetPropose(t, s, a, ids[0], in)
	}
	history, e := s.BudgetFor(ctx, a, ids[0], 3)
	if e != nil || history.EventTotal != 25 || len(history.Events) != 1 || history.Events[0].Version != 1 || history.Events[0].Snapshot.Title != "PRIVATE budget 00" {
		t.Fatal("same-second history lost its original", history, e)
	}
	expenseApprove(t, s, b, expenses[0])
	comparison := budgetComparison(t, s, a, budget)
	if comparison.RecordedExpensesPaise != 1 || comparison.PendingExpenses != 12 {
		t.Fatal("page display manufactured confirmed spending", comparison)
	}
}

func TestBudgetCurrentReadKeysIgnoreObservationTimeAndDetectExactSourceChanges(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	id := budgetPropose(t, s, a, "", budgetProposal())
	budgetApprove(t, s, b, id)
	before := budgetComparison(t, s, a, id)
	time.Sleep(1100 * time.Millisecond)
	again := budgetComparison(t, s, a, id)
	if before.CurrentKey == "" || before.CurrentKey != again.CurrentKey || again.CheckedAt <= before.CheckedAt {
		t.Fatal("observation time destabilised the current source boundary", before, again)
	}
	first := received("0.01")
	first.Reference = "CURRENT-BUDGET-RECEIPT"
	post(t, s, a, first)
	cash := budgetComparison(t, s, a, id)
	if cash.CurrentKey == before.CurrentKey || cash.CollectionsPaise != 1 {
		t.Fatal("new original money did not invalidate held totals", cash)
	}
	expense := expensePropose(t, s, a, "", expenseProposal(id, "0.01", "CURRENT-PAID-VOUCHER"))
	pending := budgetComparison(t, s, a, id)
	if pending.CurrentKey == cash.CurrentKey || pending.RecordedExpensesPaise != 0 || pending.PendingExpenses != 1 {
		t.Fatal(pending)
	}
	revision := expenseProposal(id, "0.01", "CURRENT-PAID-VOUCHER")
	revision.Version = 1
	revision.Payee = "A different supplied fictional payee"
	expensePropose(t, s, a, expense, revision)
	revised := budgetComparison(t, s, a, id)
	if revised.CurrentKey == pending.CurrentKey || revised.RecordedExpensesPaise != 0 || revised.PendingExpenses != 1 {
		t.Fatal("same-count changed pending original did not invalidate source", revised)
	}
	expenseApprove(t, s, b, expense)
	approved := budgetComparison(t, s, a, id)
	if approved.CurrentKey == revised.CurrentKey || approved.RecordedExpensesPaise != 1 || approved.DifferencePaise != 0 {
		t.Fatal(approved)
	}
	snapshot := budgetDetail(t, s, a, id)
	if snapshot.Version != 2 {
		t.Fatal("an unrelated expense silently changed the plan resource", snapshot)
	}
	if _, e := s.BudgetComparisonFor(ctx, a, id, snapshot.Version); e != nil {
		t.Fatal(e)
	}
}
