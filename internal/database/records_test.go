package database

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func recordFixture(t *testing.T) (*Store, string) {
	t.Helper()
	s, token := adminFixture(t)
	if err := s.SeedDemoTreasury(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, token
}
func supplied(kind, amount string) EntryInput {
	return EntryInput{OperationKey: randomToken(), FlatID: "demo-flat-A-101", Kind: kind, Amount: amount, Date: "2026-01-01", Description: "Fictional supplied amount"}
}
func received(amount string) EntryInput {
	in := supplied("RECEIVED", amount)
	in.Payer = "Demo Owner A-101"
	in.Method = "BANK_TRANSFER"
	in.Reference = "DEMO-REF-101"
	return in
}
func post(t *testing.T, s *Store, token string, in EntryInput) string {
	t.Helper()
	id, err := s.CreateEntry(context.Background(), token, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PostEntry(context.Background(), token, id, EntryAction{OperationKey: randomToken(), Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestExactAmountsRejectAmbiguousOrInvalidValues(t *testing.T) {
	for _, item := range []struct {
		value string
		paise int64
	}{{"0.01", 1}, {"0.10", 10}, {"12.3", 1230}, {"10000000", 1000000000}} {
		got, err := ParseAmount(item.value)
		if err != nil || got != item.paise {
			t.Fatalf("%s => %d %v", item.value, got, err)
		}
	}
	for _, value := range []string{"0", "0.00", "-1", "+1", "1e3", "1.001", "1,000.00", " 1", "01", ".1", "NaN", "10000000.01", "99999999"} {
		if _, err := ParseAmount(value); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted %q", value)
		}
	}
}
func TestDraftReviewExactBalancesAndImmutableLinkedCorrections(t *testing.T) {
	s, token := recordFixture(t)
	ctx := context.Background()
	opening := post(t, s, token, supplied("OPENING_DEBIT", "1000.00"))
	in := received("400.00")
	id, err := s.CreateEntry(ctx, token, in)
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.EntriesFor(ctx, token, "demo-flat-A-101", "", "", false, 1)
	if err != nil || page.BalancePaise != 100000 || page.Drafts != 1 {
		t.Fatalf("draft affected ledger: %+v %v", page, err)
	}
	if _, err = s.PostEntry(ctx, token, id, EntryAction{OperationKey: randomToken()}); !errors.Is(err, ErrInvalid) {
		t.Fatal("posted without review")
	}
	key := randomToken()
	action := EntryAction{OperationKey: key, Confirmed: true}
	for i := 0; i < 2; i++ {
		if _, err = s.PostEntry(ctx, token, id, action); err != nil {
			t.Fatal(err)
		}
	}
	page, err = s.EntriesFor(ctx, token, "demo-flat-A-101", "no matching note", "", false, 1)
	if err != nil || page.BalancePaise != 60000 || page.Total != 0 {
		t.Fatalf("expected ₹600 independent of search: %+v %v", page, err)
	}
	e, err := s.EntryFor(ctx, token, id)
	if err != nil || e.ReceiptID == "" || e.ReceiptNumber != "SOS-"+time.Now().In(time.FixedZone("IST", 19800)).Format("2006")+"-000001" {
		t.Fatalf("receipt %+v %v", e, err)
	}
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&count)
	if count != 1 {
		t.Fatal("duplicate receipt")
	}
	for _, query := range []string{"UPDATE entries SET amount_paise=1 WHERE id=?", "DELETE FROM entries WHERE id=?", "UPDATE receipts SET number='changed' WHERE entry_id=?", "DELETE FROM receipts WHERE entry_id=?"} {
		if _, err = s.DB.Exec(query, id); err == nil {
			t.Fatal("mutable posted history", query)
		}
	}
	reverse := EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Supplied reference was incorrect"}
	for i := 0; i < 2; i++ {
		if _, err = s.ReverseEntry(ctx, token, id, reverse); err != nil {
			t.Fatal(err)
		}
	}
	page, err = s.EntriesFor(ctx, token, "demo-flat-A-101", "", "", false, 1)
	if err != nil || page.BalancePaise != 100000 {
		t.Fatalf("reversal balance %+v %v", page, err)
	}
	e, err = s.EntryFor(ctx, token, id)
	if err != nil || e.State != "REVERSED" || e.AmountPaise != 40000 || e.ReceiptID == "" {
		t.Fatalf("original lost %+v %v", e, err)
	}
	if _, err = s.DB.Exec("UPDATE entry_reversals SET reason='overwritten' WHERE entry_id=?", id); err == nil {
		t.Fatal("reversal overwritten")
	}
	if _, err = s.ReverseEntry(ctx, token, opening, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "bad"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("short reason accepted")
	}
	for _, kind := range []string{"CHARGE", "OPENING_CREDIT"} {
		post(t, s, token, supplied(kind, "0.01"))
	}
	page, _ = s.EntriesFor(ctx, token, "demo-flat-A-101", "", "", false, 1)
	if page.BalancePaise != 100000 {
		t.Fatal("one-paise arithmetic", page.BalancePaise)
	}
}
func TestConcurrentRetriesAndRevokedTreasurer(t *testing.T) {
	s, token := recordFixture(t)
	ctx := context.Background()
	in := received("23.45")
	var wg sync.WaitGroup
	results := make(chan string, 6)
	failures := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); id, err := s.CreateEntry(ctx, token, in); results <- id; failures <- err }()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for result := range results {
		if id != "" && result != id {
			t.Fatal("duplicate identity")
		}
		id = result
	}
	in.Amount = "99.00"
	if _, err := s.CreateEntry(ctx, token, in); !errors.Is(err, ErrConflict) {
		t.Fatal("changed retry payload accepted")
	}
	action := EntryAction{OperationKey: randomToken(), Confirmed: true}
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.PostEntry(ctx, token, id, action); failuresSafe(t, err) }()
	}
	wg.Wait()
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&count)
	if count != 1 {
		t.Fatal("duplicate receipt under concurrent retry")
	}
	s.DB.Exec("UPDATE role_grants SET revoked_at=? WHERE role='TREASURER'", time.Now().Unix())
	if _, err := s.PostEntry(ctx, token, id, action); !errors.Is(err, ErrForbidden) {
		t.Fatal("replay bypassed revoked role", err)
	}
}
func failuresSafe(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Error(err)
	}
}

