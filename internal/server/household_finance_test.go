package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
)

func TestHouseholdFinanceHTTPCurrentTreasuryStrictCSRFOriginalReplyAndPrivateAudit(t *testing.T) {
	s, app, logs := handler(t)
	h := app.Handler()
	registry := signIn(t, h, "admin@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	path := "/api/flats/demo-flat-A-101/finance-visibility"
	input := database.HouseholdFinanceInput{OperationKey: "http-household-finance-original-2026", PersonID: "demo-owner-A-101", Version: 1, Action: "REVOKE", Confirmed: true, Note: "PRIVATE_HTTP_FINANCE verified supplied household financial visibility"}
	body := messageJSON(t, input)
	for _, client := range []authClient{registry, owner} {
		messageStatus(t, client.request("GET", path, ""), 403)
		messageStatus(t, client.request("POST", path, body), 403)
	}
	if err := s.SeedDemoTreasury(context.Background()); err != nil {
		t.Fatal(err)
	}
	missing := registry
	missing.csrf = ""
	messageStatus(t, missing.request("POST", path, body), 403)
	for _, bad := range []string{body + "{}", strings.TrimSuffix(body, "}") + `,"actor_id":"demo-user-owner"}`, strings.Replace(body, "REVOKE", "AUTO_GRANT", 1)} {
		messageStatus(t, registry.request("POST", path, bad), 400)
	}
	for _, q := range []string{"?page=0", "?page=401", "?page=NaN", "?q=" + strings.Repeat("x", 101)} {
		messageStatus(t, registry.request("GET", path+q, ""), 400)
	}
	messageStatus(t, registry.request("GET", "/api/flats/unknown/finance-visibility", ""), 404)
	result := registry.request("POST", path, body)
	messageStatus(t, result, 200)
	var action database.HouseholdFinanceAction
	if err := json.Unmarshal(result.Body.Bytes(), &action); err != nil {
		t.Fatal(err)
	}
	if action.Visible || action.Version != 2 || len(action.After) != 1 || action.After[0].Visible {
		t.Fatal("independent revocation result", action)
	}
	for _, w := range []string{registry.request("POST", path, body).Body.String(), registry.request("GET", path+"/operations/"+input.OperationKey, "").Body.String()} {
		if w != result.Body.String() {
			t.Fatal("original HTTP reply changed")
		}
	}
	input.OperationKey = "http-household-finance-other-2026"
	input.Action = "GRANT"
	messageStatus(t, registry.request("POST", path, messageJSON(t, input)), 409)
	messageStatus(t, owner.request("GET", path+"/operations/http-household-finance-original-2026", ""), 403)
	var count int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil || count != 0 {
		t.Fatal("visibility posted money", count, err)
	}
	if _, err := s.DB.Exec("UPDATE sessions SET reauthenticated_at=?,mfa_verified_at=? WHERE token_hash=?", time.Now().Add(-6*time.Minute).Unix(), time.Now().Add(-6*time.Minute).Unix(), database.TokenHash(registry.cookie.Value)); err != nil {
		t.Fatal(err)
	}
	stale := registry.request("POST", path, body)
	messageStatus(t, stale, 403)
	if !strings.Contains(stale.Body.String(), `"error":"reauthentication_required"`) {
		t.Fatal("fresh identity challenge missing", stale.Body.String())
	}
	if strings.Contains(logs.String(), "PRIVATE_HTTP_FINANCE") || strings.Contains(logs.String(), registry.cookie.Value) {
		t.Fatal("logs exposed private decision")
	}
}
