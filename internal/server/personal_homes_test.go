package server

import (
	"encoding/json"
	"testing"

	"society.local/portal/internal/database"
)

func TestPersonalHomesOnlyNarrowsCurrentMembershipScope(t *testing.T) {
	store, app, _ := handler(t)
	h := app.Handler()
	// Add an operational appointment to a resident with two known current homes.
	if _, err := store.DB.Exec(`INSERT INTO role_grants (id,user_id,role,valid_from,valid_until,granted_by) VALUES ('personal-home-test','demo-user-owner','COMMITTEE',unixepoch()-60,unixepoch()+3600,'demo-user-admin')`); err != nil {
		t.Fatal(err)
	}
	owner := signIn(t, h, "owner@demo.society")
	for _, tc := range []struct {
		query string
		want  int
	}{{"?scope=mine", 2}, {"?scope=mine&building=B", 0}, {"?scope=mine&q=Demo%20Owner%20B-101", 0}, {"", 118}} {
		w := owner.request("GET", "/api/flats"+tc.query, "")
		var result database.FlatPage
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || result.Total != tc.want {
			t.Fatalf("%s: status=%d total=%d body=%s", tc.query, w.Code, result.Total, w.Body)
		}
	}
	if w := owner.request("GET", "/api/flats?scope=all", ""); w.Code != 400 {
		t.Fatalf("unknown scope: %d", w.Code)
	}
	if _, err := store.DB.Exec(`UPDATE flat_memberships SET end_date='2020-01-02' WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-102'`); err != nil {
		t.Fatal(err)
	}
	w := owner.request("GET", "/api/flats?scope=mine", "")
	var result database.FlatPage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || result.Total != 1 || result.Items[0].ID != "demo-flat-A-101" {
		t.Fatalf("ended membership remained: %s", w.Body)
	}
	tenant := signIn(t, h, "tenant@demo.society")
	for _, query := range []string{"", "?scope=mine"} {
		w = tenant.request("GET", "/api/flats"+query, "")
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || result.Total != 1 || result.Items[0].ID != "demo-flat-A-103" {
			t.Fatalf("tenant scope: %s", w.Body)
		}
	}
}
