package database

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

const testOffsiteBinding = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func TestOffsiteUnconfiguredNonFictionalWorkspacePausesReceiptIssuanceUntilProtectionIsVerified(t *testing.T) {
	s, token := workspaceFixtureMode(t, "LOCAL_WORKSPACE")
	ctx := context.Background()
	status, err := s.OffsiteStatusFor(ctx, token, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if status.Configured || status.Phase != "NOT_CONFIGURED" || !status.FinancePaused {
		t.Errorf("Unconfigured non-fictional protection status incorrectly permits financial changes: phase=%s paused=%v", status.Phase, status.FinancePaused)
	}
	if _, err := s.DB.Exec("INSERT INTO receipt_counter(year,next_number) VALUES('2026',1)"); !IsBackupProtectionError(err) {
		t.Fatal("Unconfigured non-fictional workspace reserved a receipt identity", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipt_counter", 0)
	if err := s.BindOffsitePolicy(ctx, testOffsiteBinding, "FIXTURE", time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatal("A local fixture was accepted as non-fictional off-site protection", err)
	}
	if err := s.BindOffsitePolicy(ctx, testOffsiteBinding, "S3", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("INSERT INTO receipt_counter(year,next_number) VALUES('2026',1)"); !IsBackupProtectionError(err) {
		t.Fatal("Configuration without a usable copy reserved a receipt identity", err)
	}
	now := time.Now()
	offsiteAcknowledgement(t, s, now, now)
	if _, err := s.DB.Exec("INSERT INTO receipt_counter(year,next_number) VALUES('2026',1)"); err != nil {
		t.Fatal("Fresh verified protection did not release the same financial boundary", err)
	}
	status, err = s.OffsiteStatusFor(ctx, token, "", 1)
	if err != nil || status.FinancePaused || status.Phase != "HEALTHY" {
		t.Fatal("Verified non-fictional workspace remained paused", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipt_counter", 1)
}

func bindOffsite(t *testing.T, s *Store, at time.Time) {
	t.Helper()
	if err := s.BindOffsitePolicy(context.Background(), testOffsiteBinding, "FIXTURE", at); err != nil {
		t.Fatal(err)
	}
}

// Domain acknowledgement metadata comes from the storage boundary. Its real
// signature, bytes, encryption and independent custody checks have separate
// integration tests; this helper supplies a controlled capture/ack clock.
func offsiteAcknowledgement(t *testing.T, s *Store, captured, acknowledged time.Time) OffsiteJob {
	t.Helper()
	ctx := context.Background()
	j, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", captured)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SealOffsite(ctx, j, strings.Repeat("c", 64), 128, captured); err != nil {
		t.Fatal(err)
	}
	if err := s.BeginOffsiteUpload(ctx, j, captured); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishOffsite(ctx, j, "VERIFIED", "", "controlled-original-version", acknowledged); err != nil {
		t.Fatal(err)
	}
	return j
}

func moneyRows(t *testing.T, s *Store) map[string]string {
	t.Helper()
	rows, err := s.DB.Query(`SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'offsite_%' AND name NOT IN('sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	return statementMessageRows(t, s, names)
}

func refusedFinancialEffect(t *testing.T, s *Store, action func() error) {
	t.Helper()
	before := moneyRows(t, s)
	if err := action(); !IsBackupProtectionError(err) {
		t.Fatal("unprotected financial effect was not refused", err)
	}
	if !reflect.DeepEqual(before, moneyRows(t, s)) {
		t.Fatal("refused effect changed business rows, audit, child records or saved operation identity")
	}
}

func TestOffsiteManualPostingPreservesIndependentAmountsDraftsOriginalRepliesAndRevocation(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	post(t, s, a, supplied("OPENING_DEBIT", "1000.00"))
	paidInput := received("400.00")
	paid, err := s.CreateEntry(ctx, a, paidInput)
	if err != nil {
		t.Fatal(err)
	}
	accepted := EntryAction{OperationKey: randomToken(), Confirmed: true}
	if _, err := s.PostEntry(ctx, a, paid, accepted); err != nil {
		t.Fatal(err)
	}
	original, _ := s.EntryFor(ctx, a, paid)
	bindOffsite(t, s, time.Now())
	charge, err := s.CreateEntry(ctx, a, supplied("CHARGE", "500.01"))
	if err != nil {
		t.Fatal("draft blocked while protection unavailable", err)
	}
	action := EntryAction{OperationKey: randomToken(), Confirmed: true}
	refusedFinancialEffect(t, s, func() error { _, err := s.PostEntry(ctx, a, charge, action); return err })
	refusedFinancialEffect(t, s, func() error {
		_, err := s.ReverseEntry(ctx, a, paid, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Verified fictional linked correction"})
		return err
	})
	before := moneyRows(t, s)
	if same, err := s.PostEntry(ctx, a, paid, accepted); err != nil || same != paid || !reflect.DeepEqual(before, moneyRows(t, s)) {
		t.Fatal("exact saved reply failed or reissued money while paused", err)
	}
	if _, err := s.PostEntry(ctx, a, paid, EntryAction{OperationKey: accepted.OperationKey, Confirmed: true, Reason: "Changed"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("changed acknowledged request accepted", err)
	}
	x, err := s.HouseholdFinanceFor(ctx, a, "demo-flat-A-101", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeHouseholdFinance(ctx, a, "demo-flat-A-101", HouseholdFinanceInput{OperationKey: randomToken(), Version: x.Version, PersonID: "demo-owner-A-101", Action: "REVOKE", Confirmed: true, Note: "Verified fictional current household finance revocation"}); err != nil {
		t.Fatal("security revocation blocked by financial protection", err)
	}
	ledger, _ := s.EntriesFor(ctx, a, "demo-flat-A-101", "", "", false, 1)
	if ledger.BalancePaise != 60000 || ledger.Drafts != 1 {
		t.Fatal("paused draft affected independent balance", ledger.BalancePaise, ledger.Drafts)
	}
	offsiteAcknowledgement(t, s, time.Now(), time.Now())
	if _, err := s.PostEntry(ctx, a, charge, action); err != nil {
		t.Fatal("same frozen operation failed after protection restored", err)
	}
	ledger, _ = s.EntriesFor(ctx, a, "demo-flat-A-101", "", "", false, 1)
	if ledger.BalancePaise != 110001 {
		t.Fatal("restored protection changed independent paise expectation", ledger.BalancePaise)
	}
	if _, err := s.ReverseEntry(ctx, a, paid, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Verified correction after protection restored"}); err != nil {
		t.Fatal(err)
	}
	current, _ := s.EntryFor(ctx, a, paid)
	ledger, _ = s.EntriesFor(ctx, a, "demo-flat-A-101", "", "", false, 1)
	if ledger.BalancePaise != 150001 || current.ReceiptNumber != original.ReceiptNumber || current.AmountPaise != 40000 {
		t.Fatal("original receipt or independent correction balance changed")
	}
}

func TestOffsiteCompositeChargesAllocationsContributionsReportsExemptionsAndPaidExpensesRollBack(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	opening := post(t, s, a, supplied("OPENING_DEBIT", "1000.00"))
	paid := post(t, s, a, received("400.00"))
	allocation := allocate(t, s, a, paid, opening, "50.00")
	fixed := createPublishedFund(t, s, a, b, fundProposal())
	vol := fundProposal()
	vol.OperationKey, vol.ContributionType = randomToken(), "VOLUNTARY"
	vol.Lines = []MaintenanceLineInput{{FlatID: "demo-flat-A-101"}}
	voluntary := createPublishedFund(t, s, a, b, vol)
	contribution, err := s.AttributeFundCredit(ctx, a, FundAttributionInput{randomToken(), voluntary.ID, "demo-flat-A-101", paid, "10.00", "Verified fictional purpose against original receipt", true})
	if err != nil {
		t.Fatal(err)
	}
	plan := budgetPropose(t, s, a, "", budgetProposal())
	budgetApprove(t, s, b, plan)
	expense := expensePropose(t, s, a, "", expenseProposal(plan, "32.51", "OFFSITE-PAID-001"))
	bindOffsite(t, s, time.Now())
	cycle, err := s.CreateMaintenanceCycle(ctx, a, maintenanceProposal())
	if err != nil {
		t.Fatal("maintenance proposal blocked", err)
	}
	fund, err := s.CreateFundCampaign(ctx, a, fundProposal())
	if err != nil {
		t.Fatal("fund proposal blocked", err)
	}
	report := createFundClaim(t, s, b, fundClaim(fixed.ID, "25.00", "OFFSITE-CLAIM-001"))
	linkedReport := createFundClaim(t, s, b, fundClaim(fixed.ID, "400.00", "DEMO-REF-101"))
	exemption, err := s.ProposeFundWaiver(ctx, a, FundWaiverInput{OperationKey: randomToken(), CampaignID: fixed.ID, FlatID: "demo-flat-A-101", ParticipantVersion: 1, Amount: "300.00", SourceReference: "Verified fictional exemption reference", Reason: "Independently supplied exemption awaiting separate decision", Confirmed: true})
	if err != nil {
		t.Fatal("exemption proposal blocked", err)
	}
	cases := []struct {
		name string
		run  func() error
	}{
		{"maintenance charges", func() error {
			_, err := s.DecideMaintenanceCycle(ctx, b, cycle, maintenanceDecision("PUBLISHED"))
			return err
		}},
		{"fixed-fund charges", func() error { _, err := s.DecideFundCampaign(ctx, b, fund, fundDecision("PUBLISHED", 1)); return err }},
		{"allocation", func() error {
			_, err := s.AllocateCredit(ctx, a, CreditAllocationInput{randomToken(), paid, opening, "25.00", "Verified fictional current same-home allocation", true})
			return err
		}},
		{"allocation reversal", func() error {
			_, err := s.ReverseAllocation(ctx, a, allocation, AllocationCorrection{randomToken(), "Verified fictional linked allocation correction", true})
			return err
		}},
		{"voluntary attribution", func() error {
			_, err := s.AttributeFundCredit(ctx, a, FundAttributionInput{randomToken(), voluntary.ID, "demo-flat-A-101", paid, "15.00", "Verified fictional remaining original credit", true})
			return err
		}},
		{"contribution reversal", func() error {
			_, err := s.CorrectFundContribution(ctx, a, contribution, AllocationCorrection{randomToken(), "Verified fictional voluntary correction", true})
			return err
		}},
		{"new verified external receipt", func() error {
			_, err := s.DecideFundReport(ctx, a, report, confirmClaim("OFFSITE-EXTERNAL-001", "25.00"))
			return err
		}},
		{"linked report without new allocation", func() error {
			in := confirmClaim("OFFSITE-LINK-001", "0.00")
			in.Mode, in.EntryID = "LINK", paid
			_, err := s.DecideFundReport(ctx, a, linkedReport, in)
			return err
		}},
		{"separate fund exemption", func() error { _, err := s.DecideFundWaiver(ctx, b, exemption, fundDecision("APPROVED", 1)); return err }},
		{"accepted already-paid expense", func() error {
			_, err := s.DecidePaidExpense(ctx, b, expense, budgetDecision(expenseDetail(t, s, b, expense).Version, "APPROVED"))
			return err
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { refusedFinancialEffect(t, s, c.run) })
	}
	// A decline and an expense proposal change no confirmed money and remain usable.
	if _, err := s.DecideMaintenanceCycle(ctx, b, cycle, maintenanceDecision("DECLINED")); err != nil {
		t.Fatal("non-financial decline blocked", err)
	}
	if _, err := s.DecidePaidExpense(ctx, b, expense, budgetDecision(expenseDetail(t, s, b, expense).Version, "DECLINED")); err != nil {
		t.Fatal("non-financial expense decline blocked", err)
	}
}

func TestOffsiteFineIssueAndLinkedWaiverCannotPartiallyPost(t *testing.T) {
	s, a, b, _, _, incident := fineFixture(t, "demo-flat-A-102")
	ctx := context.Background()
	reviewer := b
	principal, err := s.CheckSession(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if !principal.CanManageRecords {
		reviewer = maintenanceReviewer(t, s, a)
	}
	first := proposedFine(t, s, a, incident)
	notifyFine(t, s, a, b, first)
	x := issueFine(t, s, a, reviewer, first)
	fineReport, err := s.SaveFineReport(ctx, reviewLogin(t, s, "owner@demo.society"), "", fineReportInput(first))
	if err != nil {
		t.Fatal(err)
	}
	waiver, err := s.ProposeFineWaiver(ctx, a, FineWaiverInput{OperationKey: randomToken(), FineID: first, ChargeVersion: x.ChargeVersion, Kind: "WAIVER", Amount: "75.25", PolicyReference: "Fictional verified partial exemption", Reason: "Separately supplied precise fictional partial exemption", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	bindOffsite(t, s, time.Now())
	decision := FineChildAction{OperationKey: randomToken(), Version: 1, FineVersion: fineDetails(t, s, a, first).Version, Action: "APPROVED", Reason: "Independently verified fictional partial exemption", Confirmed: true}
	refusedFinancialEffect(t, s, func() error { _, err := s.DecideFineWaiver(ctx, reviewer, waiver, decision); return err })
	refusedFinancialEffect(t, s, func() error {
		_, err := s.DecideFineReport(ctx, reviewer, fineReport, confirmClaim("OFFSITE-FINE-EXTERNAL-001", "100.00"))
		return err
	})
	// A second substantiated incident follows ordinary reporting/review controls.
	rule := publishIncidentRule(t, s, a, b)
	in := incidentInput(rule)
	in.FlatID = "demo-flat-A-102"
	owner := reviewLogin(t, s, "owner@demo.society")
	otherIncident, err := s.SaveIncident(ctx, owner, "", in)
	if err != nil {
		t.Fatal(err)
	}
	incidentAction(t, s, b, otherIncident, "SUBSTANTIATED")
	second := proposedFine(t, s, a, otherIncident)
	notifyFine(t, s, a, b, second)
	resolved := resolveFine(t, s, a, reviewer, second)
	issue := fineActionInput(resolved, "ISSUE")
	issue.ResolutionKey = resolved.ResolutionKey
	refusedFinancialEffect(t, s, func() error { _, err := s.ActOnFine(ctx, reviewer, second, issue); return err })
}

func TestOffsiteConcurrentClaimsExpiredHandoffsAndRetainedIdentity(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	start := time.Now().Add(-time.Hour)
	bindOffsite(t, s, start)
	var wg sync.WaitGroup
	results := make(chan OffsiteJob, 4)
	errorsSeen := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", start)
			results <- j
			errorsSeen <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errorsSeen)
	var claimed OffsiteJob
	for j := range results {
		if j.ID != "" {
			if claimed.ID != "" {
				t.Fatal("multiple workers claimed a new recovery point")
			}
			claimed = j
		}
	}
	for err := range errorsSeen {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Fatal(err)
		}
	}
	if claimed.ID == "" {
		t.Fatal("no recovery point claimed")
	}
	s.SealOffsite(ctx, claimed, strings.Repeat("c", 64), 128, start.Add(time.Second))
	s.BeginOffsiteUpload(ctx, claimed, start.Add(time.Second))
	if _, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", start.Add(OffsiteLease+time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expired upload automatically replayed", err)
	}
	expired, _ := s.OffsiteJobForCustody(ctx, claimed.ID)
	if expired.State != "UNCERTAIN" || expired.SHA256 != strings.Repeat("c", 64) || expired.ObjectKey != claimed.ObjectKey || expired.CapturedAt != claimed.CapturedAt {
		t.Fatal("expired handoff identity lost")
	}
	if err := s.FinishOffsite(ctx, claimed, "VERIFIED", "", "late-worker-version", start.Add(OffsiteLease+time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatal("stale worker published after its lease expired", err)
	}
	if _, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", start.Add(-time.Second)); !errors.Is(err, ErrBackupProtection) {
		t.Fatal("clock rollback silently accepted", err)
	}
	for _, statement := range []string{"DELETE FROM offsite_policy", "DELETE FROM offsite_backup_jobs", "UPDATE offsite_backup_events SET note='changed'", "DELETE FROM offsite_backup_events"} {
		if _, err := s.DB.Exec(statement); err == nil {
			t.Fatal("off-site binding or history erased", statement)
		}
	}
}

func TestOffsiteExpiredSealedFifthAttemptCannotExceedItsBudget(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	start := time.Now().Add(-time.Hour)
	bindOffsite(t, s, start)
	var original OffsiteJob
	for attempt := 1; attempt <= 5; attempt++ {
		now := start.Add(time.Duration(attempt-1) * time.Minute)
		j, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", now)
		if err != nil || j.Attempts != attempt {
			t.Fatal("Capture retry budget", attempt, err)
		}
		if original.ID == "" {
			original = j
		}
		if j.ID != original.ID || j.CapturedAt != original.CapturedAt {
			t.Fatal("Retry invented a new capture")
		}
		if attempt < 5 {
			if err := s.FinishOffsite(ctx, j, "FAILED", "CAPTURE_FAILED", "", now); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := s.SealOffsite(ctx, j, strings.Repeat("c", 64), 128, now); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", start.Add(10*time.Minute)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("An expired sealed fifth attempt exceeded the budget or blocked the scheduler", err)
	}
	current, err := s.OffsiteJobForCustody(ctx, original.ID)
	if err != nil || current.State != "FAILED" || current.Attempts != 5 || current.SHA256 != strings.Repeat("c", 64) || current.ErrorCode != "LEASE_EXPIRED" {
		t.Fatal("Expired sealed original was lost or misleadingly left in progress", err)
	}
	next, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", start.Add(31*time.Minute))
	if err != nil || next.ID == original.ID || next.Attempts != 1 {
		t.Fatal("The next scheduled capture could not proceed independently", err)
	}
}

func TestOffsiteFourUnconfirmedSealedCopiesBoundStagingWithoutResending(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	start := time.Now().Add(-3 * time.Hour)
	bindOffsite(t, s, start)
	for i := range 4 {
		now := start.Add(time.Duration(i) * 31 * time.Minute)
		j, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", now)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SealOffsite(ctx, j, strings.Repeat("c", 64), 128, now); err != nil {
			t.Fatal(err)
		}
		if err := s.BeginOffsiteUpload(ctx, j, now); err != nil {
			t.Fatal(err)
		}
		if err := s.FinishOffsite(ctx, j, "UNCERTAIN", "UPLOAD_UNCERTAIN", "", now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", start.Add(4*31*time.Minute)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("Retained uncertain staging was exceeded or replayed", err)
	}
	var count int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM offsite_backup_jobs WHERE state='UNCERTAIN' AND attempts=1 AND acknowledged_at=0").Scan(&count); err != nil || count != 4 {
		t.Fatal("Uncertain originals changed under capacity pressure", count, err)
	}
}

func TestOffsiteProtectionAgesTheCaptureRatherThanLaterUploadAndHidesPrivateMetadata(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	start := time.Now().Add(-time.Hour - 10*time.Second)
	bindOffsite(t, s, start)
	// Upload acknowledgement can be delayed beyond a worker lease only through
	// independent custody reconciliation, preserving the original capture time.
	j, err := s.ClaimOffsite(ctx, testOffsiteBinding, "recovery/society", start)
	if err != nil {
		t.Fatal(err)
	}
	s.SealOffsite(ctx, j, strings.Repeat("c", 64), 128, start.Add(time.Second))
	s.BeginOffsiteUpload(ctx, j, start.Add(time.Second))
	s.FinishOffsite(ctx, j, "UNCERTAIN", "UPLOAD_UNCERTAIN", "", start.Add(2*time.Second))
	if err := s.ReconcileOffsite(ctx, j.ID, strings.Repeat("c", 64), 128, "independently-verified-old-version", "Independent exact-version decrypt verified the original old snapshot", time.Now()); err != nil {
		t.Fatal(err)
	}
	status, err := s.OffsiteStatusFor(ctx, a, "", 1)
	if err != nil || status.Phase != "PAUSED" || !status.FinancePaused || status.AgeSeconds == nil || *status.AgeSeconds < 3600 {
		t.Fatal("late upload manufactured a fresh recovery point", status, err)
	}
	maintenanceCount(t, s, "SELECT allowed FROM offsite_financial_protection", 0)
	owner := reviewLogin(t, s, "owner@demo.society")
	if _, err := s.OffsiteStatusFor(ctx, owner, "", 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("resident read global recovery metadata", err)
	}
	if _, err := s.OffsiteDetailFor(ctx, owner, j.ID, 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("resident read recovery/custody history", err)
	}
	if _, err := s.OffsiteStatusFor(ctx, a, "SECRET", 1); !errors.Is(err, ErrInvalid) {
		t.Fatal("unsupported filter accepted", err)
	}
	if _, err := s.OffsiteStatusFor(ctx, a, "", 10001); !errors.Is(err, ErrInvalid) {
		t.Fatal("unbounded history accepted", err)
	}
}

func TestOffsiteExactWarningPauseAndClockBoundaries(t *testing.T) {
	const now = int64(100000)
	for _, c := range []struct {
		age    int64
		phase  string
		paused bool
	}{{2699, "HEALTHY", false}, {2700, "WARNING", false}, {3599, "WARNING", false}, {3600, "PAUSED", true}, {3601, "PAUSED", true}} {
		phase, paused, age := offsitePhase(now, now, now-c.age, now-1)
		if phase != c.phase || paused != c.paused || age == nil || *age != c.age {
			t.Fatal("independent 45/60-minute boundary", c, phase, paused)
		}
	}
	for _, args := range [][3]int64{{now + 1, now - 10, now - 1}, {now, now + 1, now + 1}, {now, now - 10, now + 1}} {
		phase, paused, _ := offsitePhase(now, args[0], args[1], args[2])
		if phase != "CLOCK_ERROR" || !paused {
			t.Fatal("clock inconsistency described as protected")
		}
	}
	phase, paused, age := offsitePhase(now, now, 0, 0)
	if phase != "PAUSED" || !paused || age != nil {
		t.Fatal("missing acknowledgement described as a usable copy")
	}
}
