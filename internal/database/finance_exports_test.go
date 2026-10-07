package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func exportFilter(report, scope string) FinanceExportFilter {
	return FinanceExportFilter{Report: report, Scope: scope, From: "2026-01-01", To: "2026-01-31"}
}
func exportPreview(t *testing.T, s *Store, token string, in FinanceExportFilter) FinanceExport {
	t.Helper()
	out, err := s.PreviewFinanceExport(context.Background(), token, in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func exportInput(out FinanceExport) FinanceExportInput {
	return FinanceExportInput{FinanceExportFilter: out.FinanceExportFilter, OperationKey: randomToken(), PreviewHash: out.ContentHash, Confirmed: true}
}
func exportCreate(t *testing.T, s *Store, token string, in FinanceExportInput) FinanceExport {
	t.Helper()
	out, err := s.CreateFinanceExport(context.Background(), token, in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func exportCSV(t *testing.T, s *Store, token, id string) (FinanceExport, []map[string]string, []byte) {
	t.Helper()
	out, body, err := s.DownloadFinanceExport(context.Background(), token, id)
	if err != nil {
		t.Fatal(err)
	}
	all, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	if out.SHA256 != hex.EncodeToString(digest[:]) || out.Bytes != len(body) || out.Rows != len(all)-1 {
		t.Fatal("independent file dimensions/checksum", out, len(body), len(all))
	}
	rows := []map[string]string{}
	for _, values := range all[1:] {
		row := map[string]string{}
		for i, key := range all[0] {
			row[key] = values[i]
		}
		rows = append(rows, row)
	}
	return out, rows, body
}
func exportExactFixture(t *testing.T) (*Store, string, string, map[string]string) {
	t.Helper()
	s, a := recordFixture(t)
	b := maintenanceReviewer(t, s, a)
	ctx := context.Background()
	ids := map[string]string{}
	m := maintenanceProposal()
	m.Lines = []MaintenanceLineInput{{"demo-flat-A-101", "200.00"}}
	maintenance := publishMaintenance(t, s, a, b, m)
	ids["maintenance"] = maintenance.ID
	ids["maintenance-charge"] = maintenance.Lines[0].EntryID
	f := fundProposal()
	f.Lines = []MaintenanceLineInput{{"demo-flat-A-101", "432.19"}}
	fund := createPublishedFund(t, s, a, b, f)
	ids["fund"] = fund.ID
	ids["fund-charge"] = fund.Lines[0].EntryID
	ids["manual"] = post(t, s, a, supplied("CHARGE", "0.01"))
	ids["opening"] = post(t, s, a, supplied("OPENING_CREDIT", "100.00"))
	ids["received"] = post(t, s, a, received("500.00"))
	ids["reversed"] = post(t, s, a, received("1.01"))
	if _, err := s.ReverseEntry(ctx, a, ids["reversed"], EntryAction{randomToken(), true, "Wrong fictional reference retained through linked correction"}); err != nil {
		t.Fatal(err)
	}
	ids["maintenance-opening"] = allocate(t, s, a, ids["opening"], ids["maintenance-charge"], "100.00")
	ids["maintenance-cash"] = allocate(t, s, a, ids["received"], ids["maintenance-charge"], "100.00")
	ids["fund-old"] = allocate(t, s, a, ids["received"], ids["fund-charge"], "100.00")
	if _, err := s.ReverseAllocation(ctx, a, ids["fund-old"], AllocationCorrection{OperationKey: randomToken(), Reason: "Correct this fictional purpose assignment", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	ids["fund-new"] = allocate(t, s, a, ids["received"], ids["fund-charge"], "60.00")
	owner := reviewLogin(t, s, "owner@demo.society")
	ids["pending"] = createFundClaim(t, s, owner, fundClaim(fund.ID, "0.01", "PRIVATE-PENDING-EXPORT"))
	return s, a, owner, ids
}

func TestFinanceExportsIndependentExactSourcesOriginalsLinksAndSnapshotAudit(t *testing.T) {
	s, a, owner, ids := exportExactFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		report        string
		rows, sources int
		outstanding   int64
	}{{"LEDGER", 13, 6, 37220}, {"RECEIPTS", 4, 2, 0}, {"MAINTENANCE", 4, 1, 0}, {"FUNDS", 5, 1, 37219}} {
		preview := exportPreview(t, s, owner, exportFilter(tc.report, "OWN"))
		if preview.ID != "" || preview.Rows != tc.rows || preview.Summary.Sources != tc.sources || preview.Summary.CurrentNet != 3220 || preview.Summary.CurrentReceived != 50000 || preview.Summary.AvailableReceived != 34000 || preview.Summary.OpeningCredit != 10000 || preview.Summary.Outstanding != tc.outstanding {
			t.Fatalf("%s independent snapshot: %+v", tc.report, preview)
		}
		if tc.report == "LEDGER" || tc.report == "RECEIPTS" {
			if preview.Summary.OriginalReceived != 50101 || preview.Summary.ReversedReceived != 101 || preview.Summary.UsableReceived != 50000 {
				t.Fatal("original cash is distinct from usable cash", preview.Summary)
			}
		}
		in := exportInput(preview)
		created := exportCreate(t, s, owner, in)
		out, rows, body := exportCSV(t, s, owner, created.ID)
		if out.ContentHash != preview.ContentHash || out.Authority != "PERSONAL" || len(out.Homes) != 2 || out.Rows != tc.rows || len(body) == 0 {
			t.Fatal("snapshot metadata", out)
		}
		if rows[0]["row_type"] != "SCOPE" || rows[0]["current_net_rupees"] != "32.20" || rows[0]["available_received_rupees"] != "340.00" {
			t.Fatal("scope independent amounts", rows[0])
		}
		for _, secret := range []string{"PRIVATE", "Wrong fictional reference", "Correct this fictional purpose", "Separate", "demo-user-admin", ids["pending"]} {
			if strings.Contains(string(body), secret) {
				t.Fatal("private explanation leaked", secret)
			}
		}
		if tc.report == "RECEIPTS" {
			originals := map[string]string{}
			reversal := false
			for _, row := range rows {
				if row["row_type"] == "RECEIPT" {
					originals[row["entry_id"]] = row["original_rupees"]
					if row["receipt_number"] == "" {
						t.Fatal("receipt identity absent")
					}
				}
				if row["row_type"] == "ENTRY_REVERSAL" && row["entry_id"] == ids["reversed"] && row["signed_rupees"] == "1.01" {
					reversal = true
				}
			}
			if originals[ids["received"]] != "500.00" || originals[ids["reversed"]] != "1.01" || !reversal {
				t.Fatal("receipt originals rewritten", rows)
			}
		}
		if tc.report == "FUNDS" {
			var old, new, correction bool
			for _, row := range rows {
				if row["row_type"] == "ALLOCATION" && row["linked_id"] == ids["fund-old"] && row["original_rupees"] == "100.00" && row["active_rupees"] == "0.00" {
					old = true
				}
				if row["row_type"] == "ALLOCATION" && row["linked_id"] == ids["fund-new"] && row["active_rupees"] == "60.00" {
					new = true
				}
				if row["row_type"] == "ALLOCATION_REVERSAL" && row["linked_id"] == ids["fund-old"] && row["signed_rupees"] == "-100.00" {
					correction = true
				}
			}
			if !old || !new || !correction || out.Summary.PendingReports != 1 {
				t.Fatal("allocation originals/correction", rows, out.Summary)
			}
		}
		again := exportCreate(t, s, owner, in)
		if !reflect.DeepEqual(again, created) {
			t.Fatal("lost response did not return original snapshot")
		}
		for _, query := range []string{"UPDATE finance_exports SET scope_label='different' WHERE id=?", "DELETE FROM finance_exports WHERE id=?"} {
			if _, err := s.DB.Exec(query, created.ID); err == nil {
				t.Fatal("immutable snapshot bypass", query)
			}
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM finance_exports", 4)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='FINANCE_EXPORTED'", 4)
	var audit string
	if err := s.DB.QueryRow("SELECT group_concat(after_json) FROM audit_events WHERE action='FINANCE_EXPORTED'").Scan(&audit); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(audit, "Demo Owner") || strings.Contains(audit, "DEMO-REF") || strings.Contains(audit, "50000") || strings.Contains(audit, "csv_bytes") {
		t.Fatal("broad audit contains private record details", audit)
	}
	original, err := s.EntryFor(ctx, a, ids["reversed"])
	if err != nil || original.AmountPaise != 101 || original.ReceiptID == "" {
		t.Fatal("export changed the original", original, err)
	}
}

func TestFinanceExportPreviewStaleAcceptedReplayAndConcurrentIdentity(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	post(t, s, a, received("432.19"))
	preview := exportPreview(t, s, a, exportFilter("RECEIPTS", "SOCIETY"))
	in := exportInput(preview)
	post(t, s, a, received("0.01"))
	if _, err := s.CreateFinanceExport(ctx, a, in); !errors.Is(err, ErrConflict) {
		t.Fatal("stale preview created a snapshot", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM finance_exports", 0)
	in = exportInput(exportPreview(t, s, a, in.FinanceExportFilter))
	var wg sync.WaitGroup
	ids := make(chan string, 4)
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); x, err := s.CreateFinanceExport(ctx, a, in); ids <- x.ID; errs <- err }()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	expected := ""
	for id := range ids {
		if expected == "" {
			expected = id
		}
		if id != expected {
			t.Fatal("duplicate snapshot", id, expected)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM finance_exports", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='FINANCE_EXPORTED'", 1)
	before, _, body := exportCSV(t, s, a, expected)
	post(t, s, a, received("0.10"))
	after := exportCreate(t, s, a, in)
	_, _, replayed := exportCSV(t, s, a, after.ID)
	if !reflect.DeepEqual(before, after) || !bytes.Equal(body, replayed) || after.Summary.OriginalReceived != 43220 {
		t.Fatal("accepted snapshot changed on retry", before, after)
	}
	changed := in
	changed.To = "2026-01-30"
	if _, err := s.CreateFinanceExport(ctx, a, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed operation replay", err)
	}
}

func TestFinanceExportCurrentAuthorityAuditorTenantGuessesAndFreshVerification(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	committee := reviewLogin(t, s, "committee@demo.society")
	for _, token := range []string{tenant, committee} {
		if _, err := s.FinanceExportChoicesFor(ctx, token); !errors.Is(err, ErrForbidden) {
			t.Fatal("community/tenant exported", err)
		}
	}
	guess := exportFilter("LEDGER", "HOME")
	guess.HomeID = "demo-flat-B-101"
	if _, err := s.PreviewFinanceExport(ctx, owner, guess); !errors.Is(err, ErrForbidden) {
		t.Fatal("cross-home preview", err)
	}
	if _, err := s.PreviewFinanceExport(ctx, owner, exportFilter("LEDGER", "SOCIETY")); !errors.Is(err, ErrForbidden) {
		t.Fatal("resident society export", err)
	}
	grantAppointment(t, s, a, "demo-user-tenant", "AUDITOR", 30)
	auditor := reviewLogin(t, s, "tenant@demo.society")
	p, err := s.CheckSession(ctx, auditor)
	if err != nil || !p.CanReadAllRecords || p.CanExportFinance {
		t.Fatal("audit reading widened exports", p, err)
	}
	if _, err = s.FinanceExportChoicesFor(ctx, auditor); !errors.Is(err, ErrForbidden) {
		t.Fatal("auditor export", err)
	}
	post(t, s, a, received("1.01"))
	x := exportCreate(t, s, a, exportInput(exportPreview(t, s, a, exportFilter("LEDGER", "SOCIETY"))))
	if _, _, err = s.DownloadFinanceExport(ctx, owner, x.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("other actor snapshot", err)
	}
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=?,mfa_verified_at=? WHERE token_hash=?", time.Now().Add(-6*time.Minute).Unix(), time.Now().Add(-6*time.Minute).Unix(), TokenHash(a))
	for _, run := range []func() error{func() error { _, err := s.PreviewFinanceExport(ctx, a, x.FinanceExportFilter); return err }, func() error { _, _, err := s.DownloadFinanceExport(ctx, a, x.ID); return err }} {
		if err = run(); !errors.Is(err, ErrReauthRequired) {
			t.Fatal("stale privileged verification", err)
		}
	}
}

func TestFinanceExportFrozenMultiHomeMembershipReplayAndTreasuryAuthorityCannotConvert(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	p, _ := s.CheckSession(ctx, owner)
	preview := exportPreview(t, s, owner, exportFilter("LEDGER", "OWN"))
	if len(preview.Homes) != 2 {
		t.Fatal("joint scope", preview.Homes)
	}
	in := exportInput(preview)
	out := exportCreate(t, s, owner, in)
	accessExec(t, s, `INSERT INTO flat_memberships VALUES('export-extra','demo-flat-A-103',?,'OWNER','2020-01-01',NULL,0,1)`, p.ResidentID)
	again := exportCreate(t, s, owner, in)
	if !reflect.DeepEqual(out, again) {
		t.Fatal("new home changed accepted scope")
	}
	if next := exportPreview(t, s, owner, in.FinanceExportFilter); len(next.Homes) != 3 || next.ContentHash == preview.ContentHash {
		t.Fatal("fresh scope omitted additional home", next)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE resident_id=? AND flat_id='demo-flat-A-102' AND end_date IS NULL", today(), p.ResidentID)
	for _, run := range []func() error{func() error { _, err := s.CreateFinanceExport(ctx, owner, in); return err }, func() error { _, err := s.FinanceExportFor(ctx, owner, out.ID); return err }, func() error { _, _, err := s.DownloadFinanceExport(ctx, owner, out.ID); return err }} {
		if err := run(); !errors.Is(err, ErrForbidden) {
			t.Fatal("partially lost frozen scope replay/download", err)
		}
	}
	page, err := s.FinanceExportsFor(ctx, owner, 1)
	if err != nil || page.Total != 0 {
		t.Fatal("inaccessible snapshot list leak", page, err)
	}
	grantAppointment(t, s, a, "demo-user-owner", "TREASURER", 30)
	owner = reviewLogin(t, s, "owner@demo.society")
	home := exportFilter("LEDGER", "HOME")
	home.HomeID = "demo-flat-A-101"
	privileged := exportInput(exportPreview(t, s, owner, home))
	x := exportCreate(t, s, owner, privileged)
	accessExec(t, s, "UPDATE role_grants SET revoked_at=? WHERE user_id='demo-user-owner' AND role='TREASURER'", time.Now().Unix())
	if _, err = s.CreateFinanceExport(ctx, owner, privileged); !errors.Is(err, ErrForbidden) {
		t.Fatal("Treasury snapshot converted to personal replay", err)
	}
	if _, _, err = s.DownloadFinanceExport(ctx, owner, x.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("Treasury snapshot converted to personal download", err)
	}
	if own := exportPreview(t, s, owner, exportFilter("LEDGER", "OWN")); own.Authority != "PERSONAL" {
		t.Fatal("new personal export unavailable", own)
	}
}

func TestFinanceExportUnicodeCSVFormulaTextAndControlledNumericNegatives(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	in := received("432.19")
	in.Description = "＝SUM(1,2) · नमस्ते"
	in.Payer = "＋Fresh Unicode resident"
	in.Reference = "@fictional-reference"
	id, err := s.CreateEntry(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	original := "\t\r\n =HYPERLINK(\"fictional\",\"text\")\nनमस्ते, society 🌿"
	accessExec(t, s, "UPDATE entries SET description=? WHERE id=?", original, id)
	if _, err = s.PostEntry(ctx, a, id, EntryAction{OperationKey: randomToken(), Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	x := exportCreate(t, s, a, exportInput(exportPreview(t, s, a, exportFilter("LEDGER", "SOCIETY"))))
	_, rows, _ := exportCSV(t, s, a, x.ID)
	if rows[0]["current_net_rupees"] != "-432.19" || rows[1]["signed_rupees"] != "-432.19" || rows[1]["description"] != "[text] "+strings.ReplaceAll(original, "\r\n", "\n") || rows[1]["payer"] != "[text] "+in.Payer || rows[1]["reference"] != "[text] "+in.Reference {
		t.Fatal("formula guard or exact Unicode round trip", rows)
	}
	var stored string
	if err = s.DB.QueryRow("SELECT description FROM entries WHERE id=?", id).Scan(&stored); err != nil || stored != original {
		t.Fatal("formula guard modified original", stored, err)
	}
	for _, raw := range []string{"=1", "+1", "-1", "@x", "\uFEFF \t＝1", "\u200B＠x", "＋2", "－2"} {
		if got := safeFinanceCell(raw); got != "[text] "+raw {
			t.Fatal("unguarded text", raw, got)
		}
	}
	for _, raw := range []string{"नमस्ते 🌿", "ordinary, \"quoted\"\nsecond line", "owner@example.org", "2026-01-01", "[text] =1"} {
		if got := safeFinanceCell(raw); got != raw {
			t.Fatal("ordinary Unicode was altered", raw, got)
		}
	}
}

func TestFinanceExportEmptyRangeMalformedInputsAndRowByteStorageLimitsRollback(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	post(t, s, a, received("0.01"))
	empty := exportFilter("LEDGER", "SOCIETY")
	empty.From = "2026-02-01"
	empty.To = "2026-02-28"
	preview := exportPreview(t, s, a, empty)
	if preview.Rows != 1 || preview.Summary.Sources != 0 || preview.Summary.CurrentNet != -1 {
		t.Fatal("empty source range lost current scope", preview)
	}
	x := exportCreate(t, s, a, exportInput(preview))
	_, rows, _ := exportCSV(t, s, a, x.ID)
	if len(rows) != 1 || rows[0]["row_type"] != "SCOPE" {
		t.Fatal("empty CSV lacks scope", rows)
	}
	invalids := []FinanceExportFilter{exportFilter("ALL", "SOCIETY"), exportFilter("LEDGER", "AUDITOR")}
	bad := empty
	bad.From = "2026-02-30"
	invalids = append(invalids, bad)
	bad = empty
	bad.To = "2026-01-01"
	invalids = append(invalids, bad)
	bad = empty
	bad.FundID = "some-fund"
	invalids = append(invalids, bad)
	bad = empty
	bad.Scope = "HOME"
	invalids = append(invalids, bad)
	for _, in := range invalids {
		if _, err := s.PreviewFinanceExport(ctx, a, in); !errors.Is(err, ErrInvalid) {
			t.Fatal("malformed export", in, err)
		}
	}
	s.exportLimits = &financeExportLimits{Rows: 1}
	if _, err := s.PreviewFinanceExport(ctx, a, exportFilter("LEDGER", "SOCIETY")); !errors.Is(err, ErrInvalid) {
		t.Fatal("row limit silently truncated", err)
	}
	s.exportLimits = &financeExportLimits{Bytes: 500}
	if _, err := s.PreviewFinanceExport(ctx, a, empty); !errors.Is(err, ErrInvalid) {
		t.Fatal("byte limit", err)
	}
	s.exportLimits = nil
	before := exportInput(exportPreview(t, s, a, empty))
	maintenanceCount(t, s, "SELECT COUNT(*) FROM finance_exports", 1)
	for _, limit := range []*financeExportLimits{{ActorBytes: int64(x.Bytes)}, {SocietyBytes: int64(x.Bytes)}} {
		s.exportLimits = limit
		if _, err := s.CreateFinanceExport(ctx, a, before); !errors.Is(err, ErrInvalid) {
			t.Fatal("stored byte quota", err)
		}
		maintenanceCount(t, s, "SELECT COUNT(*) FROM finance_exports", 1)
		maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='FINANCE_EXPORTED'", 1)
	}
	s.exportLimits = nil
	if _, err := s.CreateFinanceExport(ctx, a, FinanceExportInput{FinanceExportFilter: empty, OperationKey: randomToken(), PreviewHash: strings.Repeat("0", 64)}); !errors.Is(err, ErrInvalid) {
		t.Fatal("unconfirmed snapshot", err)
	}
	jsonBody, _ := json.Marshal(preview)
	if !bytes.Contains(jsonBody, []byte(`"current_net_paise":"-1"`)) {
		t.Fatal("paise became a floating JSON number", string(jsonBody))
	}
}

func TestFinanceExportFundReplacementHistoryVoluntaryCorrectionAndPrivateAuthorCounts(t *testing.T) {
	s, a, owner, ids := exportExactFixture(t)
	ctx := context.Background()
	b := reviewLogin(t, s, "committee@demo.society")
	// A second person's private pending report shares the home but must not be
	// visible in the resident's report count, including an empty-text CSV.
	createFundClaim(t, s, b, fundClaim(ids["fund"], "0.01", "PRIVATE-OTHER-AUTHOR"))
	own := exportPreview(t, s, owner, exportFilter("FUNDS", "OWN"))
	all := exportPreview(t, s, a, exportFilter("FUNDS", "SOCIETY"))
	if own.Summary.PendingReports != 1 || all.Summary.PendingReports != 2 {
		t.Fatal("pending count leaked another author", own.Summary, all.Summary)
	}
	if _, err := s.ReverseAllocation(ctx, a, ids["fund-new"], AllocationCorrection{randomToken(), "Release current allocation before an independently reviewed exemption", true}); err != nil {
		t.Fatal(err)
	}
	replacements := []string{}
	for _, amount := range []string{"100.00", "50.00"} {
		x := fundDetails(t, s, a, ids["fund"])
		w, err := s.ProposeFundWaiver(ctx, a, FundWaiverInput{randomToken(), x.ID, "demo-flat-A-101", x.Lines[0].Version, amount, "PRIVATE original exemption source", "PRIVATE deliberately requested a supplied exemption", true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.DecideFundWaiver(ctx, b, w, fundDecision("APPROVED", 1)); err != nil {
			t.Fatal(err)
		}
		x = fundDetails(t, s, a, ids["fund"])
		replacements = append(replacements, x.Lines[0].EntryID)
	}
	input := exportFilter("FUNDS", "OWN")
	input.FundID = ids["fund"]
	x := exportCreate(t, s, owner, exportInput(exportPreview(t, s, owner, input)))
	_, rows, body := exportCSV(t, s, owner, x.ID)
	if x.Summary.Requested != 43219 || x.Summary.Waived != 15000 || x.Summary.Active != 28219 || x.Summary.Outstanding != 28219 {
		t.Fatal("separately approved replacements", x.Summary)
	}
	retained := map[string]bool{}
	reversed := map[string]bool{}
	for _, row := range rows {
		if row["row_type"] == "EXEMPTION" {
			retained[row["entry_id"]] = true
		}
		if row["row_type"] == "ENTRY_REVERSAL" {
			reversed[row["entry_id"]] = true
		}
	}
	if !retained[replacements[0]] || !retained[replacements[1]] || !reversed[ids["fund-charge"]] || !reversed[replacements[0]] || strings.Contains(string(body), "PRIVATE") {
		t.Fatal("intermediate replacement or private source", rows)
	}
	voluntary := fundProposal()
	voluntary.Title = "Fictional voluntary export purpose"
	voluntary.ContributionType = "VOLUNTARY"
	voluntary.Lines = []MaintenanceLineInput{{"demo-flat-A-101", ""}}
	fund := createPublishedFund(t, s, a, b, voluntary)
	attribute := func(amount string) string {
		id, err := s.AttributeFundCredit(ctx, a, FundAttributionInput{randomToken(), fund.ID, "demo-flat-A-101", ids["received"], amount, "PRIVATE deliberately attributed original confirmed credit", true})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	old := attribute("100.00")
	if _, err := s.CorrectFundContribution(ctx, a, old, AllocationCorrection{randomToken(), "PRIVATE correct this voluntary attribution", true}); err != nil {
		t.Fatal(err)
	}
	live := attribute("30.00")
	input.FundID = fund.ID
	x = exportCreate(t, s, owner, exportInput(exportPreview(t, s, owner, input)))
	_, rows, body = exportCSV(t, s, owner, x.ID)
	if x.Summary.Voluntary != 3000 || x.Summary.Active != 0 || x.Summary.Outstanding != 0 || x.Summary.CurrentReceived != 50000 || x.Summary.AvailableReceived != 37000 {
		t.Fatal("voluntary is not another receipt or a fixed charge", x.Summary)
	}
	retained = map[string]bool{}
	var corrected bool
	for _, row := range rows {
		if row["row_type"] == "CONTRIBUTION" {
			retained[row["linked_id"]] = true
		}
		if row["row_type"] == "CONTRIBUTION_REVERSAL" && row["linked_id"] == old && row["signed_rupees"] == "-100.00" {
			corrected = true
		}
	}
	if !retained[old] || !retained[live] || !corrected || strings.Contains(string(body), "PRIVATE") {
		t.Fatal("voluntary original or private comment", rows)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 2)
}

func TestFinanceExportCompetingNewIdentitiesRespectStoredQuotaAtomically(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	post(t, s, a, received("0.01"))
	preview := exportPreview(t, s, a, exportFilter("LEDGER", "SOCIETY"))
	s.exportLimits = &financeExportLimits{SocietyBytes: int64(preview.Bytes)}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		in := exportInput(preview)
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.CreateFinanceExport(ctx, a, in); errs <- err }()
	}
	wg.Wait()
	close(errs)
	success, rejected := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, ErrInvalid) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatal("quota competitors", success, rejected)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM finance_exports", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='FINANCE_EXPORTED'", 1)
}
