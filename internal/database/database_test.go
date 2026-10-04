package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t testing.TB) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "society.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedDemo(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSettingsOnEveryPooledConnection(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	// Hold all four connections at once, rather than repeatedly borrowing one.
	for i := 0; i < 4; i++ {
		conn, err := s.DB.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		engine, err := inspectEngine(ctx, conn)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateEngine(engine); err != nil {
			t.Fatalf("connection %d: %v", i, err)
		}
	}
}

func TestFixedSQLiteRequirement(t *testing.T) {
	for _, version := range []string{"3.51.3", "3.53.4", "3.50.7", "3.44.6"} {
		if !fixedSQLite(version) {
			t.Errorf("fixed release rejected: %s", version)
		}
	}
	for _, version := range []string{"3.51.2", "3.50.6", "3.44.5", "3.49.9", "invalid", "3.51.-1"} {
		if fixedSQLite(version) {
			t.Errorf("unverified release accepted: %s", version)
		}
	}
}

func TestMigrationRepeatAndChecksumProtection(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.ExecContext(ctx, "UPDATE schema_migrations SET checksum = 'tampered' WHERE version = 1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err == nil {
		t.Fatal("changed migration provenance accepted")
	}
	if err := s.VerifySchema(ctx); err == nil {
		t.Fatal("changed snapshot provenance accepted")
	}
}

func TestSyntheticSeedAndRegistry(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	before, err := s.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before.Counts.Flats != 118 || before.Counts.Buildings != 3 {
		t.Fatalf("wrong canonical registry: %+v", before.Counts)
	}
	if before.Community.Owners != 118 || before.Community.Tenants != 35 || before.Community.Occupied != 109 || before.Community.Vacant != 9 || before.Community.OwnerOccupied != 74 || before.Community.Rented != 35 {
		t.Fatalf("distinct active people / occupancy mismatch: %+v", before.Community)
	}
	if before.Buildings[0].Owners != 40 || before.Buildings[0].Tenants != 12 {
		t.Fatalf("wing people counts: %+v", before.Buildings[0])
	}
	if len(before.Buildings) != 3 || before.Buildings[0].Flats != 40 || before.Buildings[2].Flats != 38 {
		t.Fatalf("wrong wing counts: %+v", before.Buildings)
	}
	if err := s.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := s.Summary(ctx)
	if err != nil || before.Counts != after.Counts {
		t.Fatalf("repeat seed changed records: %+v %v", after.Counts, err)
	}
	page, err := s.Flats(ctx, FlatFilter{Query: "B-101", Page: 1, PageSize: 12})
	if err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("home search: %+v %v", page, err)
	}
	multi, err := s.Flats(ctx, FlatFilter{Query: "Demo Owner A-101", Page: 1, PageSize: 12})
	if err != nil || multi.Total != 2 {
		t.Fatalf("multi-flat owner lookup: %+v %v", multi, err)
	}
	detail, err := s.Flat(ctx, "demo-flat-A-101")
	if err != nil || len(detail.Members) != 2 {
		t.Fatalf("joint-owner record missing: %+v %v", detail, err)
	}
	past, err := s.Flats(ctx, FlatFilter{Query: "Demo Former Tenant", Page: 1, PageSize: 12})
	if err != nil || past.Total != 0 {
		t.Fatalf("former member treated as current contact: %+v %v", past, err)
	}
	vacant, err := s.Flats(ctx, FlatFilter{Status: "VACANT", Page: 1, PageSize: 50})
	if err != nil || vacant.Total != 9 {
		t.Fatalf("wrong vacant fixture count: %+v %v", vacant, err)
	}
}

func TestForeignKeysAndUniqueness(t *testing.T) {
	s := fixture(t)
	for _, query := range []string{
		"INSERT INTO flats(id, building_id, flat_number, floor, status) VALUES ('bad', 'missing-building', '999', 9, 'VACANT')",
		"INSERT INTO flats(id, building_id, flat_number, floor, status) VALUES ('duplicate', 'demo-building-A', '101', 1, 'VACANT')",
		"INSERT INTO flat_memberships VALUES ('bad-member', 'missing-flat', 'demo-owner-A-101', 'OWNER', '2020-01-01', NULL, 0, 0)",
	} {
		if _, err := s.DB.Exec(query); err == nil {
			t.Fatalf("invalid registry insert accepted: %s", query)
		}
	}
}

func TestActiveCountsRespectMembershipDateBoundaries(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	date := today()
	tomorrow := time.Now().In(societyZone).AddDate(0, 0, 1).Format("2006-01-02")
	for _, item := range []struct {
		start   string
		end     any
		tenants int
	}{
		{"2099-01-01", nil, 34},
		{"2020-01-01", date, 34},
		{date, tomorrow, 35},
	} {
		if _, err := s.DB.ExecContext(ctx, "UPDATE flat_memberships SET start_date = ?, end_date = ? WHERE resident_id = 'demo-tenant-A-103'", item.start, item.end); err != nil {
			t.Fatal(err)
		}
		summary, err := s.Summary(ctx)
		if err != nil || summary.Community.Tenants != item.tenants || summary.Community.Rented != 35 {
			t.Fatalf("date boundaries / homes-versus-people: %+v %v", summary.Community, err)
		}
		page, err := s.Flats(ctx, FlatFilter{Query: "Demo Tenant A-103", Page: 1, PageSize: 12})
		want := 0
		if item.tenants == 35 {
			want = 1
		}
		if err != nil || page.Total != want {
			t.Fatalf("active search date boundary: %+v %v", page, err)
		}
	}
}

func TestSeedRefusesUnmarkedData(t *testing.T) {
	s := fixture(t)
	if _, err := s.DB.Exec("DELETE FROM app_metadata"); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedDemo(context.Background()); err == nil {
		t.Fatal("fixtures mixed into unmarked data")
	}
	if err := s.RequireDemo(context.Background()); err == nil {
		t.Fatal("unmarked database served as a preview")
	}
}

func BenchmarkRegistryRead(b *testing.B) {
	s := fixture(b)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		page, err := s.Flats(ctx, FlatFilter{Page: 1, PageSize: 12})
		if err != nil || len(page.Items) != 12 {
			b.Fatalf("registry read failed: %v", err)
		}
	}
}