func TestCurrentFinanceEntitlementsAndPrivateReceiptScope(t *testing.T) {
	s, token := recordFixture(t)
	ctx := context.Background()
	id := post(t, s, token, received("400.00"))
	foreign := received("50")
	foreign.FlatID = "demo-flat-B-101"
	alien := post(t, s, token, foreign)
	owner, _, err := s.Login(ctx, "owner@demo.society", DemoPassword, HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.EntriesFor(ctx, owner, "", "", "", false, 1)
	if err != nil || page.Total != 1 || page.CreditPaise != 40000 {
		t.Fatalf("owner scope %+v %v", page, err)
	}
	if _, err = s.EntryFor(ctx, owner, alien); err == nil {
		t.Fatal("cross-home access")
	}
	if _, err = s.CreateEntry(ctx, owner, received("10")); !errors.Is(err, ErrForbidden) {
		t.Fatal("resident posted")
	}
	e, _ := s.EntryFor(ctx, token, id)
	if _, err = s.ReceiptEntryFor(ctx, owner, e.ReceiptID); err != nil {
		t.Fatal(err)
	}
	s.DB.Exec("UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101'")
	if _, err = s.ReceiptEntryFor(ctx, owner, e.ReceiptID); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked finance access remained", err)
	}
	tenant, _, err := s.Login(ctx, "tenant@demo.society", DemoPassword, HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.EntriesFor(ctx, tenant, "", "", "", false, 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("registry access implied finance access")
	}
	events, err := s.ActivityFor(ctx, token, "demo-flat-A-101")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if strings.HasPrefix(e.Action, "ENTRY_") {
			t.Fatal("finance leaked into registry history")
		}
	}
}
func TestReceiptLeaseFencingFailureAndRetryPreserveIdentity(t *testing.T) {
	s, token := recordFixture(t)
	ctx := context.Background()
	id := post(t, s, token, received("400"))
	now := time.Now()
	job, err := s.ClaimReceipt(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimReceipt(ctx, now); err == nil {
		t.Fatal("claimed a live lease twice")
	}
	replacement, err := s.ClaimReceipt(ctx, now.Add(61*time.Second))
	if err != nil || replacement.ID != job.ID || replacement.Lease == job.Lease {
		t.Fatal("stale lease not recovered", err)
	}
	if err = s.FinishReceipt(ctx, job, strings.Repeat("a", 64), true, now.Add(62*time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatal("stale worker published", err)
	}
	s.DB.Exec("UPDATE receipt_jobs SET attempts=5 WHERE receipt_id=?", job.ID)
	if err = s.FinishReceipt(ctx, replacement, "", false, now.Add(62*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = s.RetryReceipt(ctx, token, job.ID); err != nil {
		t.Fatal(err)
	}
	e, _ := s.EntryFor(ctx, token, id)
	if e.ReceiptID != job.ID || e.PDFState != "PENDING" {
		t.Fatal("retry changed posted identity")
	}
	regenerated, err := s.ClaimReceipt(ctx, now.Add(63*time.Second))
	if err != nil || regenerated.Snapshot.AmountPaise != 40000 || regenerated.Snapshot.Home != "A-101" {
		t.Fatal("snapshot changed", err)
	}
	if err = s.FinishReceipt(ctx, regenerated, strings.Repeat("b", 64), true, now.Add(64*time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestDiscardedDraftPreservesHistoryWithoutBalanceOrReceipt(t *testing.T) {
	s, token := recordFixture(t)
	ctx := context.Background()
	id, err := s.CreateEntry(ctx, token, received("400"))
	if err != nil {
		t.Fatal(err)
	}
	action := EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Draft supplied with the wrong amount"}
	for i := 0; i < 2; i++ {
		if _, err = s.DiscardDraft(ctx, token, id, action); err != nil {
			t.Fatal(err)
		}
	}
	e, err := s.EntryFor(ctx, token, id)
	if err != nil || e.State != "DISCARDED" || e.ReversalReason != action.Reason || e.ReceiptID != "" {
		t.Fatalf("discarded draft %+v %v", e, err)
	}
	page, err := s.EntriesFor(ctx, token, "", "", "DISCARDED", false, 1)
	if err != nil || page.Total != 1 || page.Drafts != 0 || page.BalancePaise != 0 {
		t.Fatalf("discard balance %+v %v", page, err)
	}
	if _, err = s.PostEntry(ctx, token, id, EntryAction{OperationKey: randomToken(), Confirmed: true}); !errors.Is(err, ErrConflict) {
		t.Fatal("discarded draft posted")
	}
	if _, err = s.DB.Exec("UPDATE entries SET amount_paise=1 WHERE id=?", id); err == nil {
		t.Fatal("discarded history overwritten")
	}
}
