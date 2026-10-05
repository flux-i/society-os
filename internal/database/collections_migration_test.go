package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"society.local/portal/internal/security"
	"testing"
)

func TestSchemaTenCollectionsUpgradePreservesPrivateAndPublishedUpkeepLedgerReceiptsAllocationsAndAuthority(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "schema-ten.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	accessExec(t, s, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT")
	checks := map[int]string{}
	for version := 1; version <= 10; version++ {
		body, e := migrationBody(version)
		if e != nil {
			t.Fatal(e)
		}
		accessExec(t, s, string(body))
		sum := sha256.Sum256(body)
		checks[version] = hex.EncodeToString(sum[:])
		accessExec(t, s, "INSERT INTO schema_migrations VALUES(?,?,?)", version, checks[version], "2026-10-05")
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
	admin := reviewLogin(t, s, "admin@demo.society")
	reviewer := maintenanceReviewer(t, s, admin)
	owner := reviewLogin(t, s, "owner@demo.society")
	cycle := publishMaintenance(t, s, admin, reviewer, maintenanceProposal())
	payment := post(t, s, admin, received("400.00"))
	accessExec(t, s, "INSERT INTO entry_allocations(id,source_id,charge_id,amount_paise,reason,actor_id,created_at) VALUES(?,?,?,30000,?,?,?)", randomToken(), payment, cycle.Lines[0].EntryID, "Legacy explicit credit allocation", "demo-user-admin", 2)
	vendor, err := s.SaveUpkeepRegister(ctx, admin, "VENDOR", "", upkeepRegisterInput("VENDOR"))
	if err != nil {
		t.Fatal(err)
	}
	in := upkeepWork()
	in.VendorID = vendor
	work := createWork(t, s, admin, in)
	publishWork(t, s, admin, work, "BUILDING", "A")
	workAction(t, s, admin, work, "START")
	workAction(t, s, admin, work, "SUBMIT_CHECK")
	wantWork := workAction(t, s, reviewer, work, "CONFIRM_DONE")
	wantPublic := workDetail(t, s, owner, work)
	tables := []string{"buildings", "flats", "residents", "flat_memberships", "users", "role_grants", "audit_events", "mfa_factors", "entries", "receipts", "record_operations", "maintenance_cycles", "maintenance_lines", "maintenance_events", "entry_allocations", "allocation_reversals", "upkeep_register", "upkeep_register_events", "upkeep_tasks", "upkeep_task_events"}
	snapshot := func() map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, table := range tables {
			rows, e := s.DB.Query("SELECT * FROM " + table + " ORDER BY rowid")
			if e != nil {
				t.Fatal(e)
			}
			columns, e := rows.Columns()
			if e != nil {
				rows.Close()
				t.Fatal(e)
			}
			values := []any{}
			for rows.Next() {
				row := make([]any, len(columns))
				pointers := make([]any, len(columns))
				for i := range row {
					pointers[i] = &row[i]
				}
				if e = rows.Scan(pointers...); e != nil {
					rows.Close()
					t.Fatal(e)
				}
				values = append(values, row)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				t.Fatal(e)
			}
			blob, e := json.Marshal(values)
			if e != nil {
				t.Fatal(e)
			}
			out[table] = string(blob)
		}
		return out
	}
	before := snapshot()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("accepted schema-ten rows changed")
	}
	if !reflect.DeepEqual(wantWork, workDetail(t, s, admin, work)) || !reflect.DeepEqual(wantPublic, workDetail(t, s, owner, work)) {
		t.Fatal("private or public upkeep changed")
	}
	got := statement(t, s, admin, "demo-flat-A-101")
	if got.DebitPaise != 100000 || got.CreditPaise != 40000 || got.AllocatedPaise != 30000 || got.OutstandingPaise != 70000 || got.UnallocatedPaise != 10000 || got.VoluntaryPaise != 0 {
		t.Fatal("legacy finance changed", got)
	}
	for version, want := range checks {
		var got string
		if err = s.DB.QueryRow("SELECT checksum FROM schema_migrations WHERE version=?", version).Scan(&got); err != nil || got != want {
			t.Fatal("migration provenance", version, err)
		}
	}
	for _, table := range []string{"fund_campaigns", "fund_reports", "fund_contributions", "fund_waivers"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
}
