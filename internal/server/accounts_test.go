package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
)

func TestPasswordOnlySessionCannotReachPrivilegedData(t *testing.T) {
	_, app, _ := handler(t)
	h := app.Handler()
	payload, _ := json.Marshal(map[string]string{"login": "admin@demo.society", "password": database.DemoPassword})
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/auth/login", bytes.NewReader(payload))
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var p database.Principal
	json.Unmarshal(w.Body.Bytes(), &p)
	if w.Code != 200 || !p.MFAPending || p.CanManageRegistry {
		t.Fatal("password-only permission", w.Body)
	}
	c := authClient{w.Result().Cookies()[0], p.CSRF, h}
	for _, path := range []string{"/api/system", "/api/registry/summary", "/api/flats", "/api/flats/demo-flat-A-101", "/api/people", "/api/admin/accounts"} {
		response := c.request("GET", path, "")
		if response.Code != 403 || !strings.Contains(response.Body.String(), "mfa_required") {
			t.Fatal("privileged data before MFA", path, response.Code, response.Body)
		}
	}
	if response := c.request("GET", "/api/auth/me", ""); response.Code != 200 {
		t.Fatal("MFA session cannot read own status")
	}
	if response := c.request("POST", "/api/auth/mfa/setup", "{}"); response.Code != 200 {
		t.Fatal("MFA setup unavailable", response.Body)
	}
	// Missing CSRF still fails inside the restricted authentication session.
	c.csrf = ""
	if response := c.request("POST", "/api/auth/mfa/demo-code", "{}"); response.Code != 403 {
		t.Fatal("MFA endpoint lacked CSRF")
	}
	c.csrf = p.CSRF
	if response := c.request("POST", "/api/auth/logout", "{}"); response.Code != 200 {
		t.Fatal("limited session cannot sign out")
	}
}
func TestAccountEndpointsPermissionFreshnessAndCredentialFreeLogs(t *testing.T) {
	s, app, logs := handler(t)
	h := app.Handler()
	admin := signIn(t, h, "admin@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	body := `{"resident_id":"demo-owner-B-101","email":"neighbour@example.test","role":"RESIDENT","term_days":90,"identity_verified":true,"note":"Verified fictional identity and email"}`
	if w := owner.request("POST", "/api/admin/invitations", body); w.Code != 403 {
		t.Fatal("resident invited user", w.Body)
	}
	if w := owner.request("GET", "/api/admin/accounts", ""); w.Code != 403 {
		t.Fatal("resident read account directory")
	}
	if w := admin.request("GET", "/api/admin/accounts?page_size=100", ""); w.Code != 400 {
		t.Fatal("unbounded account query")
	}
	s.DB.Exec("UPDATE sessions SET reauthenticated_at=? WHERE user_id='demo-user-admin'", time.Now().Add(-6*time.Minute).Unix())
	if w := admin.request("POST", "/api/admin/invitations", body); w.Code != 403 || !strings.Contains(w.Body.String(), "reauthentication_required") {
		t.Fatal("stale invitation", w.Body)
	}
	var preview struct {
		Code     string `json:"code"`
		Recovery bool   `json:"recovery"`
	}
	w := admin.request("POST", "/api/auth/mfa/demo-code", "{}")
	json.Unmarshal(w.Body.Bytes(), &preview)
	input, _ := json.Marshal(map[string]any{"password": database.DemoPassword, "code": preview.Code, "recovery": preview.Recovery})
	if w := admin.request("POST", "/api/auth/reauthenticate", string(input)); w.Code != 200 {
		t.Fatal("reauth failed", w.Body)
	}
	w = admin.request("POST", "/api/admin/invitations", body)
	var link database.IssuedLink
	json.Unmarshal(w.Body.Bytes(), &link)
	if w.Code != 201 || len(link.Token) != 43 {
		t.Fatal("invitation", w.Body)
	}
	// Anonymous link completion needs exact Origin but has no session CSRF.
	complete, _ := json.Marshal(map[string]string{"token": link.Token, "password": "A memorable new password!"})
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/auth/link/complete", bytes.NewReader(complete))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://elsewhere.example")
	denied := httptest.NewRecorder()
	h.ServeHTTP(denied, r)
	if denied.Code != 403 {
		t.Fatal("cross-origin activation allowed")
	}
	for _, secret := range []string{link.Token, preview.Code, database.DemoPassword, admin.cookie.Value, admin.csrf, "neighbour@example.test"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("secret appeared in request logs")
		}
	}
	if w := admin.request(http.MethodPost, "/api/auth/mfa/disable", "{}"); w.Code != 405 {
		t.Fatal("unexpected web MFA recovery route")
	}
}
