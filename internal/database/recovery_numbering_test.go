package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func suppliedNumbering(t *testing.T, s *Store, token, prefix string) RecoveryNumberingInput {
	t.Helper()
	p, err := s.CheckSession(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	return RecoveryNumberingInput{OperationKey: randomToken(), ScopeKey: p.ScopeKey, Title: "Supplied fictional recovery receipt series", Reason: "Independent current fictional numbering and custody records checked.", Policy: RecoveryNumberingSpec{Prefix: prefix, Digits: 6, First: 1, Last: 999999, Period: "Supplied fictional period 2026–27", AuthorityRef: "supplied-fictional-numbering-approval-1", NamespaceRef: "fictional-private-series-namespace", VerifiedAt: time.Now().Add(-time.Second).Unix()}}
}

func numberingDecision(t *testing.T, s *Store, token string, version int) RecoveryDecisionInput {
	t.Helper()
	p, err := s.CheckSession(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	return RecoveryDecisionInput{OperationKey: randomToken(), ScopeKey: p.ScopeKey, Version: version, Decision: "APPROVED", Reason: "Independent current person checked the exact supplied numbering bounds.", Attestation: "I checked this frozen version and its supplied current evidence.", Confirmed: true}
}

func approvedNumbering(t *testing.T, s *Store, one, two, prefix string) RecoveryNumberingPolicy {
	t.Helper()
	x, err := s.ProposeRecoveryNumbering(context.Background(), one, suppliedNumbering(t, s, one, prefix))
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.DecideRecoveryNumbering(context.Background(), two, x.ID, numberingDecision(t, s, two, 1))
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func seriesRequest(t *testing.T, s *Store, p RecoveryNumberingPolicy, n RecoveryNomination) RecoverySeriesRequest {
	t.Helper()
	origin, err := s.RecoveryOriginForCustody(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return RecoverySeriesRequest{FormatVersion: 1, OperationID: randomToken(), AttemptKey: randomToken(), EpochID: origin.ID, SourceSHA256: origin.SourceSHA256, NominationID: n.OperationID, PolicyID: p.ID, PolicyVersion: p.Version, Custodians: n.Custodians}
}

func TestRecoveryNumberingRequiresSeparateFrozenReviewAndPreservesExactMoney(t *testing.T) {
	s, money := recordFixture(t)
	ctx := context.Background()
	post(t, s, money, supplied("OPENING_DEBIT", "1000.00"))
	post(t, s, money, supplied("CHARGE", "500.01"))
	paid := post(t, s, money, received("400.00"))
	if _, err := s.ReverseEntry(ctx, money, paid, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Independent fictional correction retains the original receipt."}); err != nil {
		t.Fatal(err)
	}
	before := moneyRows(t, s)
	one, two, _ := currentRecoveryReviewers(t, s)
	in := suppliedNumbering(t, s, one, "TRV-R2026A")
	x, err := s.ProposeRecoveryNumbering(ctx, one, in)
	if err != nil || x.State != "PENDING" || x.FirstExample != "TRV-R2026A-000001" || x.LastExample != "TRV-R2026A-999999" || x.Reservation != nil {
		t.Fatal("Numbering changed supplied examples or issued a reservation", x, err)
	}
	if _, err = s.ProposeRecoveryNumbering(ctx, money, in); err == nil {
		t.Fatal("Copied money authority prepared a recovery policy")
	}
	same, err := s.ProposeRecoveryNumbering(ctx, one, in)
	if err != nil || same.ID != x.ID {
		t.Fatal("Exact policy retry changed identity", err)
	}
	changed := in
	changed.Policy.Prefix = "TRV-R2026B"
	if _, err = s.ProposeRecoveryNumbering(ctx, one, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("Frozen operation accepted different numbering", err)
	}
	if _, err = s.RecoveryNumberingOperationFor(ctx, two, in.OperationKey); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("Another actor read an operation result", err)
	}
	decision := numberingDecision(t, s, one, 1)
	if _, err = s.DecideRecoveryNumbering(ctx, one, x.ID, decision); !errors.Is(err, ErrForbidden) {
		t.Fatal("Author self-approved numbering", err)
	}
	decision = numberingDecision(t, s, two, 2)
	if _, err = s.DecideRecoveryNumbering(ctx, two, x.ID, decision); !errors.Is(err, ErrConflict) {
		t.Fatal("Wrong frozen version was approved", err)
	}
	decision.Version = 1
	if _, err = s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now')-301 WHERE token_hash=?", TokenHash(two)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideRecoveryNumbering(ctx, two, x.ID, decision); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("Expired identity approved numbering", err)
	}
	if _, err = s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now') WHERE token_hash=?", TokenHash(two)); err != nil {
		t.Fatal(err)
	}
	approved, err := s.DecideRecoveryNumbering(ctx, two, x.ID, decision)
	if err != nil || approved.State != "APPROVED" || approved.Version != 2 {
		t.Fatal(approved, err)
	}
	if again, err := s.DecideRecoveryNumbering(ctx, two, x.ID, decision); err != nil || again.ID != x.ID || again.Version != 2 {
		t.Fatal("Exact decision retry duplicated it", err)
	}
	changedDecision := decision
	changedDecision.Reason = "A changed reason cannot overwrite a frozen current numbering approval."
	if _, err = s.DecideRecoveryNumbering(ctx, two, x.ID, changedDecision); !errors.Is(err, ErrConflict) {
		t.Fatal("Changed decision replay was accepted", err)
	}
	for _, table := range []string{"entries", "entry_reversals", "receipts", "receipt_counter", "receipt_jobs", "role_grants", "flat_memberships"} {
		if _, ok := before[table]; !ok {
			t.Fatal("Missing independent original-table expectation", table)
		}
		if before[table] != moneyRows(t, s)[table] {
			t.Fatal("Numbering preparation changed originals", table)
		}
	}
	var amount, balance int64
	if err := s.DB.QueryRow("SELECT amount_paise FROM entries WHERE id=?", paid).Scan(&amount); err != nil || amount != 40000 {
		t.Fatal("Original paid amount changed", amount, err)
	}
	if err := s.DB.QueryRow("SELECT SUM(CASE WHEN kind='RECEIVED' THEN -amount_paise ELSE amount_paise END) FROM entries WHERE state='POSTED' AND id NOT IN(SELECT entry_id FROM entry_reversals)").Scan(&balance); err != nil || balance != 150001 {
		t.Fatal("Independent corrected balance changed", balance, err)
	}
	for _, token := range []string{one, two} {
		if _, err := s.EntryFor(ctx, token, paid); !errors.Is(err, ErrRecoveryRequired) {
			t.Fatal("Prepared numbering opened private finance", err)
		}
	}
	for _, query := range []string{"UPDATE receipt_counter SET next_number=next_number+1", "UPDATE recovery_numbering_policies SET input_json='{}'", "DELETE FROM recovery_numbering_decisions"} {
		if _, err := s.DB.Exec(query); err == nil {
			t.Fatal("A retained original could be rewritten", query)
		}
	}
	status, err := s.RecoveryNumberingStatusFor(ctx, one)
	if err != nil || status.Pending != 0 || status.Approved != 1 || status.Verified != 0 || status.CanResume {
		t.Fatal("Approval was treated as issuance/release", status, err)
	}
}

func TestRecoveryNumberingLinkedCorrectionAndConcurrentDecisionAreImmutable(t *testing.T) {
	s, _ := adminFixture(t)
	ctx := context.Background()
	one, two, _ := currentRecoveryReviewers(t, s)
	in := suppliedNumbering(t, s, one, "TRV-R2026A")
	x, err := s.ProposeRecoveryNumbering(ctx, one, in)
	if err != nil {
		t.Fatal(err)
	}
	first := numberingDecision(t, s, two, 1)
	second := first
	second.OperationKey = randomToken()
	second.Decision = "DECLINED"
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, decision := range []RecoveryDecisionInput{first, second} {
		wg.Add(1)
		go func(d RecoveryDecisionInput) {
			defer wg.Done()
			_, err := s.DecideRecoveryNumbering(ctx, two, x.ID, d)
			results <- err
		}(decision)
	}
	wg.Wait()
	close(results)
	accepted, refused := 0, 0
	for err := range results {
		if err == nil {
			accepted++
		} else if errors.Is(err, ErrConflict) {
			refused++
		} else {
			t.Fatal(err)
		}
	}
	if accepted != 1 || refused != 1 {
		t.Fatal("Conflicting reviews did not have one winner", accepted, refused)
	}
	old, err := s.RecoveryNumberingPolicyFor(ctx, one, x.ID)
	if err != nil {
		t.Fatal(err)
	}
	decisionBefore := statementMessageRows(t, s, []string{"recovery_numbering_decisions"})
	in.OperationKey, in.PredecessorID, in.PredecessorVersion, in.Policy.Prefix = randomToken(), old.ID, old.Version, "TRV-R2026B"
	linked, err := s.ProposeRecoveryNumbering(ctx, one, in)
	if err != nil || linked.Version != 3 || linked.PredecessorID != old.ID {
		t.Fatal("Linked correction lost its original", linked, err)
	}
	old, err = s.RecoveryNumberingPolicyFor(ctx, one, old.ID)
	if err != nil || old.State != "SUPERSEDED" || old.Decision == nil {
		t.Fatal("Original decision remained actionable or disappeared", old, err)
	}
	if !reflect.DeepEqual(decisionBefore, statementMessageRows(t, s, []string{"recovery_numbering_decisions"})) {
		t.Fatal("Correction rewrote its original decision")
	}
	history, err := s.RecoveryNumberingHistoryFor(ctx, one, linked.ID, 1)
	if err != nil || history.Total != 2 || len(history.Items) != 2 || history.Items[0].ID != linked.ID || history.Items[1].State != "SUPERSEDED" {
		t.Fatal(history, err)
	}
	page, err := s.RecoveryNumberingPoliciesFor(ctx, one, "SUPERSEDED", 1)
	if err != nil || page.Total != 1 {
		t.Fatal("Filtered list and original state disagree", page, err)
	}
	if _, err := s.RecoveryNumberingPoliciesFor(ctx, one, "APPROVED", 0); !errors.Is(err, ErrInvalid) {
		t.Fatal("Unbounded page accepted", err)
	}
}

func TestRecoverySeriesRetainsOneCipherAndUnknownNeverClaimsAnotherHandoff(t *testing.T) {
	s, money := recordFixture(t)
	ctx := context.Background()
	post(t, s, money, received("400.00"))
	one, two, nomination := currentRecoveryReviewers(t, s)
	policy := approvedNumbering(t, s, one, two, "TRV-R2026A")
	in := seriesRequest(t, s, policy, nomination)
	seals := 0
	seal := func(body []byte) ([]byte, error) {
		seals++
		return append([]byte("store-boundary-test-cipher-not-an-encryption-proof:"), body...), nil
	}
	intent, err := s.RetainRecoverySeries(ctx, in, strings.Repeat("b", 64), "private/fictional-series", seal)
	if err != nil || intent.State != "PREPARED" || seals != 1 {
		t.Fatal(intent.State, seals, err)
	}
	wantHash := sha256.Sum256(intent.Ciphertext)
	if intent.CipherSHA256 != hex.EncodeToString(wantHash[:]) || intent.CipherBytes != int64(len(intent.Ciphertext)) {
		t.Fatal("Stored original checksum/size differs")
	}
	again, err := s.RetainRecoverySeries(ctx, in, strings.Repeat("b", 64), "private/fictional-series", seal)
	if err != nil || seals != 1 || !bytes.Equal(intent.Ciphertext, again.Ciphertext) {
		t.Fatal("Exact retry resealed its original", err, seals)
	}
	claim, started, err := s.StartRecoverySeriesHandoff(ctx, in.OperationID, in.AttemptKey, nil)
	if err != nil || !started || claim.Attempts != 1 || claim.State != "HANDOFF" {
		t.Fatal(claim.State, started, err)
	}
	if _, started, err = s.StartRecoverySeriesHandoff(ctx, in.OperationID, in.AttemptKey, nil); err != nil || started {
		t.Fatal("Lost reply repeated a retained handoff", err)
	}
	unknown, err := s.FinishRecoverySeriesHandoff(ctx, in.OperationID, in.AttemptKey, "UNKNOWN", "UPLOAD_UNCERTAIN", "")
	if err != nil || unknown.State != "UNKNOWN" {
		t.Fatal(err)
	}
	retry := RecoverySeriesRetry{1, in.OperationID, randomToken(), nomination.Custodians}
	if _, started, err := s.StartRecoverySeriesHandoff(ctx, in.OperationID, retry.AttemptKey, &retry); !errors.Is(err, ErrConflict) || started {
		t.Fatal("Uncertainty was retried", err)
	}
	verified, err := s.FinishRecoverySeriesHandoff(ctx, in.OperationID, in.AttemptKey, "VERIFIED", "CUSTODY_VERIFIED", "independently-retrieved-exact-version-1")
	if err != nil || verified.State != "VERIFIED" || verified.Attempts != 1 || !bytes.Equal(verified.Ciphertext, intent.Ciphertext) {
		t.Fatal("Independent reconciliation replaced its original", err)
	}
	status, err := s.RecoveryNumberingStatusFor(ctx, one)
	if err != nil || status.Verified != 1 || status.Unresolved != 0 || status.CanResume {
		t.Fatal("Reservation released recovery", status, err)
	}
	for _, query := range []string{"UPDATE recovery_series_intents SET ciphertext=zeroblob(cipher_bytes)", "DELETE FROM recovery_series_events", "UPDATE receipt_counter SET next_number=next_number+1"} {
		if _, err := s.DB.Exec(query); err == nil {
			t.Fatal("Retained reservation/financial hold rewritten", query)
		}
	}
	changed := suppliedNumbering(t, s, one, "TRV-R2026A")
	if _, err = s.ProposeRecoveryNumbering(ctx, one, changed); !errors.Is(err, ErrInvalid) {
		t.Fatal("Attempted prefix was recycled", err)
	}
}

func TestRecoverySeriesKnownRejectionHasBoundedExplicitAttemptsAndStaleAcknowledgement(t *testing.T) {
	s, _ := adminFixture(t)
	ctx := context.Background()
	one, two, nomination := currentRecoveryReviewers(t, s)
	policy := approvedNumbering(t, s, one, two, "TRV-R2026A")
	in := seriesRequest(t, s, policy, nomination)
	intent, err := s.RetainRecoverySeries(ctx, in, strings.Repeat("b", 64), "private/fictional-series", func(body []byte) ([]byte, error) {
		return append([]byte("store-only-cipher-boundary-original:"), body...), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	key := in.AttemptKey
	for attempt := 1; attempt <= 3; attempt++ {
		var retry *RecoverySeriesRetry
		if attempt > 1 {
			key = randomToken()
			retry = &RecoverySeriesRetry{1, in.OperationID, key, nomination.Custodians}
		}
		claim, started, err := s.StartRecoverySeriesHandoff(ctx, in.OperationID, key, retry)
		if err != nil || !started || claim.Attempts != attempt {
			t.Fatal(attempt, claim.Attempts, started, err)
		}
		if !bytes.Equal(claim.Ciphertext, intent.Ciphertext) {
			t.Fatal("Known rejection retry changed ciphertext")
		}
		if _, err = s.FinishRecoverySeriesHandoff(ctx, in.OperationID, key, "REJECTED", "UPLOAD_DENIED", ""); err != nil {
			t.Fatal(err)
		}
	}
	retry := &RecoverySeriesRetry{1, in.OperationID, randomToken(), nomination.Custodians}
	if _, started, err := s.StartRecoverySeriesHandoff(ctx, in.OperationID, retry.AttemptKey, retry); !errors.Is(err, ErrConflict) || started {
		t.Fatal("Fourth handoff bypassed the bound", err)
	}
	second := approvedNumbering(t, s, one, two, "TRV-R2026B")
	request := seriesRequest(t, s, second, nomination)
	if _, err = s.RetainRecoverySeries(ctx, request, strings.Repeat("b", 64), "private/fictional-series", func(body []byte) ([]byte, error) {
		return append([]byte("store-only-second-cipher-boundary:"), body...), nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.StartRecoverySeriesHandoff(ctx, request.OperationID, request.AttemptKey, nil); err != nil {
		t.Fatal(err)
	}
	replacement := nomination
	replacement.OperationID = "supplied-replacement-numbering-review"
	replacement.ReplacesOperationID = nomination.OperationID
	for i := range replacement.Reviewers {
		replacement.Reviewers[i].AccountID = ""
		if err := s.DB.QueryRow("SELECT id FROM users WHERE login=?", replacement.Reviewers[i].Email).Scan(&replacement.Reviewers[i].AccountID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.NominateRecovery(ctx, replacement, RecoveryPasswords{Passwords: []RecoveryPassword{}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.StartRecoverySeriesHandoff(ctx, request.OperationID, request.AttemptKey, nil); !errors.Is(err, ErrConflict) {
		t.Fatal("Earlier nomination regained a current handoff", err)
	}
	if _, err := s.FinishRecoverySeriesHandoff(ctx, request.OperationID, request.AttemptKey, "VERIFIED", "STORAGE_CONFIRMED", "late-exact-version-2"); err != nil {
		t.Fatal("Late genuine acknowledgement was not retained historically", err)
	}
	old, err := numberingPolicy(ctx, s.DB, Principal{recoveryEpoch: nomination.EpochID, recoveryNomination: replacement.OperationID}, second.ID)
	if err != nil || old.State != "STALE" || old.Reservation.State != "STALE" || old.Reservation.OriginalState != "VERIFIED" || old.Reservation.ChecksumVerified {
		t.Fatal("Historical acknowledgement became current authority", old, err)
	}
}
