package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
)

func TestAuthenticatedScopesApplyToListsDetailsSearchCountsAndHistory(t *testing.T) {
	_, app, logs := handler(t)
	h := app.Handler()
	for _, path := range []string{"/api/auth/me", "/api/flats", "/api/flats/demo-flat-A-101", "/api/registry/summary", "/api/people", "/api/system", "/api/flats/demo-flat-A-101/activity"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:8080"+path, nil))
		if w.Code != 401 {
			t.Fatalf("anonymous %s: %d", path, w.Code)
		}
	}
	owner := signIn(t, h, "owner@demo.society")
	for _, item := range []struct {
		path   string
		status int
	}{
		{"/api/flats/demo-flat-A-101", 200}, {"/api/flats/demo-flat-A-102", 200},
		{"/api/flats/demo-flat-B-101", 404}, {"/api/registry/summary", 403}, {"/api/people", 403}, {"/api/system", 403}, {"/api/flats/demo-flat-A-101/activity", 403},
	} {
		if w := owner.request("GET", item.path, ""); w.Code != item.status {
			t.Fatalf("owner %s: %d %s", item.path, w.Code, w.Body.String())
		}
	}
	for _, item := range []struct {
		query string
		count int
	}{{"", 2}, {"?q=Demo%20Owner%20B-101", 0}, {"?building=B", 0}} {
		w := owner.request("GET", "/api/flats"+item.query, "")
		var page database.FlatPage
		json.Unmarshal(w.Body.Bytes(), &page)
		if w.Code != 200 || page.Total != item.count || len(page.Items) != item.count {
			t.Fatalf("scoped count/search: %s %d %s", item.query, w.Code, w.Body.String())
		}
	}
	tenant := signIn(t, h, "tenant@demo.society")
	w := tenant.request("GET", "/api/flats/demo-flat-A-103", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "Former Tenant") {
		t.Fatalf("tenant saw history belonging to a prior tenant: %s", w.Body.String())
	}
	if w := tenant.request("GET", "/api/flats/demo-flat-A-101", ""); w.Code != 404 {
		t.Fatal("tenant cross-flat access")
	}
	committee := signIn(t, h, "committee@demo.society")
	if w := committee.request("GET", "/api/registry/summary", ""); w.Code != 200 {
		t.Fatal("committee cannot view approved registry")
	}
	for _, c := range []authClient{owner, tenant, committee} {
		w := c.request("PATCH", "/api/flats/demo-flat-A-101", `{"version":1,"status":"VACANT","reason":"Unauthorized attempt"}`)
		if w.Code != 403 {
			t.Fatalf("non-officer changed registry: %d", w.Code)
		}
	}
	if strings.Contains(logs.String(), database.DemoPassword) || strings.Contains(logs.String(), owner.cookie.Value) || strings.Contains(logs.String(), owner.csrf) || strings.Contains(logs.String(), "Owner B-101") {
		t.Fatal("credential or private query leaked in request logs")
	}
}

func TestOriginCSRFStrictBodiesAndLogoutRevocation(t *testing.T) {
	_, app, _ := handler(t)
	h := app.Handler()
	c := signIn(t, h, "admin@demo.society")
	for _, item := range []struct{ origin, csrf string }{{"http://evil.example", c.csrf}, {"", c.csrf}, {"http://127.0.0.1:8080", ""}, {"http://127.0.0.1:8080", "incorrect"}} {
		r := httptest.NewRequest("PATCH", "http://127.0.0.1:8080/api/flats/demo-flat-A-101", strings.NewReader(`{"version":1,"status":"VACANT","reason":"Must be rejected"}`))
		r.AddCookie(c.cookie)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", item.origin)
		r.Header.Set("X-CSRF-Token", item.csrf)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("unsafe write: %d", w.Code)
		}
	}
	for _, body := range []string{`{"version":1,"status":"VACANT","reason":"Unknown role bypass","role":"ADMINISTRATOR"}`, `{} {}`, strings.Repeat("x", 9000)} {
		if w := c.request("PATCH", "/api/flats/demo-flat-A-101", body); w.Code != 400 {
			t.Fatalf("invalid body accepted: %d", w.Code)
		}
	}
	if w := c.request("POST", "/api/auth/logout", `{}`); w.Code != 200 {
		t.Fatal("logout failed")
	}
	if w := c.request("GET", "/api/flats", ""); w.Code != 401 {
		t.Fatal("logged-out bearer still accepted")
	}
}

