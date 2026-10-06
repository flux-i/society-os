package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"society.local/portal/internal/database"
	"society.local/portal/internal/documents"
	"strings"
	"testing"
	"time"
)

func TestFinancialStatementHTTPSeparateFinanceAuthorityStrictMetadataOriginalAndPublicationScope(t *testing.T) {
	s, app, logs := handler(t)
	h := app.Handler()
	a := signIn(t, h, "admin@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	data := []byte("Description,Amount\nFictional statement external figure,50000.00\n")
	sha := sha256.Sum256(data)
	in := database.StatementInput{OperationKey: "http-statement-original-12345", Title: "PRIVATE prepared fictional income statement", Kind: "INCOME", PeriodStart: "2026-09-01", PeriodEnd: "2026-09-30", PreparedBy: "PRIVATE fictional accountant", Source: "PRIVATE supplied external accounts source", Filename: "original.csv", Size: int64(len(data)), SHA256: hex.EncodeToString(sha[:]), Confirmed: true, Reason: "PRIVATE checked the exact supplied original"}
	blob, _ := json.Marshal(in)
	body := string(blob)
	if w := a.request("POST", "/api/financial-statements", body); w.Code != 403 {
		t.Fatal("registry implied finance", w.Code, w.Body)
	}
	if err := s.SeedDemoTreasury(context.Background()); err != nil {
		t.Fatal(err)
	}
	noCSRF := a
	noCSRF.csrf = ""
	if w := noCSRF.request("POST", "/api/financial-statements", body); w.Code != 403 {
		t.Fatal("CSRF bypass", w.Code)
	}
	if w := a.request("POST", "/api/financial-statements", strings.TrimSuffix(body, "}")+`,"actor_id":"demo-user-owner"}`); w.Code != 400 {
		t.Fatal("supplied actor", w.Code)
	}
	w := a.request("POST", "/api/financial-statements", body)
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); w.Code != 200 || err != nil {
		t.Fatal(w.Code, w.Body, err)
	}
	if w = a.request("POST", "/api/financial-statements/"+out.ID+"/content", "wrong original"); w.Code != 400 {
		t.Fatal("changed body accepted", w.Code)
	}
	if w = owner.request("POST", "/api/financial-statements/"+out.ID+"/content", string(data)); w.Code != 404 {
		t.Fatal("resident upload", w.Code)
	}
	for i := 0; i < 2; i++ {
		if w = a.request("POST", "/api/financial-statements/"+out.ID+"/content", string(data)); w.Code != 200 {
			t.Fatal("unchanged body retry", w.Code, w.Body)
		}
	}
	job, err := s.ClaimStatementCheck(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	mime, code := documents.ValidateStatementOriginal(context.Background(), job.Filename, job.Bytes)
	if code != "" || mime != "text/csv; charset=utf-8" {
		t.Fatal(mime, code)
	}
	if err = s.FinishStatementCheck(context.Background(), job, mime, code, time.Now()); err != nil {
		t.Fatal(err)
	}
	grant := `{"version":1,"confirmed":true,"reason":"Verified fictional separate statement finance appointment","role":"TREASURER","term_days":30}`
	if w = a.request("POST", "/api/admin/accounts/demo-user-committee/roles", grant); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	b := signIn(t, h, "committee@demo.society")
	file, err := s.StatementFor(context.Background(), databaseToken(a), out.ID, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	action := database.StatementAction{OperationKey: "http-statement-internal-12345", Version: file.Version, Action: "APPROVED", Confirmed: true, Reason: "PRIVATE separate internal original review"}
	blob, _ = json.Marshal(action)
	if w = b.request("POST", "/api/financial-statements/"+out.ID+"/actions", string(blob)); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w = tenant.request("GET", "/api/financial-statements/"+out.ID, ""); w.Code != 404 {
		t.Fatal("internal review published", w.Code)
	}
	file, err = s.StatementFor(context.Background(), databaseToken(a), out.ID, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	pi := database.StatementPublicationInput{OperationKey: "http-statement-publication-12345", FileID: out.ID, FileVersion: file.Version, Target: database.MessageTarget{Kind: "TENANTS"}, Reason: "PRIVATE deliberately share this version with current tenants", Confirmed: true}
	preview, err := s.PreviewStatementPublication(context.Background(), databaseToken(a), pi)
	if err != nil || preview.TargetPeople != 35 {
		t.Fatal(preview, err)
	}
	pi.PreviewHash = preview.PreviewHash
	blob, _ = json.Marshal(pi)
	w = a.request("POST", "/api/financial-statements/publications", string(blob))
	var publication struct{ ID string }
	if err = json.Unmarshal(w.Body.Bytes(), &publication); w.Code != 200 || err != nil {
		t.Fatal(w.Code, w.Body, err)
	}
	action = database.StatementAction{OperationKey: "http-statement-publish-12345", Version: 1, Action: "PUBLISHED", Confirmed: true, Reason: "PRIVATE independently reviewed this publication and chosen audience"}
	blob, _ = json.Marshal(action)
	if w = b.request("POST", "/api/financial-statements/publications/"+publication.ID+"/actions", string(blob)); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = tenant.request("GET", "/api/financial-statements/"+out.ID+"/download", "")
	if w.Code != 200 || w.Body.String() != string(data) || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "text/csv; charset=utf-8" || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("original download", w.Code, w.Header(), w.Body)
	}
	if w = owner.request("GET", "/api/financial-statements/"+out.ID+"/download", ""); w.Code != 404 {
		t.Fatal("cross-audience download", w.Code)
	}
	if w = tenant.request("GET", "/api/financial-statements/targets?target=PEOPLE", ""); w.Code != 403 {
		t.Fatal("tenant finance audience chooser", w.Code)
	}
	for _, path := range []string{"/api/financial-statements?page=0", "/api/financial-statements?kind=UNKNOWN", "/api/financial-statements/targets?target=PEOPLE&page=10001", "/api/financial-statements/" + out.ID + "?publication_page=-1"} {
		if w = a.request("GET", path, ""); w.Code != 400 {
			t.Fatal("bad bounds", path, w.Code)
		}
	}
	if w = owner.request("GET", "/api/statements/demo-flat-A-101", ""); w.Code != 200 {
		t.Fatal("household statement route damaged", w.Code, w.Body)
	}
	var count int
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil || count != 0 {
		t.Fatal("uploaded figure posted money", count, err)
	}
	if strings.Contains(logs.String(), "PRIVATE") || strings.Contains(logs.String(), out.ID) {
		t.Fatal("private metadata in request logs")
	}
}

func databaseToken(c authClient) string { return c.cookie.Value }
