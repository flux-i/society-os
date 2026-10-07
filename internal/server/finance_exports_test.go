package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"society.local/portal/internal/database"
)

func TestFinanceExportHTTPActorScopeFreshCSRFHeadersExactBytesAndImmutableReplay(t *testing.T) {
	s, app, logs := handler(t)
	h := app.Handler()
	ctx := context.Background()
	admin := signIn(t, h, "admin@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	filter := database.FinanceExportFilter{Report: "RECEIPTS", Scope: "OWN", From: "2026-01-01", To: "2026-01-31"}
	filterJSON, _ := json.Marshal(filter)
	for _, client := range []authClient{admin, tenant} {
		if w := client.request("POST", "/api/finance-exports/preview", string(filterJSON)); w.Code != 403 {
			t.Fatal("nonfinance export", w.Code, w.Body)
		}
	}
	for _, path := range []string{"/api/finance-exports", "/api/finance-exports/choices", "/api/finance-exports/guess/download"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:8080"+path, nil))
		if w.Code != 401 {
			t.Fatal("anonymous export", path, w.Code)
		}
	}
	if err := s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateEntry(ctx, admin.cookie.Value, database.EntryInput{OperationKey: "http-export-entry-12345", FlatID: "demo-flat-A-101", Kind: "RECEIVED", Amount: "432.19", Date: "2026-01-01", Description: "PRIVATE_HTTP export source", Payer: "PRIVATE_HTTP payer", Method: "CASH"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PostEntry(ctx, admin.cookie.Value, id, database.EntryAction{OperationKey: "http-export-post-12345", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	missing := owner
	missing.csrf = ""
	if w := missing.request("POST", "/api/finance-exports/preview", string(filterJSON)); w.Code != 403 {
		t.Fatal("preview CSRF", w.Code)
	}
	w := owner.request("POST", "/api/finance-exports/preview", string(filterJSON))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var preview database.FinanceExport
	if err = json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	input := database.FinanceExportInput{FinanceExportFilter: filter, OperationKey: "http-export-create-12345", PreviewHash: preview.ContentHash, Confirmed: true}
	body, _ := json.Marshal(input)
	if w = owner.request("POST", "/api/finance-exports", string(body[:len(body)-1])+`,"actor_id":"demo-user-admin"}`); w.Code != 400 {
		t.Fatal("unknown actor field", w.Code)
	}
	w = owner.request("POST", "/api/finance-exports", string(body))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var accepted database.FinanceExport
	json.Unmarshal(w.Body.Bytes(), &accepted)
	original := append([]byte{}, w.Body.Bytes()...)
	if w = owner.request("POST", "/api/finance-exports", string(body)); w.Code != 200 || string(original) != w.Body.String() {
		t.Fatal("lost HTTP response replay", w.Code, w.Body)
	}
	w = owner.request("GET", "/api/finance-exports/"+accepted.ID+"/download", "")
	digest := sha256.Sum256(w.Body.Bytes())
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "text/csv; charset=utf-8" || w.Header().Get("X-Export-SHA256") != hex.EncodeToString(digest[:]) || !strings.Contains(w.Header().Get("Content-Disposition"), "society-receipts-2026-01-01-2026-01-31.csv") || !strings.Contains(w.Body.String(), "432.19") {
		t.Fatal("authenticated exact CSV attachment", w.Code, w.Header(), w.Body)
	}
	if w = admin.request("GET", "/api/finance-exports/"+accepted.ID+"/download", ""); w.Code != 404 {
		t.Fatal("actor-bound download bypass", w.Code)
	}
	for _, path := range []string{"/api/finance-exports?page=bad", "/api/finance-exports?page=0"} {
		if w = owner.request("GET", path, ""); w.Code != 400 {
			t.Fatal("invalid pagination", w.Code)
		}
	}
	if _, err = s.DB.Exec("UPDATE flat_memberships SET end_date=date('now') WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/finance-exports/" + accepted.ID, "/api/finance-exports/" + accepted.ID + "/download"} {
		if w = owner.request("GET", path, ""); w.Code != 403 {
			t.Fatal("ended scope download", path, w.Code)
		}
	}
	if w = owner.request("POST", "/api/finance-exports", string(body)); w.Code != 403 {
		t.Fatal("ended scope replay", w.Code)
	}
	if strings.Contains(logs.String(), "PRIVATE_HTTP") || strings.Contains(logs.String(), owner.cookie.Value) || strings.Contains(logs.String(), "432.19") {
		t.Fatal("request log exposed private financial record")
	}
}
