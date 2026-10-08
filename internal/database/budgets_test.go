package database

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func budgetFixture(t *testing.T) (*Store, string, string) {
	t.Helper()
	s, a := recordFixture(t)
	return s, a, maintenanceReviewer(t, s, a)
}
func budgetProposal() BudgetInput {
	return BudgetInput{OperationKey: randomToken(), Action: "PLAN", Title: "Fictional January operating plan", PeriodStart: "2026-01-01", PeriodEnd: "2026-01-31", SourceReference: "PRIVATE supplied committee plan reference", PlannedCollections: "2000.00", PlannedExpenses: "1250.25", Reason: "PRIVATE independently supplied budget and inclusive dates checked", Confirmed: true}
}
func expenseProposal(budget, amount, reference string) PaidExpenseInput {
	return PaidExpenseInput{OperationKey: randomToken(), Action: "PAID", BudgetID: budget, Amount: amount, PaidDate: "2026-01-10", Payee: "Fictional service provider", Category: "Repairs", Method: "CASH", Reference: reference, SourceNote: "PRIVATE supplied external cash voucher checked independently", Reason: "PRIVATE deliberately record already-paid supplied expense", Confirmed: true}
}
func budgetDetail(t *testing.T, s *Store, a, id string) BudgetDetail {
	t.Helper()
	x, e := s.BudgetFor(context.Background(), a, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func expenseDetail(t *testing.T, s *Store, a, id string) PaidExpenseDetail {
	t.Helper()
	x, e := s.PaidExpenseFor(context.Background(), a, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func budgetPropose(t *testing.T, s *Store, a, id string, in BudgetInput) string {
	t.Helper()
	x, e := s.ProposeBudget(context.Background(), a, id, in)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func budgetDecision(version int, action string) BudgetAction {
	return BudgetAction{OperationKey: randomToken(), Version: version, Action: action, Reason: "PRIVATE separately checked the supplied exact original and deliberate decision", Confirmed: true}
}
func budgetApprove(t *testing.T, s *Store, b, id string) {
	t.Helper()
	_, e := s.DecideBudget(context.Background(), b, id, budgetDecision(budgetDetail(t, s, b, id).Version, "APPROVED"))
	if e != nil {
		t.Fatal(e)
	}
}
func expensePropose(t *testing.T, s *Store, a, id string, in PaidExpenseInput) string {
	t.Helper()
	x, e := s.ProposePaidExpense(context.Background(), a, id, in)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func expenseApprove(t *testing.T, s *Store, b, id string) {
	t.Helper()
	_, e := s.DecidePaidExpense(context.Background(), b, id, budgetDecision(expenseDetail(t, s, b, id).Version, "APPROVED"))
	if e != nil {
		t.Fatal(e)
	}
}
func budgetComparison(t *testing.T, s *Store, a, id string) BudgetComparison {
	t.Helper()
	x, e := s.BudgetComparisonFor(context.Background(), a, id, 0)
	if e != nil {
		t.Fatal(e)
	}
	return x
}

func TestBudgetIndependentCollectionsAndPaidExpenseCorrectionsPreserveOriginalMoney(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	id := budgetPropose(t, s, a, "", budgetProposal())
	if budgetComparison(t, s, a, id).Available {
		t.Fatal("unapproved plan manufactured a comparison")
	}
	budgetApprove(t, s, b, id)
	one := received("1000.00")
	one.Reference = "BUDGET-INCOMING-ONE"
	first := post(t, s, a, one)
	two := received("750.25")
	two.Reference = "BUDGET-INCOMING-TWO"
	second := post(t, s, a, two)
	if _, e := s.ReverseEntry(ctx, a, second, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "PRIVATE supplied correction of the original incoming amount"}); e != nil {
		t.Fatal(e)
	}
	post(t, s, a, supplied("OPENING_CREDIT", "200.00"))
	charge := post(t, s, a, supplied("CHARGE", "1500.00"))
	allocate(t, s, a, first, charge, "400.00")
	allocate(t, s, a, first, charge, "100.00")
	for i, date := range []string{"2025-12-31", "2026-02-01"} {
		in := received("3.00")
		in.Date = date
		in.Reference = fmt.Sprintf("BUDGET-OUTSIDE-%d", i)
		post(t, s, a, in)
	}
	request, e := s.SubmitReview(ctx, a, "", ReviewInput{OperationKey: randomToken(), Kind: "EXPENSE", Title: "Fictional expense estimate only", Body: "Approval of this supplied estimate must not count as an already-paid expense.", Estimate: "5000.00"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideReview(ctx, b, request, decision(1, "APPROVED")); e != nil {
		t.Fatal(e)
	}
	x := budgetComparison(t, s, a, id)
	if !x.Available || x.OriginalCollectionsPaise != 175025 || x.ReversedCollectionsPaise != 75025 || x.CollectionsPaise != 100000 || x.RecordedExpensesPaise != 0 || x.ReceiptCount != 2 || x.CollectionVariancePaise != -100000 || x.DifferencePaise != 100000 {
		t.Fatal("independent distinct receipt and estimate meanings", x)
	}
	receiptBefore, e := s.EntryFor(ctx, a, first)
	if e != nil {
		t.Fatal(e)
	}
	left := expensePropose(t, s, a, "", expenseProposal(id, "400.00", "VOUCHER-400"))
	right := expensePropose(t, s, a, "", expenseProposal(id, "32.51", "VOUCHER-3251"))
	if x = budgetComparison(t, s, a, id); x.RecordedExpensesPaise != 0 || x.PendingExpenses != 2 {
		t.Fatal("pending expenses added spending", x)
	}
	expenseApprove(t, s, b, left)
	expenseApprove(t, s, b, right)
	original := expenseDetail(t, s, a, right).Snapshot
	if x = budgetComparison(t, s, a, id); x.RecordedExpensesPaise != 43251 || x.DifferencePaise != 56749 || x.ExpenseHeadroomPaise != 81774 || x.ExpenseCount != 2 {
		t.Fatal("independent accepted spending", x)
	}
	correction := expenseProposal(id, "25.01", "VOUCHER-3251")
	correction.Action = "CORRECTION"
	correction.Version = expenseDetail(t, s, a, right).Version
	expensePropose(t, s, a, right, correction)
	if x = budgetComparison(t, s, a, id); x.RecordedExpensesPaise != 43251 || x.DifferencePaise != 56749 || x.PendingExpenses != 1 {
		t.Fatal("pending correction replaced its predecessor", x)
	}
	expenseApprove(t, s, b, right)
	changed := expenseDetail(t, s, a, right)
	if x = budgetComparison(t, s, a, id); x.RecordedExpensesPaise != 42501 || x.DifferencePaise != 57499 || x.ExpenseHeadroomPaise != 82524 || x.PendingExpenses != 0 {
		t.Fatal("independent correction amount", x)
	}
	retained, e := expenseSnapshotIn(ctx, s.DB, right, original.Version)
	if e != nil || !reflect.DeepEqual(retained, original) || changed.Approved.AmountPaise != 2501 || changed.Approved.PreviousVersion != 1 {
		t.Fatal("original expense or correction link changed", retained, changed, e)
	}
	receiptAfter, e := s.EntryFor(ctx, a, first)
	if e != nil || !reflect.DeepEqual(receiptBefore, receiptAfter) {
		t.Fatal("expense operation changed original cash or receipt", receiptAfter, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 4)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_resources", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_identities", 2)
}

func TestBudgetPendingReplacementPeriodAndSeparateClosePreserveAcceptedPredecessor(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	id := budgetPropose(t, s, a, "", budgetProposal())
	if _, e := s.DecideBudget(ctx, a, id, budgetDecision(1, "APPROVED")); !errors.Is(e, ErrForbidden) {
		t.Fatal("self budget approval", e)
	}
	budgetApprove(t, s, b, id)
	before := budgetDetail(t, s, a, id).Approved
	in := budgetProposal()
	in.Version = 2
	in.PlannedCollections = "3000.00"
	in.PlannedExpenses = "0.00"
	budgetPropose(t, s, a, id, in)
	if x := budgetComparison(t, s, a, id); x.PlannedCollectionsPaise != 200000 || x.PlannedExpensesPaise != 125025 {
		t.Fatal("pending plan replaced approved amounts", x)
	}
	if _, e := s.DecideBudget(ctx, b, id, budgetDecision(3, "DECLINED")); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(budgetDetail(t, s, a, id).Approved, before) {
		t.Fatal("decline changed accepted plan")
	}
	in.OperationKey = randomToken()
	in.Version = 4
	budgetPropose(t, s, a, id, in)
	budgetApprove(t, s, b, id)
	x := budgetComparison(t, s, a, id)
	if x.PlanVersion != 5 || x.PlannedCollectionsPaise != 300000 || x.PlannedExpensesPaise != 0 {
		t.Fatal("separate replacement did not become effective", x)
	}
	move := budgetProposal()
	move.Version = 6
	move.PeriodStart = "2026-01-02"
	if _, e := s.ProposeBudget(ctx, a, id, move); !errors.Is(e, ErrInvalid) {
		t.Fatal("approved period moved", e)
	}
	overlap := budgetProposal()
	overlap.Title = "Fictional overlapping January plan"
	other := budgetPropose(t, s, a, "", overlap)
	if _, e := s.DecideBudget(ctx, b, other, budgetDecision(1, "APPROVED")); !errors.Is(e, ErrInvalid) {
		t.Fatal("overlapping active budgets approved", e)
	}
	if budgetDetail(t, s, a, other).Decision != "PENDING" || budgetDetail(t, s, a, other).Approved != nil {
		t.Fatal("overlap failure changed original proposal")
	}
	close := BudgetInput{OperationKey: randomToken(), Version: 6, Action: "CLOSE", Reason: "PRIVATE retain this original plan while closing its new expense intake", Confirmed: true}
	budgetPropose(t, s, a, id, close)
	if budgetDetail(t, s, a, id).State != "OPEN" {
		t.Fatal("pending closure closed the budget")
	}
	budgetApprove(t, s, b, id)
	closed := budgetDetail(t, s, a, id)
	if closed.State != "CLOSED" || closed.CanAddExpense || closed.Approved.CollectionsPaise != 300000 {
		t.Fatal(closed)
	}
	if _, e := s.ProposePaidExpense(ctx, a, "", expenseProposal(id, "1.00", "CLOSED-VOUCHER")); !errors.Is(e, ErrInvalid) {
		t.Fatal("closed budget accepted new expense", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}
