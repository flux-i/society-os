package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/messaging"
)

func TestWhatsAppHTTPFrozenPortalSendSeparateApprovalAndAuthenticatedEvidence(t *testing.T) {
	db, app, logs := handler(t)
	var posts atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer FICTIONAL_SERVER_ACCESS_TOKEN_123456789" {
			t.Error("missing held credential")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && r.URL.Path == "/v26.0/444444" {
			fmt.Fprint(w, `{"id":"444444","name":"society_notice","language":"en","status":"APPROVED","category":"MARKETING","components":[{"type":"BODY","text":"Open your society update securely: {{1}}"}]}`)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v26.0/222222/messages" {
			posts.Add(1)
			fmt.Fprint(w, `{"messaging_product":"whatsapp","messages":[{"id":"wamid.FICTIONAL_SERVER_001"}]}`)
			return
		}
		w.WriteHeader(404)
	}))
	defer fixture.Close()
	client, err := messaging.NewWhatsAppClient(messaging.WhatsAppConfig{Origin: fixture.URL, APIVersion: "v26.0", AccountID: "111111", PhoneID: "222222", AccessToken: "FICTIONAL_SERVER_ACCESS_TOKEN_123456789", AppSecret: "FICTIONAL_SERVER_APP_SECRET_123456789", VerifyToken: "FICTIONAL_SERVER_VERIFY_TOKEN_123456789", Templates: map[string]messaging.WhatsAppTemplate{"NOTICE": {ID: "444444", Name: "society_notice", Language: "en"}}}, true)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := messaging.New(db, bytes.Repeat([]byte{51}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.WithWhatsAppFixture(client); err != nil {
		t.Fatal(err)
	}
	if err = engine.VerifyWhatsAppKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	app.Messages = engine
	h := app.Handler()
	a, b, o := signIn(t, h, "admin@demo.society"), signIn(t, h, "committee@demo.society"), signIn(t, h, "owner@demo.society")
	contact := database.ContactInput{OperationKey: "whatsapp-http-contact-12345", Phone: "+919000000101", PreferredChannel: "WHATSAPP", ConsentSource: "PRIVATE supplied fictional destination and communication choice", Reason: "PRIVATE deliberately registered a fictional WhatsApp destination", Confirmed: true, ContactPreferences: database.ContactPreferences{CommunityWhatsApp: true}}
	messageStatus(t, o.request("POST", "/api/contacts/me/register", messageJSON(t, contact)), 200)
	messageStatus(t, b.request("POST", "/api/contacts/demo-owner-A-101/actions", messageJSON(t, database.ContactAction{OperationKey: "whatsapp-http-contact-review-12345", Version: 1, Action: "VERIFIED", Reason: "PRIVATE independently verified the fictional identity and permission", Confirmed: true})), 200)
	ctx := context.Background()
	notice, err := db.SubmitReview(ctx, a.cookie.Value, "", database.ReviewInput{OperationKey: "whatsapp-http-notice-12345", Kind: "NOTICE", Title: "PRIVATE provider source title", Body: "PRIVATE original notice remains inside its authenticated audience.", Audience: "ALL_RESIDENTS"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.DecideReview(ctx, b.cookie.Value, notice, database.ReviewAction{OperationKey: "whatsapp-http-notice-review-12345", Version: 1, Decision: "APPROVED", Reason: "Reviewed the original notice and audience independently", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	in := database.MessageInput{SourceKind: "NOTICE", SourceID: notice, Channel: "WHATSAPP", Target: database.MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}}
	messageStatus(t, o.request("POST", "/api/messages/preview", messageJSON(t, in)), 403)
	messageStatus(t, a.request("POST", "/api/messages/preview", strings.TrimSuffix(messageJSON(t, in), "}")+`,"provider":{"mode":"CLOUD"}}`), 400)
	w := a.request("POST", "/api/messages/preview", messageJSON(t, in))
	messageStatus(t, w, 200)
	var preview database.MessagePreview
	json.Unmarshal(w.Body.Bytes(), &preview)
	if preview.Provider == nil || preview.Provider.Mode != "CLOUD_FIXTURE" || preview.Provider.Category != "MARKETING" || preview.Counts.Destinations != 1 || strings.Contains(preview.Envelope, "PRIVATE") {
		t.Fatal(preview)
	}
	in.OperationKey, in.PreviewHash, in.Reason, in.Confirmed = "whatsapp-http-propose-12345", preview.PreviewHash, "Reviewed the exact approved template and one fictional destination.", true
	w = a.request("POST", "/api/messages", messageJSON(t, in))
	messageStatus(t, w, 200)
	var result struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &result)
	messageStatus(t, o.request("GET", "/api/messages/"+result.ID, ""), 404)
	approve := database.MessageAction{OperationKey: "whatsapp-http-approve-12345", Version: 1, Action: "APPROVED", Reason: "Independently reviewed the frozen provider wording and audience", Confirmed: true}
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/actions", messageJSON(t, approve)), 403)
	messageStatus(t, b.request("POST", "/api/messages/"+result.ID+"/actions", messageJSON(t, approve)), 200)
	dispatch := database.MessageAction{OperationKey: "whatsapp-http-dispatch-12345", Version: 2, Action: "DISPATCH", Reason: "Reviewed current permission and deliberately requested this provider handoff", Confirmed: true}
	messageStatus(t, o.request("POST", "/api/messages/"+result.ID+"/dispatch", messageJSON(t, dispatch)), 403)
	bad := dispatch
	bad.Outcome = "READ"
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/dispatch", messageJSON(t, bad)), 400)
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/dispatch", messageJSON(t, dispatch)), 200)
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/dispatch", messageJSON(t, dispatch)), 200)
	if posts.Load() != 1 {
		t.Fatal("duplicated provider send", posts.Load())
	}
	x, err := db.MessageFor(ctx, a.cookie.Value, result.ID, 1, 1, 1)
	if err != nil || x.Outcomes["ACCEPTED"] != 1 || x.Deliveries[0].DeliveredAt != 0 || x.Deliveries[0].ReadAt != 0 {
		t.Fatal(x, err)
	}
	callback := func(payload []byte, signature string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/providers/whatsapp/webhook", bytes.NewReader(payload))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Hub-Signature-256", signature)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	payload := []byte(fmt.Sprintf(`{"object":"whatsapp_business_account","entry":[{"id":"111111","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","metadata":{"phone_number_id":"222222"},"statuses":[{"id":"wamid.FICTIONAL_SERVER_001","status":"read","timestamp":"%d","recipient_id":"919000000101"}]}}]}]}`, time.Now().Unix()))
	mac := hmac.New(sha256.New, []byte("FICTIONAL_SERVER_APP_SECRET_123456789"))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	messageStatus(t, callback(payload, "sha256="+strings.Repeat("0", 64)), 403)
	messageStatus(t, callback(payload, signature), 200)
	messageStatus(t, callback(payload, signature), 200)
	x, err = db.MessageFor(ctx, a.cookie.Value, result.ID, 1, 1, 1)
	if err != nil || x.Outcomes["READ"] != 1 || x.Deliveries[0].ReadAt == 0 || x.Deliveries[0].DeliveredAt != 0 {
		t.Fatal("signed proof", x, err)
	}
	w = o.request("GET", "/api/messages/"+result.ID, "")
	messageStatus(t, w, 200)
	if strings.Contains(w.Body.String(), `"provider":`) || strings.Contains(w.Body.String(), "FICTIONAL_SERVER") {
		t.Fatal("provider configuration leaked to resident", w.Body)
	}
	messageStatus(t, callback(bytes.Repeat([]byte{' '}, messaging.WhatsAppWebhookLimit+1), signature), 413)
	if strings.Contains(logs.String(), "PRIVATE") || strings.Contains(logs.String(), "FICTIONAL_SERVER") || strings.Contains(logs.String(), "919000000101") {
		t.Fatal("private provider data leaked into HTTP logs")
	}
}
