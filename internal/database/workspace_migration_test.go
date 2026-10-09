package database

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestSchemaTwentyFivePreservesAll96PriorPersistentTablesDefinitionsProvenanceAndOriginalMoney(t *testing.T) {
	s, a, b := communitySchemaNineteen(t)
	for _, version := range []int{20, 21, 22, 23, 24} {
		applyHistoricalCommunityMigration(t, s, version)
	}
	ctx := context.Background()
	entry := post(t, s, a, received("432.19"))
	money, err := s.EntryFor(ctx, a, entry)
	if err != nil || money.AmountPaise != 43219 || money.ReceiptID == "" {
		t.Fatal("independent pre-upgrade money", err)
	}
	p, err := s.CheckSession(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if !p.CanManageRecords {
		grantAppointment(t, s, a, "demo-user-committee", "TREASURER", 30)
		b = reviewLogin(t, s, "committee@demo.society")
	}
	plan := budgetPropose(t, s, a, "", budgetProposal())
	budgetApprove(t, s, b, plan)
	expense := expensePropose(t, s, a, "", expenseProposal(plan, "32.51", "PRIVATE-WORKSPACE-MIGRATION-25"))
	priorPlan := budgetDetail(t, s, a, plan)
	priorExpense := expenseDetail(t, s, a, expense)
	objects := map[string]string{}
	tables := []string{}
	rows, err := s.DB.Query("SELECT type,name,sql FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' AND sql IS NOT NULL ORDER BY type,name")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var kind, name, body string
		if err = rows.Scan(&kind, &name, &body); err != nil {
			t.Fatal(err)
		}
		objects[kind+":"+name] = body
		if kind == "table" && name != "schema_migrations" && name != "sessions" && name != "account_tokens" && name != "mfa_pending" && name != "mfa_recovery_codes" {
			tables = append(tables, name)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(tables) != 96 {
		t.Fatal("genuine schema24 inventory", len(tables), err)
	}
	before := statementMessageRows(t, s, tables)
	var provenance string
	if err = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations ORDER BY version)").Scan(&provenance); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, statementMessageRows(t, s, tables)) {
		t.Fatal("prior persistent rows changed")
	}
	for key, want := range objects {
		kind, name, _ := strings.Cut(key, ":")
		var got string
		if err = s.DB.QueryRow("SELECT sql FROM sqlite_schema WHERE type=? AND name=?", kind, name).Scan(&got); err != nil || want != got {
			t.Fatal("prior SQL changed", key, err)
		}
	}
	var retained string
	if err = s.DB.QueryRow("SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT * FROM schema_migrations WHERE version<=24 ORDER BY version)").Scan(&retained); err != nil || retained != provenance {
		t.Fatal("prior provenance changed", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('schema_migrations','sessions','account_tokens','mfa_pending','mfa_recovery_codes')", 99)
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := s.EntryFor(ctx, a, entry)
	if err != nil || !reflect.DeepEqual(money, after) || !reflect.DeepEqual(priorPlan, budgetDetail(t, s, a, plan)) || !reflect.DeepEqual(priorExpense, expenseDetail(t, s, a, expense)) {
		t.Fatal("original receipt or approved/pending finance changed", err)
	}
}
