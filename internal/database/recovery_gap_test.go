package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func suppliedGap(t *testing.T, s *Store, token string) RecoveryGapInput {
	t.Helper()
	p, err := s.CheckSession(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	entry := received("250.01")
	return RecoveryGapInput{
		OperationKey: randomToken(), ScopeKey: p.ScopeKey, Title: "Supplied financial recovery comparison",
		Reason:       "Independent current book records supplied for separate fictional review.",
		CoverageFrom: now - 86400, CoverageThrough: now - 1, VerifiedAt: now - 1,
		CoverageEvidence: "Supplied fictional book covers the declared records interval; source identities and missing items checked.", CoverageComplete: true,
		Items: []RecoveryGapItem{{BookRef: "fictional-account-book-2026", ItemRef: "supplied-received-item-17", FlatID: entry.FlatID, Kind: "RECEIVED", Amount: "250.01", Date: entry.Date, Evidence: "Independent fictional book confirms this already-received account amount.", EvidenceState: "SUPPORTED"}},
	}
}

func TestRecoveryGapSupportedUnmatchedEvidenceRemainsUnresolvedAfterAssessment(t *testing.T) {
	s, _, one, two, paid := gapMoneyFixture(t)
	ctx := context.Background()
	before := moneyRows(t, s)
	var auditCount int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	in := suppliedGap(t, s, one)
	in.Items = []RecoveryGapItem{
		{BookRef: "fictional-checked-fund-book", ItemRef: "supported-but-unreconstructed-attribution", Kind: "EVIDENCE", Evidence: "The supplied book is checked but its missing fund allocation has not been reconstructed.", EvidenceState: "SUPPORTED"},
		{BookRef: "fictional-original-book", ItemRef: "matched-preserved-receipt", FlatID: "demo-flat-A-101", Kind: "EVIDENCE", Amount: "400.00", EntryRef: paid, Evidence: "This existing corrected checkpoint entry is independently matched without adding money.", EvidenceState: "SUPPORTED"},
	}
	x, err := s.ProposeRecoveryGap(ctx, one, in)
	if err != nil {
		t.Fatal(err)
	}
	if x.Comparison.KnownItems != 0 || x.Comparison.UnresolvedItems != 1 || x.Comparison.CoverageUnresolved || len(x.Comparison.Matches) != 1 {
		t.Fatal("Checked supporting records cannot conceal an unmatched, unreconstructed financial item", x.Comparison)
	}
	if len(x.Comparison.Homes) != 1 || x.Comparison.Homes[0].CheckpointPaise != 150001 || x.Comparison.Homes[0].ComparisonPaise != 150001 || x.Comparison.Homes[0].DifferencePaise != 0 {
		t.Fatal("Evidence-only references changed money", x.Comparison)
	}
	x, err = s.DecideRecoveryGap(ctx, two, x.ID, numberingDecision(t, s, two, 1))
	if err != nil || x.Comparison.UnresolvedItems != 1 || x.State != "APPROVED" {
		t.Fatal("A separate assessment hid unreconstructed evidence", x, err)
	}
	status, err := s.RecoveryGapStatusFor(ctx, one)
	if err != nil || status.Unresolved != 1 || status.CanResume {
		t.Fatal("Unmatched evidence disappeared from current progress", status, err)
	}
	after := moneyRows(t, s)
	for table, original := range before {
		if table == "recovery_gap_preparations" || table == "recovery_gap_decisions" || table == "audit_events" {
			continue
		}
		if after[table] != original {
			t.Fatal("An evidence assessment rewrote an unrelated original table", table)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events", auditCount+2)
}

func gapMoneyFixture(t *testing.T) (*Store, string, string, string, string) {
	t.Helper()
	s, money := recordFixture(t)
	post(t, s, money, supplied("OPENING_DEBIT", "1000.00"))
	post(t, s, money, supplied("CHARGE", "500.01"))
	paid := post(t, s, money, received("400.00"))
	if _, err := s.ReverseEntry(context.Background(), money, paid, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Retain the independent original receipt and its linked correction during gap assessment."}); err != nil {
		t.Fatal(err)
	}
	one, two, _ := currentRecoveryReviewers(t, s)
	return s, money, one, two, paid
}

func TestRecoveryGapExactComparisonSeparateReviewAndRetriesPreserveMoney(t *testing.T) {
	s, copied, one, two, paid := gapMoneyFixture(t)
	ctx := context.Background()
	before := moneyRows(t, s)
	in := suppliedGap(t, s, one)
	x, err := s.ProposeRecoveryGap(ctx, one, in)
	if err != nil {
		t.Fatal(err)
	}
	if x.State != "PENDING" || x.Version != 1 || !x.Comparison.PreparedOnly || x.Comparison.KnownItems != 1 || x.Comparison.UnresolvedItems != 0 || len(x.Comparison.Homes) != 1 {
		t.Fatal("Wrong prepared assessment", x)
	}
	home := x.Comparison.Homes[0]
	if home.CheckpointPaise != 150001 || home.DifferencePaise != -25001 || home.ComparisonPaise != 125000 {
		t.Fatal("Independent exact-paise expectations failed", home)
	}
	same, err := s.ProposeRecoveryGap(ctx, one, in)
	if err != nil || same.ID != x.ID || !reflect.DeepEqual(same.Comparison, x.Comparison) {
		t.Fatal("Exact submission retry changed its retained result", err)
	}
	changed := in
	changed.Items = append([]RecoveryGapItem{}, in.Items...)
	changed.Items[0].Amount = "250.02"
	if _, err = s.ProposeRecoveryGap(ctx, one, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("Changed retry overwrote the supplied amount", err)
	}
	if _, err = s.ProposeRecoveryGap(ctx, copied, in); err == nil {
		t.Fatal("Copied ordinary finance prepared a gap")
	}
	if _, err = s.RecoveryGapOperationFor(ctx, two, in.OperationKey); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("Another actor resolved this operation", err)
	}
	if _, err = s.DecideRecoveryGap(ctx, one, x.ID, numberingDecision(t, s, one, 1)); !errors.Is(err, ErrForbidden) {
		t.Fatal("Author self-assessed a gap", err)
	}
	d := numberingDecision(t, s, two, 2)
	if _, err = s.DecideRecoveryGap(ctx, two, x.ID, d); !errors.Is(err, ErrConflict) {
		t.Fatal("Wrong frozen version was reviewed", err)
	}
	d.Version = 1
	if _, err = s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now')-301 WHERE token_hash=?", TokenHash(two)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideRecoveryGap(ctx, two, x.ID, d); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("Expired fresh identity reviewed a gap", err)
	}
	if _, err = s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now') WHERE token_hash=?", TokenHash(two)); err != nil {
		t.Fatal(err)
	}
	x, err = s.DecideRecoveryGap(ctx, two, x.ID, d)
	if err != nil || x.State != "APPROVED" || x.Version != 2 {
		t.Fatal(x, err)
	}
	if again, err := s.DecideRecoveryGap(ctx, two, x.ID, d); err != nil || again.ID != x.ID || again.Version != 2 {
		t.Fatal("Decision response-loss retry created another result", err)
	}
	for _, table := range []string{"entries", "entry_reversals", "receipts", "receipt_counter", "receipt_jobs", "role_grants", "flat_memberships"} {
		if _, present := before[table]; !present {
			t.Fatal("Missing original-table expectation", table)
		}
		if before[table] != moneyRows(t, s)[table] {
			t.Fatal("Gap review changed original financial/private authority", table)
		}
	}
	var amount int64
	if err := s.DB.QueryRow("SELECT amount_paise FROM entries WHERE id=?", paid).Scan(&amount); err != nil || amount != 40000 {
		t.Fatal("Original receipt amount changed", amount, err)
	}
	actual, err := s.RecoveryGapBalanceFor(ctx, one, in.Items[0].FlatID)
	if err != nil || actual.CheckpointPaise != 150001 || actual.ComparisonPaise != 150001 {
		t.Fatal("A comparison became posted money", actual, err)
	}
	if _, err = s.EntryFor(ctx, one, paid); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatal("Gap approval opened private originals", err)
	}
	status, err := s.RecoveryGapStatusFor(ctx, one)
	if err != nil || status.Pending != 0 || status.Approved != 1 || status.Known != 1 || status.CanResume {
		t.Fatal("Gap assessment became release", status, err)
	}
	for _, query := range []string{"UPDATE recovery_gap_preparations SET comparison_json='{}'", "DELETE FROM recovery_gap_decisions", "UPDATE receipt_counter SET next_number=next_number+1"} {
		if _, err := s.DB.Exec(query); err == nil {
			t.Fatal("Retained journal or receipt counter was rewritten", query)
		}
	}
}

