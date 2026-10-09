package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
)

// Keep the existing budget test at its genuine schema23/93-table boundary
// before testing the latest additive migration with the current verifier.
func applyHistoricalBudgetMigration(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	body, e := migrationBody(23)
	if e != nil {
		t.Fatal(e)
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, string(body)); e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(body)
	if _, e = tx.ExecContext(ctx, "INSERT INTO schema_migrations VALUES(?,?,?)", 23, hex.EncodeToString(digest[:]), "2026-10-08"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
}

func TestSchemaTwentyFourChecklistsPreserve93PriorTablesDefinitionsProvenanceAndMoney(t *testing.T) {
	s, a, b := communitySchemaNineteen(t)
	applyHistoricalCommunityMigration(t, s, 20)
	applyHistoricalCommunityMigration(t, s, 21)
	applyHistoricalReminderMigration(t, s)
	applyHistoricalBudgetMigration(t, s)
	ctx := context.Background()
	p, e := s.CheckSession(ctx, b)
	if e != nil {
		t.Fatal(e)
	}
	if !p.CanManageRecords {
		grantAppointment(t, s, a, "demo-user-committee", "TREASURER", 30)
		b = reviewLogin(t, s, "committee@demo.society")
	}
	entry := post(t, s, a, received("432.19"))
	money, e := s.EntryFor(ctx, a, entry)
	if e != nil || money.AmountPaise != 43219 {
		t.Fatal(money, e)
	}
	plan := budgetPropose(t, s, a, "", budgetProposal())
	budgetApprove(t, s, b, plan)
	expense := expensePropose(t, s, a, "", expenseProposal(plan, "32.51", "PRIVATE-MIGRATION-PAID-23"))
	priorPlan := budgetDetail(t, s, a, plan)
	priorExpense := expenseDetail(t, s, a, expense)
	tables := []string{}
	objects := map[string]string{}
	rows, e := s.DB.Query("SELECT type,name,sql FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' AND sql IS NOT NULL ORDER BY type,name")
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var kind, name, body string
		if e = rows.Scan(&kind, &name, &body); e != nil {
			t.Fatal(e)
		}
		objects[kind+":"+name] = body
		if kind == "table" && name != "schema_migrations" && name != "sessions" && name != "account_tokens" && name != "mfa_pending" && name != "mfa_recovery_codes" {
			tables = append(tables, name)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil || len(tables) != 93 {
		t.Fatal("genuine schema23 inventory", len(tables), e)
	}
	before := statementMessageRows(t, s, tables)
	var provenance string
	if e = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations ORDER BY version)").Scan(&provenance); e != nil {
		t.Fatal(e)
	}
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, statementMessageRows(t, s, tables)) {
		t.Fatal("prior persistent rows changed")
	}
	for key, want := range objects {
		kind, name, _ := strings.Cut(key, ":")
		var got string
		if e = s.DB.QueryRow("SELECT sql FROM sqlite_schema WHERE type=? AND name=?", kind, name).Scan(&got); e != nil || got != want {
			t.Fatal("old SQL definition changed", key, e)
		}
	}
	var retained string
	if e = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations WHERE version<=23 ORDER BY version)").Scan(&retained); e != nil || retained != provenance {
		t.Fatal("provenance changed", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes')", 100)
	if e = s.VerifySchema(ctx); e != nil {
		t.Fatal(e)
	}
	after, e := s.EntryFor(ctx, a, entry)
	if e != nil || !reflect.DeepEqual(money, after) || !reflect.DeepEqual(priorPlan, budgetDetail(t, s, a, plan)) || !reflect.DeepEqual(priorExpense, expenseDetail(t, s, a, expense)) {
		t.Fatal("accepted budget, pending expense or original receipt changed", e)
	}
}
