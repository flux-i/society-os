package server

import (
	"encoding/json"
	"society.local/portal/internal/database"
	"strings"
	"testing"
)

func TestComplaintHTTPKeepsStaffNotesPrivateAndChecksOwnerScopeOnEveryRoute(t *testing.T) {
	_, server, _ := handler(t)
	h := server.Handler()
	owner := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	admin := signIn(t, h, "admin@demo.society")
	body, _ := json.Marshal(database.ComplaintInput{OperationKey: "complaint-http-create-001", FlatID: "demo-flat-A-101", Category: "WATER", Subject: "Fictional Unicode water request", Description: strings.Repeat("आ", 3500), Priority: "HIGH"})
	w := owner.request("POST", "/api/complaints", string(body))
	if w.Code != 200 {
		t.Fatal("valid bounded Unicode request", w.Code, w.Body)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	note, _ := json.Marshal(database.ComplaintAction{OperationKey: "complaint-http-private-001", Version: 1, Action: "COMMENT", Message: "STAFF_SECRET_Private contractor discussion", Visibility: "STAFF_ONLY"})
	if w := admin.request("POST", "/api/complaints/"+result.ID+"/updates", string(note)); w.Code != 200 {
		t.Fatal("staff note failed", w.Code, w.Body)
	}
	w = owner.request("GET", "/api/complaints/"+result.ID, "")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "STAFF_SECRET") {
		t.Fatal("private staff note leaked", w.Code, w.Body)
	}
	for _, method := range []string{"GET", "POST"} {
		path := "/api/complaints/" + result.ID
		if method == "POST" {
			path += "/updates"
		}
		if w := tenant.request(method, path, string(note)); w.Code != 404 {
			t.Fatal("unrelated resident saw case existence", method, w.Code)
		}
	}
	if w := owner.request("GET", "/api/complaints/"+result.ID+"?history_page=bad", ""); w.Code != 400 {
		t.Fatal("invalid page accepted", w.Code)
	}
	if w := owner.request("GET", "/api/complaints?q=STAFF_SECRET", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatal("staff note leaked through search", w.Code, w.Body)
	}
}
