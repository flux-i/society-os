package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
)

func httpGapInput(p database.Principal) database.RecoveryGapInput {
	now := time.Now().Unix()
	return database.RecoveryGapInput{OperationKey: "http-gap-exact-preparation-17", ScopeKey: p.ScopeKey, Title: "Supplied independent financial gap", Reason: "Current fictional books checked for this separate financial assessment.", CoverageFrom: now - 86400, CoverageThrough: now - 1, VerifiedAt: now - 1, CoverageEvidence: "http-private-book-evidence-do-not-log: supplied covered interval independently checked.", CoverageComplete: true,
		Items: []database.RecoveryGapItem{{BookRef: "http-fictional-book-2026", ItemRef: "already-received-external-item-17", FlatID: "demo-flat-A-101", Kind: "RECEIVED", Amount: "250.01", Date: "2026-01-01", Evidence: "Independently supplied account money already received after the checkpoint records were gathered.", EvidenceState: "SUPPORTED"}}}
}

func prepareGapHTTPMoney(t *testing.T, s *database.Store, client authClient) {
	t.Helper()
	ctx := context.Background()
	if err := s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	post := func(kind, amount, key string) string {
		in := database.EntryInput{OperationKey: key, FlatID: "demo-flat-A-101", Kind: kind, Amount: amount, Date: "2026-01-01", Description: "Fictional independent given amount for actual HTTP recovery"}
		if kind == "RECEIVED" {
			in.Payer = "Fictional source resident"
			in.Method = "BANK_TRANSFER"
			in.Reference = "HTTP-SUPPLIED-400"
		}
		id, err := s.CreateEntry(ctx, client.cookie.Value, in)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.PostEntry(ctx, client.cookie.Value, id, database.EntryAction{OperationKey: key + "-post", Confirmed: true}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	post("OPENING_DEBIT", "1500.01", "http-source-opening-17")
	paid := post("RECEIVED", "400.00", "http-source-received-17")
	if _, err := s.ReverseEntry(ctx, client.cookie.Value, paid, database.EntryAction{OperationKey: "http-source-reversal-17", Confirmed: true, Reason: "Independent supplied correction preserves the original 40000-paise receipt."}); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryGapHTTPRealMFASeparateFreshReviewExactMoneyAndRetry(t *testing.T) {
	s, one, two, p1, p2, _, logs := numberingHTTPReviewers(t, func(s *database.Store, client authClient) { prepareGapHTTPMoney(t, s, client) })
	in := httpGapInput(p1)
	body, _ := json.Marshal(in)
	csrf := one.csrf
	one.csrf = ""
	if w := one.request("POST", "/api/recovery/financial-gap/preparations", string(body)); w.Code != 403 {
		t.Fatal("Financial gap lacked actual CSRF", w.Code)
	}
	one.csrf = csrf
	if w := one.request("POST", "/api/recovery/financial-gap/preparations", `{"unbounded_private_input":"must-not-be-retained"}`); w.Code != 400 {
		t.Fatal("Unknown input admitted", w.Code)
	}
	w := one.request("POST", "/api/recovery/financial-gap/preparations", string(body))
	var x database.RecoveryGapPreparation
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &x) != nil || x.State != "PENDING" || len(x.Comparison.Homes) != 1 {
		t.Fatal("Real HTTP preparation failed", w.Code, w.Body.String())
	}
	home := x.Comparison.Homes[0]
	if home.CheckpointPaise != 150001 || home.DifferencePaise != -25001 || home.ComparisonPaise != 125000 || !x.Comparison.PreparedOnly {
		t.Fatal("Independent comparison amounts failed", home)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("Private financial preparation was cacheable")
	}
	var same database.RecoveryGapPreparation
	w = one.request("POST", "/api/recovery/financial-gap/preparations", string(body))
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &same) != nil || same.ID != x.ID {
		t.Fatal("Lost reply retry duplicated the journal", w.Code)
	}
	if w := two.request("GET", "/api/recovery/financial-gap/operations/"+in.OperationKey, ""); w.Code != 404 {
		t.Fatal("Other actor resolved this operation", w.Code)
	}
	d := database.RecoveryDecisionInput{OperationKey: "http-gap-separate-decision-17", ScopeKey: p1.ScopeKey, Version: 1, Decision: "APPROVED", Reason: "Another current person checked this frozen supplied financial evidence.", Attestation: "I independently checked this exact assessment and its unresolved evidence.", Confirmed: true}
	decision, _ := json.Marshal(d)
	if w := one.request("POST", "/api/recovery/financial-gap/preparations/"+x.ID+"/decision", string(decision)); w.Code != 403 {
		t.Fatal("Author self-reviewed a financial gap", w.Code)
	}
	d.ScopeKey = p2.ScopeKey
	d.Version = 2
	decision, _ = json.Marshal(d)
	if w := two.request("POST", "/api/recovery/financial-gap/preparations/"+x.ID+"/decision", string(decision)); w.Code != 409 {
		t.Fatal("Wrong exact version accepted", w.Code)
	}
	d.Version = 1
	decision, _ = json.Marshal(d)
	if _, err := s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now')-301 WHERE token_hash=?", database.TokenHash(two.cookie.Value)); err != nil {
		t.Fatal(err)
	}
	if w := two.request("POST", "/api/recovery/financial-gap/preparations/"+x.ID+"/decision", string(decision)); w.Code != 403 {
		t.Fatal("Expired freshness reviewed a gap", w.Code)
	}
	if _, err := s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now') WHERE token_hash=?", database.TokenHash(two.cookie.Value)); err != nil {
		t.Fatal(err)
	}
	w = two.request("POST", "/api/recovery/financial-gap/preparations/"+x.ID+"/decision", string(decision))
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &same) != nil || same.Version != 2 || same.State != "APPROVED" {
		t.Fatal("Separate current real identity did not decide", w.Code, w.Body.String())
	}
	if w := two.request("POST", "/api/recovery/financial-gap/preparations/"+x.ID+"/decision", string(decision)); w.Code != 200 {
		t.Fatal("Lost decision reply did not retain exact result", w.Code)
	}
	w = one.request("GET", "/api/recovery/financial-gap/balance/demo-flat-A-101", "")
	var balance database.RecoveryGapHome
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &balance) != nil || balance.CheckpointPaise != 150001 || balance.ComparisonPaise != 150001 {
		t.Fatal("Assessment posted its comparison", w.Code, balance)
	}
	var original int64
	if err := s.DB.QueryRow("SELECT e.amount_paise FROM entries e JOIN receipts r ON r.entry_id=e.id").Scan(&original); err != nil || original != 40000 {
		t.Fatal("Original received receipt changed", original, err)
	}
	for _, fragment := range []string{in.CoverageEvidence, in.Items[0].Evidence, "unbounded_private_input"} {
		if strings.Contains(logs.String(), fragment) {
			t.Fatal("Private supplied evidence reached logs")
		}
	}
}

