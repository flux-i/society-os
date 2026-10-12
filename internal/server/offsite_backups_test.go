package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
)

func TestOffsiteHTTPReadBoundaryAndPausedPostingPreserveDraftAndAcknowledgementPrivacy(t *testing.T) {
	db, app, _ := handler(t)
	admin := signIn(t, app.Handler(), "admin@demo.society")
	owner := signIn(t, app.Handler(), "owner@demo.society")
	ctx := context.Background()
	if w := admin.request("GET", "/api/backups", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "NOT_CONFIGURED") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("explicit unprotected local state", w.Code)
	}
	if w := owner.request("GET", "/api/backups", ""); w.Code != 403 {
		t.Fatal("resident read global recovery state", w.Code)
	}
	if err := db.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.BindOffsitePolicy(ctx, strings.Repeat("b", 64), "FIXTURE", time.Now()); err != nil {
		t.Fatal(err)
	}
	j, err := db.ClaimOffsite(ctx, strings.Repeat("b", 64), "PRIVATE-RECOVERY-PREFIX", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	w := admin.request("GET", "/api/backups", "")
	var status database.OffsiteStatus
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &status) != nil || status.Phase != "PAUSED" || !status.FinancePaused || status.Counts["CAPTURING"] != 1 {
		t.Fatal("configured pending state inferred a usable copy", w.Code)
	}
	for _, private := range []string{"PRIVATE-RECOVERY-PREFIX", "object_key", "config_hash", "lease_token", "encrypted_sha256", "object_version"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("private storage metadata disclosed", private)
		}
	}
	if w := admin.request("GET", "/api/backups/"+j.ID, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "CLAIMED") {
		t.Fatal("operator history unavailable", w.Code)
	}
	if w := owner.request("GET", "/api/backups/"+j.ID, ""); w.Code != 403 {
		t.Fatal("resident read private history", w.Code)
	}
	for _, path := range []string{"/api/backups?page=0", "/api/backups?page=10001", "/api/backups?state=PRIVATE", "/api/backups/" + j.ID + "?page=0"} {
		if w := admin.request("GET", path, ""); w.Code != 400 {
			t.Fatal("unbounded or invalid read", path, w.Code)
		}
	}
	input := `{"operation_key":"offsite-http-draft-001","flat_id":"demo-flat-A-101","kind":"RECEIVED","amount":"400.01","date":"2026-01-01","description":"Supplied fictional money already received","payer":"Fictional owner","method":"CASH","reference":""}`
	w = admin.request("POST", "/api/entries", input)
	var created struct{ ID string }
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &created) != nil {
		t.Fatal("draft unavailable while protection pending", w.Code)
	}
	w = admin.request("POST", "/api/entries/"+created.ID+"/post", `{"operation_key":"offsite-http-post-001","confirmed":true,"reason":""}`)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "backup_protection_required") {
		t.Fatal("paused posting returned the wrong actionable boundary", w.Code, w.Body.String())
	}
	e, err := db.EntryFor(ctx, admin.cookie.Value, created.ID)
	if err != nil || e.State != "DRAFT" || e.AmountPaise != 40001 || e.ReceiptID != "" {
		t.Fatal("refused posting changed draft or original receipt", err)
	}
	var count int
	db.DB.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&count)
	if count != 0 {
		t.Fatal("unprotected receipt issued")
	}
	// This HTTP-domain fixture supplies custody metadata. Independent encryption
	// and offline-command integration tests prove the actual retained bytes.
	now := time.Now()
	if err := db.SealOffsite(ctx, j, strings.Repeat("c", 64), 128, now); err != nil {
		t.Fatal(err)
	}
	if err := db.FinishOffsite(ctx, j, "FAILED", "UPLOAD_DENIED", "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE offsite_backup_jobs SET attempts=5 WHERE id=?", j.ID); err != nil {
		t.Fatal(err)
	}
	j, err = db.OffsiteJobForCustody(ctx, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := admin.request("GET", "/api/backups", "")
	const note = "PRIVATE-CUSTODY-VERIFICATION-NOTE retained only for current operators"
	if _, err := db.RecordOffsiteCustody(ctx, database.OffsiteCustodyInput{JobID: j.ID, ArchiveID: strings.Repeat("C", 43), ManifestSHA256: strings.Repeat("d", 64), CipherSHA256: j.SHA256, CipherBytes: j.Bytes, CapturedAt: j.CapturedAt, ConfigHash: j.ConfigHash, Note: note}, now); err != nil {
		t.Fatal(err)
	}
	w = admin.request("GET", "/api/backups", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &status) != nil || status.ArchivedCount != 1 || !status.FinancePaused || status.CaptureAt != 0 || status.AcknowledgedAt != 0 || status.Counts["FAILED"] != 1 {
		t.Fatal("Custody read fabricated an upload acknowledgement or lost the original", w.Code, status)
	}
	var previous database.OffsiteStatus
	if json.Unmarshal(before.Body.Bytes(), &previous) != nil || previous.StatusKey == status.StatusKey {
		t.Fatal("Custody change did not invalidate a held metadata read")
	}
	for _, private := range []string{note, "archive_id", "archive_manifest_sha256", "encrypted_sha256", "config_hash", "object_key", "object_version"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("Bounded custody metadata disclosed private proof", private)
		}
	}
	w = admin.request("GET", "/api/backups/"+j.ID, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), note) || !strings.Contains(w.Body.String(), "archived_at") {
		t.Fatal("Current operator could not read the verification note", w.Code)
	}
	for _, private := range []string{"archive_id", "archive_manifest_sha256", "encrypted_sha256", "config_hash", "object_key", "object_version"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("Human history exposed private storage proof", private)
		}
	}
	if w := owner.request("GET", "/api/backups/"+j.ID, ""); w.Code != 403 {
		t.Fatal("Household read private custody history", w.Code)
	}
	w = admin.request("POST", "/api/entries/"+created.ID+"/post", `{"operation_key":"offsite-http-post-001","confirmed":true,"reason":""}`)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "backup_protection_required") {
		t.Fatal("Archival released financial confirmation", w.Code)
	}
	for _, path := range []string{"/api/backups", "/api/backups/" + j.ID, "/api/backups/" + j.ID + "/retry", "/api/backups/" + j.ID + "/archive", "/api/backups/override"} {
		if w := admin.request("POST", path, `{}`); w.Code != 405 {
			t.Fatal("portal exposed a backup mutation or override", path, w.Code)
		}
	}
}
