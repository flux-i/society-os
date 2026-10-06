package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/messaging"
)

func messageJSON(t *testing.T, value any) string {
	t.Helper()
	body, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	return string(body)
}
func messageStatus(t *testing.T, w *httptest.ResponseRecorder, code int) {
	t.Helper()
	if w.Code != code {
		t.Fatal(w.Code, w.Body, "expected", code)
	}
}

func TestMessageHTTPExactTenantTargetSeparateApprovalUnknownRetryAndSignedCallbacks(t *testing.T) {
	db, app, logs := handler(t)
	engine, e := messaging.New(db, bytes.Repeat([]byte{47}, 32))
	if e != nil {
		t.Fatal(e)
	}
	app.Messages = engine
	h := app.Handler()
	a, b, tenant, owner := signIn(t, h, "admin@demo.society"), signIn(t, h, "committee@demo.society"), signIn(t, h, "tenant@demo.society"), signIn(t, h, "owner@demo.society")
	contact := database.ContactInput{OperationKey: "message-http-contact-12345", Email: "PRIVATE-tenant@example.test", PreferredChannel: "EMAIL", ConsentSource: "PRIVATE supplied identity and communication permission", Reason: "PRIVATE deliberate fictional communication choice", ContactPreferences: database.ContactPreferences{CommunityEmail: true}, Confirmed: true}
	messageStatus(t, tenant.request("POST", "/api/contacts/me/register", messageJSON(t, contact)), 200)
	verify := database.ContactAction{OperationKey: "message-http-verify-12345", Version: 1, Action: "VERIFIED", Reason: "PRIVATE verified identity and independent permission", Confirmed: true}
	messageStatus(t, b.request("POST", "/api/contacts/demo-tenant-A-103/actions", messageJSON(t, verify)), 200)
	notice, e := db.SubmitReview(context.Background(), a.cookie.Value, "", database.ReviewInput{OperationKey: "message-http-notice-12345", Kind: "NOTICE", Title: "Fictional tenant water notice", Body: "Private fictional update for tenant audience only.", Audience: "TENANTS_ONLY"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.DecideReview(context.Background(), b.cookie.Value, notice, database.ReviewAction{OperationKey: "message-http-notice-review-12345", Version: 1, Decision: "APPROVED", Reason: "Reviewed the fictional tenant audience and publication.", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	in := database.MessageInput{SourceKind: "NOTICE", SourceID: notice, Channel: "EMAIL", Target: database.MessageTarget{Kind: "ALL", IDs: []string{}}}
	missingCSRF := a
	missingCSRF.csrf = ""
	messageStatus(t, missingCSRF.request("POST", "/api/messages/preview", messageJSON(t, in)), 403)
	messageStatus(t, owner.request("POST", "/api/messages/preview", messageJSON(t, in)), 403)
	for _, bad := range []string{messageJSON(t, in) + "{}", strings.TrimSuffix(messageJSON(t, in), "}") + `,"portal_origin":"https://attacker.example.test"}`, strings.Repeat(" ", 32769) + messageJSON(t, in)} {
		messageStatus(t, a.request("POST", "/api/messages/preview", bad), 400)
	}
	w := a.request("POST", "/api/messages/preview", messageJSON(t, in))
	messageStatus(t, w, 200)
	var preview database.MessagePreview
	if e = json.Unmarshal(w.Body.Bytes(), &preview); e != nil {
		t.Fatal(e)
	}
	if preview.Counts.TargetPeople != 153 || preview.Counts.SourcePeople != 35 || preview.Counts.EligiblePeople != 1 || preview.Counts.Destinations != 1 || preview.Counts.Reasons["NO_SOURCE_ACCESS"] != 118 || len(preview.Recipients) != 20 || !preview.Simulation {
		t.Fatal(preview)
	}
	in.OperationKey, in.PreviewHash, in.Reason, in.Confirmed = "message-http-propose-12345", preview.PreviewHash, "PRIVATE reviewed target counts, omissions and the exact envelope.", true
	w = a.request("POST", "/api/messages", messageJSON(t, in))
	messageStatus(t, w, 200)
	var result struct{ ID string }
	if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	messageStatus(t, a.request("POST", "/api/messages", messageJSON(t, in)), 200)
	messageStatus(t, tenant.request("GET", "/api/messages/"+result.ID, ""), 404)
	approve := database.MessageAction{OperationKey: "message-http-approve-12345", Version: 1, Action: "APPROVED", Reason: "PRIVATE independently checked the frozen delivery proposal.", Confirmed: true}
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/actions", messageJSON(t, approve)), 403)
	messageStatus(t, b.request("POST", "/api/messages/"+result.ID+"/actions", messageJSON(t, approve)), 200)
	messageStatus(t, owner.request("GET", "/api/messages/"+result.ID, ""), 404)
	dispatch := database.MessageAction{OperationKey: "message-http-dispatch-12345", Version: 2, Action: "DISPATCH", Reason: "PRIVATE checked current permissions and the synthetic outcome.", Confirmed: true, Outcome: "UNKNOWN"}
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/dispatch", messageJSON(t, dispatch)), 200)
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/dispatch", messageJSON(t, dispatch)), 200)
	read := func() database.MessageDetail {
		t.Helper()
		w := a.request("GET", "/api/messages/"+result.ID, "")
		messageStatus(t, w, 200)
		var out database.MessageDetail
		if e = json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	x := read()
	if x.Outcomes["UNKNOWN"] != 1 || x.Deliveries[0].ProviderID != "" || x.Deliveries[0].AcceptedAt != 0 || x.CanDispatch {
		t.Fatal(x)
	}
	reconcile := database.MessageAction{OperationKey: "message-http-reconcile-12345", Version: 3, Action: "RECONCILE", Reason: "PRIVATE reconciled the same durable synthetic handoff.", Confirmed: true}
	path := "/api/messages/" + result.ID + "/deliveries/" + x.Deliveries[0].ID + "/reconcile"
	messageStatus(t, a.request("POST", path, messageJSON(t, reconcile)), 200)
	messageStatus(t, a.request("POST", path, messageJSON(t, reconcile)), 200)
	x = read()
	if x.Outcomes["ACCEPTED"] != 1 || x.Deliveries[0].Attempts != 1 || x.Deliveries[0].DeliveredAt != 0 || x.Deliveries[0].ReadAt != 0 {
		t.Fatal(x)
	}
	proof := database.MessageCallback{EventID: "message-http-callback-12345", ProviderID: x.Deliveries[0].ProviderID, State: "READ", At: time.Now().Unix()}
	payload := []byte(messageJSON(t, proof))
	callback := func(body []byte, sig string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/simulation/message-events", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Simulation-Signature", sig)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	messageStatus(t, callback(payload, strings.Repeat("0", 64)), 403)
	messageStatus(t, callback(payload, engine.Signature(payload)), 200)
	messageStatus(t, callback(payload, engine.Signature(payload)), 200)
	proof.State = "FAILED"
	changed := []byte(messageJSON(t, proof))
	messageStatus(t, callback(changed, engine.Signature(changed)), 409)
	proof.EventID = "message-http-callback-unknown"
	proof.ProviderID = "simulation-unknown"
	changed = []byte(messageJSON(t, proof))
	messageStatus(t, callback(changed, engine.Signature(changed)), 404)
	proof.EventID = "message-http-callback-expired"
	proof.At = time.Now().Add(-6 * time.Minute).Unix()
	changed = []byte(messageJSON(t, proof))
	messageStatus(t, callback(changed, engine.Signature(changed)), 403)
	x = read()
	if x.Outcomes["READ"] != 1 || x.Deliveries[0].ReadAt == 0 || x.Deliveries[0].DeliveredAt != 0 {
		t.Fatal("unreported delivery manufactured", x)
	}
	w = tenant.request("GET", "/api/messages/"+result.ID, "")
	messageStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "PRIVATE") || strings.Contains(w.Body.String(), "demo-user-") || strings.Contains(w.Body.String(), "provider_id") {
		t.Fatal("staff-private message data leaked", w.Body)
	}
	for _, path := range []string{"/api/messages?page=10001", "/api/messages?kind=UNKNOWN", "/api/messages/" + result.ID + "?recipient_page=0", "/api/messages/sources?kind=RECEIPT", "/api/messages/targets?source_kind=NOTICE&source_id=" + notice + "&kind=ALL"} {
		w := a.request("GET", path, "")
		want := 400
		if strings.Contains(path, "sources?kind=RECEIPT") {
			want = 403
		}
		messageStatus(t, w, want)
	}
	var n int
	if e = db.DB.QueryRow("SELECT COUNT(*) FROM simulation_messages").Scan(&n); e != nil || n != 1 {
		t.Fatal("duplicate synthetic handoff", n, e)
	}
	if e = db.DB.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&n); e != nil || n != 0 {
		t.Fatal("messaging posted money", n, e)
	}
	if strings.Contains(logs.String(), "PRIVATE") || strings.Contains(logs.String(), a.cookie.Value) {
		t.Fatal("private delivery data leaked into logs")
	}
}

func TestMessageHTTPUnconfiguredProviderReturnsUnavailableWithoutClaimingOrFakeSuccess(t *testing.T) {
	db, app, _ := handler(t)
	h := app.Handler()
	a := signIn(t, h, "admin@demo.society")
	for _, path := range []string{"/api/messages/preview", "/api/messages", "/api/messages/missing/dispatch", "/api/messages/missing/deliveries/missing/reconcile"} {
		messageStatus(t, a.request("POST", path, "{}"), 503)
	}
	w := a.request("GET", "/api/messages/config", "")
	messageStatus(t, w, 200)
	if strings.Contains(w.Body.String(), `"simulation_enabled":true`) || strings.Contains(w.Body.String(), `"whatsapp_live":true`) || strings.Contains(w.Body.String(), `"email_live":true`) {
		t.Fatal("unconfigured provider claimed availability", w.Body)
	}
	for _, table := range []string{"message_batches", "message_attempts", "simulation_messages"} {
		var n int
		if e := db.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal(table, n, e)
		}
	}
}
