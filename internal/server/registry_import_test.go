package server

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestWorkspaceImportHTTPRequiresRealMFAOriginCSRFFreshCurrentAuthorityAndPrivateLogs(t *testing.T) {
	ctx := context.Background()
	s, err := database.Open(ctx, filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	const password = "Fictional-private-bootstrap-password!"
	setup := database.WorkspaceSetup{FormatVersion: 1, SocietyKey: "rehearsal-society", SocietyName: "Sample HTTP rehearsal", Mode: "FICTIONAL_REHEARSAL", BootstrapID: "http-setup", AdministratorName: "Sample Registry Officer", AdministratorEmail: "registry@example.test", IdentityVerified: true, VerificationNote: "Verified fictional HTTP custodian and source identity", TermDays: 90}
	if _, err = s.BootstrapWorkspace(ctx, setup, password, s.MFA.Fingerprint(), strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	app := &Server{Store: s, Logger: slog.New(slog.NewJSONHandler(logs, nil)), Web: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<title>QA</title>")}}}
	h := app.Handler()
	public := httptest.NewRecorder()
	h.ServeHTTP(public, httptest.NewRequest("GET", "http://127.0.0.1:8080/api/workspace", nil))
	var info map[string]any
	json.Unmarshal(public.Body.Bytes(), &info)
	if public.Code != 200 || len(info) != 2 || info["mode"] != "FICTIONAL_REHEARSAL" || public.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("anonymous configuration boundary", public.Code, info)
	}
	login := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"login": setup.AdministratorEmail, "password": password})
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/auth/login", bytes.NewReader(body))
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(login, r)
	var p database.Principal
	json.Unmarshal(login.Body.Bytes(), &p)
	if login.Code != 200 || p.IsDemo || !p.MFAPending {
		t.Fatal("non-demo password bypass", login.Code)
	}
	client := authClient{cookie: login.Result().Cookies()[0], csrf: p.CSRF, handler: h}
	if w := client.request("GET", "/api/registry/import", ""); w.Code != 403 || !strings.Contains(w.Body.String(), "mfa_required") {
		t.Fatal("import before MFA", w.Code)
	}
	if w := client.request("POST", "/api/auth/mfa/demo-code", "{}"); w.Code != 403 {
		t.Fatal("preview MFA bypass", w.Code)
	}
	w := client.request("POST", "/api/auth/mfa/setup", "{}")
	var factor database.MFASetup
	json.Unmarshal(w.Body.Bytes(), &factor)
	secret, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(factor.Secret)
	mfa, _ := json.Marshal(map[string]string{"code": security.Code(secret, time.Now().Unix()/30, 6)})
	if w = client.request("POST", "/api/auth/mfa/confirm", string(mfa)); w.Code != 200 {
		t.Fatal("real HTTP MFA", w.Code, w.Body)
	}
	register, err := os.ReadFile("../../testdata/registry-import-118.json")
	if err != nil {
		t.Fatal(err)
	}
	// A configured society can supply a building code outside the public demo.
	register = []byte(strings.Replace(string(register), `"code": "A"`, `"code": "WEST"`, 1))
	previewBody, _ := json.Marshal(map[string]string{"input_text": string(register)})
	noCSRF := client
	noCSRF.csrf = ""
	if w = noCSRF.request("POST", "/api/registry/import/preview", string(previewBody)); w.Code != 403 {
		t.Fatal("preview missing CSRF", w.Code)
	}
	cross := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/registry/import/preview", bytes.NewReader(previewBody))
	cross.Header.Set("Content-Type", "application/json")
	cross.Header.Set("Origin", "https://elsewhere.example")
	cross.AddCookie(client.cookie)
	cross.Header.Set("X-CSRF-Token", client.csrf)
	denied := httptest.NewRecorder()
	h.ServeHTTP(denied, cross)
	if denied.Code != 403 {
		t.Fatal("cross-origin import", denied.Code)
	}
	w = client.request("POST", "/api/registry/import/preview", string(previewBody))
	var preview database.RegistryImportPreview
	json.Unmarshal(w.Body.Bytes(), &preview)
	if w.Code != 200 || !preview.CanApply || preview.Counts.Homes != 118 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private preview", w.Code)
	}
	input := database.ApplyRegistryImport{InputText: string(register), Digest: preview.Digest, BaseDigest: preview.BaseDigest, EffectiveDate: preview.EffectiveDate, OperationKey: "http-initial-register", Confirmed: true, Note: "Verified fictional complete register and supplied source identities"}
	applyBody, _ := json.Marshal(input)
	if w = client.request("POST", "/api/registry/import/apply", string(applyBody)); w.Code != 200 {
		t.Fatal("HTTP import", w.Code, w.Body)
	}
	if w = client.request("POST", "/api/registry/import/apply", string(applyBody)); w.Code != 200 {
		t.Fatal("exact HTTP retry", w.Code)
	}
	w = client.request("GET", "/api/flats?building=WEST", "")
	var homes database.FlatPage
	if err = json.Unmarshal(w.Body.Bytes(), &homes); err != nil || w.Code != 200 || homes.Total != 40 || homes.Items[0].BuildingCode != "WEST" {
		t.Fatal("supplied building filter still assumed demo codes", w.Code, homes.Total, err)
	}
	if w = client.request("GET", "/api/entries", ""); w.Code != 403 {
		t.Fatal("registry admin obtained finance", w.Code)
	}
	if _, err = s.DB.Exec("UPDATE role_grants SET revoked_at=? WHERE role='ADMINISTRATOR'", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/registry/import", "/api/registry/summary"} {
		if w = client.request("GET", path, ""); w.Code != 403 {
			t.Fatal("revoked import authority", w.Code)
		}
	}
	if w = client.request("POST", "/api/registry/import/apply", string(applyBody)); w.Code != 403 {
		t.Fatal("revoked accepted replay", w.Code)
	}
	for _, private := range []string{password, setup.AdministratorEmail, factor.Secret, client.cookie.Value, client.csrf, "Sample Owner", preview.Digest, input.Note} {
		if strings.Contains(logs.String(), private) {
			t.Fatal("private value appeared in request logs")
		}
	}
}
