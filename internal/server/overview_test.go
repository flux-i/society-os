package server

import (
	"encoding/json"
	"testing"

	"society.local/portal/internal/database"
)

func TestOverviewHTTPScopesEachSourceUsesNoStoreAndRequiresCurrentSession(t *testing.T) {
	_, server, _ := handler(t)
	h := server.Handler()
	owner := signIn(t, h, "owner@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	for _, section := range []string{"reviews", "service", "documents", "notices", "finance", "maintenance", "upkeep", "collections"} {
		w := owner.request("GET", "/api/overview/"+section, "")
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(section, w.Code, w.Body)
		}
		var x database.Overview
		if err := json.Unmarshal(w.Body.Bytes(), &x); err != nil {
			t.Fatal(err)
		}
		if x.Section != section || x.Calendar != "Asia/Kolkata" || len(x.Items) != 0 || x.Counts == nil {
			t.Fatal(x)
		}
	}
	if w := tenant.request("GET", "/api/overview/finance", ""); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	if w := tenant.request("GET", "/api/overview/maintenance", ""); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	if w := owner.request("GET", "/api/overview/unsupported", ""); w.Code != 400 {
		t.Fatal(w.Code, w.Body)
	}
	if w := owner.request("POST", "/api/auth/logout", "{}"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := owner.request("GET", "/api/overview/service", ""); w.Code != 401 {
		t.Fatal("ended session accepted", w.Code)
	}
}
