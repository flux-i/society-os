package server

import (
	"encoding/json"
	"net/http/httptest"
	"society.local/portal/internal/database"
	"strings"
	"testing"
	"time"
)

func TestMeetingHTTPStrictCSRFSeparateReviewOwnAcknowledgementAndCurrentAccess(t *testing.T) {
	db, app, logs := handler(t)
	h := app.Handler()
	a, b := signIn(t, h, "admin@demo.society"), signIn(t, h, "committee@demo.society")
	o, tenant := signIn(t, h, "owner@demo.society"), signIn(t, h, "tenant@demo.society")
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
	w := a.request("GET", "/api/meetings/options", "")
	status(w, 200)
	var options database.CommunityOptions
	if e := json.Unmarshal(w.Body.Bytes(), &options); e != nil {
		t.Fatal(e)
	}
	if len(options.Homes) != 118 || strings.Contains(w.Body.String(), "resident") || strings.Contains(w.Body.String(), "phone") {
		t.Fatal("area options disclose people", w.Body)
	}
	in := database.MeetingInput{OperationKey: "meeting-http-create-12345", Action: "AGENDA", Title: "Fictional protected meeting", Body: "The supplied agenda is published only for this selected home.", Location: "Fictional community room", Scope: "HOMES", HomeIDs: []string{"demo-flat-A-101"}, AreaKey: options.AreaKey, StartAt: time.Now().Add(time.Hour).Unix(), AckRequired: true, Reason: "PRIVATE original agenda checked independently", Confirmed: true}
	body := encode(in)
	missing := a
	missing.csrf = ""
	status(missing.request("POST", "/api/meetings", body), 403)
	status(o.request("POST", "/api/meetings", body), 403)
	for _, bad := range []string{body + "{}", strings.TrimSuffix(body, "}") + `,"reviewed_by":"spoofed"}`, strings.Repeat(" ", 32769) + body} {
		status(a.request("POST", "/api/meetings", bad), 400)
	}
	foreign := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/meetings", strings.NewReader(body))
	foreign.AddCookie(a.cookie)
	foreign.Header.Set("Origin", "https://foreign.example.test")
	foreign.Header.Set("X-CSRF-Token", a.csrf)
	foreign.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, foreign)
	status(w, 403)
	w = a.request("POST", "/api/meetings", body)
	status(w, 200)
	var result struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	path := "/api/meetings/" + result.ID
	status(a.request("POST", "/api/meetings", body), 200)
	status(o.request("GET", path, ""), 404)
	status(o.request("GET", "/api/meetings?desk=true", ""), 403)
	status(o.request("GET", "/api/meetings/options", ""), 403)
	for _, query := range []string{"desk=invalid", "page=10001", "page=0", "state=PENDING"} {
		status(o.request("GET", "/api/meetings?"+query, ""), 400)
	}
	decision := database.MeetingAction{OperationKey: "meeting-http-review-12345", Version: 1, Action: "APPROVED", Reason: "PRIVATE independently checked this exact agenda", Confirmed: true}
	status(a.request("POST", path+"/decisions", encode(decision)), 403)
	status(b.request("POST", path+"/decisions", encode(decision)), 200)
	status(b.request("POST", path+"/decisions", encode(decision)), 200)
	w = o.request("GET", path, "")
	status(w, 200)
	var publication database.MeetingDetail
	if e := json.Unmarshal(w.Body.Bytes(), &publication); e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"PRIVATE", "submitted_by", "submitted_at", "events", "reason", "resident_id", "actor_id"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("publication disclosure", secret)
		}
	}
	status(tenant.request("GET", path, ""), 403)
	ack := database.MeetingAcknowledgementInput{OperationKey: "meeting-http-ack-12345", Version: publication.Version, Fingerprint: publication.Acknowledgement.Fingerprint, Confirmed: true}
	missing = o
	missing.csrf = ""
	status(missing.request("POST", path+"/acknowledgements", encode(ack)), 403)
	status(o.request("POST", path+"/acknowledgements", strings.TrimSuffix(encode(ack), "}")+`,"resident_id":"another-person"}`), 400)
	status(tenant.request("POST", path+"/acknowledgements", encode(ack)), 403)
	status(a.request("POST", path+"/acknowledgements", encode(ack)), 403)
	status(o.request("POST", path+"/acknowledgements", encode(ack)), 200)
	status(o.request("POST", path+"/acknowledgements", encode(ack)), 200)
	status(o.request("GET", path+"/responses", ""), 403)
	status(a.request("GET", path+"/responses", ""), 200)
	for _, query := range []string{"page=10001", "history_page=0", "history_page=10001"} {
		status(a.request("GET", path+"/responses?"+query, ""), 400)
	}
	if _, e := db.DB.Exec("UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101'", time.Now().In(time.FixedZone("IST", 19800)).Format("2006-01-02")); e != nil {
		t.Fatal(e)
	}
	status(o.request("POST", path+"/acknowledgements", encode(ack)), 403)
	status(o.request("GET", path, ""), 403)
	for _, table := range []string{"entries", "receipts", "message_batches", "simulation_messages"} {
		var n int
		if e := db.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal(table, n, e)
		}
	}
	var n int
	if e := db.DB.QueryRow("SELECT COUNT(*) FROM meeting_acknowledgements").Scan(&n); e != nil || n != 1 {
		t.Fatal("durable acknowledgement duplicates", n, e)
	}
	for _, secret := range []string{"PRIVATE", "protected meeting", "community room", publication.Acknowledgement.Fingerprint} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("private logs", secret)
		}
	}
}