func TestRecoveryGapReferencesUnknownCoverageAndInputBoundsAreExplicit(t *testing.T) {
	s, _, one, two, paid := gapMoneyFixture(t)
	ctx := context.Background()
	in := suppliedGap(t, s, one)
	var number string
	if err := s.DB.QueryRow("SELECT number FROM receipts WHERE entry_id=?", paid).Scan(&number); err != nil {
		t.Fatal(err)
	}
	in.Items = []RecoveryGapItem{
		{BookRef: "fictional-preserved-book", ItemRef: "original-receipt-17", FlatID: received("1.00").FlatID, Kind: "EVIDENCE", Amount: "400.00", EntryRef: paid, ReceiptNumber: number, Evidence: "Checked the original and its retained linked reversal independently.", EvidenceState: "SUPPORTED"},
		{BookRef: "fictional-incomplete-book", ItemRef: "unverified-fund-allocation-17", Kind: "EVIDENCE", Evidence: "The given fund allocation has insufficient independently available evidence.", EvidenceState: "UNKNOWN"},
	}
	in.CoverageComplete = false
	x, err := s.ProposeRecoveryGap(ctx, one, in)
	if err != nil || x.Comparison.KnownItems != 0 || x.Comparison.UnresolvedItems != 2 || len(x.Comparison.Matches) != 1 || !x.Comparison.Matches[0].LinkedReversal || !x.Comparison.Matches[0].ReceiptMatched {
		t.Fatal("Existing or missing evidence became new money", x, err)
	}
	if x.Comparison.Homes[0].CheckpointPaise != 150001 || x.Comparison.Homes[0].DifferencePaise != 0 || x.Comparison.Homes[0].ComparisonPaise != 150001 {
		t.Fatal("Existing corrected reference added money twice", x.Comparison)
	}
	x, err = s.DecideRecoveryGap(ctx, two, x.ID, numberingDecision(t, s, two, 1))
	if err != nil || x.Comparison.UnresolvedItems != 2 || !x.Comparison.CoverageUnresolved {
		t.Fatal("Assessment concealed unresolved evidence", x, err)
	}
	for name, mutate := range map[string]func(*RecoveryGapInput){
		"duplicate":          func(x *RecoveryGapInput) { x.Items = append(x.Items, x.Items[0]) },
		"wrong-home":         func(x *RecoveryGapInput) { x.Items[0].FlatID = "not-an-existing-home" },
		"amount-exponent":    func(x *RecoveryGapInput) { x.Items[0].Amount = "2.5001e2" },
		"amount-cap":         func(x *RecoveryGapInput) { x.Items[0].Amount = "10000000.01" },
		"empty-not-attested": func(x *RecoveryGapInput) { x.Items = []RecoveryGapItem{} },
		"uncertain-as-money": func(x *RecoveryGapInput) { x.Items[0].EvidenceState = "UNKNOWN" },
		"future-evidence":    func(x *RecoveryGapInput) { x.VerifiedAt = time.Now().Add(time.Hour).Unix() },
		"more-than-24": func(x *RecoveryGapInput) {
			for i := 0; i < 25; i++ {
				item := x.Items[0]
				item.ItemRef = fmt.Sprintf("bounded-item-%02d", i)
				x.Items = append(x.Items, item)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := suppliedGap(t, s, one)
			mutate(&bad)
			if _, err := s.ProposeRecoveryGap(ctx, one, bad); !errors.Is(err, ErrInvalid) {
				t.Fatal("Invalid supplied inventory was retained", err)
			}
		})
	}
	bad := suppliedGap(t, s, one)
	bad.Items[0].ReceiptNumber = number
	if _, err := s.ProposeRecoveryGap(ctx, one, bad); !errors.Is(err, ErrInvalid) {
		t.Fatal("Existing receipt was treated as missing money", err)
	}
	bad = suppliedGap(t, s, one)
	bad.Items[0].Kind = "EVIDENCE"
	bad.Items[0].EntryRef = paid
	bad.Items[0].Amount = "400.01"
	if _, err := s.ProposeRecoveryGap(ctx, one, bad); !errors.Is(err, ErrInvalid) {
		t.Fatal("Wrong amount matched an original reference", err)
	}
	empty := suppliedGap(t, s, one)
	empty.Items = []RecoveryGapItem{}
	empty.DeclaredEmpty = true
	if x, err := s.ProposeRecoveryGap(ctx, one, empty); err != nil || x.Comparison.KnownItems != 0 || x.Comparison.UnresolvedItems != 0 || len(x.Comparison.Homes) != 0 {
		t.Fatal("Explicit supplied empty coverage was not retained", x, err)
	}
}

