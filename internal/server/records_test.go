package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/documents"
)

func TestManualEntryHTTPPermissionsRetryAndScopedPDFDownloads(t *testing.T) {
	db, app, _ := handler(t)
	h := app.Handler()
	admin := signIn(t, h, "admin@demo.society")
	input := `{"operation_key":"http-operation-123456","flat_id":"demo-flat-A-101","kind":"RECEIVED","amount":"400.01","date":"2026-01-01","description":"Fictional payment already received","payer":"Demo Owner A-101","method":"CASH","reference":""}`
	if w := admin.request("POST", "/api/entries", input); w.Code != 403 {
		t.Fatal("registry admin posted without treasury", w.Code)
	}
	if err := db.SeedDemoTreasury(context.Background()); err != nil {
		t.Fatal(err)
	}
	var id string
	for i := 0; i < 2; i++ {
		w := admin.request("POST", "/api/entries", input)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result struct{ ID string }
		json.Unmarshal(w.Body.Bytes(), &result)
		if id != "" && id != result.ID {
			t.Fatal("duplicate draft")
		}
		id = result.ID
	}
	if w := admin.request("POST", "/api/entries", strings.Replace(input, "400.01", "500.01", 1)); w.Code != 409 {
		t.Fatal("changed retry accepted", w.Code)
	}
	if w := admin.request("POST", "/api/entries/"+id+"/post", `{"operation_key":"http-post-123456789","confirmed":true,"reason":""}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	e, err := db.EntryFor(context.Background(), admin.cookie.Value, id)
	if err != nil {
		t.Fatal(err)
	}
	if w := admin.request("GET", "/api/receipts/"+e.ReceiptID+"/download", ""); w.Code != 409 {
		t.Fatal("pending download", w.Code)
	}
	store, err := documents.Open(filepath.Join(t.TempDir(), "documents"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app.Documents = store
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); store.Run(ctx, db, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		e, err = db.EntryFor(context.Background(), admin.cookie.Value, id)
		if err != nil {
			t.Fatal(err)
		}
		if e.PDFState == "READY" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("PDF job not completed", e.PDFState)
		}
		time.Sleep(20 * time.Millisecond)
	}
	owner := signIn(t, h, "owner@demo.society")
	w := owner.request("GET", "/api/receipts/"+e.ReceiptID+"/download", "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(w.Body.String(), "%PDF-") {
		t.Fatal("owner download", w.Code, w.Body.String())
	}
	original := append([]byte{}, w.Body.Bytes()...)
	tenant := signIn(t, h, "tenant@demo.society")
	if w := tenant.request("GET", "/api/receipts/"+e.ReceiptID+"/download", ""); w.Code != 403 {
		t.Fatal("tenant finance not allowed", w.Code)
	}
	for _, path := range []string{"/api/entries?page=invalid", "/api/entries?state=invalid", "/api/entries?q=" + strings.Repeat("x", 101)} {
		if w := admin.request("GET", path, ""); w.Code != 400 {
			t.Fatal("unbounded query", path, w.Code)
		}
	}
	if w := admin.request("POST", "/api/entries/"+id+"/reverse", `{"operation_key":"http-reverse-123456789","confirmed":true,"reason":"Supplied details were incorrect"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = owner.request("GET", "/api/receipts/"+e.ReceiptID+"/download", "")
	if w.Code != 200 || string(original) != w.Body.String() || w.Header().Get("X-Receipt-Status") != "REVERSED" {
		t.Fatal("original receipt not preserved")
	}
	// A database-only restore can regenerate the PDF from its frozen snapshot.
	if _, err = db.DB.Exec("UPDATE receipt_jobs SET file_hash=? WHERE receipt_id=?", strings.Repeat("a", 64), e.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if err = store.Reconcile(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	e, _ = db.EntryFor(context.Background(), admin.cookie.Value, id)
	if e.PDFState != "PENDING" {
		t.Fatal("missing PDF not queued after restore")
	}
}
