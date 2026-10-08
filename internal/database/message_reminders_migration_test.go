package database

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

func TestSchemaTwentyTwoRemindersPreserve85PriorRows84DefinitionsAllTriggersAndHeldProof(t *testing.T) {
	s, a, b := communitySchemaNineteen(t)
	applyHistoricalCommunityMigration(t, s, 20)
	applyHistoricalCommunityMigration(t, s, 21)
	ctx := context.Background()
	entry := post(t, s, a, received("432.19"))
	money, e := s.EntryFor(ctx, a, entry)
	if e != nil || money.AmountPaise != 43219 {
		t.Fatal(money, e)
	}
	contact := contactInput()
	contact.ContactPreferences = ContactPreferences{true, true, true, true}
	if _, e = s.RegisterContact(ctx, a, "demo-owner-A-101", contact); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnContact(ctx, b, "demo-owner-A-101", contactAction(contactDetails(t, s, b, "demo-owner-A-101"), "VERIFIED")); e != nil {
		t.Fatal(e)
	}
	meeting := meetingPropose(t, s, a, "", meetingInput(t, s, a))
	meetingApprove(t, s, b, meeting)
	owner := reviewLogin(t, s, "owner@demo.society")
	if _, e = s.AcknowledgeMeeting(ctx, owner, meeting, meetingAck(meetingDetail(t, s, owner, meeting, false))); e != nil {
		t.Fatal(e)
	}
	notice, e := s.SubmitReview(ctx, a, "", proposal("NOTICE"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideReview(ctx, b, notice, decision(1, "APPROVED")); e != nil {
		t.Fatal(e)
	}
	message := whatsappApproved(t, s, a, b, notice)
	claim := whatsappClaim(t, s, a, message)
	if _, e = s.PrepareWhatsAppHandoff(ctx, a, claim.ID, whatsappProvider()); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteWhatsAppHandoff(ctx, claim.ID, WhatsAppResult{State: "UNKNOWN", Reason: "WHATSAPP_UNCERTAIN"}); e != nil {
		t.Fatal(e)
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
	if e != nil || len(tables) != 85 {
		t.Fatal("genuine schema21 inventory", len(tables), e)
	}
	before := statementMessageRows(t, s, tables)
	var provenance string
	if e = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations ORDER BY version)").Scan(&provenance); e != nil {
		t.Fatal(e)
	}
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if after := statementMessageRows(t, s, tables); !reflect.DeepEqual(before, after) {
		t.Fatal("prior persistent rows changed")
	}
	for key, want := range definitions {
		kind, name, _ := strings.Cut(key, ":")
		var got string
		if e = s.DB.QueryRow("SELECT sql FROM sqlite_schema WHERE type=? AND name=?", kind, name).Scan(&got); e != nil {
			t.Fatal(key, e)
		}
		if key == "table:message_batches" {
			if !strings.Contains(got, "MAINTENANCE_REMINDER") || !strings.Contains(got, "MEETING_REMINDER") {
				t.Fatal("parent CHECK extension missing", got)
			}
		} else if want != got {
			t.Fatal("prior table/index/trigger definition changed", key, want, got)
		}
	}
	var retained string
	if e = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations WHERE version<=21 ORDER BY version)").Scan(&retained); e != nil || retained != provenance {
		t.Fatal("prior provenance changed", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_reminder_recipients", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes')", 86)
	if e = s.VerifySchema(ctx); e != nil {
		t.Fatal(e)
	}
	connections := []*sql.Conn{}
	for i := 0; i < 4; i++ {
		conn, e := s.DB.Conn(ctx)
		if e != nil {
			t.Fatal(e)
		}
		connections = append(connections, conn)
		var enabled int
		if e = conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); e != nil || enabled != 1 {
			t.Fatal("unenforced pooled connection", i, enabled, e)
		}
	}
	for _, conn := range connections {
		conn.Close()
	}
	if got, e := s.EntryFor(ctx, a, entry); e != nil || !reflect.DeepEqual(money, got) {
		t.Fatal("original receipt changed", got, e)
	}
	if x := messageDetail(t, s, a, message); x.Outcomes["UNKNOWN"] != 1 || x.CanDispatch || x.Deliveries[0].Attempts != 1 {
		t.Fatal("retained uncertain provider handoff changed", x)
	}
	if x := meetingDetail(t, s, owner, meeting, false); !x.Acknowledgement.Acknowledged {
		t.Fatal("personal original lost", x)
	}
}
