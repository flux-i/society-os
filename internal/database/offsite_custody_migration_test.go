package database

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestSchemaTwentyNinePreservesAll104PriorTablesProvenanceSQLBlobsAndOriginalReceipt(t *testing.T) {
	s, token, _ := communitySchemaNineteen(t)
	for _, version := range []int{20, 21, 22, 23, 24, 25, 26, 27, 28} {
		applyHistoricalCommunityMigration(t, s, version)
	}
	ctx := context.Background()
	entry := post(t, s, token, received("432.19"))
	beforeMoney, err := s.EntryFor(ctx, token, entry)
	if err != nil || beforeMoney.AmountPaise != 43219 || beforeMoney.ReceiptID == "" {
		t.Fatal("Independent pre-upgrade original money was missing", err)
	}
	objects := map[string]string{}
	var tables []string
	rows, err := s.DB.Query("SELECT type,name,sql FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' AND sql IS NOT NULL ORDER BY type,name")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var kind, name, definition string
		if err := rows.Scan(&kind, &name, &definition); err != nil {
			t.Fatal(err)
		}
		objects[kind+":"+name] = definition
		if kind == "table" && name != "schema_migrations" && name != "sessions" && name != "account_tokens" && name != "mfa_pending" && name != "mfa_recovery_codes" {
			tables = append(tables, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if len(tables) != 104 {
		t.Fatal("Expected the actual preceding schema28 inventory", len(tables))
	}
	before := statementMessageRows(t, s, tables)
	var privateBlobCount int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM mfa_factors WHERE typeof(secret_ciphertext)='blob' AND length(secret_ciphertext)>0").Scan(&privateBlobCount); err != nil || privateBlobCount < 1 {
		t.Fatal("Migration fixture did not contain actual private encrypted BLOB bytes", privateBlobCount, err)
	}
	var provenance string
	if err := s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations ORDER BY version)").Scan(&provenance); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, statementMessageRows(t, s, tables)) {
		t.Fatal("Recovery migration changed prior persistent rows or BLOBs")
	}
	for key, expected := range objects {
		kind, name, _ := strings.Cut(key, ":")
		var actual string
		if err := s.DB.QueryRow("SELECT sql FROM sqlite_schema WHERE type=? AND name=?", kind, name).Scan(&actual); err != nil || actual != expected {
			t.Fatal("Recovery migration changed an existing SQL object", key, err)
		}
	}
	var retained string
	if err := s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations WHERE version<=28 ORDER BY version)").Scan(&retained); err != nil || retained != provenance {
		t.Fatal("Previous migration provenance changed", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes')", 112)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epochs", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_external_jobs", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	afterMoney, err := s.EntryFor(ctx, token, entry)
	if err != nil || !reflect.DeepEqual(beforeMoney, afterMoney) {
		t.Fatal("Original receipt/amount changed during custody migration", err)
	}
	if err := s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
}
