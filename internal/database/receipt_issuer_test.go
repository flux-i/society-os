package database

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"society.local/portal/internal/security"
)

func TestConfiguredReceiptFreezesIssuerOriginalRetryReversalAndLegacyIdentity(t *testing.T) {
	s, registry := workspaceFixture(t)
	ctx := context.Background()
	bytes, err := os.ReadFile("../../testdata/registry-rehearsal-12.json")
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewRegistryImport(ctx, registry, string(bytes))
	if err != nil || preview.Counts != (ImportCounts{Buildings: 1, Homes: 12, People: 17, Relationships: 18, Occupied: 11, Vacant: 1, Owners: 12, Tenants: 4}) {
		t.Fatal("independent twelve-home fixture", preview.Counts, err)
	}
	if _, err = s.ApplyRegistryImport(ctx, registry, applySupplied(string(bytes), preview, "issuer-import-october-2026")); err != nil {
		t.Fatal(err)
	}
	var person, home string
	if err = s.DB.QueryRow("SELECT entity_id FROM registry_import_entities WHERE entity_kind='PERSON' AND source_id='owner-a-104'").Scan(&person); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.QueryRow("SELECT entity_id FROM registry_import_entities WHERE entity_kind='HOME' AND source_id='a-101'").Scan(&home); err != nil {
		t.Fatal(err)
	}
	link, err := s.Invite(ctx, registry, Invitation{ResidentID: person, Email: "finance-issuer@example.test", Role: "RESIDENT", TermDays: 90, IdentityVerified: true, Note: "Verified fictional personal identity and imported current relationship"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteAccountLink(ctx, link.Token, workspacePassword); err != nil {
		t.Fatal(err)
	}
	var account string
	if err = s.DB.QueryRow("SELECT id FROM users WHERE login='finance-issuer@example.test'").Scan(&account); err != nil {
		t.Fatal(err)
	}
	grantAppointment(t, s, registry, account, "TREASURER", 30)
	actor, _ := loginTest(t, s, "finance-issuer@example.test", workspacePassword)
	setup, err := s.SetupMFA(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmMFA(ctx, actor, security.Code(secret, time.Now().Unix()/30, 6)); err != nil {
		t.Fatal(err)
	}
	input := received("400.00")
	input.FlatID = home
	input.Payer = "Sample Owner A 101"
	id, err := s.CreateEntry(ctx, actor, input)
	if err != nil {
		t.Fatal(err)
	}
	confirmation := EntryAction{OperationKey: "issuer-received-confirm-2026", Confirmed: true}
	if _, err = s.PostEntry(ctx, actor, id, confirmation); err != nil {
		t.Fatal(err)
	}
	var original string
	if err = s.DB.QueryRow("SELECT snapshot_json FROM receipts WHERE entry_id=?", id).Scan(&original); err != nil {
		t.Fatal(err)
	}
	var snapshot ReceiptSnapshot
	if err = json.Unmarshal([]byte(original), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.AmountPaise != 40000 || !reflect.DeepEqual(snapshot.Issuer, &ReceiptIssuer{FormatVersion: 1, Name: "The Neighbourhood Rehearsal", Mode: "FICTIONAL_REHEARSAL"}) {
		t.Fatal("frozen supplied issuer/money", snapshot)
	}
	if _, err = s.PostEntry(ctx, actor, id, confirmation); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReverseEntry(ctx, actor, id, EntryAction{OperationKey: "issuer-linked-reversal-2026", Confirmed: true, Reason: "Verified supplied correction preserves the original receipt"}); err != nil {
		t.Fatal(err)
	}
	var after string
	if err = s.DB.QueryRow("SELECT snapshot_json FROM receipts WHERE entry_id=?", id).Scan(&after); err != nil || original != after {
		t.Fatal("original snapshot changed", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM flat_memberships WHERE can_view_finances=1", 0)
	legacy, demo := recordFixture(t)
	oldID := post(t, legacy, demo, received("400.01"))
	var old string
	if err = legacy.DB.QueryRow("SELECT snapshot_json FROM receipts WHERE entry_id=?", oldID).Scan(&old); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(old), &snapshot); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	json.Unmarshal([]byte(old), &fields)
	if _, found := fields["issuer"]; found {
		t.Fatal("legacy demo format changed")
	}
}
