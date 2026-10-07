package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/documents"
	"society.local/portal/internal/messaging"
)

func TestStatementMessageHTTPFinanceAuthorityFrozenLinkRevokedSourceAndRetainedUnknownProof(t *testing.T) {
	s, app, logs := handler(t)
	ctx := context.Background()
	engine, err := messaging.New(s, bytes.Repeat([]byte{53}, 32))
	if err != nil {
		t.Fatal(err)
	}
	app.Messages = engine
	h := app.Handler()
	a := signIn(t, h, "admin@demo.society")
	community := signIn(t, h, "committee@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	messageStatus(t, a.request("GET", "/api/messages/sources?kind=STATEMENT", ""), 403)
	messageStatus(t, community.request("GET", "/api/messages/sources?kind=STATEMENT", ""), 403)
	if err = s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	messageStatus(t, a.request("POST", "/api/admin/accounts/demo-user-committee/roles", `{"version":1,"confirmed":true,"reason":"Verified fictional separate statement finance reviewer","role":"TREASURER","term_days":30}`), 201)
	b := signIn(t, h, "committee@demo.society")
	contact := database.ContactInput{OperationKey: "statement-message-contact-12345", Email: "PRIVATE-statement-tenant@example.test", PreferredChannel: "EMAIL", ConsentSource: "PRIVATE checked identity and specific finance permission", Reason: "PRIVATE deliberately supplied finance contact", Confirmed: true, ContactPreferences: database.ContactPreferences{FinanceEmail: true}}
	messageStatus(t, tenant.request("POST", "/api/contacts/me/register", messageJSON(t, contact)), 200)
	messageStatus(t, b.request("POST", "/api/contacts/demo-tenant-A-103/actions", messageJSON(t, database.ContactAction{OperationKey: "statement-message-verify-12345", Version: 1, Action: "VERIFIED", Confirmed: true, Reason: "PRIVATE independently checked identity and finance permission"})), 200)
	data := []byte("Description,Amount\nPRIVATE external figure,999999.99\n")
	sum := sha256.Sum256(data)
	input := database.StatementInput{OperationKey: "statement-message-original-12345", Title: "PRIVATE supplied tenant balance sheet", Kind: "BALANCE", PeriodStart: "2026-09-01", PeriodEnd: "2026-09-30", PreparedBy: "PRIVATE fictional preparer", Source: "PRIVATE supplied accounting source", Filename: "balance.csv", Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Confirmed: true, Reason: "PRIVATE checked this exact external original"}
	file, err := s.ReserveStatement(ctx, a.cookie.Value, input)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteStatement(ctx, a.cookie.Value, file, data); err != nil {
		t.Fatal(err)
	}
	job, err := s.ClaimStatementCheck(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	mime, code := documents.ValidateStatementOriginal(ctx, job.Filename, job.Bytes)
	if code != "" {
		t.Fatal(code)
	}
	if err = s.FinishStatementCheck(ctx, job, mime, code, time.Now()); err != nil {
		t.Fatal(err)
	}
	x, err := s.StatementFor(ctx, b.cookie.Value, file, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideStatement(ctx, b.cookie.Value, file, database.StatementAction{OperationKey: "statement-message-internal-12345", Version: x.Version, Action: "APPROVED", Confirmed: true, Reason: "PRIVATE separately reviewed the original"}); err != nil {
		t.Fatal(err)
	}
	x, err = s.StatementFor(ctx, a.cookie.Value, file, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	pi := database.StatementPublicationInput{OperationKey: "statement-message-publication-12345", FileID: file, FileVersion: x.Version, Target: database.MessageTarget{Kind: "TENANTS"}, Confirmed: true, Reason: "PRIVATE deliberately publish to current tenants"}
	preview, err := s.PreviewStatementPublication(ctx, a.cookie.Value, pi)
	if err != nil || preview.TargetPeople != 35 {
		t.Fatal(preview, err)
	}
	pi.PreviewHash = preview.PreviewHash
	pub, err := s.ProposeStatementPublication(ctx, a.cookie.Value, pi)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideStatementPublication(ctx, b.cookie.Value, pub, database.StatementAction{OperationKey: "statement-message-publish-12345", Version: 1, Action: "PUBLISHED", Confirmed: true, Reason: "PRIVATE separately checked this exact publication"}); err != nil {
		t.Fatal(err)
	}
	in := database.MessageInput{SourceKind: "STATEMENT", SourceID: pub, Channel: "EMAIL", Target: database.MessageTarget{Kind: "ALL", IDs: []string{}}}
	noCSRF := a
	noCSRF.csrf = ""
	messageStatus(t, noCSRF.request("POST", "/api/messages/preview", messageJSON(t, in)), 403)
	messageStatus(t, owner.request("POST", "/api/messages/preview", messageJSON(t, in)), 403)
	// Appointment changes invalidate the earlier session; the independently
	// signed-in Treasury reviewer has the explicit current authority.
	messageStatus(t, community.request("POST", "/api/messages/preview", messageJSON(t, in)), 401)
	bad := strings.TrimSuffix(messageJSON(t, in), "}") + `,"purpose":"COMMUNITY"}`
	messageStatus(t, a.request("POST", "/api/messages/preview", bad), 400)
	w := a.request("POST", "/api/messages/preview", messageJSON(t, in))
	messageStatus(t, w, 200)
	var mp database.MessagePreview
	if err = json.Unmarshal(w.Body.Bytes(), &mp); err != nil {
		t.Fatal(err)
	}
	if mp.Purpose != "FINANCE" || mp.Counts.TargetPeople != 153 || mp.Counts.SourcePeople != 35 || mp.Counts.EligiblePeople != 1 || mp.Counts.Destinations != 1 || mp.Source.Link != "/#statements?statement="+file || strings.Contains(mp.Envelope, "PRIVATE") || strings.Contains(mp.Envelope, "999999") {
		t.Fatal(mp)
	}
	in.OperationKey, in.PreviewHash, in.Reason, in.Confirmed = "statement-message-proposal-12345", mp.PreviewHash, "PRIVATE deliberately checked recipients and content", true
	w = a.request("POST", "/api/messages", messageJSON(t, in))
	messageStatus(t, w, 200)
	var result struct{ ID string }
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	messageStatus(t, a.request("POST", "/api/messages", messageJSON(t, in)), 200)
	action := database.MessageAction{OperationKey: "statement-message-delivery-review-12345", Version: 1, Action: "APPROVED", Confirmed: true, Reason: "PRIVATE separately reviewed delivery permissions and content"}
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/actions", messageJSON(t, action)), 403)
	messageStatus(t, b.request("POST", "/api/messages/"+result.ID+"/actions", messageJSON(t, action)), 200)
	w = tenant.request("GET", "/api/messages/"+result.ID, "")
	messageStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "publication_target") || strings.Contains(w.Body.String(), "PRIVATE") || strings.Contains(w.Body.String(), "999999") {
		t.Fatal("private publication history exposed", w.Body)
	}
	action = database.MessageAction{OperationKey: "statement-message-dispatch-12345", Version: 2, Action: "DISPATCH", Outcome: "UNKNOWN", Confirmed: true, Reason: "PRIVATE simulated a truthful uncertain handoff"}
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/dispatch", messageJSON(t, action)), 200)
	if _, err = s.DecideStatementPublication(ctx, a.cookie.Value, pub, database.StatementAction{OperationKey: "statement-message-revoke-12345", Version: 2, Action: "REVOKED", Confirmed: true, Reason: "PRIVATE deliberately removed further publication access"}); err != nil {
		t.Fatal(err)
	}
	messageStatus(t, tenant.request("GET", "/api/financial-statements/"+file+"/download", ""), 404)
	m, err := s.MessageFor(ctx, a.cookie.Value, result.ID, 1, 1, 1)
	if err != nil || m.Outcomes["UNKNOWN"] != 1 {
		t.Fatal(m, err)
	}
	action = database.MessageAction{OperationKey: "statement-message-reconcile-12345", Version: m.Version, Action: "RECONCILE", Confirmed: true, Reason: "PRIVATE reconcile the original durable handoff"}
	messageStatus(t, a.request("POST", "/api/messages/"+result.ID+"/deliveries/"+m.Deliveries[0].ID+"/reconcile", messageJSON(t, action)), 200)
	m, err = s.MessageFor(ctx, a.cookie.Value, result.ID, 1, 1, 1)
	if err != nil || m.Outcomes["ACCEPTED"] != 1 || m.Outcomes["DELIVERED"] != 0 || m.Outcomes["READ"] != 0 {
		t.Fatal("unknown proof invented delivery", m, err)
	}
	if strings.Contains(logs.String(), "PRIVATE") || strings.Contains(logs.String(), pub) || strings.Contains(logs.String(), file) {
		t.Fatal("private source in logs")
	}
}
