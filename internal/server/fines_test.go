package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"society.local/portal/internal/database"
)

func TestFinesHTTPSeparateNoticeIssueCorrectionOriginalReceiptAndCurrentScope(t *testing.T) {
	db, app, logs := handler(t)
	h := app.Handler()
	a := signIn(t, h, "admin@demo.society")
	b := signIn(t, h, "committee@demo.society")
	o := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	encode := func(x any) string {
		t.Helper()
		data, err := json.Marshal(x)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	identity := func(w *httptest.ResponseRecorder) string {
		t.Helper()
		var x struct{ ID string }
		err := json.Unmarshal(w.Body.Bytes(), &x)
		if w.Code != 200 || err != nil || x.ID == "" {
			t.Fatal(w.Code, w.Body, err)
		}
		return x.ID
	}
	assertStatus := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatal(w.Code, w.Body, "expected", code)
		}
	}
	rule := identity(a.request("POST", "/api/rules", encode(database.RuleInput{OperationKey: "fine-http-rule-propose-12345", Title: "Keep shared corridors accessible", Text: "Fictional supplied policy concerning shared corridor access.", PolicyReference: "Supplied fictional policy R-01", EffectiveFrom: "2026-01-01", FinePermitted: true, Confirmed: true})))
	identity(b.request("POST", "/api/rules/"+rule+"/actions", encode(database.RuleAction{OperationKey: "fine-http-rule-publish-12345", Version: 1, Action: "PUBLISHED", Reason: "Reviewed the supplied fictional rule separately", Confirmed: true})))
	incident := identity(o.request("POST", "/api/incidents", encode(database.IncidentInput{OperationKey: "fine-http-incident-propose-12345", RuleID: rule, FlatID: "demo-flat-A-101", IncidentDate: "2026-01-02", Comment: "PRIVATE original reporter observation must stay in the incident case", Confirmed: true})))
	identity(b.request("POST", "/api/incidents/"+incident+"/actions", encode(database.IncidentAction{OperationKey: "fine-http-incident-substantiate-12345", Version: 1, Action: "SUBSTANTIATED", Reason: "PRIVATE independently reviewed original case details", Confirmed: true})))
	proposal := database.FineInput{OperationKey: "fine-http-proposal-12345", IncidentID: incident, Title: "Shared corridor fine", Amount: "250.25", PolicyReference: "Supplied fictional amount policy F-01", Reason: "PRIVATE supplied calculation and independent review source", DueDate: "2099-02-01", ResponseBy: "2099-01-01", NoticeBody: "A supplied fictional policy proposes this amount. Please provide your account.", Confirmed: true}
	proposal.SourceKey = strings.Repeat("a", 64)
	assertStatus(a.request("POST", "/api/fines", encode(proposal)), 403)
	if err := db.SeedDemoTreasury(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertStatus(a.request("POST", "/api/admin/accounts/demo-user-committee/roles", `{"version":1,"confirmed":true,"reason":"Checked separate fictional treasury appointment against supplied source","role":"TREASURER","term_days":30}`), 201)
	b = signIn(t, h, "committee@demo.society")
	w := a.request("GET", "/api/fine-sources/"+incident, "")
	var source database.FineSource
	if err := json.Unmarshal(w.Body.Bytes(), &source); w.Code != 200 || err != nil || source.SourceKey == "" {
		t.Fatal(w.Code, w.Body, err)
	}
	for _, private := range []string{"PRIVATE", "reporter_id", "comment", "picture_id"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("source disclosure", private, w.Body)
		}
	}
	proposal.SourceKey = source.SourceKey
	body := encode(proposal)
	noCSRF := a
	noCSRF.csrf = ""
	assertStatus(noCSRF.request("POST", "/api/fines", body), 403)
	for _, bad := range []string{body + "{}", strings.TrimSuffix(body, "}") + `,"author_id":"spoofed"}`, strings.Repeat(" ", 32769) + body} {
		assertStatus(a.request("POST", "/api/fines", bad), 400)
	}
	foreign := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/fines", strings.NewReader(body))
	foreign.AddCookie(a.cookie)
	foreign.Header.Set("Origin", "https://foreign.example.test")
	foreign.Header.Set("X-CSRF-Token", a.csrf)
	foreign.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, foreign)
	assertStatus(w, 403)
	fine := identity(a.request("POST", "/api/fines", body))
	if again := identity(a.request("POST", "/api/fines", body)); again != fine {
		t.Fatal("proposal replay", again, fine)
	}
	assertStatus(o.request("GET", "/api/fines/"+fine, ""), 404)
	assertStatus(o.request("GET", "/api/fine-sources/"+incident, ""), 403)
	load := func() database.FineDetail {
		t.Helper()
		w := b.request("GET", "/api/fines/"+fine, "")
		var x database.FineDetail
		err := json.Unmarshal(w.Body.Bytes(), &x)
		if w.Code != 200 || err != nil {
			t.Fatal(w.Code, w.Body, err)
		}
		return x
	}
	x := load()
	action := database.FineAction{OperationKey: "fine-http-notify-12345", Version: x.Version, Action: "NOTIFY", Reason: "Checked exact supplied amount and household notice separately", Confirmed: true}
	assertStatus(a.request("POST", "/api/fines/"+fine+"/actions", encode(action)), 403)
	identity(b.request("POST", "/api/fines/"+fine+"/actions", encode(action)))
	for _, table := range []string{"entries", "receipts"} {
		var n int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatal("notice created money", table, n, err)
		}
	}
	x = load()
	notice := x.NoticeID
	w = o.request("GET", "/api/fine-notices/"+notice, "")
	assertStatus(w, 200)
	for _, private := range []string{"PRIVATE", "reporter_id", "incident_id", "source_key", "material_key", "author_id", "events"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("household notice leak", private, w.Body)
		}
	}
	assertStatus(tenant.request("GET", "/api/fine-notices/"+notice, ""), 404)
	response := database.IncidentResponseInput{OperationKey: "fine-http-household-response-12345", Version: 1, Body: "The current household supplies its retained fictional explanation.", Confirmed: true}
	identity(o.request("POST", "/api/fine-notices/"+notice+"/responses", encode(response)))
	x = load()
	resolve := database.FineAction{OperationKey: "fine-http-resolve-12345", Version: x.Version, Action: "RESOLVE", Reason: "Reviewed every current supplied response separately", SourceKey: x.CurrentSourceKey, ResponseCount: x.ResponseTotal, Resolution: "The supplied policy and household response were reviewed with the exact amount.", EarlyIssueReference: "Supplied fictional early decision authority F-02", Confirmed: true}
	identity(b.request("POST", "/api/fines/"+fine+"/actions", encode(resolve)))
	x = load()
	issue := database.FineAction{OperationKey: "fine-http-issue-12345", Version: x.Version, Action: "ISSUE", Reason: "Issued only after independent resolution of the current household account", ResolutionKey: x.ResolutionKey, Confirmed: true}
	identity(b.request("POST", "/api/fines/"+fine+"/actions", encode(issue)))
	identity(b.request("POST", "/api/fines/"+fine+"/actions", encode(issue)))
	w = o.request("GET", "/api/fines/"+fine, "")
	var owned database.FineDetail
	if err := json.Unmarshal(w.Body.Bytes(), &owned); w.Code != 200 || err != nil || owned.OutstandingPaise != 25025 {
		t.Fatal(w.Code, w.Body, err)
	}
	for _, private := range []string{"PRIVATE", "incident_id", "material_key", "source_key", "resolved_by", "resolved_at", "author_id", "events", "resolution_key"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("finance projection leak", private, w.Body)
		}
	}
	claim := database.FineReportInput{OperationKey: "fine-http-already-paid-12345", FineID: fine, Amount: "100.00", PaymentDate: "2026-01-03", Payer: "Fictional owner", Method: "BANK_TRANSFER", Reference: "PRIVATE original external payment reference", Comment: "PRIVATE supplied payment observation", Confirmed: true}
	report := identity(o.request("POST", "/api/fine-reports", encode(claim)))
	assertStatus(o.request("GET", "/api/fine-reports/"+report+"/options", ""), 403)
	verify := database.FundReportDecision{OperationKey: "fine-http-verify-paid-12345", Version: 1, Action: "CONFIRMED", Reason: "PRIVATE independently verified supplied original external source", VerificationSource: "PRIVATE original bank statement", PaymentIdentity: "PRIVATE original row identifier", Mode: "NEW", AllocationAmount: "100.00", Confirmed: true}
	identity(a.request("POST", "/api/fine-reports/"+report+"/actions", encode(verify)))
	identity(a.request("POST", "/api/fine-reports/"+report+"/actions", encode(verify)))
	w = o.request("GET", "/api/fine-reports/"+report, "")
	var paid database.FineReport
	if err := json.Unmarshal(w.Body.Bytes(), &paid); w.Code != 200 || err != nil || paid.ReceiptID == "" || paid.State != "CONFIRMED" {
		t.Fatal(w.Code, w.Body, err)
	}
	for _, private := range []string{verify.VerificationSource, verify.PaymentIdentity, "verification_source", "payment_identity"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("original verification disclosure", private, w.Body)
		}
	}
	x = load()
	if x.OutstandingPaise != 15025 {
		t.Fatal("independent paid balance", x.OutstandingPaise)
	}
	assertStatus(a.request("POST", "/api/entries/"+x.CurrentEntryID+"/reverse", encode(database.EntryAction{OperationKey: "fine-http-generic-reverse-12345", Reason: "Must instead request a separately reviewed fine correction", Confirmed: true})), 400)
	waiver := identity(a.request("POST", "/api/fine-waivers", encode(database.FineWaiverInput{OperationKey: "fine-http-waiver-propose-12345", FineID: fine, ChargeVersion: 1, Kind: "WAIVER", Amount: "75.25", PolicyReference: "Supplied fictional correction policy C-01", Reason: "PRIVATE independently supplied correction calculation", Confirmed: true})))
	w = b.request("GET", "/api/fine-waivers/"+waiver, "")
	var correction database.FineWaiver
	if err := json.Unmarshal(w.Body.Bytes(), &correction); w.Code != 200 || err != nil {
		t.Fatal(w.Code, w.Body, err)
	}
	decision := database.FineChildAction{OperationKey: "fine-http-waiver-approve-12345", Version: correction.Version, FineVersion: correction.CurrentFineVersion, Action: "APPROVED", Reason: "Verified exact linked correction and explicit release of original credit", Confirmed: true}
	assertStatus(a.request("POST", "/api/fine-waivers/"+waiver+"/actions", encode(decision)), 403)
	identity(b.request("POST", "/api/fine-waivers/"+waiver+"/actions", encode(decision)))
	identity(b.request("POST", "/api/fine-waivers/"+waiver+"/actions", encode(decision)))
	x = load()
	if x.ActivePaise != 17500 || x.AllocatedPaise != 0 || x.OutstandingPaise != 17500 {
		t.Fatal("independent correction balance", x)
	}
	w = o.request("GET", "/api/statements/demo-flat-A-101", "")
	var statement database.HomeStatement
	if err := json.Unmarshal(w.Body.Bytes(), &statement); w.Code != 200 || err != nil || statement.UnallocatedPaise != 10000 {
		t.Fatal("released credit", w.Code, w.Body, err)
	}
	for _, path := range []string{"/api/fines?page=0", "/api/fines?state=UNKNOWN", "/api/fine-sources?page=100001", "/api/fines/" + fine + "?response_page=-1", "/api/fine-notices?page=bad", "/api/fine-reports?state=UNKNOWN", "/api/fine-reports/" + report + "/options?page=0", "/api/fine-appeals?page=0", "/api/fine-waivers?page=bad", "/api/fine-waivers/" + waiver + "?event_page=100001"} {
		assertStatus(a.request("GET", path, ""), 400)
	}
	if _, err := db.DB.Exec("UPDATE flat_memberships SET end_date='2020-01-02' WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/fines/" + fine, "/api/fine-notices/" + notice, "/api/fine-reports/" + report} {
		assertStatus(o.request("GET", path, ""), 404)
	}
	assertStatus(o.request("POST", "/api/fine-notices/"+notice+"/responses", encode(response)), 404)
	assertStatus(o.request("POST", "/api/fine-reports", encode(claim)), 404)
	var n int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&n); err != nil || n != 1 {
		t.Fatal("retained original receipt", n, err)
	}
	if strings.Contains(logs.String(), "PRIVATE") || strings.Contains(logs.String(), proposal.Title) {
		t.Fatal("private content in logs")
	}
}
