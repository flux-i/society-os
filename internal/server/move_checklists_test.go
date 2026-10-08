package server

import (
	"context"
	"encoding/json"
	"society.local/portal/internal/database"
	"strings"
	"testing"
)

func TestMoveChecklistHTTPStrictScopeSourcesSeparateReviewCSRFAndNoAutomaticSideEffects(t *testing.T) {
	s, app, logs := handler(t)
	h := app.Handler()
	ctx := context.Background()
	a := signIn(t, h, "admin@demo.society")
	b := signIn(t, h, "committee@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	messageStatus(t, b.request("GET", "/api/move-checklists", ""), 403)
	messageStatus(t, owner.request("GET", "/api/move-checklists", ""), 200)
	if _, e := s.GrantAppointment(ctx, a.cookie.Value, "demo-user-committee", database.AppointmentInput{AccessChange: database.AccessChange{Version: 1, Reason: "PRIVATE_HTTP_CHECKLIST separately verified registry reviewer", Confirmed: true}, Role: "ADMINISTRATOR", TermDays: 30}); e != nil {
		t.Fatal(e)
	}
	messageStatus(t, b.request("GET", "/api/move-checklists", ""), 401)
	b = signIn(t, h, "committee@demo.society")
	in := database.MoveChecklistInput{OperationKey: "move-http-original-12345", FlatID: "demo-flat-A-101", Kind: "CONTACT_REVIEW", EffectiveDate: "2026-10-10", Note: "PRIVATE_HTTP_CHECKLIST supplied personal registry and contact review.", Reason: "PRIVATE_HTTP_CHECKLIST deliberate confirmed original submission.", Confirmed: true}
	body := messageJSON(t, in)
	missing := owner
	missing.csrf = ""
	messageStatus(t, missing.request("POST", "/api/move-checklists", body), 403)
	for _, bad := range []string{body + "{}", strings.TrimSuffix(body, "}") + `,"actor_id":"demo-user-committee"}`, strings.Replace(body, "CONTACT_REVIEW", "FINANCIAL_REVIEW", 1)} {
		messageStatus(t, owner.request("POST", "/api/move-checklists", bad), 400)
	}
	w := owner.request("POST", "/api/move-checklists", body)
	messageStatus(t, w, 200)
	var created struct{ ID string }
	if e := json.Unmarshal(w.Body.Bytes(), &created); e != nil {
		t.Fatal(e)
	}
	repeated := owner.request("POST", "/api/move-checklists", body)
	messageStatus(t, repeated, 200)
	if repeated.Body.String() != w.Body.String() {
		t.Fatal("HTTP same-operation identity changed")
	}
	id := created.ID
	messageStatus(t, tenant.request("GET", "/api/move-checklists/"+id, ""), 404)
	messageStatus(t, owner.request("GET", "/api/move-checklists/options?home=demo-flat-A-103", ""), 404)
	detail := func(client authClient) database.MoveChecklistDetail {
		t.Helper()
		w := client.request("GET", "/api/move-checklists/"+id, "")
		messageStatus(t, w, 200)
		var out database.MoveChecklistDetail
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	action := func(x database.MoveChecklistDetail, kind string) database.MoveChecklistAction {
		return database.MoveChecklistAction{OperationKey: "move-http-" + kind + "-" + strings.Repeat("x", 12), Version: x.Version, Action: kind, Reason: "PRIVATE_HTTP_CHECKLIST deliberately reviewed exact source and effect.", Confirmed: true}
	}
	for _, kind := range []string{"IDENTITY", "REGISTRY", "CONTACT", "DOCUMENTS", "HANDOVER"} {
		x := detail(a)
		check := action(x, "CHECK")
		check.OperationKey += "-" + kind
		check.CheckKind = kind
		check.CheckState = "CHECKED"
		check.Reference = "PRIVATE_HTTP_CHECKLIST supplied source reference " + kind
		check.SourceKey = x.Source.Key
		messageStatus(t, owner.request("POST", "/api/move-checklists/"+id+"/actions", messageJSON(t, check)), 403)
		messageStatus(t, a.request("POST", "/api/move-checklists/"+id+"/actions", messageJSON(t, check)), 200)
	}
	x := detail(a)
	ready := action(x, "READY")
	ready.SourceKey = x.Source.Key
	messageStatus(t, a.request("POST", "/api/move-checklists/"+id+"/actions", messageJSON(t, ready)), 200)
	x = detail(b)
	approve := action(x, "APPROVED")
	approve.SourceKey = x.Source.Key
	messageStatus(t, a.request("POST", "/api/move-checklists/"+id+"/actions", messageJSON(t, approve)), 403)
	messageStatus(t, b.request("POST", "/api/move-checklists/"+id+"/actions", messageJSON(t, approve)), 200)
	messageStatus(t, b.request("POST", "/api/move-checklists/"+id+"/actions", messageJSON(t, approve)), 200)
	x = detail(owner)
	if x.Version != 8 || x.EventTotal != 8 || x.State != "COMPLETED" || x.Approved == nil || x.Approved.Version != 8 {
		t.Fatal("independent completed version/events", x)
	}
	for _, path := range []string{"/api/move-checklists?page=0", "/api/move-checklists?page=10001", "/api/move-checklists/options?page=10001", "/api/move-checklists/" + id + "?event_page=10001"} {
		messageStatus(t, a.request("GET", path, ""), 400)
	}
	for _, table := range []string{"entries", "receipts", "message_batches", "resident_contacts"} {
		var count int
		if e := s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); e != nil || count != 0 {
			t.Fatal("checklist changed unrelated records", table, count, e)
		}
	}
	var version int
	if e := s.DB.QueryRow("SELECT version FROM flats WHERE id='demo-flat-A-101'").Scan(&version); e != nil || version != 1 {
		t.Fatal("checklist automatically changed registry", version, e)
	}
	if strings.Contains(logs.String(), "PRIVATE_HTTP_CHECKLIST") || strings.Contains(logs.String(), owner.cookie.Value) {
		t.Fatal("request logs exposed private checklist source")
	}
}
