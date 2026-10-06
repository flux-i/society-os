package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func fineFixture(t *testing.T, home string) (*Store, string, string, string, string, string) {
	t.Helper()
	s, a, b, o, tenant := incidentFixture(t)
	rule := publishIncidentRule(t, s, a, b)
	in := incidentInput(rule)
	in.FlatID = home
	id, e := s.SaveIncident(context.Background(), o, "", in)
	if e != nil {
		t.Fatal(e)
	}
	incidentAction(t, s, b, id, "SUBSTANTIATED")
	if e = s.SeedDemoTreasury(context.Background()); e != nil {
		t.Fatal(e)
	}
	return s, a, b, o, tenant, id
}
func fineProposal(t *testing.T, s *Store, token, incident string) FineInput {
	t.Helper()
	source, e := s.FineSourceFor(context.Background(), token, incident, 1)
	if e != nil {
		t.Fatal(e)
	}
	return FineInput{OperationKey: randomToken(), IncidentID: incident, SourceKey: source.SourceKey, Title: "Supplied corridor fine", Amount: "250.25", PolicyReference: "Fictional committee fine decision F-01", Reason: "Supplied fictional amount independently considered after incident review", ResponseBy: today(), DueDate: time.Now().In(societyZone).AddDate(0, 0, 7).Format("2006-01-02"), NoticeBody: "A separately proposed fictional fine of Rs250.25 is supplied for your response.", Confirmed: true}
}
func fineDetails(t *testing.T, s *Store, token, id string) FineDetail {
	t.Helper()
	x, e := s.FineFor(context.Background(), token, id, 1, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func fineActionInput(x FineDetail, action string) FineAction {
	return FineAction{OperationKey: randomToken(), Version: x.Version, Action: action, Reason: "Independently checked the supplied decision and current audience", Confirmed: true}
}
func proposedFine(t *testing.T, s *Store, a, incident string) string {
	t.Helper()
	id, e := s.CreateFine(context.Background(), a, fineProposal(t, s, a, incident))
	if e != nil {
		t.Fatal(e)
	}
	return id
}
func notifyFine(t *testing.T, s *Store, a, b, id string) FineDetail {
	t.Helper()
	x := fineDetails(t, s, a, id)
	if _, e := s.ActOnFine(context.Background(), b, id, fineActionInput(x, "NOTIFY")); e != nil {
		t.Fatal(e)
	}
	return fineDetails(t, s, a, id)
}
func resolveFine(t *testing.T, s *Store, a, b, id string) FineDetail {
	t.Helper()
	x := fineDetails(t, s, a, id)
	in := fineActionInput(x, "RESOLVE")
	in.SourceKey = x.CurrentSourceKey
	in.ResponseCount = x.ResponseTotal
	in.Resolution = "Supplied fictional response resolution deliberately reviewed before the charge"
	in.EarlyIssueReference = "Fictional supplied policy permits this reviewed early decision"
	if _, e := s.ActOnFine(context.Background(), b, id, in); e != nil {
		t.Fatal(e)
	}
	return fineDetails(t, s, a, id)
}
func issueFine(t *testing.T, s *Store, a, b, id string) FineDetail {
	t.Helper()
	x := resolveFine(t, s, a, b, id)
	in := fineActionInput(x, "ISSUE")
	in.ResolutionKey = x.ResolutionKey
	if _, e := s.ActOnFine(context.Background(), b, id, in); e != nil {
		t.Fatal(e)
	}
	return fineDetails(t, s, a, id)
}
func fineReportInput(id string) FineReportInput {
	return FineReportInput{OperationKey: randomToken(), FineID: id, Amount: "100.00", PaymentDate: today(), Payer: "Fictional home member", Method: "UPI", Reference: "FINE-EXTERNAL-100", Comment: "Money already paid outside this platform", Confirmed: true}
}
func verifyFinePayment(t *testing.T, s *Store, reporter, reviewer, fine string) FineReport {
	t.Helper()
	ctx := context.Background()
	id, e := s.SaveFineReport(ctx, reporter, "", fineReportInput(fine))
	if e != nil {
		t.Fatal(e)
	}
	in := FundReportDecision{OperationKey: randomToken(), Version: 1, Action: "CONFIRMED", Reason: "Verified against a supplied fictional external source", VerificationSource: "Fictional external register", PaymentIdentity: "EXTERNAL-PAYMENT-100", Mode: "NEW", AllocationAmount: "100.00", Confirmed: true}
	if _, e = s.DecideFineReport(ctx, reviewer, id, in); e != nil {
		t.Fatal(e)
	}
	x, e := s.FineReportFor(ctx, reviewer, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}

func TestFineSeparateNoticeResolutionAndConcurrentIssueRetainOneExactCharge(t *testing.T) {
	s, a, b, o, _, incident := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	proposal := fineProposal(t, s, a, incident)
	id, e := s.CreateFine(ctx, a, proposal)
	if e != nil {
		t.Fatal(e)
	}
	if again, e := s.CreateFine(ctx, a, proposal); e != nil || again != id {
		t.Fatal(again, e)
	}
	x := fineDetails(t, s, a, id)
	self := fineActionInput(x, "NOTIFY")
	if _, e = s.ActOnFine(ctx, a, id, self); !errors.Is(e, ErrForbidden) {
		t.Fatal("self approval", e)
	}
	if _, e = s.FineFor(ctx, o, id, 1, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("unpublished proposal", e)
	}
	if _, e = s.FineSourceFor(ctx, o, incident, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("source exposed", e)
	}
	notifyFine(t, s, a, b, id)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	x = fineDetails(t, s, a, id)
	unresolved := fineActionInput(x, "ISSUE")
	if _, e = s.ActOnFine(ctx, b, id, unresolved); !errors.Is(e, ErrConflict) {
		t.Fatal("unresolved issue", e)
	}
	x = resolveFine(t, s, a, b, id)
	issue := fineActionInput(x, "ISSUE")
	issue.ResolutionKey = x.ResolutionKey
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, e := s.ActOnFine(ctx, b, id, issue)
			if e == nil && result != id {
				e = errors.New("wrong retry identity")
			}
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal(e)
		}
	}
	x = fineDetails(t, s, a, id)
	if x.State != "ISSUED" || x.ActivePaise != 25025 || x.OutstandingPaise != 25025 || x.CurrentEntryID == "" || x.CurrentEntryID != x.OriginalEntryID {
		t.Fatal(x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries WHERE kind='CHARGE'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	owner := fineDetails(t, s, o, id)
	data, _ := json.Marshal(owner)
	for _, secret := range []string{"incident_id", "source_key", "material_key", "current_source", "reporter", "Supplied fictional amount independently"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("private fine projection", secret, string(data))
		}
	}
	changed := issue
	changed.Reason = "A changed retry must not become another decision"
	if _, e = s.ActOnFine(ctx, b, id, changed); !errors.Is(e, ErrConflict) {
		t.Fatal("changed retry", e)
	}
}

func TestFineMaterialContextPrivateNotesAndNewResponseInvalidatePreparedResolution(t *testing.T) {
	s, a, b, o, _, incident := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	id := proposedFine(t, s, a, incident)
	x := notifyFine(t, s, a, b, id)
	before, e := s.FineNoticeFor(ctx, o, x.NoticeID, 1)
	if e != nil {
		t.Fatal(e)
	}
	incidentAction(t, s, b, incident, "NOTE")
	if !fineDetails(t, s, a, id).SourceCurrent {
		t.Fatal("private operational note changed material outcome")
	}
	x = resolveFine(t, s, a, b, id)
	in := fineActionInput(x, "ISSUE")
	in.ResolutionKey = x.ResolutionKey
	response := IncidentResponseInput{OperationKey: randomToken(), Version: 1, Body: "A supplied household response changes the resolution context", Confirmed: true}
	if _, e = s.RespondToFineNotice(ctx, o, x.NoticeID, response); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnFine(ctx, b, id, in); !errors.Is(e, ErrConflict) {
		t.Fatal("stale resolution", e)
	}
	x = fineDetails(t, s, a, id)
	if x.ResolutionCurrent || x.ResponseTotal != 1 {
		t.Fatal(x)
	}
	in = fineActionInput(x, "ISSUE")
	in.ResolutionKey = x.ResolutionKey
	if _, e = s.ActOnFine(ctx, b, id, in); !errors.Is(e, ErrInvalid) {
		t.Fatal("new reply bypassed resolution", e)
	}
	after, e := s.FineNoticeFor(ctx, o, x.NoticeID, 1)
	if e != nil || after.CreatedAt != before.CreatedAt || after.Version != before.Version {
		t.Fatal("notice mutated by private response", after, e)
	}
	issued := issueFine(t, s, a, b, id)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	incidentAction(t, s, b, incident, "REOPEN")
	x = fineDetails(t, s, a, id)
	if !x.NeedsReview || x.ActivePaise != 25025 || x.CurrentEntryID != issued.CurrentEntryID {
		t.Fatal("source reopened silently changed money", x)
	}
}

func TestFinePaymentDuplicatesCorrectionsAndSharedCreditExactAmounts(t *testing.T) {
	s, a, b, o, _, incident := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	id := proposedFine(t, s, a, incident)
	notifyFine(t, s, a, b, id)
	x := issueFine(t, s, a, b, id)
	payment := verifyFinePayment(t, s, o, a, id)
	x = fineDetails(t, s, a, id)
	if x.AllocatedPaise != 10000 || x.OutstandingPaise != 15025 {
		t.Fatal("independent partial-money expectation", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	duplicate, e := s.SaveFineReport(ctx, a, "", fineReportInput(id))
	if e != nil {
		t.Fatal(e)
	}
	decision := FundReportDecision{OperationKey: randomToken(), Version: 1, Action: "CONFIRMED", Reason: "Compared duplicate against the original external payment", VerificationSource: "fictional EXTERNAL register", PaymentIdentity: "external-payment-100", Mode: "NEW", AllocationAmount: "100.00", Confirmed: true}
	if _, e = s.DecideFineReport(ctx, b, duplicate, decision); e != nil {
		t.Fatal(e)
	}
	dup, e := s.FineReportFor(ctx, b, duplicate, 1)
	if e != nil || dup.State != "DUPLICATE" || dup.EntryID != payment.EntryID || dup.ReceiptID != payment.ReceiptID || dup.AllocationID != "" {
		t.Fatal(dup, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries WHERE kind='RECEIVED'", 1)
	if _, e = s.ReverseEntry(ctx, b, x.CurrentEntryID, EntryAction{OperationKey: randomToken(), Reason: "Attempted direct reversal bypassing the independent fine workflow", Confirmed: true}); !errors.Is(e, ErrInvalid) {
		t.Fatal("direct charge reversal", e)
	}
	w, e := s.ProposeFineWaiver(ctx, a, FineWaiverInput{OperationKey: randomToken(), FineID: id, ChargeVersion: x.ChargeVersion, Kind: "WAIVER", Amount: "75.25", PolicyReference: "Supplied fictional partial exemption", Reason: "Separately proposed a precise fictional partial exemption", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	x = fineDetails(t, s, a, id)
	action := FineChildAction{OperationKey: randomToken(), Version: 1, FineVersion: x.Version, Action: "APPROVED", Reason: "Independently reviewed the precise supplied partial exemption", Confirmed: true}
	if _, e = s.DecideFineWaiver(ctx, a, w, action); !errors.Is(e, ErrForbidden) {
		t.Fatal("self waiver", e)
	}
	if _, e = s.DecideFineWaiver(ctx, b, w, action); e != nil {
		t.Fatal(e)
	}
	x = fineDetails(t, s, a, id)
	if x.ActivePaise != 17500 || x.AllocatedPaise != 0 || x.OutstandingPaise != 17500 || x.WaivedPaise != 7525 || x.OriginalEntryID == x.CurrentEntryID {
		t.Fatal("partial correction", x)
	}
	home := statement(t, s, a, "demo-flat-A-102")
	if home.UnallocatedPaise != 10000 {
		t.Fatal("released credit", home)
	}
	allocate(t, s, a, payment.EntryID, x.CurrentEntryID, "100.00")
	x = fineDetails(t, s, a, id)
	if x.OutstandingPaise != 7500 {
		t.Fatal("explicit reallocation", x)
	}
	full, e := s.ProposeFineWaiver(ctx, a, FineWaiverInput{OperationKey: randomToken(), FineID: id, ChargeVersion: x.ChargeVersion, Kind: "REVERSAL", Amount: "175.00", PolicyReference: "Supplied fictional full correction", Reason: "Separately proposed reversal of the remaining supplied charge", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	x = fineDetails(t, s, a, id)
	action.OperationKey = randomToken()
	action.FineVersion = x.Version
	if _, e = s.DecideFineWaiver(ctx, b, full, action); e != nil {
		t.Fatal(e)
	}
	x = fineDetails(t, s, a, id)
	if x.State != "WAIVED" || x.ActivePaise != 0 || x.OutstandingPaise != 0 || x.WaivedPaise != 25025 {
		t.Fatal(x)
	}
	if statement(t, s, a, "demo-flat-A-102").UnallocatedPaise != 10000 {
		t.Fatal("full released credit")
	}
	original, e := s.FineReportFor(ctx, a, payment.ID, 1)
	if e != nil || original.ReceiptID != payment.ReceiptID {
		t.Fatal("receipt rewritten", original, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
}

func TestFineNonfinancialHouseholdNoticeAppealAndPausePreserveLedger(t *testing.T) {
	s, a, b, o, tenant, incident := fineFixture(t, "demo-flat-A-103")
	ctx := context.Background()
	id := proposedFine(t, s, a, incident)
	x := notifyFine(t, s, a, b, id)
	notice, e := s.FineNoticeFor(ctx, tenant, x.NoticeID, 1)
	if e != nil || !notice.CanRespond || notice.CanReadFinance {
		t.Fatal(notice, e)
	}
	response := IncidentResponseInput{OperationKey: randomToken(), Version: 1, Body: "A fictional tenant response is permitted without ledger access", Confirmed: true}
	if _, e = s.RespondToFineNotice(ctx, tenant, notice.ID, response); e != nil {
		t.Fatal(e)
	}
	x = issueFine(t, s, a, b, id)
	if _, e = s.FineFor(ctx, tenant, id, 1, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("tenant ledger grant", e)
	}
	if _, e = s.SaveFineReport(ctx, tenant, "", fineReportInput(id)); !errors.Is(e, ErrForbidden) {
		t.Fatal("tenant payment grant", e)
	}
	if _, e = s.FineNoticeFor(ctx, o, notice.ID, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("cross-home notice", e)
	}
	appeal, e := s.CreateFineAppeal(ctx, tenant, FineAppealInput{OperationKey: randomToken(), NoticeID: notice.ID, Body: "Supplied appeal asks for independent review of the issued decision", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	x = fineDetails(t, s, a, id)
	act := FineChildAction{OperationKey: randomToken(), Version: 1, FineVersion: x.Version, Action: "PAUSED", Reason: "Supplied policy requires a deliberate pause during this appeal", PolicyReference: "Fictional appeal policy A-01", PauseUntil: today(), Confirmed: true}
	if _, e = s.ActOnFineAppeal(ctx, b, appeal, act); e != nil {
		t.Fatal(e)
	}
	x = fineDetails(t, s, a, id)
	if x.PauseUntil != today() || x.ActivePaise != 25025 || x.OutstandingPaise != 25025 {
		t.Fatal("pause changed ledger", x)
	}
	mine, e := s.FineAppealFor(ctx, tenant, appeal, 1)
	if e != nil || mine.State != "PAUSED" || mine.CurrentFineVersion != 0 || mine.CanDecide {
		t.Fatal(mine, e)
	}
	// A passed pause date still needs an explicit decision; no automatic ledger change.
	_, e = s.DB.Exec(`UPDATE fines SET pause_until='2020-01-01',version=version+1 WHERE id=?`, id)
	if e != nil {
		t.Fatal(e)
	}
	list, e := s.FinesFor(ctx, a, "", "", 1)
	if e != nil || list.Totals.Paused != 1 || list.Totals.OutstandingPaise != 25025 {
		t.Fatal("expired pause auto-resolved", list, e)
	}
	x = fineDetails(t, s, a, id)
	act.OperationKey = randomToken()
	act.Version = 2
	act.FineVersion = x.Version
	act.Action = "RESOLVED"
	act.PauseUntil = ""
	if _, e = s.ActOnFineAppeal(ctx, b, appeal, act); e != nil {
		t.Fatal(e)
	}
	if fineDetails(t, s, a, id).PauseUntil != "" {
		t.Fatal("resolved pause retained")
	}
	if _, e = s.DB.Exec("UPDATE flat_memberships SET end_date=? WHERE flat_id='demo-flat-A-103' AND relationship='TENANT'", today()); e != nil {
		t.Fatal(e)
	}
	if _, e = s.FineNoticeFor(ctx, tenant, notice.ID, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("ended notice", e)
	}
	if _, e = s.RespondToFineNotice(ctx, tenant, notice.ID, response); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("ended replay", e)
	}
}

func TestFineReporterConflictRuleRetirementAndMaterialChangeBeforeIssue(t *testing.T) {
	s, a, b, o, _, incident := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	id := proposedFine(t, s, a, incident)
	notifyFine(t, s, a, b, id)
	// Retirement preserves the frozen historical permission of this applicable rule.
	report := incidentCase(t, s, a, incident)
	rule, e := s.RuleFor(ctx, a, report.RuleID, 1)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideRule(ctx, b, rule.ID, RuleAction{OperationKey: randomToken(), Version: rule.Version, Action: "RETIRED", Reason: "Supplied policy retired this version for future reports", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	x := resolveFine(t, s, a, b, id)
	if !x.SourceCurrent || !x.ResolutionCurrent {
		t.Fatal("historical permission changed", x)
	}
	before, e := s.FineNoticeFor(ctx, o, x.NoticeID, 1)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnFine(ctx, a, id, fineActionInput(x, "NOTE")); e != nil {
		t.Fatal(e)
	}
	after, e := s.FineNoticeFor(ctx, o, x.NoticeID, 1)
	if e != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("private note changed frozen notice", e)
	}
	incidentAction(t, s, b, incident, "REOPEN")
	x = fineDetails(t, s, a, id)
	in := fineActionInput(x, "ISSUE")
	in.ResolutionKey = x.ResolutionKey
	if _, e = s.ActOnFine(ctx, b, id, in); e == nil {
		t.Fatal("reopened case issued")
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
}

func TestFineGlobalExternalIdentityAcrossPurposesAndCreditCap(t *testing.T) {
	s, a, b, o, _, incident := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	id := proposedFine(t, s, a, incident)
	notifyFine(t, s, a, b, id)
	issueFine(t, s, a, b, id)
	report, e := s.SaveFineReport(ctx, o, "", fineReportInput(id))
	if e != nil {
		t.Fatal(e)
	}
	decision := FundReportDecision{OperationKey: randomToken(), Version: 1, Action: "CONFIRMED", Reason: "Verified one shared external payment independently", VerificationSource: "Fictional external register", PaymentIdentity: "EXTERNAL-PAYMENT-100", Mode: "NEW", AllocationAmount: "60.00", Confirmed: true}
	if _, e = s.DecideFineReport(ctx, a, report, decision); e != nil {
		t.Fatal(e)
	}
	paid, e := s.FineReportFor(ctx, a, report, 1)
	if e != nil {
		t.Fatal(e)
	}
	voluntary := fundProposal()
	voluntary.ContributionType = "VOLUNTARY"
	voluntary.Lines = []MaintenanceLineInput{{FlatID: "demo-flat-A-102", Amount: ""}}
	fund := createPublishedFund(t, s, a, b, voluntary)
	claim := FundReportInput{OperationKey: randomToken(), CampaignID: fund.ID, FlatID: "demo-flat-A-102", Amount: "100.00", PaymentDate: today(), Payer: "Fictional home member", Method: "UPI", Reference: "FINE-EXTERNAL-100", Comment: "Same externally paid money, intentionally split across purposes", Confirmed: true}
	other, e := s.SaveFundReport(ctx, o, "", claim)
	if e != nil {
		t.Fatal(e)
	}
	decision.OperationKey = randomToken()
	decision.AllocationAmount = "40.01"
	if _, e = s.DecideFundReport(ctx, b, other, decision); !errors.Is(e, ErrConflict) {
		t.Fatal("combined credit cap", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	decision.AllocationAmount = "40.00"
	if _, e = s.DecideFundReport(ctx, b, other, decision); e != nil {
		t.Fatal(e)
	}
	confirmed, e := s.FundReportFor(ctx, b, other, 1)
	if e != nil || confirmed.EntryID != paid.EntryID || confirmed.ReceiptID != paid.ReceiptID {
		t.Fatal("cross-purpose original", confirmed, e)
	}
	x := fineDetails(t, s, a, id)
	if x.AllocatedPaise != 6000 || x.OutstandingPaise != 19025 {
		t.Fatal("independent fine use", x)
	}
	home := statement(t, s, a, "demo-flat-A-102")
	if home.UnallocatedPaise != 0 || home.VoluntaryPaise != 4000 || home.AllocatedPaise != 6000 {
		t.Fatal("independent shared cap", home)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries WHERE kind='RECEIVED'", 1)
	if _, e = s.AllocateCredit(ctx, a, CreditAllocationInput{OperationKey: randomToken(), SourceID: paid.EntryID, ChargeID: x.CurrentEntryID, Amount: "0.01", Reason: "Attempting to exceed combined fine and voluntary uses", Confirmed: true}); !errors.Is(e, ErrConflict) {
		t.Fatal("another purpose bypassed shared cap", e)
	}
}

func TestFineReporterFreshAuthorityAndRevocationBeforeAcceptedReplay(t *testing.T) {
	s, a, _, _, _, incident := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	proposal := fineProposal(t, s, a, incident)
	id, e := s.CreateFine(ctx, a, proposal)
	if e != nil {
		t.Fatal(e)
	}
	grantAppointment(t, s, a, "demo-user-owner", "TREASURER", 30)
	reporter := reviewLogin(t, s, "owner@demo.society")
	x := fineDetails(t, s, reporter, id)
	if x.CanDecide {
		t.Fatal("reporter marked eligible")
	}
	if _, e = s.ActOnFine(ctx, reporter, id, fineActionInput(x, "NOTIFY")); !errors.Is(e, ErrForbidden) {
		t.Fatal("reporter approved financial consequence", e)
	}
	_, e = s.DB.Exec("UPDATE sessions SET reauthenticated_at=? WHERE token_hash=?", time.Now().Add(-6*time.Minute).Unix(), TokenHash(a))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.CreateFine(ctx, a, proposal); !errors.Is(e, ErrReauthRequired) {
		t.Fatal("expired fresh replay", e)
	}
	_, e = s.DB.Exec("UPDATE sessions SET reauthenticated_at=? WHERE token_hash=?", time.Now().Unix(), TokenHash(a))
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.DB.Exec("UPDATE role_grants SET revoked_at=? WHERE id='demo-treasury-grant'", time.Now().Unix())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.CreateFine(ctx, a, proposal); !errors.Is(e, ErrForbidden) {
		t.Fatal("revoked treasury replay", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM fines", 1)
}

func TestFineOverviewIndependentAmountsQueuesPrivacyAndNonfinancialCurrentHome(t *testing.T) {
	s, a, b, o, tenant, incident := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	id := proposedFine(t, s, a, incident)
	read := func(token string) Overview {
		t.Helper()
		x, e := s.OverviewFor(ctx, token, "fines")
		if e != nil {
			t.Fatal(e)
		}
		return x
	}
	if x := read(b); x.Counts["pending_review"] != 1 || x.Counts["active_paise"] != 0 || len(x.Items) != 1 || x.Items[0].Kind != "FINE" {
		t.Fatal("pending snapshot", x)
	}
	if x := read(a); x.Counts["pending_review"] != 0 {
		t.Fatal("author is not independent reviewer", x)
	}
	n := notifyFine(t, s, a, b, id)
	if x := read(o); x.Counts["responses_needed"] != 1 || x.Counts["outstanding_paise"] != 0 || len(x.Items) != 1 || x.Items[0].Kind != "FINE_NOTICE" {
		t.Fatal("notice needs response without debt", x)
	}
	if _, e := s.RespondToFineNotice(ctx, o, n.NoticeID, IncidentResponseInput{OperationKey: randomToken(), Version: 1, Body: "PRIVATE household supplied account for independent review", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	if x := read(o); x.Counts["responses_needed"] != 0 {
		t.Fatal("own reply clears queue", x)
	}
	issueFine(t, s, a, b, id)
	claim, e := s.SaveFineReport(ctx, o, "", fineReportInput(id))
	if e != nil {
		t.Fatal(e)
	}
	if x := read(a); x.Counts["needs_your_verification"] != 1 || x.Counts["outstanding_paise"] != 25025 {
		t.Fatal("claim is not received money", x)
	}
	if x := read(o); x.Counts["awaiting_verification"] != 1 || x.Counts["needs_your_verification"] != 0 {
		t.Fatal("own claim cannot verify", x)
	}
	if _, e = s.DecideFineReport(ctx, a, claim, FundReportDecision{OperationKey: randomToken(), Version: 1, Action: "CONFIRMED", Reason: "PRIVATE independently verified the external payment before its exact allocation", VerificationSource: "PRIVATE bank statement", PaymentIdentity: "PRIVATE bank identity", Mode: "NEW", AllocationAmount: "100.00", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	for _, token := range []string{a, b, o} {
		x := read(token)
		for key, want := range map[string]int64{"active_paise": 25025, "allocated_paise": 10000, "outstanding_paise": 15025, "overdue_paise": 0, "awaiting_verification": 0, "needs_your_verification": 0, "paused": 0} {
			if x.Counts[key] != want {
				t.Fatal(key, x.Counts[key], want)
			}
		}
		blob, _ := json.Marshal(x)
		for _, private := range []string{"PRIVATE", "source_key", "policy_reference", "body", "author_id", "events", "response_total"} {
			if strings.Contains(string(blob), private) {
				t.Fatal("overview private disclosure", private, string(blob))
			}
		}
	}
	if x := read(tenant); len(x.Items) != 0 {
		t.Fatal("other household", x)
	} else if _, exists := x.Counts["active_paise"]; exists {
		t.Fatal("unentitled finance amount even zero", x)
	}
	appeal, e := s.CreateFineAppeal(ctx, o, FineAppealInput{OperationKey: randomToken(), NoticeID: n.NoticeID, Body: "PRIVATE supplied appeal requests consideration of the stated circumstances", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	appealDetail, e := s.FineAppealFor(ctx, b, appeal, 1)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnFineAppeal(ctx, b, appeal, FineChildAction{OperationKey: randomToken(), Version: appealDetail.Version, FineVersion: appealDetail.CurrentFineVersion, Action: "PAUSED", Reason: "Supplied policy explicitly pauses collection pending separate review", PolicyReference: "Fictional supplied pause authority P-01", PauseUntil: today(), Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	x := read(b)
	if x.Counts["paused"] != 1 || x.Counts["active_appeals"] != 1 || x.Counts["appeals_for_review"] != 1 || x.Counts["outstanding_paise"] != 15025 {
		t.Fatal("pause exact ledger", x)
	}
	// A later source response is material; a private staff note is not.
	if _, e = s.RespondToFineNotice(ctx, o, n.NoticeID, IncidentResponseInput{OperationKey: randomToken(), Version: 1, Body: "PRIVATE newly supplied material response after charge issuance", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	if x := read(b); x.Counts["needs_review"] != 1 {
		t.Fatal("issued context changed", x)
	}
	x = read(o)
	if _, exists := x.Counts["needs_review"]; exists {
		t.Fatal("resident private context flag", x)
	}
	if _, e = s.DB.Exec("UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101'"); e != nil {
		t.Fatal(e)
	}
	x = read(o)
	if _, exists := x.Counts["outstanding_paise"]; exists || x.Counts["active_appeals"] != 1 {
		t.Fatal("household appeal without ledger", x)
	}
	if _, e = s.DB.Exec("UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-102'", today()); e != nil {
		t.Fatal(e)
	}
	x = read(o)
	if x.Counts["active_appeals"] != 0 || len(x.Items) != 0 {
		t.Fatal("ended current-home snapshot", x)
	}
}

func TestFinePrivateNotesKeepResidentPublicStampAndListOrder(t *testing.T) {
	s, a, b, o, _, incident := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	first := proposedFine(t, s, a, incident)
	notifyFine(t, s, a, b, first)
	issueFine(t, s, a, b, first)
	rule := fineDetails(t, s, a, first).RuleID
	in := incidentInput(rule)
	in.FlatID = "demo-flat-A-102"
	in.Comment = "A different supplied occurrence receives its own retained incident decision"
	secondIncident, e := s.SaveIncident(ctx, o, "", in)
	if e != nil {
		t.Fatal(e)
	}
	incidentAction(t, s, b, secondIncident, "SUBSTANTIATED")
	second := proposedFine(t, s, a, secondIncident)
	notifyFine(t, s, a, b, second)
	issueFine(t, s, a, b, second)
	for id, stamp := range map[string]int{first: 100, second: 200} {
		if _, e = s.DB.Exec("UPDATE fines SET updated_at=?,public_updated_at=?,version=version+1 WHERE id=?", stamp, stamp, id); e != nil {
			t.Fatal(e)
		}
	}
	before, e := s.FinesFor(ctx, o, "", "", 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(before.Items) != 2 || before.Items[0].ID != second {
		t.Fatal("independent public order", before)
	}
	x := fineDetails(t, s, a, first)
	note := fineActionInput(x, "NOTE")
	note.Reason = "PRIVATE treasury coordination does not alter a household snapshot"
	if _, e = s.ActOnFine(ctx, a, first, note); e != nil {
		t.Fatal(e)
	}
	after, e := s.FinesFor(ctx, o, "", "", 1)
	if e != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("private note leaked through public list order or stamp", e, before, after)
	}
}

func TestFinePendingCorrectionCannotAuthorizeADirectLedgerReversal(t *testing.T) {
	s, a, b, _, _, incident := fineFixture(t, "demo-flat-A-102")
	id := proposedFine(t, s, a, incident)
	notifyFine(t, s, a, b, id)
	x := issueFine(t, s, a, b, id)
	if _, e := s.ProposeFineWaiver(context.Background(), a, FineWaiverInput{OperationKey: randomToken(), FineID: id, ChargeVersion: 1, Kind: "WAIVER", Amount: "75.25", PolicyReference: "Supplied fictional correction source C-01", Reason: "A pending proposal is not an approved financial correction", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("INSERT INTO entry_reversals VALUES(?,?,?,?)", x.CurrentEntryID, "No reviewed decision exists for this raw reversal", "demo-user-committee", time.Now().Unix()); e == nil {
		t.Fatal("pending correction incorrectly authorizes a ledger reversal")
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entry_reversals", 0)
}

func TestFineBoundedSourcesNoticesOwnResponsesAndImmutablePublicationBeforeAnyCharge(t *testing.T) {
	s, a, b, o, _, first := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	ids := []string{first}
	source, e := s.FineSourceFor(ctx, a, first, 1)
	if e != nil {
		t.Fatal(e)
	}
	for i := 1; i < 27; i++ {
		in := incidentInput(source.Rule.ID)
		in.FlatID = "demo-flat-A-102"
		in.Comment = fmt.Sprintf("PRIVATE separately retained fictional occurrence %02d", i)
		id, e := s.SaveIncident(ctx, o, "", in)
		if e != nil {
			t.Fatal(e)
		}
		incidentAction(t, s, b, id, "SUBSTANTIATED")
		ids = append(ids, id)
	}
	sources, e := s.FineSourcesFor(ctx, a, "", 1)
	if e != nil || sources.Total != 27 || len(sources.Items) != 12 {
		t.Fatal("independent bounded sources", sources, e)
	}
	last, e := s.FineSourcesFor(ctx, a, "", 10000)
	if e != nil || last.Page != 3 || len(last.Items) != 3 {
		t.Fatal("clamped source page", last, e)
	}
	for _, item := range sources.Items {
		body, _ := json.Marshal(item)
		for _, secret := range []string{"PRIVATE", "comment", "picture", "reporter"} {
			if strings.Contains(string(body), secret) {
				t.Fatal("source selector leaks operational content", string(body))
			}
		}
	}
	fines := []string{}
	for i, id := range ids {
		in := fineProposal(t, s, a, id)
		in.Title = fmt.Sprintf("Bounded supplied fine %02d", i)
		fine, e := s.CreateFine(ctx, a, in)
		if e != nil {
			t.Fatal(e)
		}
		notifyFine(t, s, a, b, fine)
		fines = append(fines, fine)
	}
	remaining, e := s.FineSourcesFor(ctx, a, "", 1)
	if e != nil || remaining.Total != 0 {
		t.Fatal("prepared cases remain offered", remaining, e)
	}
	all, e := s.FinesFor(ctx, a, "Bounded supplied fine", "", 2)
	if e != nil || all.Total != 27 || len(all.Items) != 12 || all.Page != 2 {
		t.Fatal("bounded fine list", all, e)
	}
	own, e := s.FinesFor(ctx, o, "", "", 1)
	if e != nil || own.Total != 0 {
		t.Fatal("unissued financial proposals visible to residents", own, e)
	}
	notices, e := s.FineNoticesFor(ctx, o, "Bounded supplied fine", 3)
	if e != nil || notices.Total != 27 || len(notices.Items) != 3 {
		t.Fatal("deliberate notice audience pages", notices, e)
	}
	x := fineDetails(t, s, a, fines[0])
	for i := 0; i < 22; i++ {
		_, e = s.RespondToFineNotice(ctx, o, x.NoticeID, IncidentResponseInput{OperationKey: randomToken(), Version: 1, Body: fmt.Sprintf("PRIVATE immutable own household response %02d", i), Confirmed: true})
		if e != nil {
			t.Fatal(e)
		}
	}
	notice, e := s.FineNoticeFor(ctx, o, x.NoticeID, 2)
	if e != nil || notice.ResponseTotal != 22 || len(notice.Responses) != 2 || notice.Version != 1 {
		t.Fatal("bounded frozen notice own replies", notice, e)
	}
	staff, e := s.FineFor(ctx, b, fines[0], 2, 2)
	if e != nil || staff.ResponseTotal != 22 || staff.Notice.ResponseTotal != 22 || len(staff.Notice.Responses) != 2 {
		t.Fatal("bounded staff context", staff, e)
	}
	if _, e = s.DB.Exec("UPDATE fine_notices SET body='Changed original wording' WHERE id=?", x.NoticeID); e == nil {
		t.Fatal("published fine notice edited")
	}
	if _, e = s.FineSourcesFor(ctx, a, "", 100001); !errors.Is(e, ErrInvalid) {
		t.Fatal("unbounded source page", e)
	}
	if _, e = s.FineNoticesFor(ctx, o, "", 0); !errors.Is(e, ErrInvalid) {
		t.Fatal("unbounded notice page", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}
