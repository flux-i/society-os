package server

import (
	"encoding/json"
	"society.local/portal/internal/database"
	"strings"
	"testing"
)

func TestReviewHTTPProtectsPrivateSubmissionsAndSupportsUnicodeNoticeBody(t *testing.T) {
	_, server, _ := handler(t)
	handler := server.Handler()
	owner := signIn(t, handler, "owner@demo.society")
	tenant := signIn(t, handler, "tenant@demo.society")
	admin := signIn(t, handler, "admin@demo.society")
	// More than the common 8 KiB JSON limit, within the documented 4000-character notice body.
	body, _ := json.Marshal(database.ReviewInput{OperationKey: "unicode-review-operation-001", Kind: "NOTICE", Title: "Fictional Unicode community notice", Body: strings.Repeat("आ", 3500), Audience: "OWNERS_ONLY"})
	w := owner.request("POST", "/api/reviews", string(body))
	if w.Code != 200 {
		t.Fatalf("bounded Unicode body %d %s", w.Code, w.Body)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w := tenant.request("GET", "/api/reviews/"+result.ID, ""); w.Code != 404 {
		t.Fatal("private request existence leaked", w.Code)
	}
	if w := owner.request("GET", "/api/notices/"+result.ID, ""); w.Code != 404 {
		t.Fatal("unreviewed notice published", w.Code)
	}
	action, _ := json.Marshal(database.ReviewAction{OperationKey: "unicode-review-approval-001", Version: 1, Decision: "APPROVED", Reason: "Reviewed the audience and fictional source", Confirmed: true})
	if w := owner.request("POST", "/api/reviews/"+result.ID+"/decision", string(action)); w.Code != 403 {
		t.Fatal("resident self approval", w.Code)
	}
	if w := admin.request("POST", "/api/reviews/"+result.ID+"/decision", string(action)); w.Code != 200 {
		t.Fatal("separate reviewer failed", w.Code, w.Body)
	}
	if w := tenant.request("GET", "/api/notices/"+result.ID, ""); w.Code != 404 {
		t.Fatal("audience scope leaked", w.Code)
	}
	w = owner.request("GET", "/api/notices/"+result.ID, "")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "Reviewed the audience") {
		t.Fatal("published notice missing or private review exposed", w.Code)
	}
}
