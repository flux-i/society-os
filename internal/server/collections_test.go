package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"society.local/portal/internal/database"
	"strings"
	"testing"
)

func TestCollectionsHTTPStrictClaimsSeparateConfirmationOriginalReceiptsAndCurrentScopes(t *testing.T) {
	db, app, logs := handler(t)
	h := app.Handler()
	admin := signIn(t, h, "admin@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	in := database.FundInput{OperationKey: "http-fund-create-12345", Title: "Fictional shared garden fund", Purpose: "A supplied fictional fund for shared garden work.", ContributionType: "FIXED", StartDate: "2026-01-01", DueDate: "2026-01-10", SourceReference: "PRIVATE approved garden resolution", Note: "PRIVATE internal source note", Lines: []database.MaintenanceLineInput{{FlatID: "demo-flat-A-101", Amount: "1000.00"}, {FlatID: "demo-flat-A-102", Amount: "500.25"}}, Confirmed: true}
	encode := func(value any) string {
		t.Helper()
		b, e := json.Marshal(value)
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	body := encode(in)
	if w := admin.request("POST", "/api/collections", body); w.Code != 403 {
		t.Fatal("registry implied treasury", w.Code)
	}
	if err := db.SeedDemoTreasury(context.Background()); err != nil {
		t.Fatal(err)
	}
	noCSRF := admin
	noCSRF.csrf = ""
	if w := noCSRF.request("POST", "/api/collections", body); w.Code != 403 {
		t.Fatal("CSRF", w.Code)
	}
	if w := admin.request("POST", "/api/collections", strings.TrimSuffix(body, "}")+`,"actor_id":"demo-user-owner"}`); w.Code != 400 {
		t.Fatal("supplied actor", w.Code)
	}
	for _, bad := range []string{body + "{}", strings.Repeat(" ", 98305) + body} {
		if w := admin.request("POST", "/api/collections", bad); w.Code != 400 {
			t.Fatal("strict body", w.Code, w.Body)
		}
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/collections", strings.NewReader(body))
	r.AddCookie(admin.cookie)
	r.Header.Set("Origin", "https://unrelated.example.test")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", admin.csrf)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign origin", w.Code)
	}
	w = admin.request("POST", "/api/collections", body)
	var result struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &result); w.Code != 200 || err != nil {
		t.Fatal(w.Code, w.Body, err)
	}
	campaign := result.ID
	if w = owner.request("GET", "/api/collections/"+campaign, ""); w.Code != 404 {
		t.Fatal("unpublished proposal", w.Code)
	}
	decision := database.FundAction{OperationKey: "http-fund-publication-12345", Version: 1, Action: "PUBLISHED", Reason: "PRIVATE checked the supplied garden amounts separately", Confirmed: true}
	if w = admin.request("POST", "/api/collections/"+campaign+"/actions", encode(decision)); w.Code != 403 {
		t.Fatal("self publication", w.Code)
	}
	grant := `{"version":1,"confirmed":true,"reason":"Verified fictional separate treasury appointment against source","role":"TREASURER","term_days":30}`
	if w = admin.request("POST", "/api/admin/accounts/demo-user-committee/roles", grant); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	reviewer := signIn(t, h, "committee@demo.society")
	if w = reviewer.request("POST", "/api/collections/"+campaign+"/actions", encode(decision)); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = owner.request("GET", "/api/collections/"+campaign, "")
	var fund database.FundDetail
	if err := json.Unmarshal(w.Body.Bytes(), &fund); w.Code != 200 || err != nil || fund.OutstandingPaise != 150025 || fund.Participants != 2 {
		t.Fatal(w.Code, w.Body, err)
	}
	for _, private := range []string{"PRIVATE", "source_reference", "author_id", "events"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("private proposal", private)
		}
	}
	claim := database.FundReportInput{OperationKey: "http-already-paid-claim-12345", CampaignID: campaign, FlatID: "demo-flat-A-101", Amount: "400.00", PaymentDate: "2026-01-01", Payer: "Fictional private payer", Method: "BANK_TRANSFER", Reference: "PRIVATE external reference", Comment: "PRIVATE claimant detail", Confirmed: true}
	for _, actor := range []authClient{noCSRF} {
		if w = actor.request("POST", "/api/payment-reports", encode(claim)); w.Code != 403 {
			t.Fatal("claim CSRF", w.Code)
		}
	}
	w = owner.request("POST", "/api/payment-reports", encode(claim))
	if err := json.Unmarshal(w.Body.Bytes(), &result); w.Code != 200 || err != nil {
		t.Fatal(w.Code, w.Body, err)
	}
	report := result.ID
	var receipts int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&receipts); err != nil || receipts != 0 {
		t.Fatal("claim issued receipt", receipts, err)
	}
	if w = owner.request("GET", "/api/payment-reports/"+report+"/options", ""); w.Code != 403 {
		t.Fatal("private choices", w.Code)
	}
	confirmation := database.FundReportDecision{OperationKey: "http-verify-external-payment-12345", Version: 1, Action: "CONFIRMED", Reason: "PRIVATE separately checked the fictional external source", VerificationSource: "PRIVATE verified bank statement", PaymentIdentity: "PRIVATE bank source row 12", Mode: "NEW", AllocationAmount: "400.00", Confirmed: true}
	if w = owner.request("POST", "/api/payment-reports/"+report+"/actions", encode(confirmation)); w.Code != 403 {
		t.Fatal("resident confirmed", w.Code)
	}
	if w = admin.request("POST", "/api/payment-reports/"+report+"/actions", strings.TrimSuffix(encode(confirmation), "}")+`,"receipt_id":"pretend"}`); w.Code != 400 {
		t.Fatal("supplied receipt", w.Code)
	}
	for i := 0; i < 2; i++ {
		if w = admin.request("POST", "/api/payment-reports/"+report+"/actions", encode(confirmation)); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&receipts); err != nil || receipts != 1 {
		t.Fatal("confirmation duplicate receipt", receipts, err)
	}
	w = owner.request("GET", "/api/payment-reports/"+report, "")
	var verified database.FundReport
	if err := json.Unmarshal(w.Body.Bytes(), &verified); w.Code != 200 || err != nil || verified.State != "CONFIRMED" || verified.Receipt == "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body, err)
	}
	for _, private := range []string{confirmation.VerificationSource, confirmation.PaymentIdentity, "verification_source", "payment_identity"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("private external source in report", private)
		}
	}
	w = owner.request("GET", "/api/overview/collections", "")
	var overview database.Overview
	if err := json.Unmarshal(w.Body.Bytes(), &overview); w.Code != 200 || err != nil || overview.Counts["confirmed_paise"] != 40000 || overview.Counts["outstanding_paise"] != 110025 || overview.Counts["awaiting_verification"] != 0 {
		t.Fatal("independent overview", w.Code, w.Body, err)
	}
	claim.OperationKey = "http-repeat-claim-12345"
	w = owner.request("POST", "/api/payment-reports", encode(claim))
	if err := json.Unmarshal(w.Body.Bytes(), &result); w.Code != 200 || err != nil {
		t.Fatal(w.Code, w.Body, err)
	}
	duplicate := result.ID
	w = admin.request("GET", "/api/payment-reports/"+duplicate+"/options", "")
	var choices database.FundConfirmOptions
	if err := json.Unmarshal(w.Body.Bytes(), &choices); w.Code != 200 || err != nil || len(choices.Entries) != 1 || choices.Entries[0].VerificationSource != confirmation.VerificationSource || choices.Entries[0].PaymentIdentity != confirmation.PaymentIdentity {
		t.Fatal("preserved compatible identity", w.Code, w.Body, err)
	}
	confirmation.OperationKey = "http-duplicate-verification-12345"
	confirmation.Mode = "LINK"
	confirmation.EntryID = verified.EntryID
	confirmation.AllocationAmount = "0.00"
	if w = admin.request("POST", "/api/payment-reports/"+duplicate+"/actions", encode(confirmation)); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = owner.request("GET", "/api/payment-reports/"+duplicate, "")
	var repeated database.FundReport
	if err := json.Unmarshal(w.Body.Bytes(), &repeated); err != nil || repeated.State != "DUPLICATE" || repeated.Receipt != verified.Receipt {
		t.Fatal("same original receipt", repeated, err)
	}
	for _, path := range []string{"/api/collections?page=0", "/api/collections?page=bad", "/api/collections?state=UNKNOWN", "/api/collections/" + campaign + "?line_page=100001", "/api/payment-reports?state=UNKNOWN", "/api/payment-reports/" + report + "?event_page=-1", "/api/fund-waivers?state=UNKNOWN", "/api/fund-contributions?page=100001"} {
		if w = admin.request("GET", path, ""); w.Code != 400 {
			t.Fatal("bounded query", path, w.Code, w.Body)
		}
	}
	for _, path := range []string{"/api/fund-waivers", "/api/fund-waivers/missing"} {
		if w = owner.request("GET", path, ""); w.Code != 403 {
			t.Fatal("private exemptions", path, w.Code)
		}
	}
	if _, err := db.DB.Exec("UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'"); err != nil {
		t.Fatal(err)
	}
	if w = owner.request("GET", "/api/payment-reports/"+report, ""); w.Code != 404 {
		t.Fatal("ended home report", w.Code)
	}
	w = owner.request("GET", "/api/collections/"+campaign, "")
	if err := json.Unmarshal(w.Body.Bytes(), &fund); w.Code != 200 || err != nil || fund.Participants != 1 || fund.OutstandingPaise != 50025 {
		t.Fatal("remaining home", w.Code, w.Body, err)
	}
	for _, private := range []string{admin.cookie.Value, admin.csrf, claim.Payer, claim.Reference, claim.Comment, confirmation.VerificationSource, confirmation.PaymentIdentity} {
		if strings.Contains(logs.String(), private) {
			t.Fatal("private HTTP logs", private)
		}
	}
}
