package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSchemaSixteenStatementUpgradePreservesEveryPriorTableAndMigrationChecksum(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "schema-fifteen.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	accessExec(t, s, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT")
	for v := 1; v <= 15; v++ {
		body, e := migrationBody(v)
		if e != nil {
			t.Fatal(e)
		}
		accessExec(t, s, string(body))
		sha := sha256.Sum256(body)
		accessExec(t, s, "INSERT INTO schema_migrations VALUES(?,?,?)", v, hex.EncodeToString(sha[:]), "2026-10-06")
	}
	for _, seed := range []func(context.Context) error{s.SeedDemo, s.SeedDemoAccounts, s.SeedDemoTreasury} {
		if err = seed(ctx); err != nil {
			t.Fatal(err)
		}
	}
	accessExec(t, s, `INSERT INTO entries VALUES('statement-existing-charge','demo-flat-A-101','CHARGE',43219,'2026-01-01','Retained prior supplied charge','','','','Prior fictional source','POSTED','demo-user-admin',1,'demo-user-admin',2)`)
	tables := []string{}
	rows, err := s.DB.Query(`SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	snapshot := func(table string) string {
		t.Helper()
		query := `SELECT * FROM "` + table + `"`
		if table == "schema_migrations" {
			query += " WHERE version<=15"
		}
		query += " ORDER BY rowid"
		r, e := s.DB.Query(query)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Close()
		cols, e := r.Columns()
		if e != nil {
			t.Fatal(e)
		}
		result := [][]any{}
		for r.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if e = r.Scan(ptrs...); e != nil {
				t.Fatal(e)
			}
			result = append(result, values)
		}
		if e = r.Err(); e != nil {
			t.Fatal(e)
		}
		blob, e := json.Marshal(result)
		if e != nil {
			t.Fatal(e)
		}
		return string(blob)
	}
	before := map[string]string{}
	for _, table := range tables {
		before[table] = snapshot(table)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		if before[table] != snapshot(table) {
			t.Fatal("prior table changed during statement migration", table)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM schema_migrations", 16)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name LIKE 'statement_%'", 7)
	var integrity string
	if err = s.DB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal(integrity, err)
	}
	violations, err := s.DB.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer violations.Close()
	if violations.Next() {
		t.Fatal("foreign key violation after migration")
	}
	if err = violations.Err(); err != nil {
		t.Fatal(err)
	}
}
