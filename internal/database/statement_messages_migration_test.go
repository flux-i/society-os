package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"society.local/portal/internal/security"
)

func statementMessageSchemaSixteen(t *testing.T) (*Store, string, string) {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "schema-sixteen.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	accessExec(t, s, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT")
	for version := 1; version <= 16; version++ {
		body, err := migrationBody(version)
		if err != nil {
			t.Fatal(err)
		}
		accessExec(t, s, string(body))
		sum := sha256.Sum256(body)
		accessExec(t, s, "INSERT INTO schema_migrations VALUES(?,?,?)", version, hex.EncodeToString(sum[:]), "2026-10-07")
	}
	for _, seed := range []func(context.Context) error{s.SeedDemo, s.SeedDemoAccounts, s.SeedDemoTreasury} {
		if err = seed(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s.MFA, err = security.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	a := reviewLogin(t, s, "admin@demo.society")
	b := maintenanceReviewer(t, s, a)
	return s, a, b
}

func historicalMessageReads(t *testing.T, s *Store) {
	t.Helper()
	// Current readers have two additive provider lookups. Empty TEMP views let
	// this historical fixture build legacy envelopes without introducing any
	// persistent tables or provider bindings into the schema being migrated.
	// One connection keeps these fixture-only views connection-local. Remove
	// them and restore the normal pool before running the actual migration.
	s.DB.SetMaxOpenConns(1)
	accessExec(t, s, "CREATE TEMP VIEW message_provider_bindings AS SELECT '' AS batch_id,0 AS snapshot_version,'null' AS provider_json WHERE 0")
	accessExec(t, s, "CREATE TEMP VIEW whatsapp_handoffs AS SELECT '' AS attempt_id,0 AS retry_at WHERE 0")
}

func statementMessageRows(t *testing.T, s *Store, tables []string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range tables {
		rows, err := s.DB.Query("SELECT * FROM " + table + " ORDER BY rowid")
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		all := []any{}
		for rows.Next() {
			values := make([]any, len(columns))
			dest := make([]any, len(columns))
			for i := range values {
				dest[i] = &values[i]
			}
			if err = rows.Scan(dest...); err != nil {
				t.Fatal(err)
			}
			all = append(all, values)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(all)
		if err != nil {
			t.Fatal(err)
		}
		out[table] = string(data)
	}
	return out
}

func TestSchemaSixteenStatementMessagesPreserveAll74TablesOriginalsAndUnknownProof(t *testing.T) {
	s, a, b := statementMessageSchemaSixteen(t)
	historicalMessageReads(t, s)
	ctx := context.Background()
	contact := contactInput()
	contact.Phone, contact.Email = "+919000000101", "owner@example.test"
	contact.ContactPreferences = ContactPreferences{true, true, true, true}
	if _, err := s.RegisterContact(ctx, a, "demo-owner-A-101", contact); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ActOnContact(ctx, b, "demo-owner-A-101", contactAction(contactDetails(t, s, b, "demo-owner-A-101"), "VERIFIED")); err != nil {
		t.Fatal(err)
	}
	entry := post(t, s, a, received("432.19"))
	original, err := s.EntryFor(ctx, a, entry)
	if err != nil || original.AmountPaise != 43219 {
		t.Fatal(original, err)
	}
	notice, err := s.SubmitReview(ctx, a, "", proposal("NOTICE"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, b, notice, decision(1, "APPROVED")); err != nil {
		t.Fatal(err)
	}
	input := messageInput(notice)
	input.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}
	id := messageApproved(t, s, a, b, input)
	claims, _, err := s.ClaimMessageDispatch(ctx, a, id, statementMessageDispatch(messageDetail(t, s, a, id)))
	if err != nil || len(claims) != 1 {
		t.Fatal(claims, err)
	}
	proof, err := s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "UNKNOWN")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteSyntheticMessage(ctx, claims[0].ID); err != nil {
		t.Fatal(err)
	}
	receiptInput := input
	receiptInput.SourceKind, receiptInput.SourceID = "RECEIPT", original.ReceiptID
	receipt := messageApproved(t, s, a, b, receiptInput)
	file, pub := publishedMessageStatement(t, s, a, b, "ALL")
	data := []byte("Description,Amount\nPending replacement,0.01\n")
	replacement := statementInput(data)
	replacement.Replaces = file
	replacement.Version = statementDetail(t, s, a, file).Version
	if _, err = s.ReserveStatement(ctx, a, replacement); err != nil {
		t.Fatal(err)
	}
	rows, err := s.DB.Query("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	tables := []string{}
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(tables) != 75 {
		t.Fatal("74 persistent plus migration provenance", len(tables), err)
	}
	before := statementMessageRows(t, s, tables)
	accessExec(t, s, "DROP VIEW temp.message_provider_bindings")
	accessExec(t, s, "DROP VIEW temp.whatsapp_handoffs")
	s.DB.SetMaxOpenConns(4)
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	after := statementMessageRows(t, s, tables)
	delete(before, "schema_migrations")
	delete(after, "schema_migrations")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("migration changed prior persistent rows or durable handoff")
	}
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	if x := messageDetail(t, s, a, id); x.Outcomes["UNKNOWN"] != 1 {
		t.Fatal("lost unknown outcome", x)
	}
	if x := messageDetail(t, s, a, receipt); x.Outcomes["QUEUED"] != 1 {
		t.Fatal("lost approved receipt queue", x)
	}
	if _, err = s.ReconcileMessage(ctx, a, id, claims[0].DeliveryID, messageDecision(messageDetail(t, s, a, id), "RECONCILE")); err != nil {
		t.Fatal(err)
	}
	if x := messageDetail(t, s, a, id); x.Outcomes["ACCEPTED"] != 1 || x.Deliveries[0].ProviderID != proof.ProviderID || x.Outcomes["DELIVERED"] != 0 || x.Outcomes["READ"] != 0 {
		t.Fatal("migration reconciliation invented outcome", x)
	}
	if _, err = s.MessagePreviewFor(ctx, a, statementMessageInput(pub), 1); err != nil {
		t.Fatal("new source unavailable", err)
	}
	got, err := s.EntryFor(ctx, a, entry)
	if err != nil || !reflect.DeepEqual(got, original) {
		t.Fatal("money changed", got, err)
	}
	for _, query := range []string{"DELETE FROM message_batches", "UPDATE message_batches SET source_kind='STATEMENT' WHERE source_kind='RECEIPT'", "UPDATE message_batches SET purpose='FINANCE' WHERE source_kind='NOTICE'"} {
		if _, err = s.DB.Exec(query); err == nil {
			t.Fatal("retained parent constraints removed", query)
		}
	}
}

func TestStatementMessageFailedMigrationRestoresForeignKeysOnEveryConnection(t *testing.T) {
	s, _, _ := statementMessageSchemaSixteen(t)
	ctx := context.Background()
	accessExec(t, s, "UPDATE schema_migrations SET checksum='deliberately-wrong' WHERE version=1")
	if err := s.Migrate(ctx); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatal("bad provenance migrated", err)
	}
	var version int
	if err := s.DB.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil || version != 16 {
		t.Fatal("failure partially migrated", version, err)
	}
	connections := []*sql.Conn{}
	defer func() {
		for _, c := range connections {
			c.Close()
		}
	}()
	for i := 0; i < 4; i++ {
		c, err := s.DB.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, c)
	}
	for _, c := range connections {
		var enabled int
		if err := c.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
			t.Fatal("unenforced connection returned after failure", enabled, err)
		}
		_, err := c.ExecContext(ctx, "INSERT INTO message_deliveries(id,batch_id,snapshot_version,destination,state,updated_at) VALUES(?,'nonexistent-batch',1,'fictional@example.test','QUEUED',1)", randomToken())
		if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
			t.Fatal("actual missing parent accepted", err)
		}
	}
}
