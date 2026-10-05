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

func TestSchemaNineUpkeepUpgradePreservesMaintenanceReceiptAllocationAndAuthority(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "schema-nine.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	accessExec(t, s, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT")
	checks := map[int]string{}
	for version := 1; version <= 9; version++ {
		body, e := migrationBody(version)
		if e != nil {
			t.Fatal(e)
		}
		accessExec(t, s, string(body))
		sum := sha256.Sum256(body)
		checks[version] = hex.EncodeToString(sum[:])
		accessExec(t, s, "INSERT INTO schema_migrations VALUES(?,?,?)", version, checks[version], "2026-10-05")
	}
	if err = s.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	s.MFA, err = security.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	admin := reviewLogin(t, s, "admin@demo.society")
	reviewer := maintenanceReviewer(t, s, admin)
	cycle := publishMaintenance(t, s, admin, reviewer, maintenanceProposal())
	received := post(t, s, admin, received("400.00"))
	// Build the accepted legacy allocation using its schema-nine contract. The
	// current reader/writer require views introduced by later migrations.
	accessExec(t, s, "INSERT INTO entry_allocations(id,source_id,charge_id,amount_paise,reason,actor_id,created_at) VALUES(?,?,?,30000,?,?,?)", randomToken(), received, cycle.Lines[0].EntryID, "Legacy supplied same-home allocation", "demo-user-admin", 2)
	beforeEntry, err := s.EntryFor(ctx, admin, received)
	if err != nil {
		t.Fatal(err)
	}
	tables := []string{"buildings", "flats", "residents", "flat_memberships", "users", "role_grants", "audit_events", "mfa_factors", "entries", "receipts", "record_operations", "maintenance_cycles", "maintenance_lines", "maintenance_events", "entry_allocations", "allocation_reversals"}
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
				ptrs := make([]any, len(columns))
				for i := range row {
					ptrs[i] = &row[i]
				}
				if e = rows.Scan(ptrs...); e != nil {
					rows.Close()
					t.Fatal(e)
				}
				values = append(values, row)
			}
			if e = rows.Err(); e != nil {
				rows.Close()
				t.Fatal(e)
			}
			rows.Close()
			data, e := json.Marshal(values)
			if e != nil {
				t.Fatal(e)
			}
			out[table] = string(data)
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
		t.Fatal("accepted schema-nine records changed")
	}
	gotStatement := statement(t, s, admin, "demo-flat-A-101")
	if gotStatement.DebitPaise != 100000 || gotStatement.CreditPaise != 40000 || gotStatement.AllocatedPaise != 30000 || gotStatement.OutstandingPaise != 70000 || gotStatement.UnallocatedPaise != 10000 || gotStatement.VoluntaryPaise != 0 {
		t.Fatal("legacy statement changed", gotStatement)
	}
	entry, err := s.EntryFor(ctx, admin, received)
	if err != nil || !reflect.DeepEqual(entry, beforeEntry) {
		t.Fatal("receipt identity changed", entry, err)
	}
	for version, want := range checks {
		var got string
		if err = s.DB.QueryRow("SELECT checksum FROM schema_migrations WHERE version=?", version).Scan(&got); err != nil || got != want {
			t.Fatal(version, err)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM upkeep_tasks", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM upkeep_register", 0)
}
