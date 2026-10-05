package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
)

func TestAccountAdministrationHTTPUsesCurrentAuthorityCSRFVersionAndPrivateMetadata(t *testing.T) {
	s, app, logs := handler(t)
	h := app.Handler()
	admin := signIn(t, h, "admin@demo.society")
	tenant := signIn(t, h, "tenant@demo.society")
	path := "/api/admin/accounts/demo-user-tenant"
	if w := tenant.request("GET", path, ""); w.Code != 403 {
		t.Fatal("resident directory detail", w.Code, w.Body)
	}
	if w := admin.request("GET", path+"?history_page=10001", ""); w.Code != 400 {
		t.Fatal("unbounded detail", w.Code)
	}
	w := admin.request("GET", path, "")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private read", w.Code, w.Header())
	}
	for _, key := range []string{"password_hash", "secret_ciphertext", "csrf_token", "token_hash", "recovery_codes"} {
		if strings.Contains(w.Body.String(), key) {
			t.Fatal("credential in metadata", key)
		}
	}
	body := `{"version":1,"confirmed":true,"reason":"Verified fictional appointment and approved register","role":"TREASURER","term_days":30}`
	if w := tenant.request("POST", path+"/roles", body); w.Code != 403 {
		t.Fatal("resident appointed treasury", w.Code)
	}
	noCSRF := admin
	noCSRF.csrf = ""
	if w := noCSRF.request("POST", path+"/roles", body); w.Code != 403 {
		t.Fatal("missing CSRF", w.Code)
	}
	if w := admin.request("POST", path+"/roles", strings.TrimSuffix(body, "}")+`,"actor":"demo-user-tenant"}`); w.Code != 400 {
		t.Fatal("supplied actor accepted", w.Code)
	}
	w = admin.request("POST", path+"/roles", body)
	if w.Code != 201 {
		t.Fatal("grant", w.Code, w.Body)
	}
	if w := admin.request("POST", path+"/roles", body); w.Code != 409 {
		t.Fatal("stale write", w.Code, w.Body)
	}
	if w := tenant.request("GET", "/api/auth/me", ""); w.Code != 401 {
		t.Fatal("recipient not signed out", w.Code)
	}
	w = admin.request("GET", path, "")
	var detail database.AccountDetails
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil || detail.Version != 2 || len(detail.Grants) != 1 || detail.Grants[0].Role != "TREASURER" {
		t.Fatal("new appointment", detail, err)
	}
	if w := admin.request("POST", path+"/status", `{"version":2,"confirmed":true,"reason":"Verified account suspension against the fictional register","action":"SUSPEND"}`); w.Code != 200 {
		t.Fatal("suspension", w.Code, w.Body)
	}
	if w := admin.request("POST", path+"/link", `{"purpose":"PASSWORD_RESET","identity_verified":true,"note":"Verified fictional recovery request"}`); w.Code != 400 {
		t.Fatal("recovery bypass", w.Code, w.Body)
	}
	if w := admin.request("POST", path+"/status", `{"version":3,"confirmed":true,"reason":"Verified resumed identity against the fictional register","action":"RESUME"}`); w.Code != 200 {
		t.Fatal("resumption", w.Code, w.Body)
	}
	s.DB.Exec("UPDATE sessions SET reauthenticated_at=? WHERE user_id='demo-user-admin'", time.Now().Add(-6*time.Minute).Unix())
	if w := admin.request("POST", path+"/roles", strings.Replace(body, `"version":1`, `"version":4`, 1)); w.Code != 403 || !strings.Contains(w.Body.String(), "reauthentication_required") {
		t.Fatal("stale actor", w.Code, w.Body)
	}
	s.DB.Exec("UPDATE role_grants SET revoked_at=? WHERE user_id='demo-user-admin'", time.Now().Unix())
	if w := admin.request("GET", path, ""); w.Code != 403 {
		t.Fatal("revoked actor retained detail", w.Code, w.Body)
	}
	for _, secret := range []string{admin.cookie.Value, admin.csrf, database.DemoPassword, "Verified fictional appointment", "tenant@demo.society"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("private value in logs", secret)
		}
	}
}
