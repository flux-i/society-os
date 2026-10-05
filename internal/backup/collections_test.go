package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"path/filepath"
	"reflect"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"testing"
	"time"
)

func TestCollectionsRecoveryRetainsSeparateReviewsPrivateEvidenceOriginalReceiptDuplicatesCreditExemptionsAndPurposeHistory(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	for _, seed := range []func(context.Context) error{s.SeedDemoAccounts, s.SeedDemoTreasury} {
		if err := seed(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	admin := upkeepRecoveryLogin(t, s, "admin@demo.society")
	if _, err := s.GrantAppointment(ctx, admin, "demo-user-committee", database.AppointmentInput{AccessChange: database.AccessChange{Version: 1, Confirmed: true, Reason: "Verified fictional separate treasury reviewer for recovery"}, Role: "TREASURER", TermDays: 30}); err != nil {
		t.Fatal(err)
	}
	reviewer := upkeepRecoveryLogin(t, s, "committee@demo.society")
	owner := upkeepRecoveryLogin(t, s, "owner@demo.society")
	proposal := database.FundInput{OperationKey: "recovery-fixed-fund-12345", Title: "Checkpoint water reserve", Purpose: "Supplied fictional collection for shared water services.", ContributionType: "FIXED", StartDate: "2026-01-01", DueDate: "2026-01-10", SourceReference: "PRIVATE supplied fund approval", Note: "PRIVATE supporting source", Lines: []database.MaintenanceLineInput{{FlatID: "demo-flat-A-101", Amount: "1000.00"}, {FlatID: "demo-flat-A-102", Amount: "500.25"}}, Confirmed: true}
	create := func(in database.FundInput) string {
		t.Helper()
		id, e := s.CreateFundCampaign(ctx, admin, in)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.DecideFundCampaign(ctx, reviewer, id, database.FundAction{OperationKey: in.OperationKey + "-publish", Version: 1, Action: "PUBLISHED", Reason: "Separately reviewed supplied fund before publication", Confirmed: true}); e != nil {
			t.Fatal(e)
		}
		return id
	}
	fixed := create(proposal)
	proposal.OperationKey = "recovery-voluntary-fund-12345"
	proposal.ContributionType = "VOLUNTARY"
	proposal.Title = "Checkpoint voluntary purpose"
	proposal.Lines = []database.MaintenanceLineInput{{FlatID: "demo-flat-A-101", Amount: ""}}
	voluntary := create(proposal)
	var pngFile bytes.Buffer
	if err := png.Encode(&pngFile, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	photo := pngFile.Bytes()
	sum := sha256.Sum256(photo)
	evidence, err := s.ReserveDocument(ctx, owner, database.DocumentInput{OperationKey: "recovery-private-evidence-12345", Title: "Checkpoint private payment evidence", Filename: "evidence.png", Size: int64(len(photo)), SHA256: hex.EncodeToString(sum[:]), Category: "PAYMENT_EVIDENCE", Visibility: "FLAT_SPECIFIC", FlatID: "demo-flat-A-101"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteDocument(ctx, owner, evidence, photo); err != nil {
		t.Fatal(err)
	}
	job, err := s.ClaimDocumentCheck(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishDocumentCheck(ctx, job, "image/png", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	claim := database.FundReportInput{OperationKey: "recovery-primary-report-12345", CampaignID: fixed, FlatID: "demo-flat-A-101", Amount: "1600.00", PaymentDate: "2026-01-01", Payer: "PRIVATE checkpoint payer", Method: "BANK_TRANSFER", Reference: "PRIVATE checkpoint payment reference", Comment: "PRIVATE checkpoint claim detail", EvidenceID: evidence, Confirmed: true}
	report, err := s.SaveFundReport(ctx, owner, "", claim)
	if err != nil {
		t.Fatal(err)
	}
	confirmation := database.FundReportDecision{OperationKey: "recovery-primary-confirm-12345", Version: 1, Action: "CONFIRMED", Reason: "PRIVATE independently checked the checkpoint payment", VerificationSource: "PRIVATE checkpoint bank statement", PaymentIdentity: "PRIVATE checkpoint bank row 22", Mode: "NEW", AllocationAmount: "1000.00", Confirmed: true}
	if _, err = s.DecideFundReport(ctx, admin, report, confirmation); err != nil {
		t.Fatal(err)
	}
	original, err := s.FundReportFor(ctx, owner, report, 1)
	if err != nil || original.State != "CONFIRMED" || original.Receipt == "" {
		t.Fatal(original, err)
	}
	claim.OperationKey = "recovery-duplicate-report-12345"
	claim.EvidenceID = ""
	duplicate, err := s.SaveFundReport(ctx, reviewer, "", claim)
	if err != nil {
		t.Fatal(err)
	}
	dupDecision := confirmation
	dupDecision.OperationKey = "recovery-duplicate-confirm-12345"
	dupDecision.Mode = "LINK"
	dupDecision.EntryID = original.EntryID
	dupDecision.AllocationAmount = "0.00"
	if _, err = s.DecideFundReport(ctx, admin, duplicate, dupDecision); err != nil {
		t.Fatal(err)
	}
	attribution, err := s.AttributeFundCredit(ctx, admin, database.FundAttributionInput{OperationKey: "recovery-voluntary-attribute-12345", CampaignID: voluntary, FlatID: "demo-flat-A-101", SourceID: original.EntryID, Amount: "500.00", Reason: "PRIVATE explicitly assigned confirmed receipt credit to purpose", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	waiver, err := s.ProposeFundWaiver(ctx, admin, database.FundWaiverInput{OperationKey: "recovery-exemption-submit-12345", CampaignID: fixed, FlatID: "demo-flat-A-101", ParticipantVersion: 1, Amount: "300.00", SourceReference: "PRIVATE explicit exception source", Reason: "PRIVATE supplied partial exception before snapshot", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideFundWaiver(ctx, reviewer, waiver, database.FundAction{OperationKey: "recovery-exemption-approve-12345", Version: 1, Action: "APPROVED", Reason: "PRIVATE separately checked exception and original charge", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	claim.OperationKey = "recovery-pending-report-12345"
	claim.Amount = "37.25"
	claim.Reference = "PRIVATE separate pending payment"
	pending, err := s.SaveFundReport(ctx, owner, "", claim)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideFundCampaign(ctx, admin, fixed, database.FundAction{OperationKey: "recovery-close-fund-12345", Version: 2, Action: "CLOSED", Reason: "Closed new reports while retaining the checkpoint history", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	fund, _ := s.FundCampaignFor(ctx, admin, fixed, 1, 1)
	vol, _ := s.FundCampaignFor(ctx, admin, voluntary, 1, 1)
	wantWaiver, _ := s.FundWaiverFor(ctx, admin, waiver, 1)
	wantContrib, _ := s.FundContributionsFor(ctx, admin, voluntary, "", 1)
	wantDuplicate, _ := s.FundReportFor(ctx, admin, duplicate, 1)
	wantPending, _ := s.FundReportFor(ctx, owner, pending, 1)
	statement, _ := s.HomeStatementFor(ctx, admin, "demo-flat-A-101", 1, 1, 1)
	if fund.OutstandingPaise != 120025 || fund.WaivedPaise != 30000 || fund.AllocatedPaise != 0 || vol.VoluntaryPaise != 50000 || statement.UnallocatedPaise != 110000 || wantDuplicate.State != "DUPLICATE" || wantWaiver.AuthorID == "" || wantWaiver.Reviewer == wantWaiver.Author {
		t.Fatal("independent checkpoint amounts/reviews", fund, statement, wantDuplicate, wantWaiver)
	}
	bundle := filepath.Join(root, "collections-checkpoint")
	if _, err = Snapshot(ctx, s, bundle, "collections-test"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CorrectFundContribution(ctx, admin, attribution, database.AllocationCorrection{OperationKey: "after-snapshot-purpose-correction-12345", Reason: "Later correction excluded from the restored checkpoint", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReverseEntry(ctx, admin, original.EntryID, database.EntryAction{OperationKey: "after-snapshot-receipt-reverse-12345", Reason: "Later original receipt reversal excluded from checkpoint", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored-collections.db")
	if _, err = Restore(ctx, bundle, target); err != nil {
		t.Fatal(err)
	}
	restored, err := database.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restored.MFA = s.MFA
	if err = restored.VerifyMFAKey(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = restored.CheckSession(ctx, admin); !errors.Is(err, database.ErrUnauthenticated) {
		t.Fatal("old session revived", err)
	}
	current := upkeepRecoveryLogin(t, restored, "admin@demo.society")
	resident := upkeepRecoveryLogin(t, restored, "owner@demo.society")
	gotFund, e := restored.FundCampaignFor(ctx, current, fixed, 1, 1)
	if e != nil || !reflect.DeepEqual(gotFund, fund) {
		t.Fatal("frozen fund restore", gotFund, e)
	}
	gotVol, e := restored.FundCampaignFor(ctx, current, voluntary, 1, 1)
	if e != nil || !reflect.DeepEqual(gotVol, vol) {
		t.Fatal("voluntary restore", gotVol, e)
	}
	gotStatement, e := restored.HomeStatementFor(ctx, current, "demo-flat-A-101", 1, 1, 1)
	if e != nil || !reflect.DeepEqual(gotStatement, statement) {
		t.Fatal("credit restore", gotStatement, e)
	}
	gotWaiver, e := restored.FundWaiverFor(ctx, current, waiver, 1)
	if e != nil || !reflect.DeepEqual(gotWaiver, wantWaiver) {
		t.Fatal("exemption restore", gotWaiver, e)
	}
	gotContrib, e := restored.FundContributionsFor(ctx, current, voluntary, "", 1)
	if e != nil || !reflect.DeepEqual(gotContrib, wantContrib) {
		t.Fatal("purpose history restore", gotContrib, e)
	}
	gotDuplicate, e := restored.FundReportFor(ctx, current, duplicate, 1)
	if e != nil || !reflect.DeepEqual(gotDuplicate, wantDuplicate) {
		t.Fatal("duplicate original restore", gotDuplicate, e)
	}
	gotPending, e := restored.FundReportFor(ctx, resident, pending, 1)
	if e != nil || !reflect.DeepEqual(gotPending, wantPending) {
		t.Fatal("pending claim restore", gotPending, e)
	}
	doc, originalBytes, e := restored.FundReportEvidenceFor(ctx, resident, report)
	if e != nil || !bytes.Equal(originalBytes, photo) || doc.State != "PENDING" {
		t.Fatal("private evidence restore", doc, e)
	}
	if _, err = restored.DecideFundReport(ctx, current, report, confirmation); err != nil {
		t.Fatal("current-authority original operation replay", err)
	}
	var receipts, received int
	if err = restored.DB.QueryRow("SELECT (SELECT COUNT(*) FROM receipts),(SELECT COUNT(*) FROM entries WHERE kind='RECEIVED')").Scan(&receipts, &received); err != nil || receipts != 1 || received != 1 {
		t.Fatal("restored duplicate issued receipt", receipts, received, err)
	}
	if _, err = restored.FundWaiverFor(ctx, resident, waiver, 1); !errors.Is(err, database.ErrForbidden) {
		t.Fatal("private exemption resident access", err)
	}
}
