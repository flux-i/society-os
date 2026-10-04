package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func handler(t *testing.T) (*database.Store, *Server, *bytes.Buffer) {
	t.Helper()
	s, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "society.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedDemo(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedDemoAccounts(context.Background()); err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	return s, &Server{Store: s, Version: "test", Logger: slog.New(slog.NewJSONHandler(logs, nil)), Web: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>Preview</title>")}}}, logs
}

type authClient struct {
	cookie  *http.Cookie
	csrf    string
	handler http.Handler
}

func signIn(t *testing.T, h http.Handler, login string) authClient {
	t.Helper()
	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"login": login, "password": database.DemoPassword})
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/auth/login", bytes.NewReader(body))
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	var p database.Principal
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing session cookie")
	}
	if !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("weak session cookie")
	}
	c := authClient{cookies[0], p.CSRF, h}
	if p.MFAPending {
		if !p.MFAEnrolled {
			if w := c.request("POST", "/api/auth/mfa/setup", "{}"); w.Code != 200 {
				t.Fatalf("setup %s", w.Body)
			}
		}
		w := c.request("POST", "/api/auth/mfa/demo-code", "{}")
		var code struct {
			Code     string `json:"code"`
			Recovery bool   `json:"recovery"`
		}
		json.Unmarshal(w.Body.Bytes(), &code)
		if w.Code != 200 {
			t.Fatalf("demo-code %s", w.Body)
		}
		path := "/api/auth/mfa/verify"
		if !p.MFAEnrolled {
			path = "/api/auth/mfa/confirm"
		}
		payload, _ := json.Marshal(map[string]any{"code": code.Code, "recovery": code.Recovery})
		if !p.MFAEnrolled {
			payload, _ = json.Marshal(map[string]string{"code": code.Code})
		}
		if w := c.request("POST", path, string(payload)); w.Code != 200 {
			t.Fatalf("MFA %d %s", w.Code, w.Body)
		}
	}
	return c
}
func (c authClient) request(method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	r.AddCookie(c.cookie)
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", c.csrf)
	w := httptest.NewRecorder()
	c.handler.ServeHTTP(w, r)
	return w
}

func TestHealthReadinessAndNoInternalErrorDisclosure(t *testing.T) {
	s, app, _ := handler(t)
	h := app.Handler()
	client := signIn(t, h, "admin@demo.society")
	check := func(path string, expected int) {
		t.Helper()
		w := client.request("GET", path, "")
		if w.Code != expected {
			t.Fatalf("%s status = %d, want %d", path, w.Code, expected)
		}
		if strings.Contains(w.Body.String(), "database is closed") || strings.Contains(w.Body.String(), s.Path) {
			t.Fatal("internal database details disclosed")
		}
	}
	check("/health", 200)
	check("/ready", 200)
	s.Close()
	check("/health", 200)
	check("/ready", 503)
	check("/api/flats", 503)
}

func TestRegistryFiltersAndInputBounds(t *testing.T) {
	_, app, logs := handler(t)
	h := app.Handler()
	client := signIn(t, h, "admin@demo.society")
	for _, path := range []string{"/api/flats?page_size=0", "/api/flats?page_size=51", "/api/flats?page=-1", "/api/flats?building=unknown", "/api/flats?status=unknown"} {
		w := client.request("GET", path, "")
		if w.Code != 400 {
			t.Errorf("invalid filter accepted: %s => %d", path, w.Code)
		}
	}
	w := client.request("GET", "/api/flats?q=Demo%20Owner%20A-101&page_size=12", "")
	var result database.FlatPage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("wrong person search: %+v", result)
	}
	if logs.Len() == 0 || strings.Contains(logs.String(), "Demo Owner") || strings.Contains(logs.String(), "q=") {
		t.Fatal("logs missing or query data disclosed")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("registry response can be cached")
	}
}

func TestPreviewBoundaries(t *testing.T) {
	_, app, _ := handler(t)
	h := app.Handler()
	client := signIn(t, h, "admin@demo.society")
	for _, item := range []struct {
		method, url string
		status      int
	}{
		{"GET", "http://untrusted.example/api/flats", 403},
		{"POST", "http://127.0.0.1:8080/api/flats", 405},
		{"GET", "http://127.0.0.1:8080/api/payments", 404},
		{"GET", "http://127.0.0.1:8080/api/flats/missing", 404},
		{"GET", "http://127.0.0.1:8080/", 200},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(item.method, item.url, nil)
		r.AddCookie(client.cookie)
		r.Header.Set("Origin", "http://127.0.0.1:8080")
		h.ServeHTTP(w, r)
		if w.Code != item.status {
			t.Errorf("%s %s = %d, want %d", item.method, item.url, w.Code, item.status)
		}
	}
	for _, address := range []string{"0.0.0.0:8080", "[::]:8080", ":8080", "192.168.1.2:8080", "127.0.0.1:0"} {
		if err := ValidateAddress(address); err == nil {
			t.Errorf("unsafe preview address accepted: %s", address)
		}
	}
	if err := ValidateAddress("127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
}
