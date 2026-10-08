package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"
)

// Preserve the genuine historical parent-rebuild boundary before later additive migrations.
func applyHistoricalReminderMigration(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	body, e := migrationBody(22)
	if e != nil {
		t.Fatal(e)
	}
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	if _, e = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
			t.Error(err)
		}
	}()
	tx, e := conn.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, string(body)); e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256(body)
	if _, e = tx.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(?,?,?)", 22, hex.EncodeToString(hash[:]), "2026-10-08"); e != nil {
		t.Fatal(e)
	}
	rows, e := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if e != nil {
		t.Fatal(e)
	}
	if rows.Next() {
		rows.Close()
		t.Fatal("historical reminder migration lost a retained reference")
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
}

func TestSchemaTwentyThreeBudgetsPreserve86PriorDefinitionsRowsProvenanceAndOriginalMoney(t *testing.T) {
	s, a, b := communitySchemaNineteen(t)
	applyHistoricalCommunityMigration(t, s, 20)
	applyHistoricalCommunityMigration(t, s, 21)
	applyHistoricalReminderMigration(t, s)
	ctx := context.Background()
	id := post(t, s, a, received("432.19"))
	money, e := s.EntryFor(ctx, a, id)
	if e != nil || money.AmountPaise != 43219 {
		t.Fatal(money, e)
	}
	contact := contactInput()
	contact.ContactPreferences = ContactPreferences{FinanceEmail: true, CommunityEmail: true}
	if _, e = s.RegisterContact(ctx, a, "demo-owner-A-101", contact); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnContact(ctx, b, "demo-owner-A-101", contactAction(contactDetails(t, s, b, "demo-owner-A-101"), "VERIFIED")); e != nil {
		t.Fatal(e)
	}
	cycle := publishMaintenance(t, s, a, b, maintenanceProposal())
	allocate(t, s, a, id, cycle.Lines[0].EntryID, "400.00")
	reminder := reminderTestInput("MAINTENANCE_REMINDER", cycle.ID)
	reminder.Channel = "EMAIL"
	reminder.Target = MessageTarget{Kind: "HOMES", IDs: []string{"demo-flat-A-101"}}
	message := messageApproved(t, s, a, b, reminder)
	reminderDispatch(t, s, a, message, "UNKNOWN")
	originalMessage := messageDetail(t, s, a, message)
	if originalMessage.Outcomes["UNKNOWN"] != 1 || originalMessage.Deliveries[0].Attempts != 1 {
		t.Fatal(originalMessage)
	}
	var binding int64
	if e = s.DB.QueryRow("SELECT outstanding_paise FROM message_reminder_recipients WHERE batch_id=? AND resident_id='demo-owner-A-101'", message).Scan(&binding); e != nil || binding != 60000 {
		t.Fatal("original selected-home binding", binding, e)
	}
	rows, e := s.DB.Query("SELECT type,name,sql FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' AND sql IS NOT NULL ORDER BY type,name")
	if e != nil {
		t.Fatal(e)
	}
	definitions := map[string]string{}
	tables := []string{}
	for rows.Next() {
		var kind, name, definition string
		if e = rows.Scan(&kind, &name, &definition); e != nil {
			t.Fatal(e)
		}
		definitions[kind+":"+name] = definition
		if kind == "table" && name != "schema_migrations" && name != "sessions" && name != "account_tokens" && name != "mfa_pending" && name != "mfa_recovery_codes" {
			tables = append(tables, name)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil || len(tables) != 86 {
		t.Fatal("genuine schema22 table inventory", len(tables), e)
	}
	before := statementMessageRows(t, s, tables)
	var provenance string
	if e = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations ORDER BY version)").Scan(&provenance); e != nil {
		t.Fatal(e)
	}
	applyHistoricalBudgetMigration(t, s)
	if !reflect.DeepEqual(before, statementMessageRows(t, s, tables)) {
		t.Fatal("prior persistent rows changed")
	}
	for key, want := range definitions {
		var got string
		split := 0
		for key[split] != ':' {
			split++
		}
		if e = s.DB.QueryRow("SELECT sql FROM sqlite_schema WHERE type=? AND name=?", key[:split], key[split+1:]).Scan(&got); e != nil || got != want {
			t.Fatal("prior SQL definition changed", key, want, got, e)
		}
	}
	var retained string
	if e = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations WHERE version<=22 ORDER BY version)").Scan(&retained); e != nil || retained != provenance {
		t.Fatal("migration provenance changed", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes')", 93)
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.VerifySchema(ctx); e != nil {
		t.Fatal(e)
	}
	if got := messageDetail(t, s, a, message); !reflect.DeepEqual(originalMessage, got) {
		t.Fatal("prior reminder binding or uncertain handoff changed", got)
	}
	after, e := s.EntryFor(ctx, a, id)
	if e != nil || !reflect.DeepEqual(money, after) {
		t.Fatal("original receipt changed", after, e)
	}
}
