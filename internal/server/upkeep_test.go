package server

import (
	"encoding/json"
	"society.local/portal/internal/database"
	"strings"
	"testing"
)

func TestUpkeepHTTPStrictWritesCurrentScopeAndPrivatePublicationHistory(t *testing.T) {
	db, app, logs := handler(t)
	h := app.Handler()
	admin := signIn(t, h, "admin@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	committee := signIn(t, h, "committee@demo.society")
	in := database.UpkeepTaskInput{OperationKey: "http-upkeep-task-123456", Title: "PRIVATE supplied operational task", Body: "PRIVATE coordination and evidence for the fictional work", Category: "WATER", Priority: "HIGH", DueDate: "2027-01-01", AssignedTo: "demo-user-admin", Confirmed: true}
	bytes, _ := json.Marshal(in)
	body := string(bytes)
	if w := owner.request("POST", "/api/upkeep/tasks", body); w.Code != 403 {
		t.Fatal("resident write", w.Code)
	}
	noCSRF := admin
	noCSRF.csrf = ""
	if w := noCSRF.request("POST", "/api/upkeep/tasks", body); w.Code != 403 {
		t.Fatal("CSRF bypass", w.Code)
	}
	if w := admin.request("POST", "/api/upkeep/tasks", strings.TrimSuffix(body, "}")+",\"actor_id\":\"demo-user-owner\"}"); w.Code != 400 {
		t.Fatal("supplied actor", w.Code)
	}
	w := admin.request("POST", "/api/upkeep/tasks", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var result struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	id := result.ID
	if w = owner.request("GET", "/api/upkeep/tasks/"+id, ""); w.Code != 404 {
		t.Fatal("internal work visible", w.Code)
	}
	action := func(actor authClient, version int, kind string) {
		t.Helper()
		in := database.UpkeepAction{OperationKey: "http-action-" + kind + "-123456", Version: version, Action: kind, Reason: "PRIVATE supplied evidence checked by the operator", Confirmed: true}
		data, _ := json.Marshal(in)
		w := actor.request("POST", "/api/upkeep/tasks/"+id+"/actions", string(data))
		if w.Code != 200 {
			t.Fatal(kind, w.Code, w.Body)
		}
	}
	action(admin, 1, "START")
	action(admin, 2, "SUBMIT_CHECK")
	self := `{"operation_key":"http-self-complete-123456","version":3,"action":"CONFIRM_DONE","reason":"Fictional separate completion evidence","confirmed":true}`
	if w = admin.request("POST", "/api/upkeep/tasks/"+id+"/actions", self); w.Code != 403 {
		t.Fatal("self completion", w.Code)
	}
	action(committee, 3, "CONFIRM_DONE")
	publication := `{"operation_key":"http-publication-123456","version":4,"action":"PUBLISH","reason":"PRIVATE reviewed explicit public wording","audience":"BUILDING","building_code":"A","public_title":"Shared care update","public_body":"This fictional work has been independently checked.","confirmed":true}`
	if w = admin.request("POST", "/api/upkeep/tasks/"+id+"/actions", publication); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = owner.request("GET", "/api/upkeep/tasks/"+id, "")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body)
	}
	publicBefore := w.Body.String()
	for _, private := range []string{"PRIVATE", "events", "created_by", "assigned_to", "ready_by", "checked_by", "public_snapshot"} {
		if strings.Contains(publicBefore, private) {
			t.Fatal("private HTTP field", private)
		}
	}
	action(admin, 5, "COMMENT")
	if w = owner.request("GET", "/api/upkeep/tasks/"+id, ""); w.Body.String() != publicBefore {
		t.Fatal("private mutation changed public HTTP", w.Body)
	}
	for _, path := range []string{"/api/upkeep?page=0", "/api/upkeep?page=bad", "/api/upkeep?state=UNKNOWN", "/api/upkeep/register/OTHER", "/api/upkeep/tasks/" + id + "?event_page=100001"} {
		if w = admin.request("GET", path, ""); w.Code != 400 {
			t.Fatal("invalid query", path, w.Code)
		}
	}
	for _, path := range []string{"/api/upkeep/options", "/api/upkeep/register/ASSET", "/api/upkeep/register/VENDOR/missing"} {
		if w = owner.request("GET", path, ""); w.Code != 403 {
			t.Fatal("resident directory", path, w.Code)
		}
	}
	if _, err := db.DB.Exec("UPDATE flat_memberships SET end_date=date('now','+5 hours','+30 minutes') WHERE resident_id='demo-owner-A-101'"); err != nil {
		t.Fatal(err)
	}
	if w = owner.request("GET", "/api/upkeep/tasks/"+id, ""); w.Code != 404 {
		t.Fatal("ended resident audience", w.Code)
	}
	for _, private := range []string{admin.cookie.Value, admin.csrf, "PRIVATE coordination", "PRIVATE supplied evidence", "PRIVATE reviewed explicit"} {
		if strings.Contains(logs.String(), private) {
			t.Fatal("private logs", private)
		}
	}
}
