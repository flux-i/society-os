package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"society.local/portal/internal/security"
)

func TestSchemaThirteenContactUpgradePreservesFineMoneyHistoryAndAllPriorPersistentRows(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, filepath.Join(t.TempDir(), "schema-thirteen.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	accessExec(t, s, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT")
	checks := map[int]string{}
	for version := 1; version <= 13; version++ {
		body, e := migrationBody(version)
		if e != nil {
			t.Fatal(e)
		}
		accessExec(t, s, string(body))
		sum := sha256.Sum256(body)
		checks[version] = hex.EncodeToString(sum[:])
		accessExec(t, s, "INSERT INTO schema_migrations VALUES(?,?,?)", version, checks[version], "2026-10-06")
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
	b, o := maintenanceReviewer(t, s, a), reviewLogin(t, s, "owner@demo.society")
	// A real schema-13 fine and original received receipt make this upgrade
	// meaningful independently of empty new contact tables.
	rule := publishIncidentRule(t, s, a, b)
	in := incidentInput(rule)
	in.FlatID = "demo-flat-A-101"
	incident, e := s.SaveIncident(ctx, o, "", in)
	if e != nil {
		t.Fatal(e)
	}
	incidentAction(t, s, b, incident, "SUBSTANTIATED")
	fine := proposedFine(t, s, a, incident)
	notifyFine(t, s, a, b, fine)
	resolveFine(t, s, a, b, fine)
	issueFine(t, s, a, b, fine)
	paid := verifyFinePayment(t, s, o, b, fine)
	wantFine := fineDetails(t, s, a, fine)
	wantStatement := statement(t, s, a, "demo-flat-A-101")
	if wantFine.OutstandingPaise != 15025 || paid.AmountPaise != 10000 || paid.ReceiptID == "" {
		t.Fatal("independent populated money", wantFine, paid)
	}
	snapshot := func() map[string]string {
		t.Helper()
		out := map[string]string{}
		rows, e := s.DB.Query(`SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes','resident_contacts','contact_events') ORDER BY name`)
		if e != nil {
			t.Fatal(e)
		}
		tables := []string{}
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
				ptr := make([]any, len(cols))
				for i := range row {
					ptr[i] = &row[i]
				}
				if e = rows.Scan(ptr...); e != nil {
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
	before := snapshot()
	if len(before) != 56 {
		t.Fatal("independent prior persistent table inventory", len(before))
	}
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.VerifySchema(ctx); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, snapshot()) || !reflect.DeepEqual(wantFine, fineDetails(t, s, a, fine)) || !reflect.DeepEqual(wantStatement, statement(t, s, a, "demo-flat-A-101")) {
		t.Fatal("populated prior rows or amounts changed")
	}
	for version, want := range checks {
		var got string
		if e = s.DB.QueryRow("SELECT checksum FROM schema_migrations WHERE version=?", version).Scan(&got); e != nil || got != want {
			t.Fatal("prior provenance", version, e)
		}
	}
	for _, table := range []string{"resident_contacts", "contact_events"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
}
