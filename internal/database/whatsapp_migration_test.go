package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"
)

func TestSchemaNineteenProviderPreparationPreservesAll75PriorTablesAndOriginalMoney(t *testing.T) {
	s, a, b := statementMessageSchemaSixteen(t)
	ctx := context.Background()
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	for version := 17; version <= 18; version++ {
		body, failure := migrationBody(version)
		if failure != nil {
			t.Fatal(failure)
		}
		if _, err = conn.ExecContext(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		if _, err = conn.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(?,?,?)", version, hex.EncodeToString(sum[:]), "2026-10-07"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	historicalMessageReads(t, s)
	entry := post(t, s, a, received("432.19"))
	original, err := s.EntryFor(ctx, a, entry)
	if err != nil || original.AmountPaise != 43219 || original.ReceiptID == "" {
		t.Fatal(original, err)
	}
	// Existing legacy messages also retain their exact row shape and content.
	notice, err := s.SubmitReview(ctx, a, "", proposal("NOTICE"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, b, notice, decision(1, "APPROVED")); err != nil {
		t.Fatal(err)
	}
	contact := contactInput()
	contact.Email = "legacy-provider-upgrade@example.test"
	contact.ContactPreferences = ContactPreferences{CommunityEmail: true}
	if _, err = s.RegisterContact(ctx, a, "demo-owner-A-101", contact); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ActOnContact(ctx, b, "demo-owner-A-101", contactAction(contactDetails(t, s, b, "demo-owner-A-101"), "VERIFIED")); err != nil {
		t.Fatal(err)
	}
	legacyInput := messageInput(notice)
	legacyInput.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}
	legacyID := messagePropose(t, s, a, legacyInput)
	rows, err := s.DB.Query("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name")
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
	if err != nil || len(tables) != 75 {
		t.Fatal("previous accepted inventory", len(tables), err)
	}
	before := statementMessageRows(t, s, tables)
	var provenance string
	if err = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations ORDER BY version)").Scan(&provenance); err != nil {
		t.Fatal(err)
	}
	accessExec(t, s, "DROP VIEW temp.message_provider_bindings")
	accessExec(t, s, "DROP VIEW temp.whatsapp_handoffs")
	s.DB.SetMaxOpenConns(4)
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if after := statementMessageRows(t, s, tables); !reflect.DeepEqual(before, after) {
		t.Fatal("prior rows, columns or original bytes changed")
	}
	var retained string
	if err = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations WHERE version<=18 ORDER BY version)").Scan(&retained); err != nil || provenance != retained {
		t.Fatal("migration provenance changed", err)
	}
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"message_provider_bindings", "whatsapp_handoffs", "whatsapp_status_events"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
	legacy := messageDetail(t, s, a, legacyID)
	if legacy.Provider != nil || legacy.ProviderMode != "SIMULATION" || legacy.State != "PENDING" || legacy.Counts.Destinations != 1 {
		t.Fatal("legacy proposal was rebound to a different provider", legacy)
	}
	after, err := s.EntryFor(ctx, a, entry)
	if err != nil || !reflect.DeepEqual(original, after) {
		t.Fatal("original receipt changed", after, err)
	}
}
