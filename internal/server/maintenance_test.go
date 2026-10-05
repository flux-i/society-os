package server

import (
	"context"
	"encoding/json"
	"society.local/portal/internal/database"
	"strings"
	"testing"
)

func TestMaintenanceHTTPPrivateProposalSeparateReviewerAndScopedStatement(t *testing.T) {
	db, app, logs := handler(t)
	if _, err := db.DB.Exec("UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-102'"); err != nil {
		t.Fatal(err)
	}
	h := app.Handler()
	admin := signIn(t, h, "admin@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	in := database.MaintenanceInput{OperationKey: "http-maintenance-create-1234", Title: "Fictional January maintenance", PeriodStart: "2026-01-01", PeriodEnd: "2026-01-31", DueDate: "2026-01-10", SourceReference: "PRIVATE-SOURCE-REGISTER-1", Note: "PRIVATE-STAFF-NOTE-1", Confirmed: true, Lines: []database.MaintenanceLineInput{{FlatID: "demo-flat-A-101", Amount: "1000.00"}, {FlatID: "demo-flat-A-102", Amount: "750.25"}}}
	bytes, _ := json.Marshal(in)
	body := string(bytes)
	if w := admin.request("POST", "/api/maintenance", body); w.Code != 403 {
		t.Fatal("registry admin implied treasury", w.Code)
	}
	if err := db.SeedDemoTreasury(context.Background()); err != nil {
		t.Fatal(err)
	}
	noCSRF := admin
	noCSRF.csrf = ""
	if w := noCSRF.request("POST", "/api/maintenance", body); w.Code != 403 {
		t.Fatal("CSRF bypass", w.Code)
	}
	if w := admin.request("POST", "/api/maintenance", strings.TrimSuffix(body, "}")+",\"actor_id\":\"demo-user-owner\"}"); w.Code != 400 {
		t.Fatal("supplied actor", w.Code)
	}
	w := admin.request("POST", "/api/maintenance", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var result struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w := owner.request("GET", "/api/maintenance/"+result.ID, ""); w.Code != 404 {
		t.Fatal("resident saw proposal", w.Code)
	}
	action := `{"operation_key":"http-maintenance-publish-123","version":1,"decision":"PUBLISHED","reason":"PRIVATE-REVIEW-REASON checked all supplied amounts","confirmed":true}`
	if w := admin.request("POST", "/api/maintenance/"+result.ID+"/decision", action); w.Code != 403 {
		t.Fatal("self publication", w.Code)
	}
	committee := signIn(t, h, "committee@demo.society")
	if w := committee.request("POST", "/api/maintenance/"+result.ID+"/decision", action); w.Code != 403 {
		t.Fatal("implicit reviewer treasury", w.Code)
	}
	grant := `{"version":1,"confirmed":true,"reason":"Verified fictional treasury appointment against source","role":"TREASURER","term_days":30}`
	if w := admin.request("POST", "/api/admin/accounts/demo-user-committee/roles", grant); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	committee = signIn(t, h, "committee@demo.society")
	for i := 0; i < 2; i++ {
		if w := committee.request("POST", "/api/maintenance/"+result.ID+"/decision", action); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	w = owner.request("GET", "/api/maintenance/"+result.ID, "")
	var detail database.MaintenanceDetails
	if err := json.Unmarshal(w.Body.Bytes(), &detail); w.Code != 200 || err != nil || detail.Participants != 1 || detail.ActivePaise != 100000 {
		t.Fatal("owner charge", w.Code, detail, err)
	}
	for _, private := range []string{"PRIVATE-SOURCE", "PRIVATE-STAFF", "PRIVATE-REVIEW", "A-102", "events"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("private metadata", private)
		}
	}
	if w := owner.request("GET", "/api/statements/demo-flat-A-102", ""); w.Code != 404 {
		t.Fatal("cross-home statement", w.Code)
	}
	if w := owner.request("POST", "/api/allocations", `{"operation_key":"http-allocation-denied-123","source_id":"missing","charge_id":"missing","amount":"10.00","reason":"Fictional allocation","confirmed":true}`); w.Code != 403 {
		t.Fatal("resident posted allocation", w.Code)
	}
	w = owner.request("GET", "/api/statements/demo-flat-A-101", "")
	var statement database.HomeStatement
	if err := json.Unmarshal(w.Body.Bytes(), &statement); w.Code != 200 || err != nil || statement.OutstandingPaise != 100000 || statement.CreditPaise != 0 || statement.ChargeTotal != 1 {
		t.Fatal("statement", w.Code, statement, err)
	}
	for _, path := range []string{"/api/maintenance?page=0", "/api/maintenance?page=bad", "/api/maintenance?state=UNSUPPORTED", "/api/maintenance/" + result.ID + "?line_page=100001", "/api/statements/demo-flat-A-101?allocation_page=-1"} {
		if w := admin.request("GET", path, ""); w.Code != 400 {
			t.Fatal("invalid query", path, w.Code)
		}
	}
	var receipts int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&receipts); err != nil || receipts != 0 {
		t.Fatal("charges issued receipts", receipts, err)
	}
	if _, err := db.DB.Exec("UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101'"); err != nil {
		t.Fatal(err)
	}
	if w := owner.request("GET", "/api/maintenance", ""); w.Code != 403 {
		t.Fatal("current entitlement not checked", w.Code)
	}
	for _, secret := range []string{admin.cookie.Value, admin.csrf, "PRIVATE-SOURCE", "PRIVATE-STAFF", "PRIVATE-REVIEW"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("private financial metadata in logs", secret)
		}
	}
}
