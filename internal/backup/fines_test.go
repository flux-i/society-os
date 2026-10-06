package backup

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/jpeg"
	"path/filepath"
	"reflect"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"testing"
	"time"
)

func TestFineRecoveryRetainsOriginalSourceNoticeResponsesPaymentIdentityAppealPauseCorrectionAndReleasedCredit(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if e := s.SeedDemoAccounts(ctx); e != nil {
		t.Fatal(e)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	a := upkeepRecoveryLogin(t, s, "admin@demo.society")
	b := upkeepRecoveryLogin(t, s, "committee@demo.society")
	tenant := upkeepRecoveryLogin(t, s, "tenant@demo.society")
	owner := upkeepRecoveryLogin(t, s, "owner@demo.society")
	ruleInput := database.RuleInput{OperationKey: "recovery-rule-proposal-12345", Title: "Supplied shared corridor policy", Text: "A fictional supplied policy preserves clear shared access.", PolicyReference: "Supplied fictional resolution R-01", EffectiveFrom: "2026-01-01", FinePermitted: true, Confirmed: true}
	rule, e := s.CreateRule(ctx, a, ruleInput)
	if e != nil {
		t.Fatal(e)
	}
	publish := database.RuleAction{OperationKey: "recovery-rule-publish-12345", Version: 1, Action: "PUBLISHED", Reason: "Separately checked the supplied fictional rule authority", Confirmed: true}
	if _, e = s.DecideRule(ctx, b, rule, publish); e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 16, 12)), nil); e != nil {
		t.Fatal(e)
	}
	raw := encoded.Bytes()
	private := []byte("Exif\x00\x00PRIVATE device and GPS metadata")
	segment := append([]byte{0xff, 0xe1, 0, byte(len(private) + 2)}, private...)
	original := append(append(append([]byte{}, raw[:2]...), segment...), raw[2:]...)
	pic, e := s.UploadIncidentPicture(ctx, tenant, "recovery-picture-12345", "scene.jpg", original)
	if e != nil {
		t.Fatal(e)
	}
	reportInput := database.IncidentInput{OperationKey: "recovery-incident-12345", RuleID: rule, FlatID: "demo-flat-A-101", IncidentDate: "2026-01-02", Comment: "PRIVATE fictional reporter observation and evidence detail.", PictureID: pic.ID, Confirmed: true}
	id, e := s.SaveIncident(ctx, tenant, "", reportInput)
	if e != nil {
		t.Fatal(e)
	}
	act := func(store *database.Store, token, action string, extra database.IncidentAction) {
		t.Helper()
		x, e := store.IncidentFor(ctx, token, id, 1)
		if e != nil {
			t.Fatal(e)
		}
		extra.OperationKey = "recovery-incident-" + action + "-12345"
		extra.Version = x.Version
		extra.Action = action
		extra.Reason = "PRIVATE independent review of supplied fictional evidence"
		extra.Confirmed = true
		if _, e = store.ActOnIncident(ctx, token, id, extra); e != nil {
			t.Fatal(action, e)
		}
	}
	act(s, a, "NOTE", database.IncidentAction{})
	act(s, b, "UNDER_REVIEW", database.IncidentAction{})
	act(s, b, "ISSUE_NOTICE", database.IncidentAction{Title: "Please supply your household account", Body: "A fictional corridor observation needs your household response.", ResponseBy: time.Now().AddDate(0, 0, 4).Format("2006-01-02")})
	detail, e := s.IncidentFor(ctx, a, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	responseInput := database.IncidentResponseInput{OperationKey: "recovery-subject-response-12345", Version: 1, Body: "PRIVATE household response retained only for this person and handlers.", Confirmed: true}
	response, e := s.RespondToIncident(ctx, owner, detail.NoticeID, responseInput)
	if e != nil {
		t.Fatal(e)
	}
	detail, e = s.IncidentFor(ctx, a, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	own, e := s.IncidentFor(ctx, tenant, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	notice, e := s.IncidentNoticeFor(ctx, owner, detail.NoticeID, 1)
	if e != nil {
		t.Fatal(e)
	}
	preview, e := s.IncidentPictureFor(ctx, owner, pic.ID, false)
	if e != nil || bytes.Contains(preview.Bytes, private) {
		t.Fatal("safe preview", e)
	}
	act(s, b, "SUBSTANTIATED", database.IncidentAction{})
	if e = s.SeedDemoTreasury(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = s.GrantAppointment(ctx, a, "demo-user-committee", database.AppointmentInput{AccessChange: database.AccessChange{Version: 1, Confirmed: true, Reason: "Supplied separate treasury reviewer for fine recovery"}, Role: "TREASURER", TermDays: 30}); e != nil {
		t.Fatal(e)
	}
	b = upkeepRecoveryLogin(t, s, "committee@demo.society")
	source, e := s.FineSourceFor(ctx, a, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	fine, e := s.CreateFine(ctx, a, database.FineInput{OperationKey: "fine-recovery-proposal-12345", IncidentID: id, SourceKey: source.SourceKey, Title: "Recovery fine with supplied amount", Amount: "250.25", PolicyReference: "Supplied fictional amount authority F-01", Reason: "PRIVATE amount and source deliberately prepared", ResponseBy: time.Now().AddDate(0, 0, 3).Format("2006-01-02"), DueDate: time.Now().AddDate(0, 0, 7).Format("2006-01-02"), NoticeBody: "A fictional supplied fine is proposed for your household account.", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	fineRead := func(store *database.Store, token string) database.FineDetail {
		t.Helper()
		x, e := store.FineFor(ctx, token, fine, 1, 1)
		if e != nil {
			t.Fatal(e)
		}
		return x
	}
	fineAct := func(action string) {
		t.Helper()
		x := fineRead(s, a)
		in := database.FineAction{OperationKey: "fine-recovery-" + action + "-12345", Version: x.Version, Action: action, Reason: "PRIVATE separately considered financial effect", Confirmed: true}
		if action == "RESOLVE" {
			in.SourceKey = x.CurrentSourceKey
			in.ResponseCount = x.ResponseTotal
			in.Resolution = "Supplied retained household responses deliberately considered"
			in.EarlyIssueReference = "Supplied fictional early decision authority"
		}
		if action == "ISSUE" {
			in.ResolutionKey = x.ResolutionKey
		}
		if _, e = s.ActOnFine(ctx, b, fine, in); e != nil {
			t.Fatal(e)
		}
	}
	fineAct("NOTIFY")
	f := fineRead(s, a)
	if _, e = s.RespondToFineNotice(ctx, owner, f.NoticeID, database.IncidentResponseInput{OperationKey: "fine-recovery-household-response-12345", Version: 1, Body: "PRIVATE supplied immutable household fine response", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	fineAct("RESOLVE")
	fineAct("ISSUE")
	report, e := s.SaveFineReport(ctx, owner, "", database.FineReportInput{OperationKey: "fine-recovery-money-report-12345", FineID: fine, Amount: "100.00", PaymentDate: "2026-01-01", Payer: "PRIVATE supplied payer", Method: "UPI", Reference: "PRIVATE original external payment reference", Comment: "PRIVATE external already-paid claim", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	confirmation := database.FundReportDecision{OperationKey: "fine-recovery-money-confirm-12345", Version: 1, Action: "CONFIRMED", Reason: "PRIVATE independently verified original external money", VerificationSource: "PRIVATE external source statement", PaymentIdentity: "PRIVATE external row one", Mode: "NEW", AllocationAmount: "100.00", Confirmed: true}
	if _, e = s.DecideFineReport(ctx, b, report, confirmation); e != nil {
		t.Fatal(e)
	}
	paid, e := s.FineReportFor(ctx, owner, report, 1)
	if e != nil || paid.Receipt == "" {
		t.Fatal(paid, e)
	}
	appeal, e := s.CreateFineAppeal(ctx, owner, database.FineAppealInput{OperationKey: "fine-recovery-appeal-12345", NoticeID: f.NoticeID, Body: "PRIVATE supplied appeal retained with explicit collection review", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	f = fineRead(s, a)
	if _, e = s.ActOnFineAppeal(ctx, b, appeal, database.FineChildAction{OperationKey: "fine-recovery-pause-12345", Version: 1, FineVersion: f.Version, Action: "PAUSED", Reason: "PRIVATE separately supplied pause while appeal is considered", PolicyReference: "Supplied fictional pause source", PauseUntil: time.Now().AddDate(0, 0, 4).Format("2006-01-02"), Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	f = fineRead(s, a)
	waiver, e := s.ProposeFineWaiver(ctx, a, database.FineWaiverInput{OperationKey: "fine-recovery-correction-12345", FineID: fine, ChargeVersion: f.ChargeVersion, Kind: "WAIVER", Amount: "75.25", PolicyReference: "Supplied fictional reduction authority", Reason: "PRIVATE separately supplied partial correction", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	f = fineRead(s, a)
	if _, e = s.DecideFineWaiver(ctx, b, waiver, database.FineChildAction{OperationKey: "fine-recovery-correction-approve-12345", Version: 1, FineVersion: f.Version, Action: "APPROVED", Reason: "PRIVATE separate reviewer accepts the supplied reduction", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	wantFine := fineRead(s, a)
	wantNotice, e := s.FineNoticeFor(ctx, owner, wantFine.NoticeID, 1)
	if e != nil {
		t.Fatal(e)
	}
	wantAppeal, e := s.FineAppealFor(ctx, owner, appeal, 1)
	if e != nil {
		t.Fatal(e)
	}
	wantWaiver, e := s.FineWaiverFor(ctx, a, waiver, 1)
	if e != nil {
		t.Fatal(e)
	}
	wantStatement, e := s.HomeStatementFor(ctx, a, "demo-flat-A-101", 1, 1, 1)
	if e != nil {
		t.Fatal(e)
	}
	if wantFine.ActivePaise != 17500 || wantFine.AllocatedPaise != 0 || wantFine.OutstandingPaise != 17500 || wantStatement.UnallocatedPaise != 10000 || wantFine.PauseUntil == "" {
		t.Fatal("independent corrected and paused amounts", wantFine, wantStatement)
	}
	detail, e = s.IncidentFor(ctx, a, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	own, e = s.IncidentFor(ctx, tenant, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	notice, e = s.IncidentNoticeFor(ctx, owner, detail.NoticeID, 1)
	if e != nil {
		t.Fatal(e)
	}
	bundle := filepath.Join(root, "fine-snapshot")
	if _, e = Snapshot(ctx, s, bundle, "fine-test"); e != nil {
		t.Fatal(e)
	}
	act(s, b, "REMOVE_NOTICE", database.IncidentAction{})
	act(s, a, "REOPEN", database.IncidentAction{})
	if _, e = s.DecideRule(ctx, a, rule, database.RuleAction{OperationKey: "recovery-later-retirement-12345", Version: 2, Action: "RETIRED", Reason: "A later supplied retirement must be outside the snapshot", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(root, "restored-fines.db")
	if _, e = Restore(ctx, bundle, target); e != nil {
		t.Fatal(e)
	}
	restored, e := database.Open(ctx, target)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	restored.MFA = s.MFA
	if e = restored.VerifyMFAKey(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = restored.CheckSession(ctx, a); !errors.Is(e, database.ErrUnauthenticated) {
		t.Fatal("old session revived", e)
	}
	current := upkeepRecoveryLogin(t, restored, "admin@demo.society")
	reporter := upkeepRecoveryLogin(t, restored, "tenant@demo.society")
	subject := upkeepRecoveryLogin(t, restored, "owner@demo.society")
	got, e := restored.IncidentFor(ctx, current, id, 1)
	if e != nil || !reflect.DeepEqual(got, detail) {
		t.Fatal("private snapshot restore", got, e)
	}
	got, e = restored.IncidentFor(ctx, reporter, id, 1)
	if e != nil || !reflect.DeepEqual(got, own) {
		t.Fatal("reporter snapshot restore", got, e)
	}
	gotNotice, e := restored.IncidentNoticeFor(ctx, subject, detail.NoticeID, 1)
	if e != nil || !reflect.DeepEqual(gotNotice, notice) {
		t.Fatal("notice/response restore", gotNotice, e)
	}
	gotPicture, e := restored.IncidentPictureFor(ctx, reporter, pic.ID, true)
	if e != nil || !bytes.Equal(gotPicture.Bytes, original) || gotPicture.SHA256 != pic.SHA256 {
		t.Fatal("original restore", e)
	}
	gotPreview, e := restored.IncidentPictureFor(ctx, subject, pic.ID, false)
	if e != nil || !bytes.Equal(gotPreview.Bytes, preview.Bytes) {
		t.Fatal("preview restore", e)
	}
	if _, e = restored.IncidentPictureFor(ctx, subject, pic.ID, true); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("subject original after restore", e)
	}
	if _, e = restored.IncidentFor(ctx, subject, id, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("subject private case after restore", e)
	}
	if again, e := restored.SaveIncident(ctx, reporter, "", reportInput); e != nil || again != id {
		t.Fatal("report replay", e)
	}
	if again, e := restored.RespondToIncident(ctx, subject, detail.NoticeID, responseInput); e != nil || again != response {
		t.Fatal("response replay", e)
	}
	if !reflect.DeepEqual(wantFine, fineRead(restored, current)) {
		t.Fatal("fine original context/pause/correction changed")
	}
	gotFineNotice, e := restored.FineNoticeFor(ctx, subject, wantFine.NoticeID, 1)
	if e != nil || !reflect.DeepEqual(wantNotice, gotFineNotice) {
		t.Fatal("fine household response changed", e)
	}
	gotAppeal, e := restored.FineAppealFor(ctx, subject, appeal, 1)
	if e != nil || !reflect.DeepEqual(wantAppeal, gotAppeal) {
		t.Fatal("appeal/pause changed", e)
	}
	gotWaiver, e := restored.FineWaiverFor(ctx, current, waiver, 1)
	if e != nil || !reflect.DeepEqual(wantWaiver, gotWaiver) {
		t.Fatal("linked correction changed", e)
	}
	gotPaid, e := restored.FineReportFor(ctx, subject, report, 1)
	if e != nil || gotPaid.Receipt != paid.Receipt || gotPaid.EntryID != paid.EntryID {
		t.Fatal("original payment/receipt changed", e)
	}
	gotStatement, e := restored.HomeStatementFor(ctx, current, "demo-flat-A-101", 1, 1, 1)
	if e != nil || !reflect.DeepEqual(wantStatement, gotStatement) {
		t.Fatal("released credit/ledger changed", e)
	}
	var originalEntry string
	if e = restored.DB.QueryRow("SELECT entry_id FROM verified_fund_payments WHERE verification_source=? AND payment_identity=?", confirmation.VerificationSource, confirmation.PaymentIdentity).Scan(&originalEntry); e != nil || originalEntry != paid.EntryID {
		t.Fatal("global original payment identity changed", originalEntry, e)
	}
	for table, want := range map[string]int{"entries": 3, "receipts": 1, "entry_reversals": 1, "fine_waivers": 1, "fine_appeals": 1} {
		var n int
		if e = restored.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != want {
			t.Fatal("retained exact records", table, n, want, e)
		}
	}
}
