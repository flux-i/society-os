package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"society.local/portal/internal/database"
)

func TestBudgetHTTPExplicitFinanceCSRFStrictDecodingExactComparisonAndDuplicatePaidIdentity(t *testing.T) {
	s, app, logs := handler(t)
	h := app.Handler()
	ctx := context.Background()
	a := signIn(t, h, "admin@demo.society")
	b := signIn(t, h, "committee@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	for _, client := range []authClient{a, b, owner} {
		messageStatus(t, client.request("GET", "/api/budgets", ""), 403)
	}
	if e := s.SeedDemoTreasury(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e := s.GrantAppointment(ctx, a.cookie.Value, "demo-user-committee", database.AppointmentInput{AccessChange: database.AccessChange{Version: 1, Reason: "PRIVATE explicit separate fictional finance reviewer", Confirmed: true}, Role: "TREASURER", TermDays: 30}); e != nil {
		t.Fatal(e)
	}
	messageStatus(t, b.request("GET", "/api/budgets", ""), 401)
	b = signIn(t, h, "committee@demo.society")
	in := database.BudgetInput{OperationKey: "budget-http-plan-123456", Action: "PLAN", Title: "PRIVATE_HTTP supplied January budget", PeriodStart: "2026-01-01", PeriodEnd: "2026-01-31", SourceReference: "PRIVATE_HTTP supplied budget reference", PlannedCollections: "1000.00", PlannedExpenses: "400.01", Reason: "PRIVATE_HTTP reviewed the supplied dated plan and exact amounts", Confirmed: true}
	body := messageJSON(t, in)
	missing := a
	missing.csrf = ""
	messageStatus(t, missing.request("POST", "/api/budgets", body), 403)
	for _, bad := range []string{body + "{}", strings.TrimSuffix(body, "}") + `,"actor_id":"demo-user-committee"}`, strings.Replace(body, "1000.00", "1e3", 1)} {
		messageStatus(t, a.request("POST", "/api/budgets", bad), 400)
	}
	w := a.request("POST", "/api/budgets", body)
	messageStatus(t, w, 200)
	var result struct{ ID string }
	if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	repeated := a.request("POST", "/api/budgets", body)
	messageStatus(t, repeated, 200)
	if repeated.Body.String() != w.Body.String() {
		t.Fatal("HTTP lost-response replay changed the budget identity")
	}
	approve := database.BudgetAction{OperationKey: "budget-http-decision-12345", Version: 1, Action: "APPROVED", Reason: "PRIVATE_HTTP independently reviewed the supplied exact plan", Confirmed: true}
	messageStatus(t, a.request("POST", "/api/budgets/"+result.ID+"/decisions", messageJSON(t, approve)), 403)
	messageStatus(t, b.request("POST", "/api/budgets/"+result.ID+"/decisions", messageJSON(t, approve)), 200)
	expense := database.PaidExpenseInput{OperationKey: "budget-http-paid-123456", Action: "PAID", BudgetID: result.ID, Amount: "32.51", PaidDate: "2026-01-31", Payee: "PRIVATE_HTTP fictional payee", Category: "Repairs", Method: "CASH", Reference: "PRIVATE_HTTP VOUCHER 17", SourceNote: "PRIVATE_HTTP supplied paid cash-voucher evidence", Reason: "PRIVATE_HTTP record the independently supplied already-paid amount", Confirmed: true}
	w = a.request("POST", "/api/paid-expenses", messageJSON(t, expense))
	messageStatus(t, w, 200)
	var paid struct{ ID string }
	if e := json.Unmarshal(w.Body.Bytes(), &paid); e != nil {
		t.Fatal(e)
	}
	duplicate := expense
	duplicate.OperationKey = "budget-http-duplicate-123456"
	duplicate.Reference = "private_http  voucher 17"
	w = a.request("POST", "/api/paid-expenses", messageJSON(t, duplicate))
	messageStatus(t, w, 409)
	if !strings.Contains(w.Body.String(), "duplicate_paid_expense") {
		t.Fatal("duplicate lacks a definite rejection code", w.Body)
	}
	approve.OperationKey = "budget-http-paid-review-12345"
	messageStatus(t, b.request("POST", "/api/paid-expenses/"+paid.ID+"/decisions", messageJSON(t, approve)), 200)
	w = a.request("GET", "/api/budgets/"+result.ID+"/comparison?version=2", "")
	messageStatus(t, w, 200)
	var comparison database.BudgetComparison
	if e := json.Unmarshal(w.Body.Bytes(), &comparison); e != nil || comparison.CollectionsPaise != 0 || comparison.RecordedExpensesPaise != 3251 || comparison.DifferencePaise != -3251 || comparison.ExpenseHeadroomPaise != 36750 {
		t.Fatal("independent HTTP money", comparison, e)
	}
	messageStatus(t, a.request("GET", "/api/budgets/"+result.ID+"/comparison?version=1", ""), 409)
	for _, path := range []string{"/api/budgets?page=0", "/api/budgets?page=10001", "/api/budgets/" + result.ID + "?event_page=10001", "/api/budgets/" + result.ID + "/comparison?version=0", "/api/paid-expenses?budget_id=" + result.ID + "&page=10001"} {
		messageStatus(t, a.request("GET", path, ""), 400)
	}
	for _, path := range []string{"/api/budgets", "/api/budgets/" + result.ID, "/api/budgets/" + result.ID + "/comparison", "/api/paid-expenses?budget_id=" + result.ID, "/api/paid-expenses/" + paid.ID} {
		messageStatus(t, owner.request("GET", path, ""), 403)
	}
	if strings.Contains(logs.String(), "PRIVATE_HTTP") || strings.Contains(logs.String(), a.cookie.Value) || strings.Contains(logs.String(), "32.51") {
		t.Fatal("request log exposed private financial source", logs.String())
	}
	for _, table := range []string{"entries", "receipts", "message_batches"} {
		var count int
		if e := s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); e != nil || count != 0 {
			t.Fatal("budget/paid expense initiated unrelated financial/message work", table, count, e)
		}
	}
}
