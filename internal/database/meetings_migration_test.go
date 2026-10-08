package database

import (
	"context"
	"reflect"
	"testing"
)

func TestSchemaTwentyOneMeetingsPreserveAll81PriorDefinitionsRowsProvenanceAndOriginals(t *testing.T) {
	s, a, b := communitySchemaNineteen(t)
	applyHistoricalCommunityMigration(t, s, 20)
	ctx := context.Background()
	entry := post(t, s, a, received("432.19"))
	original, e := s.EntryFor(ctx, a, entry)
	if e != nil || original.AmountPaise != 43219 || original.ReceiptID == "" {
		t.Fatal(original, e)
	}
	contact := contactInput()
	contact.ContactPreferences = ContactPreferences{CommunityWhatsApp: true}
	if _, e = s.RegisterContact(ctx, a, "demo-owner-A-101", contact); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnContact(ctx, b, "demo-owner-A-101", contactAction(contactDetails(t, s, b, "demo-owner-A-101"), "VERIFIED")); e != nil {
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
	in := communityTestInput(t, s, a, "CONTACT")
	directory := communityPropose(t, s, a, "", in)
	communityApprove(t, s, b, directory)
	in.OperationKey, in.Version, in.Phone = randomToken(), 2, "+919000000202"
	communityPropose(t, s, a, directory, in)
	wantDirectory := communityDetail(t, s, a, directory, true)
	rows, e := s.DB.Query("SELECT name,sql FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name")
	if e != nil {
		t.Fatal(e)
	}
	tables, definitions := []string{}, map[string]string{}
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
	if e != nil || len(tables) != 81 {
		t.Fatal("genuine prior schema inventory", len(tables), e)
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
		t.Fatal("prior immutable or operational rows changed")
	}
	for name, definition := range definitions {
		var retained string
		if e = s.DB.QueryRow("SELECT sql FROM sqlite_schema WHERE type='table' AND name=?", name).Scan(&retained); e != nil || retained != definition {
			t.Fatal("prior definition changed", name, e)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes')", 85)
	var retained string
	if e = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations WHERE version<=20 ORDER BY version)").Scan(&retained); e != nil || retained != provenance {
		t.Fatal("migration provenance changed", e)
	}
	for _, table := range []string{"meeting_resources", "meeting_versions", "meeting_events", "meeting_acknowledgements"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
	if e = s.VerifySchema(ctx); e != nil {
		t.Fatal(e)
	}
	if got, e := s.EntryFor(ctx, a, entry); e != nil || !reflect.DeepEqual(original, got) {
		t.Fatal("original money or receipt changed", got, e)
	}
	if got := communityDetail(t, s, a, directory, true); !reflect.DeepEqual(wantDirectory, got) {
		t.Fatal("pending contact or approved predecessor changed", got)
	}
	if got := messageDetail(t, s, a, message); got.Outcomes["UNKNOWN"] != 1 || got.CanDispatch || got.Deliveries[0].Attempts != 1 {
		t.Fatal("uncertain provider evidence changed", got)
	}
}
