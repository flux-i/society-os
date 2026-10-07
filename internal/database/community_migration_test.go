package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"
)

func TestSchemaTwentyCommunityPreservesAll78PriorPersistentTablesOriginalMoneyAndUnknownHandoff(t *testing.T) {
	s, a, b := statementMessageSchemaSixteen(t)
	ctx := context.Background()
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); e != nil {
		t.Fatal(e)
	}
	for version := 17; version <= 19; version++ {
		body, err := migrationBody(version)
		if err != nil {
			t.Fatal(err)
		}
		if _, e = conn.ExecContext(ctx, string(body)); e != nil {
			t.Fatal(e)
		}
		hash := sha256.Sum256(body)
		if _, e = conn.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(?,?,?)", version, hex.EncodeToString(hash[:]), "2026-10-07"); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); e != nil {
		t.Fatal(e)
	}
	conn.Close()
	entry := post(t, s, a, received("432.19"))
	original, e := s.EntryFor(ctx, a, entry)
	if e != nil || original.AmountPaise != 43219 || original.ReceiptID == "" {
		t.Fatal(original, e)
	}
	notice, e := s.SubmitReview(ctx, a, "", proposal("NOTICE"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideReview(ctx, b, notice, decision(1, "APPROVED")); e != nil {
		t.Fatal(e)
	}
	contact := contactInput()
	contact.ContactPreferences = ContactPreferences{CommunityWhatsApp: true}
	if _, e = s.RegisterContact(ctx, a, "demo-owner-A-101", contact); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnContact(ctx, b, "demo-owner-A-101", contactAction(contactDetails(t, s, b, "demo-owner-A-101"), "VERIFIED")); e != nil {
		t.Fatal(e)
	}
	id := whatsappApproved(t, s, a, b, notice)
	claim := whatsappClaim(t, s, a, id)
	if _, e = s.PrepareWhatsAppHandoff(ctx, a, claim.ID, whatsappProvider()); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteWhatsAppHandoff(ctx, claim.ID, WhatsAppResult{State: "UNKNOWN", Reason: "WHATSAPP_UNCERTAIN"}); e != nil {
		t.Fatal(e)
	}
	rows, e := s.DB.Query("SELECT name,sql FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name")
	if e != nil {
		t.Fatal(e)
	}
	tables := []string{}
	definitions := map[string]string{}
	for rows.Next() {
		var name, definition string
		if e = rows.Scan(&name, &definition); e != nil {
			t.Fatal(e)
		}
		tables = append(tables, name)
		definitions[name] = definition
	}
	e = rows.Err()
	rows.Close()
	if e != nil || len(tables) != 78 {
		t.Fatal("prior persistent inventory", len(tables), e)
	}
	before := statementMessageRows(t, s, tables)
	var provenance string
	if e = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations ORDER BY version)").Scan(&provenance); e != nil {
		t.Fatal(e)
	}
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if got := statementMessageRows(t, s, tables); !reflect.DeepEqual(before, got) {
		t.Fatal("prior original rows changed")
	}
	for name, definition := range definitions {
		var retainedDefinition string
		if e = s.DB.QueryRow("SELECT sql FROM sqlite_schema WHERE type='table' AND name=?", name).Scan(&retainedDefinition); e != nil || retainedDefinition != definition {
			t.Fatal("prior column or constraint definition changed", name, e)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes')", 81)
	var retained string
	if e = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations WHERE version<=19 ORDER BY version)").Scan(&retained); e != nil || retained != provenance {
		t.Fatal("prior provenance changed", e)
	}
	if e = s.VerifySchema(ctx); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"community_resources", "community_versions", "community_events"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
	if got, e := s.EntryFor(ctx, a, entry); e != nil || !reflect.DeepEqual(original, got) {
		t.Fatal("original money/receipt changed", got, e)
	}
	message := messageDetail(t, s, a, id)
	if message.Outcomes["UNKNOWN"] != 1 || message.CanDispatch || message.Deliveries[0].Attempts != 1 {
		t.Fatal("prior uncertain handoff changed", message)
	}
}
