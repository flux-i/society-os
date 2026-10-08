package database

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestBudgetExplicitTreasuryAuditorAccessExpiryFreshnessAndReplay(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	in := budgetProposal()
	id := budgetPropose(t, s, a, "", in)
	approval := budgetDecision(1, "APPROVED")
	if _, e := s.DecideBudget(ctx, b, id, approval); e != nil {
		t.Fatal(e)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	for _, token := range []string{owner, tenant} {
		if _, e := s.BudgetsFor(ctx, token, "", "", 1); !errors.Is(e, ErrForbidden) {
			t.Fatal("household access granted private budgets", e)
		}
		if _, e := s.BudgetComparisonFor(ctx, token, id, 0); !errors.Is(e, ErrForbidden) {
			t.Fatal("private comparison disclosed", e)
		}
	}
	_, e := s.GrantAppointment(ctx, a, "demo-user-owner", AppointmentInput{AccessChange: AccessChange{Version: 1, Reason: "PRIVATE explicit independent audit appointment for the fictional finance reader", Confirmed: true}, Role: "AUDITOR", TermDays: 30})
	if e != nil {
		t.Fatal(e)
	}
	auditor := reviewLogin(t, s, "owner@demo.society")
	read := budgetDetail(t, s, auditor, id)
	if read.CanRevise || read.CanDecide || read.CanClose || read.CanAddExpense {
		t.Fatal("auditor received a finance write action", read)
	}
	if _, e = s.ProposeBudget(ctx, auditor, "", budgetProposal()); !errors.Is(e, ErrForbidden) {
		t.Fatal("auditor prepared a budget", e)
	}
	if _, e = s.ProposePaidExpense(ctx, auditor, "", expenseProposal(id, "1.00", "AUDIT-VOUCHER")); !errors.Is(e, ErrForbidden) {
		t.Fatal("auditor prepared a paid expense", e)
	}
	old := time.Now().Add(-time.Minute).Unix()
	if _, e = s.DB.Exec("UPDATE role_grants SET valid_from=?-172800,valid_until=? WHERE user_id='demo-user-committee' AND role='TREASURER' AND revoked_at IS NULL", old, old); e != nil {
		t.Fatal(e)
	}
	committee, e := s.CheckSession(ctx, b)
	if e != nil || !committee.CanReadAllRecords || committee.CanManageRecords {
		t.Fatal("independent committee-only ledger scope not established", committee, e)
	}
	if _, e = s.BudgetsFor(ctx, b, "", "", 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("broad committee ledger flag granted private budgets", e)
	}
	if _, e = s.DecideBudget(ctx, b, id, approval); !errors.Is(e, ErrForbidden) {
		t.Fatal("expired treasury replay retained write authority", e)
	}
	if _, e = s.DB.Exec("UPDATE sessions SET reauthenticated_at=?,mfa_verified_at=? WHERE token_hash=?", time.Now().Add(-6*time.Minute).Unix(), time.Now().Add(-6*time.Minute).Unix(), TokenHash(a)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.BudgetFor(ctx, a, id, 1); e != nil {
		t.Fatal("ordinary permitted read unexpectedly required fresh write verification", e)
	}
	if _, e = s.ProposeBudget(ctx, a, "", in); !errors.Is(e, ErrReauthRequired) {
		t.Fatal("stale privileged accepted retry bypassed fresh verification", e)
	}
	if _, e = s.DB.Exec("UPDATE role_grants SET valid_from=?-172800,valid_until=? WHERE user_id='demo-user-owner' AND role='AUDITOR' AND revoked_at IS NULL", old, old); e != nil {
		t.Fatal(e)
	}
	if _, e = s.BudgetFor(ctx, auditor, id, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("ended auditor retained own-household private read", e)
	}
	if _, e = s.DB.Exec("UPDATE role_grants SET valid_from=?-172800,valid_until=? WHERE user_id='demo-user-admin' AND role='TREASURER' AND revoked_at IS NULL", old, old); e != nil {
		t.Fatal(e)
	}
	registry, e := s.CheckSession(ctx, a)
	if e != nil || !registry.CanManageRegistry || registry.CanManageRecords {
		t.Fatal(registry, e)
	}
	if _, e = s.BudgetFor(ctx, a, id, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("registry-only officer retained private budget", e)
	}
}

func TestPaidExpenseClosedBudgetRetainsPreparedReviewVoidAndLinkedRestoration(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	budget := budgetPropose(t, s, a, "", budgetProposal())
	budgetApprove(t, s, b, budget)
	expense := expensePropose(t, s, a, "", expenseProposal(budget, "32.51", "CLOSED-ORIGINAL-VOUCHER"))
	original := expenseDetail(t, s, a, expense).Snapshot
	close := BudgetInput{OperationKey: randomToken(), Version: 2, Action: "CLOSE", Reason: "PRIVATE close new expense intake and retain existing proposal review", Confirmed: true}
	budgetPropose(t, s, a, budget, close)
	budgetApprove(t, s, b, budget)
	expenseApprove(t, s, b, expense)
	x := budgetComparison(t, s, a, budget)
	if x.RecordedExpensesPaise != 3251 || x.DifferencePaise != -3251 || x.PlanVersion != 1 {
		t.Fatal("closed period lost supplied spending or original plan version", x)
	}
	void := PaidExpenseInput{OperationKey: randomToken(), Version: 2, Action: "VOID", Reason: "PRIVATE linked correction of this mistaken accepted expense record", Confirmed: true}
	expensePropose(t, s, a, expense, void)
	if x = budgetComparison(t, s, a, budget); x.RecordedExpensesPaise != 3251 {
		t.Fatal("pending void erased accepted expense", x)
	}
	expenseApprove(t, s, b, expense)
	current := expenseDetail(t, s, a, expense)
	if current.State != "VOID" || !current.CanCorrect || current.CanVoid || current.Approved.PreviousVersion != 1 {
		t.Fatal(current)
	}
	restore := expenseProposal(budget, "25.01", "CLOSED-ORIGINAL-VOUCHER")
	restore.Action = "CORRECTION"
	restore.Version = 4
	expensePropose(t, s, a, expense, restore)
	expenseApprove(t, s, b, expense)
	current = expenseDetail(t, s, a, expense)
	x = budgetComparison(t, s, a, budget)
	if current.Approved.PreviousVersion != 3 || current.Approved.BudgetVersion != 1 || x.RecordedExpensesPaise != 2501 || x.DifferencePaise != -2501 {
		t.Fatal("linked restoration changed the lineage or budget", current, x)
	}
	retained, e := expenseSnapshotIn(ctx, s.DB, expense, 1)
	if e != nil || !reflect.DeepEqual(retained, original) {
		t.Fatal("original paid record was rewritten", retained, e)
	}
	if _, e = s.ProposePaidExpense(ctx, a, "", expenseProposal(budget, "1.00", "CLOSED-NEW-VOUCHER")); !errors.Is(e, ErrInvalid) {
		t.Fatal("closed period accepted a new expense resource", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_identities", 1)
}

func TestPaidExpenseDuplicateReferencesStayBoundAcrossBudgetsRevisionsAndCorrections(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	budget := budgetPropose(t, s, a, "", budgetProposal())
	budgetApprove(t, s, b, budget)
	first := expensePropose(t, s, a, "", expenseProposal(budget, "1.00", "Voucher 12"))
	if _, e := s.ProposePaidExpense(ctx, a, "", expenseProposal(budget, "2.00", " VOUCHER   12 ")); !errors.Is(e, ErrDuplicatePaidExpense) {
		t.Fatal("normalised duplicate created another expense", e)
	}
	if _, e := s.DecidePaidExpense(ctx, b, first, budgetDecision(1, "DECLINED")); e != nil {
		t.Fatal(e)
	}
	revised := expenseProposal(budget, "1.01", "Voucher 13")
	revised.Version = 2
	expensePropose(t, s, a, first, revised)
	expenseApprove(t, s, b, first)
	in := budgetProposal()
	in.Title = "Fictional February operating plan"
	in.PeriodStart = "2026-02-01"
	in.PeriodEnd = "2026-02-28"
	secondBudget := budgetPropose(t, s, a, "", in)
	budgetApprove(t, s, b, secondBudget)
	duplicate := expenseProposal(secondBudget, "1.00", "VOUCHER 12")
	duplicate.PaidDate = "2026-02-10"
	if _, e := s.ProposePaidExpense(ctx, a, "", duplicate); !errors.Is(e, ErrDuplicatePaidExpense) {
		t.Fatal("declined old identifier was reused across a new budget", e)
	}
	correction := expenseProposal(budget, "0.01", "Voucher 14")
	correction.Action = "CORRECTION"
	correction.Version = 4
	expensePropose(t, s, a, first, correction)
	expenseApprove(t, s, b, first)
	for _, reference := range []string{"Voucher 12", "Voucher 13", "Voucher 14"} {
		if _, e := s.ProposePaidExpense(ctx, a, "", expenseProposal(budget, "1.00", reference)); !errors.Is(e, ErrDuplicatePaidExpense) {
			t.Fatal("original payment identity released", reference, e)
		}
	}
	page, e := s.PaidExpensesFor(ctx, a, budget, "Voucher 12", "", 1)
	if e != nil || page.Total != 1 || page.Items[0].ID != first {
		t.Fatal("retained identifier cannot find its existing lineage", page, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_resources", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM paid_expense_identities", 3)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}
