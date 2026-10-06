package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"society.local/portal/internal/security"
	"testing"
)

func TestSchemaTwelveFineUpgradePreservesAllPriorPersistentRowsBytesResponsesMoneyAndProvenance(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, filepath.Join(t.TempDir(), "schema-twelve.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	accessExec(t, s, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT")
	checks := map[int]string{}
	for version := 1; version <= 12; version++ {
		body, e := migrationBody(version)
		if e != nil {
			t.Fatal(e)
		}
		accessExec(t, s, string(body))
		sum := sha256.Sum256(body)
		checks[version] = hex.EncodeToString(sum[:])
		accessExec(t, s, "INSERT INTO schema_migrations VALUES(?,?,?)", version, checks[version], "2026-10-05")
	}
	for _, seed := range []func(context.Context) error{s.SeedDemo, s.SeedDemoAccounts, s.SeedDemoTreasury} {
		if e = seed(ctx); e != nil {
			t.Fatal(e)
		}
	}
	s.MFA, e = security.NewBox(make([]byte, 32))
	if e != nil {
		t.Fatal(e)
	}
	a := reviewLogin(t, s, "admin@demo.society")
	b := maintenanceReviewer(t, s, a)
	o := reviewLogin(t, s, "owner@demo.society")
	fund := createPublishedFund(t, s, a, b, fundProposal())
	report := createFundClaim(t, s, o, fundClaim(fund.ID, "400.00", "MIGRATION-SUPPLIED-RECEIPT"))
	paid := confirmFund(t, s, a, report, "MIGRATION-EXTERNAL-SOURCE", "400.00")
	work := createWork(t, s, a, upkeepWork())
	publishWork(t, s, a, work, "BUILDING", "A")
	workAction(t, s, a, work, "START")
	rule := publishIncidentRule(t, s, a, b)
	photo, e := s.UploadIncidentPicture(ctx, o, randomToken(), "retained-scene.png", incidentPNG(t))
	if e != nil {
		t.Fatal(e)
	}
	in := incidentInput(rule)
	in.FlatID, in.PictureID = "demo-flat-A-101", photo.ID
	incident, e := s.SaveIncident(ctx, o, "", in)
	if e != nil {
		t.Fatal(e)
	}
	incidentAction(t, s, b, incident, "UNDER_REVIEW")
	x := incidentCase(t, s, b, incident)
	if _, e = s.ActOnIncident(ctx, b, incident, IncidentAction{OperationKey: randomToken(), Version: x.Version, Action: "ISSUE_NOTICE", Reason: "Retained deliberate household notice before the upgrade", Title: "Retained household notice", Body: "Supplied original wording retained across the fine upgrade.", ResponseBy: today(), Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	x = incidentCase(t, s, b, incident)
	if _, e = s.RespondToIncident(ctx, o, x.NoticeID, IncidentResponseInput{OperationKey: randomToken(), Version: 1, Body: "Retained private household response before the upgrade", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	incidentAction(t, s, b, incident, "SUBSTANTIATED")
	beforeIncident := incidentCase(t, s, b, incident)
	beforeStatement := statement(t, s, a, "demo-flat-A-101")
	tables := []string{}
	rows, e := s.DB.Query(`SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name`)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var name string
		if e = rows.Scan(&name); e != nil {
			t.Fatal(e)
		}
		tables = append(tables, name)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		t.Fatal(e)
	}
	snapshot := func() map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, table := range tables {
			rows, e := s.DB.Query("SELECT * FROM " + table + " ORDER BY rowid")
			if e != nil {
				t.Fatal(e)
			}
			cols, e := rows.Columns()
			if e != nil {
				t.Fatal(e)
			}
			values := []any{}
			for rows.Next() {
				row := make([]any, len(cols))
				pointers := make([]any, len(cols))
				for i := range row {
					pointers[i] = &row[i]
				}
				if e = rows.Scan(pointers...); e != nil {
					t.Fatal(e)
				}
				values = append(values, row)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				t.Fatal(e)
			}
			blob, e := json.Marshal(values)
			if e != nil {
				t.Fatal(e)
			}
			out[table] = string(blob)
		}
		return out
	}
	if len(tables) != 49 {
		t.Fatal("expected independently enumerated 49 persistent schema-12 tables", len(tables))
	}
	before := snapshot()
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.VerifySchema(ctx); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("prior persistent rows changed")
	}
	if !reflect.DeepEqual(beforeStatement, statement(t, s, a, "demo-flat-A-101")) {
		t.Fatal("original credit changed")
	}
	if claimDetails(t, s, o, report).Receipt != paid.Receipt {
		t.Fatal("original receipt changed")
	}
	for version, want := range checks {
		var got string
		if e = s.DB.QueryRow("SELECT checksum FROM schema_migrations WHERE version=?", version).Scan(&got); e != nil || got != want {
			t.Fatal("provenance", version, e)
		}
	}
	if !reflect.DeepEqual(beforeIncident, incidentCase(t, s, b, incident)) {
		t.Fatal("original private incident/notice/response changed")
	}
	for _, table := range []string{"fines", "fine_events", "fine_notices", "fine_responses", "fine_reports", "fine_appeals", "fine_waivers"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
}