func TestCurrentSessionAccountRoleAndMembershipChecks(t *testing.T) {
	s, app, _ := handler(t)
	h := app.Handler()
	owner := signIn(t, h, "owner@demo.society")
	if _, err := s.DB.Exec("UPDATE flat_memberships SET end_date = ? WHERE flat_id = 'demo-flat-A-101' AND resident_id = 'demo-owner-A-101'", time.Now().In(time.FixedZone("IST", 19800)).Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	w := owner.request("GET", "/api/flats", "")
	var page database.FlatPage
	json.Unmarshal(w.Body.Bytes(), &page)
	if page.Total != 1 || page.Items[0].Number != "102" {
		t.Fatalf("ended membership retained access / other active membership lost: %s", w.Body.String())
	}
	if w := owner.request("GET", "/api/flats/demo-flat-A-101", ""); w.Code != 404 {
		t.Fatal("membership revocation not enforced on existing session")
	}
	admin := signIn(t, h, "admin@demo.society")
	if _, err := s.DB.Exec("UPDATE role_grants SET valid_from = ?, valid_until = ? WHERE user_id = 'demo-user-admin'", time.Now().Add(-time.Hour).Unix(), time.Now().Add(-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if w := admin.request("GET", "/api/registry/summary", ""); w.Code != 403 {
		t.Fatal("expired role retained privileges")
	}
	if w := admin.request("PATCH", "/api/flats/demo-flat-A-101", `{"version":1,"status":"VACANT","reason":"Expired role attempt"}`); w.Code != 403 {
		t.Fatal("expired role can write")
	}
	for _, query := range []string{
		"UPDATE sessions SET last_seen_at = 0 WHERE user_id = 'demo-user-owner'",
		"UPDATE sessions SET expires_at = 0 WHERE user_id = 'demo-user-owner'",
		"UPDATE users SET auth_version = auth_version + 1 WHERE id = 'demo-user-owner'",
		"UPDATE users SET status = 'DISABLED' WHERE id = 'demo-user-owner'",
	} {
		// Login before each independent invalidation.
		c := signIn(t, h, "owner@demo.society")
		if _, err := s.DB.Exec(query); err != nil {
			t.Fatal(err)
		}
		if w := c.request("GET", "/api/auth/me", ""); w.Code != 401 {
			t.Fatalf("session accepted after %s", query)
		}
	}
}

func TestLoginThrottlingAndNoAutomaticIdleExtension(t *testing.T) {
	s, app, _ := handler(t)
	h := app.Handler()
	c := signIn(t, h, "owner@demo.society")
	lastSeen := time.Now().Add(-10 * time.Minute).Unix()
	if _, err := s.DB.Exec("UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?", lastSeen, database.TokenHash(c.cookie.Value)); err != nil {
		t.Fatal(err)
	}
	if w := c.request("GET", "/api/auth/me", ""); w.Code != 200 {
		t.Fatal("live session failed")
	}
	var got int64
	s.DB.QueryRow("SELECT last_seen_at FROM sessions WHERE token_hash = ?", database.TokenHash(c.cookie.Value)).Scan(&got)
	if got != lastSeen {
		t.Fatal("background session check prevents idle expiry")
	}
	for i := 0; i < 6; i++ {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/auth/login", strings.NewReader(`{"login":"missing@demo.society","password":"wrong"}`))
		r.Header.Set("Origin", "http://127.0.0.1:8080")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := 401
		if i == 5 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("attempt %d: %d want %d", i, w.Code, want)
		}
	}
}
