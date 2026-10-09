package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func suppliedRegister(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile("../../testdata/registry-import-118.json")
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
func applySupplied(body string, p RegistryImportPreview, key string) ApplyRegistryImport {
	return ApplyRegistryImport{InputText: body, Digest: p.Digest, BaseDigest: p.BaseDigest, EffectiveDate: p.EffectiveDate, OperationKey: key, Confirmed: true, Note: "Verified supplied fictional register against its source"}
}

func TestRegistryImportReadOnlyPreviewIndependentTotalsExactApplyRetryAndProvenance(t *testing.T) {
	s, token := workspaceFixture(t)
	ctx := context.Background()
	body := suppliedRegister(t)
	before, err := CountRecords(ctx, s.DB)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewRegistryImport(ctx, token, body)
	if err != nil || !p.CanApply || p.ErrorCount != 0 {
		t.Fatal("valid preview", p.Errors, err)
	}
	want := ImportCounts{Buildings: 3, Homes: 118, People: 154, Relationships: 155, Occupied: 109, Vacant: 9, Owners: 118, Tenants: 35}
	if p.Counts != want {
		t.Fatal("independent supplied totals", p.Counts)
	}
	after, _ := CountRecords(ctx, s.DB)
	if before != after {
		t.Fatal("preview mutated registry")
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM registry_imports", 0)
	input := applySupplied(body, p, "reviewed-import-2026")
	r, err := s.ApplyRegistryImport(ctx, token, input)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := s.SummaryFor(ctx, token)
	if err != nil || summary.Counts != (Counts{3, 118, 154, 155}) || summary.Community != (CommunityStats{Owners: 118, Tenants: 35, Occupied: 109, Vacant: 9, OwnerOccupied: 74, Rented: 35}) {
		t.Fatal("imported counts", summary, err)
	}
	if summary.Buildings[0].Flats != 40 || summary.Buildings[1].Flats != 40 || summary.Buildings[2].Flats != 38 || summary.Buildings[0].Tenants != 12 || summary.Buildings[2].Tenants != 11 {
		t.Fatal("independent wing totals", summary.Buildings)
	}
	for _, key := range []string{input.OperationKey, "different-retry-key"} {
		input.OperationKey = key
		again, err := s.ApplyRegistryImport(ctx, token, input)
		if err != nil || !reflect.DeepEqual(r, again) {
			t.Fatal("retry duplicated or changed outcome", again, err)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM registry_imports", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM registry_import_entities", 430)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='REGISTRY_IMPORTED'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM flat_memberships WHERE can_view_finances=1", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM users", 1)
	changedNote := applySupplied(body, p, "reviewed-import-2026")
	changedNote.Note = "Changed verification details under an already accepted operation"
	if _, err = s.ApplyRegistryImport(ctx, token, changedNote); !errors.Is(err, ErrConflict) {
		t.Fatal("changed operation verification note accepted", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM registry_import_entities WHERE entity_kind='PERSON' AND source_id='joint-owner-a-101'", 1)
	for _, query := range []string{"UPDATE registry_imports SET source_key='changed'", "DELETE FROM registry_imports", "UPDATE registry_import_entities SET source_id='changed'", "DELETE FROM registry_import_entities"} {
		if _, err = s.DB.Exec(query); err == nil {
			t.Fatal("immutable provenance changed")
		}
	}
	// Same source re-preview preserves the original snapshot even after reviewed changes.
	flat := importedID("verified-register-october-2026", "HOME", "a-101")
	if err = s.ChangeOccupancy(ctx, token, flat, OccupancyChange{RegistryChange: RegistryChange{Version: 1, Reason: "Fictional reviewed occupancy correction"}, Status: "VACANT"}); err != nil {
		t.Fatal(err)
	}
	read, err := s.PreviewRegistryImport(ctx, token, body)
	if err != nil || read.AlreadyApplied == nil || read.AlreadyApplied.ID != r.ID || read.CanApply {
		t.Fatal("same source replay did not preserve result", err)
	}
	input = applySupplied(body+"\n", p, "reviewed-import-2026")
	input.Digest = InputDigest([]byte(input.InputText))
	if _, err = s.ApplyRegistryImport(ctx, token, input); !errors.Is(err, ErrConflict) {
		t.Fatal("changed operation input accepted", err)
	}
}

func TestRegistryImportInvalidRowsRollbackAndBoundedActionableErrors(t *testing.T) {
	s, token := workspaceFixture(t)
	ctx := context.Background()
	body := suppliedRegister(t)
	var good RegistryImportInput
	if err := json.Unmarshal([]byte(body), &good); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, section, field string
		change               func(*RegistryImportInput)
	}{
		{"missing-building", "homes", "building", func(x *RegistryImportInput) { x.Homes[0].Building = "missing" }},
		{"repeated-person", "people", "source_id", func(x *RegistryImportInput) { x.People[1].SourceID = x.People[0].SourceID }},
		{"repeated-home-number", "homes", "number", func(x *RegistryImportInput) { x.Homes[1].Number = x.Homes[0].Number }},
		{"invalid-date", "relationships", "start_date", func(x *RegistryImportInput) { x.Relationships[0].StartDate = "2025-02-30" }},
		{"future-date", "relationships", "start_date", func(x *RegistryImportInput) { x.Relationships[0].StartDate = "2099-01-01" }},
		{"vacant-tenant", "homes", "occupancy", func(x *RegistryImportInput) { x.Homes[2].Occupancy = "VACANT" }},
		{"wrong-society", "file", "society_key", func(x *RegistryImportInput) { x.SocietyKey = "other-society" }},
		{"missing-person", "relationships", "person", func(x *RegistryImportInput) { x.Relationships[0].Person = "missing" }},
		{"overlapping-role", "relationships", "start_date", func(x *RegistryImportInput) {
			r := x.Relationships[0]
			r.SourceID = "overlap"
			r.StartDate = "2021-01-01"
			x.Relationships = append(x.Relationships, r)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var input RegistryImportInput
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				t.Fatal(err)
			}
			tc.change(&input)
			raw, _ := json.Marshal(input)
			p, err := s.PreviewRegistryImport(ctx, token, string(raw))
			if err != nil || p.CanApply || p.ErrorCount == 0 {
				t.Fatal("invalid preview accepted", err)
			}
			found := false
			for _, issue := range p.Errors {
				if issue.Section == tc.section && issue.Field == tc.field {
					found = true
					if issue.Section != "file" && issue.Row < 1 {
						t.Fatal("missing row number")
					}
				}
			}
			if !found {
				t.Fatal("missing actionable field", p.Errors)
			}
			if _, err = s.ApplyRegistryImport(ctx, token, applySupplied(string(raw), p, "invalid-"+tc.name)); err == nil {
				t.Fatal("invalid import applied")
			}
			counts, _ := CountRecords(ctx, s.DB)
			if counts != (Counts{}) {
				t.Fatal("partial invalid mutation", counts)
			}
		})
	}
	for _, raw := range []string{"{", body + " {}", strings.Repeat("x", RegistryImportMaxBytes+1), strings.Replace(body, `"format_version": 1`, `"format_version": 1, "account_password": "do-not-echo"`, 1)} {
		p, err := s.PreviewRegistryImport(ctx, token, raw)
		if err != nil || p.CanApply || p.ErrorCount == 0 || strings.Contains(p.Errors[0].Message, "do-not-echo") {
			t.Fatal("malformed/private-field boundary", err)
		}
	}
	for _, raw := range []string{
		strings.Replace(body, `"format_version": 1`, `"format_version": 1, "format_version": 2`, 1),
		strings.Replace(body, `"floor": 1,`, "", 1),
		strings.Replace(body, `"floor": 1`, `"floor": null`, 1),
		strings.Replace(body, `"primary_contact": true`, `"primary_contact": null`, 1),
		string([]byte{0xff, 0xfe}),
	} {
		p, err := s.PreviewRegistryImport(ctx, token, raw)
		if err != nil || p.CanApply || p.ErrorCount == 0 {
			t.Fatal("ambiguous or unsupplied fields accepted", err)
		}
	}
	var many RegistryImportInput
	json.Unmarshal([]byte(body), &many)
	for i := range many.Relationships {
		many.Relationships[i].StartDate = "bad"
	}
	raw, _ := json.Marshal(many)
	p, err := s.PreviewRegistryImport(ctx, token, string(raw))
	if err != nil || len(p.Errors) != 50 || p.ErrorCount < 155 {
		t.Fatal("error response not bounded", p.ErrorCount, len(p.Errors), err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM registry_imports", 0)
}

func TestRegistryImportStaleConcurrentPermissionAndAtomicFailure(t *testing.T) {
	s, token := workspaceFixture(t)
	ctx := context.Background()
	body := suppliedRegister(t)
	p, err := s.PreviewRegistryImport(ctx, token, body)
	if err != nil {
		t.Fatal(err)
	}
	// Force a late constraint failure after all home rows would have been inserted.
	accessExec(t, s, `CREATE TRIGGER synthetic_import_failure BEFORE INSERT ON flat_memberships BEGIN SELECT RAISE(ABORT,'synthetic late failure'); END`)
	if _, err = s.ApplyRegistryImport(ctx, token, applySupplied(body, p, "late-failure")); err == nil {
		t.Fatal("forced late failure accepted")
	}
	counts, _ := CountRecords(ctx, s.DB)
	if counts != (Counts{}) {
		t.Fatal("partial late-failure rows", counts)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM registry_imports", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM registry_import_entities", 0)
	accessExec(t, s, "DROP TRIGGER synthetic_import_failure")
	accessExec(t, s, "INSERT INTO buildings VALUES('concurrent','Z','Concurrent reviewed building')")
	if _, err = s.ApplyRegistryImport(ctx, token, applySupplied(body, p, "stale-preview")); !errors.Is(err, ErrConflict) {
		t.Fatal("stale registry accepted", err)
	}
	accessExec(t, s, "DELETE FROM buildings WHERE id='concurrent'")
	// Freshness and authority are checked even for accepted replay identities.
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=1 WHERE token_hash=?", TokenHash(token))
	if _, err = s.ApplyRegistryImport(ctx, token, applySupplied(body, p, "stale-identity")); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("stale auth accepted", err)
	}
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=strftime('%s','now') WHERE token_hash=?", TokenHash(token))
	var wg sync.WaitGroup
	out := make(chan RegistryImportResult, 2)
	fail := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.ApplyRegistryImport(ctx, token, applySupplied(body, p, "concurrent-one"))
			if e != nil {
				fail <- e
			} else {
				out <- r
			}
		}()
	}
	wg.Wait()
	close(fail)
	for e := range fail {
		t.Fatal("concurrent exact retry", e)
	}
	close(out)
	var id string
	for r := range out {
		if id != "" && id != r.ID {
			t.Fatal("duplicate concurrent result")
		}
		id = r.ID
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM registry_imports", 1)
	accessExec(t, s, "UPDATE role_grants SET revoked_at=strftime('%s','now') WHERE role='ADMINISTRATOR'")
	if _, err = s.ApplyRegistryImport(ctx, token, applySupplied(body, p, "concurrent-one")); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked replay exposed result", err)
	}
	if _, err = s.RegistryImportStatus(ctx, token); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked read exposed status", err)
	}
}