func TestRecoveryGapConcurrentDecisionsLinkedCorrectionsAndSharedOperations(t *testing.T) {
	s, _, one, two, _ := gapMoneyFixture(t)
	ctx := context.Background()
	in := suppliedGap(t, s, one)
	x, err := s.ProposeRecoveryGap(ctx, one, in)
	if err != nil {
		t.Fatal(err)
	}
	d := numberingDecision(t, s, two, 1)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			candidate := d
			candidate.OperationKey = randomToken()
			if i == 1 {
				candidate.Decision = "DECLINED"
			}
			_, err := s.DecideRecoveryGap(ctx, two, x.ID, candidate)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("Two reviewers' frozen outcomes both won", wins, conflicts)
	}
	old, err := s.RecoveryGapFor(ctx, one, x.ID)
	if err != nil {
		t.Fatal(err)
	}
	in.OperationKey = randomToken()
	in.PredecessorID = old.ID
	in.PredecessorVersion = old.Version
	in.Items[0].Amount = "250.02"
	linked, err := s.ProposeRecoveryGap(ctx, one, in)
	if err != nil || linked.Version != 3 || linked.Comparison.Homes[0].ComparisonPaise != 124999 {
		t.Fatal("Linked correction lost exact prior/current values", linked, err)
	}
	history, err := s.RecoveryGapHistoryFor(ctx, one, linked.ID, 1)
	if err != nil || history.Total != 2 || history.Items[1].State != "SUPERSEDED" || history.Items[1].Comparison.Homes[0].ComparisonPaise != 125000 {
		t.Fatal("Linked history was rewritten", history, err)
	}
	duplicate := suppliedGap(t, s, one)
	if _, err := s.ProposeRecoveryGap(ctx, one, duplicate); !errors.Is(err, ErrInvalid) {
		t.Fatal("Another current preparation duplicated a supplied source item", err)
	}
	numbering := suppliedNumbering(t, s, one, "TRV-GAP2026A")
	numbering.OperationKey = in.OperationKey
	if _, err := s.ProposeRecoveryNumbering(ctx, one, numbering); !errors.Is(err, ErrConflict) {
		t.Fatal("Numbering reused a financial-gap operation", err)
	}
	summary, err := s.RecoverySummaryFor(ctx, one)
	if err != nil || summary.Pending != 1 || summary.Approved != 0 {
		t.Fatal("Shared progress counted superseded financial work as current", summary, err)
	}
	if _, err := s.RecoveryGapPreparationsFor(ctx, one, "APPROVED", 0); !errors.Is(err, ErrInvalid) {
		t.Fatal("Unbounded page admitted", err)
	}
	if _, err := s.DB.Exec("INSERT INTO recovery_gap_preparations SELECT (SELECT COALESCE(MAX(sequence),0)+1 FROM recovery_gap_preparations),?,? ,epoch_id,nomination_id,author_id,author_identity,1,NULL,input_digest,'{}',comparison_json,known_count,unresolved_count,created_at FROM recovery_gap_preparations LIMIT 1", strings.Repeat("Z", 43), randomToken()); err == nil {
		t.Fatal("Malformed direct SQL bypassed retained journal shape/current sequence")
	}
}

