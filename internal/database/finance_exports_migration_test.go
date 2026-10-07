package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"
)

func TestSchemaEighteenExportsPreserveAllPriorPersistentTablesProvenanceAndOriginalMoney(t *testing.T) {
	s, a, _ := statementMessageSchemaSixteen(t)
	ctx := context.Background()
	// Construct the exact published schema-seventeen boundary before applying
	// eighteen. The parent-table rebuild occurs on one dedicated connection.
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	body, err := migrationBody(17)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	if _, err = conn.ExecContext(ctx, string(body)); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(17,?,?)", hex.EncodeToString(digest[:]), "2026-10-07"); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	entry := post(t, s, a, received("432.19"))
	original, err := s.EntryFor(ctx, a, entry)
	if err != nil || original.ReceiptID == "" {
		t.Fatal(original, err)
	}
	rows, err := s.DB.Query("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN ('sessions','account_tokens','mfa_pending','mfa_recovery_codes','schema_migrations') ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	tables := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(tables) != 74 {
		t.Fatal("published prior persistent inventory", len(tables), err)
	}
	before := statementMessageRows(t, s, tables)
	var priorHistory string
	if err = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations ORDER BY version)").Scan(&priorHistory); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if after := statementMessageRows(t, s, tables); !reflect.DeepEqual(before, after) {
		t.Fatal("export migration changed original persistent rows")
	}
	var afterHistory string
	if err = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations WHERE version<=17 ORDER BY version)").Scan(&afterHistory); err != nil || priorHistory != afterHistory {
		t.Fatal("prior migration provenance changed", err)
	}
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM finance_exports", 0)
	maintenanceCount(t, s, "SELECT MAX(version) FROM schema_migrations", 18)
	owner := reviewLogin(t, s, "owner@demo.society")
	x := exportCreate(t, s, owner, exportInput(exportPreview(t, s, owner, exportFilter("RECEIPTS", "OWN"))))
	_, data, _ := exportCSV(t, s, owner, x.ID)
	if x.Summary.OriginalReceived != 43219 || data[1]["receipt_number"] != original.ReceiptNumber {
		t.Fatal("new snapshot lost prior original", x, data)
	}
	after, err := s.EntryFor(ctx, a, entry)
	if err != nil || !reflect.DeepEqual(original, after) {
		t.Fatal("source changed by upgrade", after, err)
	}
}
