package server

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"society.local/portal/internal/database"
	"strings"
	"testing"
)

func TestIncidentsHTTPStrictPrivateEvidenceSeparateReviewSubjectNoticeAndCurrentHome(t *testing.T) {
	store, app, logs := handler(t)
	h := app.Handler()
	admin := signIn(t, h, "admin@demo.society")
	committee := signIn(t, h, "committee@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	encode := func(x any) string {
		t.Helper()
		b, e := json.Marshal(x)
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	identity := func(w *httptest.ResponseRecorder) string {
		t.Helper()
		var x struct{ ID string }
		if e := json.Unmarshal(w.Body.Bytes(), &x); e != nil || w.Code != 200 || x.ID == "" {
			t.Fatal(w.Code, w.Body, e)
		}
		return x.ID
	}
	ruleBody := `{"operation_key":"http-rule-proposal-12345","title":"Shared corridors stay clear","text":"Supplied fictional policy protects shared corridor access.","policy_reference":"Fictional supplied committee policy R-01","effective_from":"2026-01-01","effective_until":"","fine_permitted":true,"confirmed":true}`
	if w := owner.request("POST", "/api/rules", ruleBody); w.Code != 403 {
		t.Fatal("resident proposed policy", w.Code)
	}
	noCSRF := admin
	noCSRF.csrf = ""
	if w := noCSRF.request("POST", "/api/rules", ruleBody); w.Code != 403 {
		t.Fatal("CSRF", w.Code)
	}
	for _, body := range []string{ruleBody + "{}", strings.TrimSuffix(ruleBody, "}") + `,"author_id":"spoofed"}`, strings.Repeat(" ", 32769) + ruleBody} {
		if w := admin.request("POST", "/api/rules", body); w.Code != 400 {
			t.Fatal("strict body", w.Code)
		}
	}
	rule := identity(admin.request("POST", "/api/rules", ruleBody))
	action := database.RuleAction{OperationKey: "http-rule-publication-12345", Version: 1, Action: "PUBLISHED", Reason: "Separately checked the supplied fictional rule policy", Confirmed: true}
	if w := admin.request("POST", "/api/rules/"+rule+"/actions", encode(action)); w.Code != 403 {
		t.Fatal("self approval", w.Code)
	}
	identity(committee.request("POST", "/api/rules/"+rule+"/actions", encode(action)))
	var pngBytes bytes.Buffer
	if e := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 12, 9))); e != nil {
		t.Fatal(e)
	}
	upload := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/incident-pictures", bytes.NewReader(pngBytes.Bytes()))
		r.AddCookie(tenant.cookie)
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", tenant.csrf)
		r.Header.Set("Content-Type", "application/octet-stream")
		r.Header.Set("X-Operation-Key", "http-private-picture-12345")
		r.Header.Set("X-Picture-Filename", "private-scene.png")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := upload("https://foreign.example.test"); w.Code != 403 {
		t.Fatal("foreign origin upload", w.Code)
	}
	picture := identity(upload("http://127.0.0.1:8080"))
	if again := identity(upload("http://127.0.0.1:8080")); again != picture {
		t.Fatal("picture duplicate")
	}
	if w := owner.request("GET", "/api/incident-pictures/"+picture+"/preview", ""); w.Code != 404 {
		t.Fatal("unshared picture", w.Code)
	}
	in := database.IncidentInput{OperationKey: "http-private-case-12345", RuleID: rule, FlatID: "demo-flat-A-101", IncidentDate: "2026-01-02", Comment: "PRIVATE reporter observation and supporting evidence detail.", PictureID: picture, Confirmed: true}
	report := identity(tenant.request("POST", "/api/incidents", encode(in)))
	for _, actor := range []authClient{owner} {
		if w := actor.request("GET", "/api/incidents/"+report, ""); w.Code != 404 {
			t.Fatal("subject private case", w.Code)
		}
	}
	if w := tenant.request("GET", "/api/incidents/options", ""); w.Code != 200 || strings.Contains(w.Body.String(), "Demo Owner") || strings.Contains(w.Body.String(), "memberships") {
		t.Fatal("tag selector privacy", w.Code, w.Body)
	}
	review := database.IncidentAction{OperationKey: "http-private-note-12345", Version: 1, Action: "NOTE", Reason: "PRIVATE staff coordination that the subject never sees", Confirmed: true}
	identity(admin.request("POST", "/api/incidents/"+report+"/actions", encode(review)))
	review.OperationKey = "http-response-notice-12345"
	review.Version = 2
	review.Action = "ISSUE_NOTICE"
	review.Title = "Please provide your account"
	review.Body = "A fictional corridor observation asks for a response."
	review.ResponseBy = "2099-01-01"
	identity(committee.request("POST", "/api/incidents/"+report+"/actions", encode(review)))
	w := owner.request("GET", "/api/incident-notices", "")
	var notices database.IncidentNoticePage
	if e := json.Unmarshal(w.Body.Bytes(), &notices); e != nil || w.Code != 200 || notices.Total != 1 {
		t.Fatal(w.Code, w.Body, e)
	}
	notice := notices.Items[0].ID
	w = owner.request("GET", "/api/incident-notices/"+notice, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE") || strings.Contains(w.Body.String(), "reporter_id") || strings.Contains(w.Body.String(), "Demo Tenant") {
		t.Fatal("notice leak", w.Code, w.Body)
	}
	preview := owner.request("GET", "/api/incident-pictures/"+picture+"/preview", "")
	if preview.Code != 200 || preview.Header().Get("Content-Type") != "image/png" || preview.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private preview", preview.Code, preview.Header())
	}
	if w = owner.request("GET", "/api/incident-pictures/"+picture+"/original", ""); w.Code != 404 {
		t.Fatal("original shared", w.Code)
	}
	original := tenant.request("GET", "/api/incident-pictures/"+picture+"/original", "")
	if original.Code != 200 || !strings.HasPrefix(original.Header().Get("Content-Disposition"), "attachment") || !bytes.Equal(original.Body.Bytes(), pngBytes.Bytes()) {
		t.Fatal("original contract", original.Code)
	}
	answer := database.IncidentResponseInput{OperationKey: "http-subject-response-12345", Version: 1, Body: "The household's supplied fictional explanation is retained.", Confirmed: true}
	response := identity(owner.request("POST", "/api/incident-notices/"+notice+"/responses", encode(answer)))
	if again := identity(owner.request("POST", "/api/incident-notices/"+notice+"/responses", encode(answer))); again != response {
		t.Fatal("response retry")
	}
	if _, e := store.DB.Exec("UPDATE flat_memberships SET end_date='2020-01-02' WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'"); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/api/incident-notices/" + notice, "/api/incident-pictures/" + picture + "/preview"} {
		if w = owner.request("GET", path, ""); w.Code != 404 {
			t.Fatal("ended home", path, w.Code)
		}
	}
	if w = owner.request("POST", "/api/incident-notices/"+notice+"/responses", encode(answer)); w.Code != 404 {
		t.Fatal("ended replay", w.Code)
	}
	for _, table := range []string{"entries", "receipts"} {
		var n int
		if e := store.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal("incident financial side effect", table, n, e)
		}
	}
	if strings.Contains(logs.String(), "PRIVATE") || strings.Contains(logs.String(), "private-scene") {
		t.Fatal("private log content")
	}
}
