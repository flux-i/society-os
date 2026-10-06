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

func TestSchemaFourteenMessagingUpgradePreservesAll58PriorPersistentTablesAndPopulatedContactsMoney(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, filepath.Join(t.TempDir(), "schema-fourteen.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	accessExec(t, s, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT")
	checks := map[int]string{}
	for version := 1; version <= 14; version++ {
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
	a, b := reviewLogin(t, s, "admin@demo.society"), reviewLogin(t, s, "committee@demo.society")
	in := contactInput()
	in.ContactPreferences = ContactPreferences{true, true, true, true}
	if _, e = s.RegisterContact(ctx, a, "demo-owner-A-101", in); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnContact(ctx, b, "demo-owner-A-101", contactAction(contactDetails(t, s, b, "demo-owner-A-101"), "VERIFIED")); e != nil {
		t.Fatal(e)
	}
	wantContact := contactDetails(t, s, a, "demo-owner-A-101")
	entry := post(t, s, a, received("123.45"))
	wantMoney, e := s.EntryFor(ctx, a, entry)
	if e != nil || wantMoney.AmountPaise != 12345 || wantMoney.ReceiptID == "" {
		t.Fatal(wantMoney, e)
	}
	rows, e := s.DB.Query("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name")
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
	if e != nil || len(tables) != 58 {
		t.Fatal("prior persistent inventory", len(tables), e)
	}
	digest := func() map[string]string {
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
	before := digest()
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.VerifySchema(ctx); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, digest()) || !reflect.DeepEqual(wantContact, contactDetails(t, s, a, "demo-owner-A-101")) {
		t.Fatal("upgrade changed prior rows or immutable contact decisions")
	}
	got, e := s.EntryFor(ctx, a, entry)
	if e != nil || !reflect.DeepEqual(got, wantMoney) {
		t.Fatal("upgrade changed original money/receipt", got, e)
	}
	for version, want := range checks {
		var got string
		if e = s.DB.QueryRow("SELECT checksum FROM schema_migrations WHERE version=?", version).Scan(&got); e != nil || got != want {
			t.Fatal("prior provenance", version, e)
		}
	}
	for _, table := range []string{"message_batches", "message_deliveries", "message_recipients", "message_events", "message_attempts", "message_attempt_recipients", "message_delivery_events", "simulation_messages", "message_callbacks"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
	var integrity string
	if e = s.DB.QueryRow("PRAGMA integrity_check").Scan(&integrity); e != nil || integrity != "ok" {
		t.Fatal(integrity, e)
	}
	rows, e = s.DB.Query("PRAGMA foreign_key_check")
	if e != nil {
		t.Fatal(e)
	}
	if rows.Next() {
		t.Fatal("foreign key violation")
	}
	rows.Close()
}
