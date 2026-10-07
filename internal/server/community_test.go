package server

import (
	"encoding/json"
	"net/http/httptest"
	"society.local/portal/internal/database"
	"strings"
	"testing"
)

func TestCommunityHTTPStrictCSRFIndependentReviewCurrentAccessAndNoFinanceSideEffects(t *testing.T) {
	db, app, logs := handler(t)
	h := app.Handler()
	a := signIn(t, h, "admin@demo.society")
	b := signIn(t, h, "committee@demo.society")
	o := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	status := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatal("expected", code, w.Code, w.Body)
		}
	}
	encode := func(value any) string {
		data, e := json.Marshal(value)
		if e != nil {
			t.Fatal(e)
		}
		return string(data)
	}
	read := a.request("GET", "/api/community/options", "")
	status(read, 200)
	var options database.CommunityOptions
	if e := json.Unmarshal(read.Body.Bytes(), &options); e != nil {
		t.Fatal(e)
	}
	if len(options.Homes) != 118 || len(options.Buildings) != 3 || len(options.AreaKey) != 64 {
		t.Fatal(options)
	}
	in := database.CommunityInput{OperationKey: "community-http-create-12345", Kind: "CONTACT", Action: "PUBLISH", Title: "Fictional supplied help contact", Body: "This supplied fictional contact is deliberately published for one current home.", Service: "OTHER", Phone: "+91 (90000) 00101", Availability: "Supplied weekday availability", Attestation: "PRIVATE authority and consent attestation", Scope: "HOMES", HomeIDs: []string{"demo-flat-A-101"}, AreaKey: options.AreaKey, Reason: "PRIVATE checked supplied directory contact information", Confirmed: true}
	body := encode(in)
	missing := a
	missing.csrf = ""
	status(missing.request("POST", "/api/community", body), 403)
	status(o.request("POST", "/api/community", body), 403)
	for _, bad := range []string{body + "{}", strings.TrimSuffix(body, "}") + `,"reviewed_by":"spoofed"}`, strings.Repeat(" ", 32769) + body} {
		status(a.request("POST", "/api/community", bad), 400)
	}
	foreign := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/community", strings.NewReader(body))
	foreign.AddCookie(a.cookie)
	foreign.Header.Set("Origin", "https://foreign.example.test")
	foreign.Header.Set("X-CSRF-Token", a.csrf)
	foreign.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, foreign)
	status(w, 403)
	created := a.request("POST", "/api/community", body)
	status(created, 200)
	var result struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal(created.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	status(a.request("POST", "/api/community", body), 200)
	status(o.request("GET", "/api/community/"+result.ID, ""), 404)
	status(o.request("GET", "/api/community?desk=true", ""), 403)
	status(o.request("GET", "/api/community/options", ""), 403)
	status(a.request("GET", "/api/community?desk=invalid", ""), 400)
	status(a.request("GET", "/api/community?page=10001", ""), 400)
	decision := database.CommunityAction{OperationKey: "community-http-review-12345", Version: 1, Action: "APPROVED", Reason: "PRIVATE independently reviewed the exact contact and frozen home", Confirmed: true}
	status(a.request("POST", "/api/community/"+result.ID+"/decisions", encode(decision)), 403)
	status(b.request("POST", "/api/community/"+result.ID+"/decisions", encode(decision)), 200)
	status(b.request("POST", "/api/community/"+result.ID+"/decisions", encode(decision)), 200)
	public := o.request("GET", "/api/community/"+result.ID, "")
	status(public, 200)
	for _, secret := range []string{"PRIVATE", "attestation", "submitted_by", "submitted_at", "events", "reason"} {
		if strings.Contains(public.Body.String(), secret) {
			t.Fatal("public disclosure", secret)
		}
	}
	if !strings.Contains(public.Body.String(), "+919000000101") {
		t.Fatal("approved directory phone missing")
	}
	status(tenant.request("GET", "/api/community/"+result.ID, ""), 403)
	for _, table := range []string{"entries", "receipts", "message_batches", "simulation_messages"} {
		var n int
		if e := db.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal(table, n, e)
		}
	}
	for _, secret := range []string{"PRIVATE", "9000000101", "authority and consent"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("private logs", secret)
		}
	}
}
