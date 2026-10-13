package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
)

func TestRecoveryHTTPWithholdsPrivateRoutesFromCopiedAdminAndResidentWhileStatusAndSignoutWork(t *testing.T) {
	s, app, _ := handler(t)
	h := app.Handler()
	admin := signIn(t, h, "admin@demo.society")
	owner := signIn(t, h, "owner@demo.society")
	ctx := context.Background()
	if w := admin.request("GET", "/api/flats", ""); w.Code != 200 {
		t.Fatal("Fixture did not start with real allowed registry access", w.Code)
	}
	if _, err := s.QuarantineRecovery(ctx, database.RecoveryOrigin{FormatVersion: 1, SourceSHA256: strings.Repeat("a", 64), SourceBytes: 1585152, SourceSchema: database.SchemaVersion, SourceApplication: "0.32.0-dev", SourceCreatedAt: time.Now().Add(-time.Minute).Unix()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, actor := range []authClient{admin, owner} {
		w := actor.request("GET", "/api/auth/me", "")
		var p database.Principal
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &p) != nil || !p.RecoveryRequired || len(p.Roles) != 0 || p.CanReadRecords || p.CanReadRegistry {
			t.Fatal("Own identity status disclosed checkpoint authority", w.Code)
		}
		for _, private := range []string{"source_sha256", "source_bytes", "source_application", "job_id", "epoch_id", "resident_id", "TREASURER", "ADMINISTRATOR"} {
			if strings.Contains(w.Body.String(), private) {
				t.Fatal("Public recovery boundary disclosed private journal or grants", private)
			}
		}
		for _, path := range []string{
			"/api/flats", "/api/flats/demo-flat-A-101", "/api/registry/summary", "/api/people", "/api/admin/accounts",
			"/api/entries", "/api/receipts/unavailable-original/download", "/api/backups", "/api/maintenance", "/api/collections", "/api/documents",
			"/api/contacts", "/api/messages", "/api/financial-statements", "/api/reviews", "/api/complaints",
		} {
			w := actor.request("GET", path, "")
			if w.Code != 423 || !strings.Contains(w.Body.String(), "recovery_review_required") || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("Private route crossed quarantine or returned a misleading boundary", path, w.Code)
			}
		}
		for _, path := range []string{"/api/entries", "/api/auth/mfa/setup", "/api/auth/mfa/verify", "/api/auth/change-password"} {
			if w := actor.request("POST", path, `{}`); w.Code != 423 {
				t.Fatal("Copied authority created a private change during review", path, w.Code)
			}
		}
	}
	// A new password login identifies the old account without restoring grants.
	fresh := signIn(t, h, "admin@demo.society")
	if w := fresh.request("GET", "/api/flats", ""); w.Code != 423 {
		t.Fatal("Fresh authentication alone resumed checkpoint authority", w.Code)
	}
	if w := fresh.request("POST", "/api/auth/logout", `{}`); w.Code != 200 {
		t.Fatal("Quarantine prevented real sign-out", w.Code)
	}
	if w := fresh.request("GET", "/api/auth/me", ""); w.Code != 401 {
		t.Fatal("Signed-out recovery session remained usable", w.Code)
	}
	for _, table := range []string{"entries", "receipts", "entry_reversals", "receipt_counter"} {
		var count int
		if err := s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("Refused recovery actions created money effects", table, count, err)
		}
	}
}