func TestRecoveryGapHTTPBoundedReadsNoFinancialReleaseAndActualRevocation(t *testing.T) {
	s, one, _, p, _, nomination, _ := numberingHTTPReviewers(t)
	in := httpGapInput(p)
	body, _ := json.Marshal(in)
	if w := one.request("POST", "/api/recovery/financial-gap/preparations", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := one.request("GET", "/api/recovery/financial-gap/preparations?page=0", ""); w.Code != 400 {
		t.Fatal("Bad page accepted", w.Code)
	}
	if w := one.request("GET", "/api/recovery/financial-gap/balance/not-an-existing-home", ""); w.Code != 404 {
		t.Fatal("Missing home became zero balance", w.Code)
	}
	for _, path := range []string{"/api/recovery/financial-gap/post", "/api/recovery/financial-gap/release", "/api/recovery/financial-gap/issue"} {
		if w := one.request("POST", path, `{}`); w.Code != 405 {
			t.Fatal("Preparation exposed a mutation route", path, w.Code)
		}
	}
	for _, path := range []string{"/api/recovery/financial-gap/status", "/api/recovery/financial-gap/preparations", "/api/recovery/financial-gap/balance/demo-flat-A-101"} {
		w := one.request("GET", path, "")
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("Private bounded read failed", path, w.Code)
		}
	}
	var status database.RecoveryGapStatus
	w := one.request("GET", "/api/recovery/financial-gap/status", "")
	if json.Unmarshal(w.Body.Bytes(), &status) != nil || status.CanResume || status.Pending != 1 {
		t.Fatal("Status claimed financial release", w.Body.String())
	}
	if w := one.request("POST", "/api/entries", `{"operation_key":"http-held-source-entry-17","flat_id":"demo-flat-A-101","kind":"RECEIVED","amount":"1.00","date":"2026-01-01","description":"Known effects must stay held","payer":"Fictional person","method":"CASH"}`); w.Code != 423 {
		t.Fatal("Gap preparation resumed financial changes", w.Code)
	}
	origin, err := s.RecoveryOriginForCustody(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = s.RevokeRecoveryNomination(context.Background(), database.RecoveryRevocation{FormatVersion: 1, OperationID: "http-gap-current-revocation-17", EpochID: origin.ID, SourceSHA256: origin.SourceSHA256, NominationID: nomination.OperationID, Reason: "Current supplied reviewers revoked after independent fresh recovery review.", Custodians: nomination.Custodians})
	if err != nil {
		t.Fatal(err)
	}
	if w := one.request("GET", "/api/recovery/financial-gap/status", ""); w.Code != 401 {
		t.Fatal("Revoked current authority retained gap reads", w.Code)
	}
	var sessions int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE token_hash=?", database.TokenHash(one.cookie.Value)).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatal("Revocation retained the earlier bearer session", sessions, err)
	}
}
