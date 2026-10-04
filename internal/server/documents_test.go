package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/documents"
)

func TestDocumentHTTPProtectsUploadsMetadataOriginalDownloadsAndReviewHistory(t *testing.T) {
	store, server, _ := handler(t)
	h := server.Handler()
	owner := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	admin := signIn(t, h, "admin@demo.society")
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	data := out.Bytes()
	sum := sha256.Sum256(data)
	in := database.DocumentInput{OperationKey: "document-http-reserve-001", Title: strings.Repeat("आ", 120), Filename: "अभिलेख.png", Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Category: "CIRCULAR", Visibility: "ALL_AUTHORIZED_RESIDENTS"}
	body, _ := json.Marshal(in)
	response := owner.request("POST", "/api/documents", string(body))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/documents/" + result.ID, "/api/documents/" + result.ID + "/download", "/api/documents/" + result.ID + "/content"} {
		method := "GET"
		if strings.HasSuffix(path, "/content") {
			method = "POST"
		}
		if w := tenant.request(method, path, string(data)); w.Code != 404 {
			t.Fatal("pending filename/existence leaked", path, w.Code, w.Body)
		}
	}
	if w := owner.request("GET", "/api/documents/"+result.ID+"/download", ""); w.Code != 409 {
		t.Fatal("unvalidated original downloaded", w.Code)
	}
	if w := owner.request("POST", "/api/documents/"+result.ID+"/content", string(data)); w.Code != 200 {
		t.Fatal("upload bytes", w.Code, w.Body)
	}
	ctx := context.Background()
	job, err := store.ClaimDocumentCheck(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	kind, code := documents.ValidateOriginal(ctx, job.Filename, job.Bytes)
	if code != "" {
		t.Fatal(code)
	}
	if err = store.FinishDocumentCheck(ctx, job, kind, code, time.Now()); err != nil {
		t.Fatal(err)
	}
	var doc database.LibraryDocument
	w := admin.request("GET", "/api/documents/"+result.ID, "")
	if err = json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	action, _ := json.Marshal(database.DocumentAction{OperationKey: "document-http-approve-001", Version: doc.Version, Action: "APPROVED", Reason: "Fictional content and audience independently checked", Confirmed: true})
	if w := admin.request("POST", "/api/documents/"+result.ID+"/actions", string(action)); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = tenant.request("GET", "/api/documents/"+result.ID+"/download", "")
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment;") || !strings.Contains(w.Header().Get("Content-Disposition"), "filename*") || !strings.HasPrefix(w.Header().Get("Content-Security-Policy"), "sandbox;") {
		t.Fatal("original attachment headers/content", w.Code, w.Header())
	}
	w = tenant.request("GET", "/api/documents/"+result.ID, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), `"events"`) {
		t.Fatal("private review history exposed", w.Code, w.Body)
	}
	if w = tenant.request("GET", "/api/documents/subjects", ""); w.Code != 403 {
		t.Fatal("resident searched private subjects", w.Code)
	}
	if w = owner.request("GET", "/api/documents?page=bad", ""); w.Code != 400 {
		t.Fatal("invalid page", w.Code)
	}
}
