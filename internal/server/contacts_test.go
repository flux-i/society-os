package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"society.local/portal/internal/database"
)

func TestContactsHTTPSeparateIdentityReviewPrivacyStrictWritesAndImmediateOptOut(t *testing.T) {
	db, app, logs := handler(t)
	h := app.Handler()
	a := signIn(t, h, "admin@demo.society")
	b := signIn(t, h, "committee@demo.society")
	o := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	encode := func(x any) string {
		t.Helper()
		data, e := json.Marshal(x)
		if e != nil {
			t.Fatal(e)
		}
		return string(data)
	}
	status := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatal(w.Code, w.Body, "expected", code)
		}
	}
	input := database.ContactInput{OperationKey: "contact-http-register-12345", Phone: "+919000000101", Email: "http-contact@example.test", PreferredChannel: "EMAIL", ConsentSource: "PRIVATE supplied identity and permission reference", Reason: "PRIVATE deliberately supplied this fictional communication choice", ContactPreferences: database.ContactPreferences{CommunityEmail: true, FinanceEmail: true}, Confirmed: true}
	body := encode(input)
	noCSRF := o
	noCSRF.csrf = ""
	status(noCSRF.request("POST", "/api/contacts/me/register", body), 403)
	for _, bad := range []string{body + "{}", strings.TrimSuffix(body, "}") + `,"reviewed_by":"spoofed"}`, strings.Repeat(" ", 32769) + body} {
		status(o.request("POST", "/api/contacts/me/register", bad), 400)
	}
	foreign := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/contacts/me/register", strings.NewReader(body))
	foreign.AddCookie(o.cookie)
	foreign.Header.Set("Origin", "https://foreign.example.test")
	foreign.Header.Set("X-CSRF-Token", o.csrf)
	foreign.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, foreign)
	status(w, 403)
	status(o.request("POST", "/api/contacts/me/register", body), 200)
	status(o.request("POST", "/api/contacts/me/register", body), 200)
	read := func() database.ContactDetail {
		t.Helper()
		w := o.request("GET", "/api/contacts/me", "")
		status(w, 200)
		var x database.ContactDetail
		if e := json.Unmarshal(w.Body.Bytes(), &x); e != nil {
			t.Fatal(e)
		}
		return x
	}
	x := read()
	if x.State != "PENDING" || x.Version != 1 || x.Eligible != (database.ContactPreferences{}) {
		t.Fatal(x)
	}
	action := database.ContactAction{OperationKey: "contact-http-verify-12345", Version: 1, Action: "VERIFIED", Reason: "PRIVATE independently verified identity and deliberate permission", Confirmed: true}
	status(o.request("POST", "/api/contacts/me/actions", encode(action)), 403)
	status(b.request("POST", "/api/contacts/"+x.ID+"/actions", encode(action)), 200)
	status(tenant.request("GET", "/api/contacts/"+x.ID, ""), 404)
	status(tenant.request("GET", "/api/contacts/me?event_page=10001", ""), 400)
	status(a.request("GET", "/api/contacts?relationship=ALL", ""), 400)
	x = read()
	if x.Eligible != (database.ContactPreferences{CommunityEmail: true, FinanceEmail: true}) || x.EventTotal != 2 {
		t.Fatal(x)
	}
	stop := database.ContactAction{OperationKey: "contact-http-stop-12345", Version: x.Version, Action: "OPTED_OUT", Channel: "EMAIL", Purpose: "FINANCE", Reason: "Please stop these selected fictional financial messages now", Confirmed: true}
	status(o.request("POST", "/api/contacts/me/actions", encode(stop)), 200)
	status(o.request("POST", "/api/contacts/me/actions", encode(stop)), 200)
	x = read()
	if x.State != "VERIFIED" || x.Eligible != (database.ContactPreferences{CommunityEmail: true}) || x.Version != 3 || x.EventTotal != 3 {
		t.Fatal(x)
	}
	var n int
	if e := db.DB.QueryRow("SELECT COUNT(*) FROM entries").Scan(&n); e != nil || n != 0 {
		t.Fatal("contacts posted money", n, e)
	}
	for _, private := range []string{"PRIVATE", "9000000101", "http-contact@example.test"} {
		if strings.Contains(logs.String(), private) {
			t.Fatal("private contact data leaked into HTTP logs", private)
		}
	}
}
