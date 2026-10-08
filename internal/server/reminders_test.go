package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/messaging"
)

func TestReminderHTTPStrictCriteriaCSRFSeparateReviewCurrentEligibilityAndPrivateQueue(t *testing.T) {
	s, app, logs := handler(t)
	engine, e := messaging.New(s, bytes.Repeat([]byte{47}, 32))
	if e != nil {
		t.Fatal(e)
	}
	app.Messages = engine
	h := app.Handler()
	ctx := context.Background()
	a, b, o, tenant := signIn(t, h, "admin@demo.society"), signIn(t, h, "committee@demo.society"), signIn(t, h, "owner@demo.society"), signIn(t, h, "tenant@demo.society")
	contact := database.ContactInput{OperationKey: "reminder-http-contact-12345", Email: "PRIVATE-reminder@example.test", PreferredChannel: "EMAIL", ConsentSource: "PRIVATE explicit fictional community permission", Reason: "PRIVATE deliberate reminder contact preference", ContactPreferences: database.ContactPreferences{CommunityEmail: true}, Confirmed: true}
	messageStatus(t, o.request("POST", "/api/contacts/me/register", messageJSON(t, contact)), 200)
	messageStatus(t, b.request("POST", "/api/contacts/demo-owner-A-101/actions", messageJSON(t, database.ContactAction{OperationKey: "reminder-http-verify-12345", Version: 1, Action: "VERIFIED", Reason: "PRIVATE independently verified the person and destination", Confirmed: true})), 200)
	options, e := s.CommunityOptionsFor(ctx, a.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	meeting, e := s.ProposeMeeting(ctx, a.cookie.Value, "", database.MeetingInput{OperationKey: "reminder-http-meeting-12345", Action: "AGENDA", Title: "PRIVATE protected meeting source", Body: "The supplied fictional agenda requires deliberate acknowledgement.", Location: "Fictional community room", Scope: "HOMES", HomeIDs: []string{"demo-flat-A-101"}, AreaKey: options.AreaKey, StartAt: time.Now().Add(time.Hour).Unix(), AckRequired: true, Reason: "PRIVATE supplied original meeting and exact audience", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideMeeting(ctx, b.cookie.Value, meeting, database.MeetingAction{OperationKey: "reminder-http-meeting-review-12345", Version: 1, Action: "APPROVED", Reason: "PRIVATE separately checked this meeting version", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	in := database.MessageInput{SourceKind: "MEETING_REMINDER", SourceID: meeting, ReminderBasis: "OUTSTANDING", Channel: "EMAIL", Target: database.MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}}
	missing := a
	missing.csrf = ""
	messageStatus(t, missing.request("POST", "/api/messages/preview", messageJSON(t, in)), 403)
	messageStatus(t, o.request("POST", "/api/messages/preview", messageJSON(t, in)), 403)
	for _, bad := range []string{messageJSON(t, in) + "{}", strings.TrimSuffix(messageJSON(t, in), "}") + `,"outstanding_paise":1}`, strings.Replace(messageJSON(t, in), "OUTSTANDING", "PAID", 1)} {
		messageStatus(t, a.request("POST", "/api/messages/preview", bad), 400)
	}
	w := a.request("POST", "/api/messages/preview", messageJSON(t, in))
	messageStatus(t, w, 200)
	var preview database.MessagePreview
	if e = json.Unmarshal(w.Body.Bytes(), &preview); e != nil {
		t.Fatal(e)
	}
	if preview.Counts.EligiblePeople != 1 || preview.Counts.Destinations != 1 || preview.Recipients[0].Reminder == nil || preview.Recipients[0].Reminder.OutstandingPaise != 0 {
		t.Fatal("exact personal source", preview)
	}
	in.OperationKey, in.PreviewHash, in.Reason, in.Confirmed = "reminder-http-proposal-12345", preview.PreviewHash, "PRIVATE exact reminder content and eligible person checked", true
	w = a.request("POST", "/api/messages", messageJSON(t, in))
	messageStatus(t, w, 200)
	var result struct{ ID string }
	if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	approve := database.MessageAction{OperationKey: "reminder-http-approval-12345", Version: 1, Action: "APPROVED", Reason: "PRIVATE separate review of exact reminder and recipient", Confirmed: true}
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/actions", messageJSON(t, approve)), 403)
	messageStatus(t, b.request("POST", "/api/messages/"+result.ID+"/actions", messageJSON(t, approve)), 200)
	messageStatus(t, tenant.request("GET", "/api/messages/"+result.ID, ""), 404)
	w = o.request("GET", "/api/messages/"+result.ID, "")
	messageStatus(t, w, 200)
	for _, secret := range []string{"PRIVATE", "fingerprint", "outstanding_paise", "publication_version", "demo-joint-owner", "provider_id"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("private reminder disclosure", secret, w.Body)
		}
	}
	published, e := s.MeetingFor(ctx, o.cookie.Value, meeting, false, 1)
	if e != nil {
		t.Fatal(e)
	}
	messageStatus(t, o.request("POST", "/api/meetings/"+meeting+"/acknowledgements", messageJSON(t, database.MeetingAcknowledgementInput{OperationKey: "reminder-http-ack-12345", Version: published.Version, Fingerprint: published.Acknowledgement.Fingerprint, Confirmed: true})), 200)
	dispatch := database.MessageAction{OperationKey: "reminder-http-dispatch-12345", Version: 2, Action: "DISPATCH", Outcome: "UNKNOWN", Reason: "PRIVATE deliberately checked the approved reminder before dispatch", Confirmed: true}
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/dispatch", messageJSON(t, dispatch)), 200)
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/dispatch", messageJSON(t, dispatch)), 200)
	w = a.request("GET", "/api/messages/exceptions", "")
	messageStatus(t, w, 200)
	var queue database.MessageExceptionPage
	if e = json.Unmarshal(w.Body.Bytes(), &queue); e != nil || queue.Total != 0 {
		t.Fatal("acknowledged reminder still actionable", queue, e)
	}
	for _, path := range []string{"/api/messages/exceptions?page=0", "/api/messages/exceptions?page=10001", "/api/messages/exceptions?state=DELIVERED"} {
		messageStatus(t, a.request("GET", path, ""), 400)
	}
	messageStatus(t, o.request("GET", "/api/messages/exceptions", ""), 403)
	messageStatus(t, b.request("GET", "/api/messages/sources?kind=MAINTENANCE_REMINDER", ""), 403)
	for _, table := range []string{"message_attempts", "simulation_messages", "receipts", "entries"} {
		var count int
		if e = s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); e != nil || count != 0 {
			t.Fatal("reminder side effect", table, count, e)
		}
	}
	if strings.Contains(logs.String(), "PRIVATE") || strings.Contains(logs.String(), preview.PreviewHash) {
		t.Fatal("private content in logs")
	}
}