func TestRecoveryGapVoluntaryAndCorrectedFundPurposesKeepIndependentBalance(t *testing.T) {
	s, admin := recordFixture(t)
	ctx := context.Background()
	reviewer := maintenanceReviewer(t, s, admin)
	post(t, s, admin, supplied("OPENING_DEBIT", "1500.01"))
	original := post(t, s, admin, received("400.00"))
	if _, err := s.ReverseEntry(ctx, admin, original, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Keep the independently checked original 40000-paise receipt and linked correction."}); err != nil {
		t.Fatal(err)
	}
	voluntary := fundProposal()
	voluntary.ContributionType = "VOLUNTARY"
	voluntary.Target = "2500.00"
	voluntary.Lines = []MaintenanceLineInput{{FlatID: "demo-flat-A-101"}}
	garden := createPublishedFund(t, s, admin, reviewer, voluntary)
	active := post(t, s, admin, received("100.00"))
	corrected := post(t, s, admin, received("50.00"))
	if _, err := s.AttributeFundCredit(ctx, admin, FundAttributionInput{randomToken(), garden.ID, "demo-flat-A-101", active, "100.00", "Independently supplied live voluntary purpose for this original received amount", true}); err != nil {
		t.Fatal(err)
	}
	old, err := s.AttributeFundCredit(ctx, admin, FundAttributionInput{randomToken(), garden.ID, "demo-flat-A-101", corrected, "50.00", "Independently supplied prior purpose preserved through a linked correction", true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CorrectFundContribution(ctx, admin, old, AllocationCorrection{randomToken(), "A linked correction removes this attribution while preserving its received amount", true}); err != nil {
		t.Fatal(err)
	}
	before := moneyRows(t, s)
	one, _, _ := currentRecoveryReviewers(t, s)
	balance, err := s.RecoveryGapBalanceFor(ctx, one, "demo-flat-A-101")
	// 150001 debit - 15000 confirmed credits + 10000 live voluntary purpose.
	// The original40000 receipt is reversed; the corrected5000 attribution is
	// excluded while its original received credit remains preserved.
	if err != nil || balance.DebitPaise != 150001 || balance.CreditPaise != 15000 || balance.VoluntaryPaise != 10000 || balance.CheckpointPaise != 145001 {
		t.Fatal("Voluntary purposes changed account meaning", balance, err)
	}
	x, err := s.ProposeRecoveryGap(ctx, one, suppliedGap(t, s, one))
	if err != nil || x.Comparison.Homes[0].CheckpointPaise != 145001 || x.Comparison.Homes[0].ComparisonPaise != 120000 {
		t.Fatal("Independent 145001-minus25001 comparison failed", x, err)
	}
	for _, table := range []string{"entries", "entry_reversals", "receipts", "receipt_counter", "receipt_jobs"} {
		if _, present := before[table]; !present || before[table] != moneyRows(t, s)[table] {
			t.Fatal("Fund comparison changed originals", table)
		}
	}
}
