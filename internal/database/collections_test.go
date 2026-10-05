package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func fundProposal() FundInput {
	return FundInput{OperationKey: randomToken(), Title: "Fictional water reserve fund", Purpose: "Reviewed fictional repair contribution for shared water services.", ContributionType: "FIXED", StartDate: "2026-01-01", DueDate: time.Now().In(societyZone).AddDate(0, 0, 5).Format("2006-01-02"), SourceReference: "PRIVATE approved fund resolution F-1", Note: "PRIVATE proposal support", Lines: []MaintenanceLineInput{{"demo-flat-A-101", "1000.00"}, {"demo-flat-A-102", "500.25"}}, Confirmed: true}
}
func fundDecision(action string, version int) FundAction {
	return FundAction{OperationKey: randomToken(), Action: action, Version: version, Reason: "Separately reviewed the supplied fictional fund decision", Confirmed: true}
}
func createPublishedFund(t *testing.T, s *Store, admin, reviewer string, in FundInput) FundDetail {
	t.Helper()
	ctx := context.Background()
	id, err := s.CreateFundCampaign(ctx, admin, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideFundCampaign(ctx, reviewer, id, fundDecision("PUBLISHED", 1)); err != nil {
		t.Fatal(err)
	}
	return fundDetails(t, s, admin, id)
}
func fundDetails(t *testing.T, s *Store, token, id string) FundDetail {
	t.Helper()
	out, err := s.FundCampaignFor(context.Background(), token, id, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func fundClaim(campaign, amount, reference string) FundReportInput {
	return FundReportInput{OperationKey: randomToken(), CampaignID: campaign, FlatID: "demo-flat-A-101", Amount: amount, PaymentDate: "2026-01-01", Payer: "Demo Owner A-101", Method: "BANK_TRANSFER", Reference: reference, Comment: "PRIVATE fictional claim detail", Confirmed: true}
}
func createFundClaim(t *testing.T, s *Store, token string, in FundReportInput) string {
	t.Helper()
	id, err := s.SaveFundReport(context.Background(), token, "", in)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func confirmClaim(identity, amount string) FundReportDecision {
	return FundReportDecision{OperationKey: randomToken(), Version: 1, Action: "CONFIRMED", Reason: "Checked external fictional bank statement before confirming", VerificationSource: "Fictional Society Account Statement", PaymentIdentity: identity, Mode: "NEW", AllocationAmount: amount, Confirmed: true}
}
func claimDetails(t *testing.T, s *Store, token, id string) FundReport {
	t.Helper()
	out, err := s.FundReportFor(context.Background(), token, id, 1)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func confirmFund(t *testing.T, s *Store, token, id, identity, amount string) FundReport {
	t.Helper()
	if _, err := s.DecideFundReport(context.Background(), token, id, confirmClaim(identity, amount)); err != nil {
		t.Fatal(err)
	}
	return claimDetails(t, s, token, id)
}

func TestFundFrozenSeparatePublicationCurrentAuthorityAndNoClaimReceipts(t *testing.T) {
	s, admin := recordFixture(t)
	ctx := context.Background()
	in := fundProposal()
	id, err := s.CreateFundCampaign(ctx, admin, in)
	if err != nil {
		t.Fatal(err)
	}
	if same, e := s.CreateFundCampaign(ctx, admin, in); e != nil || same != id {
		t.Fatal("proposal retry", same, e)
	}
	changed := in
	changed.Purpose += " changed"
	if _, e := s.CreateFundCampaign(ctx, admin, changed); !errors.Is(e, ErrConflict) {
		t.Fatal("changed retry", e)
	}
	pending, err := s.FundCampaignsFor(ctx, admin, "", "", "", 1)
	if err != nil || pending.Total != 1 || pending.Totals.RequestedPaise != 150025 || pending.Totals.ActivePaise != 0 {
		t.Fatal("frozen request", pending, err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	owner := reviewLogin(t, s, "owner@demo.society")
	private, e := s.FundCampaignsFor(ctx, owner, "", "", "", 1)
	if e != nil || private.Total != 0 {
		t.Fatal("private proposal", private, e)
	}
	if _, e = s.DecideFundCampaign(ctx, admin, id, fundDecision("PUBLISHED", 1)); !errors.Is(e, ErrForbidden) {
		t.Fatal("self publication", e)
	}
	committee := reviewLogin(t, s, "committee@demo.society")
	if _, e = s.DecideFundCampaign(ctx, committee, id, fundDecision("PUBLISHED", 1)); !errors.Is(e, ErrForbidden) {
		t.Fatal("operational role posted money", e)
	}
	for _, query := range []string{"UPDATE fund_campaigns SET purpose='Changed supplied purpose' WHERE id=?", "UPDATE fund_participants SET requested_paise=1 WHERE campaign_id=?", "DELETE FROM fund_participants WHERE campaign_id=?", "INSERT INTO fund_participants(campaign_id,flat_id,requested_paise) VALUES(?,'demo-flat-A-103',100)"} {
		if _, e = s.DB.Exec(query, id); e == nil {
			t.Fatal("mutable proposal", query)
		}
	}
	reviewer := maintenanceReviewer(t, s, admin)
	action := fundDecision("PUBLISHED", 1)
	for i := 0; i < 2; i++ {
		if same, e := s.DecideFundCampaign(ctx, reviewer, id, action); e != nil || same != id {
			t.Fatal("publication retry", same, e)
		}
	}
	out := fundDetails(t, s, admin, id)
	if out.ActivePaise != 150025 || out.OutstandingPaise != 150025 || out.OverduePaise != 0 || out.UnpaidHomes != 2 || len(out.Events) != 2 {
		t.Fatal("independent future totals", out)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	accessExec(t, s, "UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-committee' AND role='TREASURER'", time.Now().Unix()-60, time.Now().Unix()-1)
	if _, e = s.DecideFundCampaign(ctx, reviewer, id, action); !errors.Is(e, ErrForbidden) {
		t.Fatal("expired replay authority", e)
	}
}

func TestFundOverviewSeparatesClaimsCurrentReviewersVoluntaryMoneyAndExplicitDueBalances(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	owner := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	input := fundProposal()
	input.DueDate = "2026-01-10"
	id, err := s.CreateFundCampaign(ctx, admin, input)
	if err != nil {
		t.Fatal(err)
	}
	author := mustOverview(t, s, admin, "collections")
	other := mustOverview(t, s, reviewer, "collections")
	if author.Counts["pending_review"] != 0 || author.Counts["awaiting_other_reviewer"] != 1 || other.Counts["pending_review"] != 1 || other.Counts["outstanding_paise"] != 0 || len(other.Items) != 1 || other.Items[0].ID != id {
		t.Fatal("proposal counts", author, other)
	}
	if _, err = s.DecideFundCampaign(ctx, reviewer, id, fundDecision("PUBLISHED", 1)); err != nil {
		t.Fatal(err)
	}
	claim := fundClaim(id, "400.00", "OVERVIEW-FIXED-PAYMENT")
	report := createFundClaim(t, s, owner, claim)
	before := mustOverview(t, s, admin, "collections")
	if before.Counts["outstanding_paise"] != 150025 || before.Counts["overdue_paise"] != 150025 || before.Counts["confirmed_paise"] != 0 || before.Counts["needs_your_verification"] != 1 || before.Counts["awaiting_verification"] != 1 {
		t.Fatal("claim isn't money", before)
	}
	for _, item := range before.Items {
		if item.Kind == "PAYMENT_REPORT" && item.ID != report {
			t.Fatal("report link", item)
		}
	}
	if _, err = s.DecideFundReport(ctx, admin, report, FundReportDecision{OperationKey: randomToken(), Version: 1, Action: "NEEDS_INFO", Reason: "Please supply the original external payment source line", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	own := mustOverview(t, s, owner, "collections")
	if own.Counts["changes_requested"] != 1 || own.Counts["needs_your_verification"] != 0 || own.Counts["awaiting_verification"] != 0 {
		t.Fatal("own actionable claim", own)
	}
	claim.Version = 2
	claim.OperationKey = randomToken()
	if _, err = s.SaveFundReport(ctx, owner, report, claim); err != nil {
		t.Fatal(err)
	}
	confirmation := confirmClaim("OVERVIEW-BANK-1", "400.00")
	confirmation.Version = 3
	if _, err = s.DecideFundReport(ctx, admin, report, confirmation); err != nil {
		t.Fatal(err)
	}
	after := mustOverview(t, s, owner, "collections")
	if after.Counts["confirmed_paise"] != 40000 || after.Counts["outstanding_paise"] != 110025 || after.Counts["overdue_paise"] != 110025 || after.Counts["needs_your_verification"] != 0 {
		t.Fatal("verified balances", after)
	}
	exemption, err := s.ProposeFundWaiver(ctx, admin, FundWaiverInput{OperationKey: randomToken(), CampaignID: id, FlatID: "demo-flat-A-101", ParticipantVersion: 1, Amount: "300.00", SourceReference: "PRIVATE supplied exception source", Reason: "PRIVATE reviewed supplied exception for the overview", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if mustOverview(t, s, admin, "collections").Counts["pending_exemptions"] != 0 || mustOverview(t, s, reviewer, "collections").Counts["pending_exemptions"] != 1 || mustOverview(t, s, owner, "collections").Counts["pending_exemptions"] != 0 {
		t.Fatal("exemption current separate reviewer")
	}
	voluntary := fundProposal()
	voluntary.Title = "Voluntary overview purpose"
	voluntary.ContributionType = "VOLUNTARY"
	voluntary.Lines = []MaintenanceLineInput{{FlatID: "demo-flat-A-101"}}
	fund := createPublishedFund(t, s, admin, reviewer, voluntary)
	volReport := createFundClaim(t, s, reviewer, fundClaim(fund.ID, "500.00", "OVERVIEW-VOLUNTARY-PAYMENT"))
	confirmFund(t, s, admin, volReport, "OVERVIEW-BANK-2", "500.00")
	if _, err = s.DecideFundWaiver(ctx, reviewer, exemption, fundDecision("APPROVED", 1)); err != nil {
		t.Fatal(err)
	}
	final := mustOverview(t, s, admin, "collections")
	if final.Counts["confirmed_paise"] != 50000 || final.Counts["outstanding_paise"] != 120025 || final.Counts["overdue_paise"] != 120025 || final.Counts["pending_exemptions"] != 0 || len(final.Items) > 4 {
		t.Fatal("voluntary no debt and released prior links", final)
	}
	accessExec(t, s, "UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'")
	scoped := mustOverview(t, s, owner, "collections")
	if scoped.Counts["outstanding_paise"] != 50025 || scoped.Counts["confirmed_paise"] != 0 || scoped.Counts["changes_requested"] != 0 {
		t.Fatal("ended home overview", scoped)
	}
}
func TestFundDuplicateExternalClaimsOneReceiptExactRetryAndOriginalReversal(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	owner := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	campaign := createPublishedFund(t, s, admin, reviewer, fundProposal())
	claim := fundClaim(campaign.ID, "400.00", "FUND-REF-400")
	id := createFundClaim(t, s, owner, claim)
	if same, e := s.SaveFundReport(ctx, owner, "", claim); e != nil || same != id {
		t.Fatal("report retry", same, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	out := fundDetails(t, s, admin, campaign.ID)
	if out.AllocatedPaise != 0 || out.PendingReports != 1 || out.OutstandingPaise != 150025 {
		t.Fatal("claim counted as cash", out)
	}
	action := confirmClaim("FUND-REF-400", "400.00")
	if _, e := s.DecideFundReport(ctx, owner, id, action); !errors.Is(e, ErrForbidden) {
		t.Fatal("resident confirmed cash", e)
	}
	for i := 0; i < 2; i++ {
		if same, e := s.DecideFundReport(ctx, admin, id, action); e != nil || same != id {
			t.Fatal("confirmation retry", same, e)
		}
	}
	first := claimDetails(t, s, owner, id)
	if first.Receipt == "" || first.State != "CONFIRMED" || first.Version != 2 || first.EventTotal != 2 || first.EntryID == "" || first.AllocationID == "" {
		t.Fatal("confirmed original", first)
	}
	changed := action
	changed.AllocationAmount = "399.99"
	if _, e := s.DecideFundReport(ctx, admin, id, changed); !errors.Is(e, ErrConflict) {
		t.Fatal("changed confirmation retry", e)
	}
	secondID := createFundClaim(t, s, reviewer, fundClaim(campaign.ID, "400.00", "FUND-REF-400"))
	second := confirmFund(t, s, admin, secondID, "fund-ref-400", "400.00")
	if second.State != "DUPLICATE" || second.EntryID != first.EntryID || second.ReceiptID != first.ReceiptID || second.DuplicateReportID != id || second.AllocationID != "" {
		t.Fatal("duplicate created cash", second, first)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries WHERE kind='RECEIVED'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entry_allocations", 1)
	out = fundDetails(t, s, admin, campaign.ID)
	if out.AllocatedPaise != 40000 || out.OutstandingPaise != 110025 || out.PartialHomes != 1 || out.PendingReports != 0 {
		t.Fatal("independent partial", out)
	}
	if _, e := s.ReverseEntry(ctx, admin, first.EntryID, EntryAction{OperationKey: randomToken(), Reason: "Corrected fictional received source with original retained", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	corrected := claimDetails(t, s, owner, id)
	out = fundDetails(t, s, admin, campaign.ID)
	if corrected.State != "CONFIRMED" || corrected.CurrentState != "CORRECTED" || corrected.ReceiptID != first.ReceiptID || out.OutstandingPaise != 150025 || out.AllocatedPaise != 0 {
		t.Fatal("reversal erased original or retained live money", corrected, out)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
}
func TestFundExcessCreditVoluntaryAttributionAcrossPurposesAndCurrentScopes(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	owner := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	campaign := createPublishedFund(t, s, admin, reviewer, fundProposal())
	first := confirmFund(t, s, admin, createFundClaim(t, s, owner, fundClaim(campaign.ID, "400.00", "PART-400")), "PART-400", "400.00")
	second := confirmFund(t, s, admin, createFundClaim(t, s, owner, fundClaim(campaign.ID, "1200.00", "EXCESS-1200")), "EXCESS-1200", "600.00")
	out := fundDetails(t, s, admin, campaign.ID)
	if out.AllocatedPaise != 100000 || out.OutstandingPaise != 50025 || out.PaidHomes != 1 || out.UnpaidHomes != 1 {
		t.Fatal("another home's debt settled", out)
	}
	home := statement(t, s, owner, "demo-flat-A-101")
	if home.CreditPaise != 160000 || home.AllocatedPaise != 100000 || home.UnallocatedPaise != 60000 || home.OutstandingPaise != 0 {
		t.Fatal("excess credit", home)
	}
	if _, e := s.AllocateCredit(ctx, admin, CreditAllocationInput{randomToken(), second.EntryID, campaign.Lines[1].EntryID, "500.25", "Cannot transfer an unrelated home's credit", true}); !errors.Is(e, ErrInvalid) {
		t.Fatal("cross-home credit", e)
	}
	voluntary := fundProposal()
	voluntary.OperationKey = randomToken()
	voluntary.Title = "Fictional voluntary garden fund"
	voluntary.ContributionType = "VOLUNTARY"
	voluntary.Lines = []MaintenanceLineInput{{"demo-flat-A-101", ""}}
	voluntary.Target = "2500.00"
	garden := createPublishedFund(t, s, admin, reviewer, voluntary)
	if garden.ActivePaise != 0 || garden.OutstandingPaise != 0 || garden.OverduePaise != 0 || garden.RequestedPaise != 0 {
		t.Fatal("voluntary invented debt", garden)
	}
	attribution := FundAttributionInput{randomToken(), garden.ID, "demo-flat-A-101", second.EntryID, "500.00", "Reviewed voluntary purpose against usable original receipt", true}
	link, e := s.AttributeFundCredit(ctx, admin, attribution)
	if e != nil {
		t.Fatal(e)
	}
	if same, e := s.AttributeFundCredit(ctx, admin, attribution); e != nil || same != link {
		t.Fatal("attribution retry", same, e)
	}
	garden = fundDetails(t, s, owner, garden.ID)
	home = statement(t, s, owner, "demo-flat-A-101")
	if garden.VoluntaryPaise != 50000 || garden.PaidHomes != 1 || garden.OutstandingPaise != 0 || home.VoluntaryPaise != 50000 || home.UnallocatedPaise != 10000 || home.AllocatedPaise+home.VoluntaryPaise+home.UnallocatedPaise != home.CreditPaise {
		t.Fatal("shared usable credit", garden, home)
	}
	entries, err := s.EntriesFor(ctx, owner, "demo-flat-A-101", "", "", false, 1)
	if err != nil || entries.DebitPaise != 100000 || entries.CreditPaise != 160000 || entries.VoluntaryPaise != 50000 || entries.BalancePaise != -10000 {
		t.Fatal("purpose credit displayed as held home money", entries, err)
	}
	overview := mustOverview(t, s, owner, "finance")
	if overview.Counts["positive_balance_paise"] != 50025 || overview.Counts["credit_balance_paise"] != 10000 || overview.Counts["balance_paise"] != 40025 || overview.Counts["voluntary_paise"] != 50000 {
		t.Fatal("independent home overview balances", overview)
	}
	if _, e = s.AllocateCredit(ctx, admin, CreditAllocationInput{randomToken(), second.EntryID, campaign.Lines[0].EntryID, "100.01", "Cannot reuse credit already attributed to a fund", true}); !errors.Is(e, ErrConflict) {
		t.Fatal("voluntary credit reused", e)
	}
	correction := AllocationCorrection{randomToken(), "Corrected the explicitly supplied voluntary purpose", true}
	for i := 0; i < 2; i++ {
		if _, e = s.CorrectFundContribution(ctx, admin, link, correction); e != nil {
			t.Fatal(e)
		}
	}
	home = statement(t, s, owner, "demo-flat-A-101")
	garden = fundDetails(t, s, owner, garden.ID)
	if home.UnallocatedPaise != 60000 || home.VoluntaryPaise != 0 || garden.VoluntaryPaise != 0 {
		t.Fatal("attribution correction", home, garden)
	}
	contributions, e := s.FundContributionsFor(ctx, owner, garden.ID, "", 1)
	if e != nil || contributions.Total != 1 || contributions.Items[0].State != "CORRECTED" || contributions.Items[0].Reason != "" || contributions.Items[0].CorrectionReason != "" {
		t.Fatal("resident history leaked private reason", contributions, e)
	}
	if _, e = s.ReverseEntry(ctx, admin, first.EntryID, EntryAction{randomToken(), true, "Reversed original partial received money"}); e != nil {
		t.Fatal(e)
	}
	out = fundDetails(t, s, admin, campaign.ID)
	if out.OutstandingPaise != 90025 {
		t.Fatal("independent reversal outstanding", out)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 2)
}
func TestFundLinksCompatibleManualReceiptsAndRevisesUnconfirmedClaims(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	owner := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	campaign := createPublishedFund(t, s, admin, reviewer, fundProposal())
	entryID := post(t, s, admin, received("400.00"))
	id := createFundClaim(t, s, owner, fundClaim(campaign.ID, "400.00", "DEMO-REF-101"))
	decision := confirmClaim("CASHBOOK-BANK-101", "400.00")
	if _, e := s.DecideFundReport(ctx, admin, id, decision); !errors.Is(e, ErrConflict) {
		t.Fatal("matching manual receipt duplicated", e)
	}
	options, e := s.FundConfirmOptionsFor(ctx, admin, id)
	if e != nil || len(options.Entries) != 1 || options.Entries[0].ID != entryID || options.Entries[0].RemainingPaise != 40000 {
		t.Fatal("compatible original options", options, e)
	}
	decision.OperationKey = randomToken()
	decision.Mode = "LINK"
	decision.EntryID = entryID
	if _, e = s.DecideFundReport(ctx, admin, id, decision); e != nil {
		t.Fatal(e)
	}
	result := claimDetails(t, s, owner, id)
	if result.EntryID != entryID {
		t.Fatal("linked another entry", result)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	revisedID := createFundClaim(t, s, owner, fundClaim(campaign.ID, "100.00", "NEEDS-CHECK-100"))
	request := FundReportDecision{OperationKey: randomToken(), Version: 1, Action: "NEEDS_INFO", Reason: "Please check the supplied reference against the external source", Confirmed: true}
	if _, e = s.DecideFundReport(ctx, admin, revisedID, request); e != nil {
		t.Fatal(e)
	}
	revision := fundClaim(campaign.ID, "125.25", "CHECKED-REF-12525")
	revision.Version = 2
	if _, e = s.SaveFundReport(ctx, reviewer, revisedID, revision); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("another actor revised claim", e)
	}
	if _, e = s.SaveFundReport(ctx, owner, revisedID, revision); e != nil {
		t.Fatal(e)
	}
	edited := claimDetails(t, s, owner, revisedID)
	if edited.AmountPaise != 12525 || edited.State != "PENDING" || edited.Version != 3 || edited.EventTotal != 3 {
		t.Fatal("revision lost old claim", edited)
	}
	close := fundDecision("CLOSED", 2)
	if _, e = s.DecideFundCampaign(ctx, admin, campaign.ID, close); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveFundReport(ctx, owner, "", fundClaim(campaign.ID, "100.00", "CLOSED-REF")); !errors.Is(e, ErrInvalid) {
		t.Fatal("new closed claim", e)
	}
	confirm := confirmClaim("CHECKED-REF-12525", "125.25")
	confirm.Version = 3
	if _, e = s.DecideFundReport(ctx, admin, revisedID, confirm); e != nil {
		t.Fatal("existing claim on closed campaign", e)
	}
	if _, e = s.DecideFundCampaign(ctx, admin, campaign.ID, fundDecision("REOPEN", 3)); e != nil {
		t.Fatal(e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 2)
}
func TestFundSeparatePartialAndFullExemptionsPreserveReceiptsAndReleaseLinks(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	owner := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	campaign := createPublishedFund(t, s, admin, reviewer, fundProposal())
	claim := confirmFund(t, s, admin, createFundClaim(t, s, owner, fundClaim(campaign.ID, "400.00", "WAIVER-PART-400")), "WAIVER-PART-400", "400.00")
	in := FundWaiverInput{randomToken(), campaign.ID, "demo-flat-A-101", 1, "300.00", "PRIVATE supplied exemption approval EX-1", "Reviewed a supplied partial exemption with separate decision", true}
	id, e := s.ProposeFundWaiver(ctx, admin, in)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideFundWaiver(ctx, admin, id, fundDecision("APPROVED", 1)); !errors.Is(e, ErrForbidden) {
		t.Fatal("self exemption", e)
	}
	if out := fundDetails(t, s, owner, campaign.ID); out.WaivedPaise != 0 || out.OutstandingPaise != 110025 {
		t.Fatal("proposal changed money", out)
	}
	action := fundDecision("APPROVED", 1)
	for i := 0; i < 2; i++ {
		if _, e = s.DecideFundWaiver(ctx, reviewer, id, action); e != nil {
			t.Fatal(e)
		}
	}
	out := fundDetails(t, s, owner, campaign.ID)
	home := statement(t, s, owner, "demo-flat-A-101")
	if out.WaivedPaise != 30000 || out.ActivePaise != 120025 || out.OutstandingPaise != 120025 || home.UnallocatedPaise != 40000 || home.DebitPaise != 70000 || out.Lines[0].OriginalEntryID != campaign.Lines[0].OriginalEntryID {
		t.Fatal("partial exemption correction", out, home)
	}
	corrected := claimDetails(t, s, owner, claim.ID)
	if corrected.ReceiptID != claim.ReceiptID || corrected.CurrentState != "CORRECTED" {
		t.Fatal("exemption changed receipt", corrected)
	}
	allocate(t, s, admin, claim.EntryID, out.Lines[0].EntryID, "400.00")
	home = statement(t, s, owner, "demo-flat-A-101")
	if home.OutstandingPaise != 30000 || home.UnallocatedPaise != 0 {
		t.Fatal("explicit reallocation", home)
	}
	in.OperationKey = randomToken()
	in.ParticipantVersion = 2
	in.Amount = "700.00"
	in.SourceReference = "PRIVATE approved full exemption EX-2"
	full, e := s.ProposeFundWaiver(ctx, admin, in)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideFundWaiver(ctx, reviewer, full, fundDecision("APPROVED", 1)); e != nil {
		t.Fatal(e)
	}
	out = fundDetails(t, s, owner, campaign.ID)
	home = statement(t, s, owner, "demo-flat-A-101")
	if out.WaivedPaise != 100000 || out.ExemptHomes != 1 || out.ActivePaise != 50025 || out.Lines[0].Status != "EXEMPT" || home.UnallocatedPaise != 40000 || home.OutstandingPaise != 0 {
		t.Fatal("full exemption", out, home)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	if _, e = s.FundWaiverFor(ctx, owner, id, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("resident private waiver source", e)
	}
}
func TestFundPrivateValidatedEvidenceCurrentHomeAndOtherReporterDenial(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	owner := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	campaign := createPublishedFund(t, s, admin, reviewer, fundProposal())
	doc, bytes := documentFixtureInput()
	doc.Category = "PAYMENT_EVIDENCE"
	doc.Visibility = "FLAT_SPECIFIC"
	doc.FlatID = "demo-flat-A-101"
	evidence := checkedLibrary(t, s, owner, doc, bytes)
	input := fundClaim(campaign.ID, "25.00", "PRIVATE-PICTURE-CLAIM")
	input.EvidenceID = evidence
	id := createFundClaim(t, s, owner, input)
	if _, got, e := s.FundReportEvidenceFor(ctx, admin, id); e != nil || string(got) != string(bytes) {
		t.Fatal("validated private original", e)
	}
	other := createFundClaim(t, s, reviewer, fundClaim(campaign.ID, "25.00", "OTHER-PERSON-CLAIM"))
	if _, e := s.FundReportFor(ctx, owner, other, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("same-home other reporter", e)
	}
	page, e := s.FundReportsFor(ctx, owner, "", campaign.ID, "", "", 1)
	if e != nil || page.Total != 1 || page.Items[0].ID != id {
		t.Fatal("other reports in count", page, e)
	}
	resident := fundDetails(t, s, owner, campaign.ID)
	blob, _ := json.Marshal(resident)
	if strings.Contains(string(blob), "PRIVATE") || resident.EventTotal != 0 {
		t.Fatal("private campaign source", string(blob))
	}
	doc2, _ := documentFixtureInput()
	doc2.Category = "PAYMENT_EVIDENCE"
	doc2.Visibility = "FLAT_SPECIFIC"
	doc2.FlatID = "demo-flat-A-101"
	unchecked, e := s.ReserveDocument(ctx, owner, doc2)
	if e != nil {
		t.Fatal(e)
	}
	bad := fundClaim(campaign.ID, "25.00", "UNCHECKED-CLAIM")
	bad.EvidenceID = unchecked
	if _, e = s.SaveFundReport(ctx, owner, "", bad); !errors.Is(e, ErrInvalid) {
		t.Fatal("unchecked evidence", e)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'", today())
	if _, e = s.FundReportFor(ctx, owner, id, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("ended home report", e)
	}
	if _, _, e = s.FundReportEvidenceFor(ctx, owner, id); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("ended home evidence", e)
	}
	scoped := fundDetails(t, s, owner, campaign.ID)
	if scoped.Participants != 1 || scoped.RequestedPaise != 50025 || scoped.Lines[0].Home != "A-102" {
		t.Fatal("remaining home current scope", scoped)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}
func TestFundConcurrentConfirmationAndCreditUseHaveOneWinner(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	owner := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	campaign := createPublishedFund(t, s, admin, reviewer, fundProposal())
	id := createFundClaim(t, s, owner, fundClaim(campaign.ID, "400.00", "CONCURRENT-400"))
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, token := range []string{admin, reviewer} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			<-start
			_, e := s.DecideFundReport(ctx, token, id, confirmClaim("CONCURRENT-400", "0.00"))
			results <- e
		}(token)
	}
	close(start)
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for e := range results {
		if e == nil {
			wins++
		} else if errors.Is(e, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("confirmation race", wins, conflicts)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	report := claimDetails(t, s, admin, id)
	vol := fundProposal()
	vol.OperationKey = randomToken()
	vol.ContributionType = "VOLUNTARY"
	vol.Title = "Fictional concurrent voluntary fund"
	vol.Lines = []MaintenanceLineInput{{"demo-flat-A-101", ""}}
	garden := createPublishedFund(t, s, admin, reviewer, vol)
	start = make(chan struct{})
	results = make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, e := s.AttributeFundCredit(ctx, admin, FundAttributionInput{randomToken(), garden.ID, "demo-flat-A-101", report.EntryID, "250.00", "Explicit voluntary attribution from one usable receipt", true})
		results <- e
	}()
	go func() {
		defer wg.Done()
		<-start
		_, e := s.AllocateCredit(ctx, reviewer, CreditAllocationInput{randomToken(), report.EntryID, campaign.Lines[0].EntryID, "250.00", "Explicit supplied charge allocation from same receipt", true})
		results <- e
	}()
	close(start)
	wg.Wait()
	close(results)
	wins, conflicts = 0, 0
	for e := range results {
		if e == nil {
			wins++
		} else if errors.Is(e, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("shared usable credit race", wins, conflicts)
	}
	home := statement(t, s, admin, "demo-flat-A-101")
	if home.UnallocatedPaise != 15000 || home.AllocatedPaise+home.VoluntaryPaise != 25000 {
		t.Fatal("independent shared source total", home)
	}
	accessExec(t, s, "UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-admin' AND role='TREASURER'", time.Now().Unix()-60, time.Now().Unix()-1)
	if _, e := s.DecideFundReport(ctx, admin, id, confirmClaim("CONCURRENT-400", "0.00")); !errors.Is(e, ErrForbidden) {
		t.Fatal("expired confirmation role", e)
	}
}

func TestFundDistinctCashSourcesBoundedClaimPagesAndFactorRequiredOnReplay(t *testing.T) {
	s, admin := recordFixture(t)
	reviewer := maintenanceReviewer(t, s, admin)
	owner := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	campaign := createPublishedFund(t, s, admin, reviewer, fundProposal())
	for _, identity := range []string{"CASHBOOK-1", "CASHBOOK-2"} {
		in := fundClaim(campaign.ID, "25.00", "")
		in.Method = "CASH"
		id := createFundClaim(t, s, owner, in)
		confirmFund(t, s, admin, id, identity, "25.00")
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 2)
	out := fundDetails(t, s, owner, campaign.ID)
	if out.AllocatedPaise != 5000 || out.OutstandingPaise != 145025 {
		t.Fatal("distinct cash sources merged", out)
	}
	self := fundClaim(campaign.ID, "25.00", "SELF-REPORT")
	id := createFundClaim(t, s, admin, self)
	action := confirmClaim("SELF-REPORT", "25.00")
	if _, e := s.DecideFundReport(ctx, admin, id, action); !errors.Is(e, ErrForbidden) {
		t.Fatal("treasurer confirmed own report", e)
	}
	if _, e := s.DecideFundReport(ctx, reviewer, id, action); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 13; i++ {
		in := fundClaim(campaign.ID, "1.00", "PENDING-"+randomToken())
		createFundClaim(t, s, owner, in)
	}
	page, e := s.FundReportsFor(ctx, owner, "", campaign.ID, "", "PENDING", 99)
	if e != nil || page.Total != 13 || page.Page != 2 || len(page.Items) != 1 || page.Counts["PENDING"] != 13 {
		t.Fatal("filtered scoped pagination", page, e)
	}
	if _, e := s.FundReportsFor(ctx, owner, "", campaign.ID, "", "UNKNOWN", 1); !errors.Is(e, ErrInvalid) {
		t.Fatal("unsupported claim state", e)
	}
	accessExec(t, s, "DELETE FROM mfa_factors WHERE user_id='demo-user-committee'")
	if _, e := s.DecideFundReport(ctx, reviewer, id, action); !errors.Is(e, ErrMFARequired) {
		t.Fatal("factorless successful replay", e)
	}
}
