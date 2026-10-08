package database

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestBudgetAndPaidExpenseConcurrentIdenticalRetriesKeepOneDecisionAndCorrection(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	id := budgetPropose(t, s, a, "", budgetProposal())
	approval := budgetDecision(1, "APPROVED")
	var group sync.WaitGroup
	errs := make([]error, 2)
	results := make([]string, 2)
	for i := range errs {
		group.Add(1)
		go func(i int) { defer group.Done(); results[i], errs[i] = s.DecideBudget(ctx, b, id, approval) }(i)
	}
	group.Wait()
	for i, e := range errs {
		if e != nil || results[i] != id {
			t.Fatal("identical approval replay", results, errs)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM budget_events", 2)
	changed := approval
	changed.Reason = "PRIVATE changed decision payload cannot reuse its accepted operation identity"
	if _, e := s.DecideBudget(ctx, b, id, changed); !errors.Is(e, ErrConflict) {
		t.Fatal("changed retry accepted", e)
	}
	expense := expenseProposal(id, "1.00", "CONCURRENT-PAID-VOUCHER")
	for i := range errs {
		group.Add(1)
		go func(i int) { defer group.Done(); results[i], errs[i] = s.ProposePaidExpense(ctx, a, "", expense) }(i)
	}
	group.Wait()
	if errs[0] != nil || errs[1] != nil || results[0] == "" || results[0] != results[1] {
		t.Fatal("identical expense duplicated", results, errs)
	}
	paid := results[0]
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_resources", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_events", 1)
	approveExpense := budgetDecision(1, "APPROVED")
	for i := range errs {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			results[i], errs[i] = s.DecidePaidExpense(ctx, b, paid, approveExpense)
		}(i)
	}
	group.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatal(errs)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_events", 2)
	corrections := []PaidExpenseInput{expenseProposal(id, "1.01", "CONCURRENT-PAID-VOUCHER"), expenseProposal(id, "1.02", "CONCURRENT-PAID-VOUCHER")}
	for i := range corrections {
		corrections[i].Version = 2
		corrections[i].Action = "CORRECTION"
		group.Add(1)
		go func(i int) {
			defer group.Done()
			results[i], errs[i] = s.ProposePaidExpense(ctx, a, paid, corrections[i])
		}(i)
	}
	group.Wait()
	successes, conflicts := 0, 0
	for _, e := range errs {
		if e == nil {
			successes++
		} else if errors.Is(e, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("competing correction did not retain one expected-version winner", errs)
	}
	current := expenseDetail(t, s, a, paid)
	if current.Version != 3 || current.Approved.AmountPaise != 100 || current.Snapshot.PreviousVersion != 1 {
		t.Fatal(current)
	}
	x := budgetComparison(t, s, a, id)
	if x.RecordedExpensesPaise != 100 || x.PendingExpenses != 1 {
		t.Fatal("competing pending correction changed effective money", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_identities", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}

func TestBudgetMoneyDatePeriodAndPaidExpenseBoundariesRejectWithoutSideEffects(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	bad := []func(*BudgetInput){func(x *BudgetInput) { x.PlannedCollections = "-1" }, func(x *BudgetInput) { x.PlannedCollections = "01" }, func(x *BudgetInput) { x.PlannedCollections = "1,000" }, func(x *BudgetInput) { x.PlannedCollections = "0.001" }, func(x *BudgetInput) { x.PlannedExpenses = "10000000.01" }, func(x *BudgetInput) { x.PeriodEnd = "2027-01-02" }, func(x *BudgetInput) { x.PeriodStart = "2026-02-30" }, func(x *BudgetInput) { x.PeriodStart = "2026-02-01" }, func(x *BudgetInput) { x.Confirmed = false }}
	for _, change := range bad {
		in := budgetProposal()
		change(&in)
		if _, e := s.ProposeBudget(ctx, a, "", in); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid plan accepted", in, e)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM budget_resources", 0)
	zero := budgetProposal()
	zero.PlannedCollections = "0.00"
	zero.PlannedExpenses = "0"
	id := budgetPropose(t, s, a, "", zero)
	budgetApprove(t, s, b, id)
	leap := budgetProposal()
	leap.Title = "Fictional exact 366-day leap period"
	leap.PeriodStart = "2024-01-01"
	leap.PeriodEnd = "2024-12-31"
	leap.PlannedCollections = "10000000"
	leap.PlannedExpenses = "0.0"
	budgetPropose(t, s, a, "", leap)
	leap.OperationKey = randomToken()
	leap.PeriodEnd = "2025-01-01"
	if _, e := s.ProposeBudget(ctx, a, "", leap); !errors.Is(e, ErrInvalid) {
		t.Fatal("367 inclusive days accepted", e)
	}
	for _, amount := range []string{"0", "0.00", "-1", "1.001", "1,000", "10000000.01"} {
		if _, e := s.ProposePaidExpense(ctx, a, "", expenseProposal(id, amount, "BAD-VOUCHER")); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid paid money accepted", amount, e)
		}
	}
	for _, date := range []string{"2030-01-10", "2026-02-01", "2025-12-31", "2026-02-30"} {
		in := expenseProposal(id, "0.01", "BAD-DATE-VOUCHER")
		in.PaidDate = date
		if _, e := s.ProposePaidExpense(ctx, a, "", in); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid/future/outside paid date accepted", date, e)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_resources", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_identities", 0)
	in := expenseProposal(id, "0.01", "ONE-PAISE-VOUCHER")
	in.PaidDate = "2026-01-01"
	paid := expensePropose(t, s, a, "", in)
	expenseApprove(t, s, b, paid)
	x := budgetComparison(t, s, a, id)
	if x.RecordedExpensesPaise != 1 || x.DifferencePaise != -1 || x.ExpenseHeadroomPaise != -1 || x.PlannedCollectionsPaise != 0 {
		t.Fatal("one-paise negative difference or meaningful zero plan", x)
	}
	if _, e := s.BudgetComparisonFor(ctx, a, id, 1); !errors.Is(e, ErrConflict) {
		t.Fatal("stale expected budget version accepted", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}
