package database

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
)

func reviewLogin(t *testing.T, s *Store, email string) string {
	t.Helper()
	token, p := loginTest(t, s, email, DemoPassword)
	if p.MFAPending {
		if !p.MFAEnrolled {
			if _, err := s.SetupMFA(context.Background(), token); err != nil {
				t.Fatal(err)
			}
		}
		code, recovery, err := s.DemoVerificationCode(context.Background(), token)
		if err != nil {
			t.Fatal(err)
		}
		if p.MFAEnrolled {
			_, err = s.VerifyMFA(context.Background(), token, code, recovery)
		} else {
			_, err = s.ConfirmMFA(context.Background(), token, code)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return token
}
func proposal(kind string) ReviewInput {
	in := ReviewInput{OperationKey: randomToken(), Kind: kind, Title: "Fictional community request", Body: "A fictional proposal with enough detail for the reviewer."}
	if kind == "NOTICE" {
		in.Audience = "ALL_RESIDENTS"
	} else {
		in.FlatID = "demo-flat-A-101"
	}
	return in
}
func decision(version int, state string) ReviewAction {
	return ReviewAction{OperationKey: randomToken(), Version: version, Decision: state, Reason: "Reviewed against the fictional source", Confirmed: true}
}

func TestReviewsRequireSeparateReviewerAndKeepImmutableHistory(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	in := proposal("NOTICE")
	id, err := s.SubmitReview(ctx, admin, "", in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, admin, id, decision(1, "APPROVED")); !errors.Is(err, ErrForbidden) {
		t.Fatal("self approval accepted", err)
	}
	committee := reviewLogin(t, s, "committee@demo.society")
	approve := decision(1, "APPROVED")
	for i := 0; i < 2; i++ {
		if _, err = s.DecideReview(ctx, committee, id, approve); err != nil {
			t.Fatal(err)
		}
	}
	x, err := s.ReviewFor(ctx, admin, id, false)
	if err != nil || x.State != "APPROVED" || x.Version != 2 || len(x.Events) != 2 || x.Events[1].Actor == x.Author {
		t.Fatalf("decision history %+v %v", x, err)
	}
	for _, query := range []string{"DELETE FROM review_requests WHERE id=?", "UPDATE review_requests SET body='different content' WHERE id=?", "UPDATE review_requests SET state='PENDING' WHERE id=?", "DELETE FROM review_events WHERE request_id=?", "UPDATE review_events SET reason='overwritten' WHERE request_id=?"} {
		if _, err = s.DB.Exec(query, id); err == nil {
			t.Fatal("immutable decision/history changed", query)
		}
	}
	if _, err = s.DecideReview(ctx, committee, id, decision(1, "REJECTED")); !errors.Is(err, ErrConflict) {
		t.Fatal("stale decision accepted", err)
	}
}

func TestReviewsRequestChangesResubmitRejectAndWithdrawal(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	in := proposal("MAINTENANCE")
	id, err := s.SubmitReview(ctx, owner, "", in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, admin, id, decision(1, "CHANGES_REQUESTED")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, admin, id, decision(2, "APPROVED")); !errors.Is(err, ErrConflict) {
		t.Fatal("unresubmitted item approved", err)
	}
	in.OperationKey = randomToken()
	in.Version = 2
	in.Body = "Updated fictional scope, following the reviewer's requested changes."
	for i := 0; i < 2; i++ {
		if _, err = s.SubmitReview(ctx, owner, id, in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.DecideReview(ctx, admin, id, decision(3, "REJECTED")); err != nil {
		t.Fatal(err)
	}
	x, err := s.ReviewFor(ctx, owner, id, false)
	if err != nil || len(x.Events) != 4 || x.State != "REJECTED" {
		t.Fatal(x, err)
	}
	in = proposal("EXPENSE")
	in.Estimate = "123.45"
	id, err = s.SubmitReview(ctx, owner, "", in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, owner, id, decision(1, "WITHDRAWN")); err != nil {
		t.Fatal(err)
	}
	x, err = s.ReviewFor(ctx, owner, id, false)
	if err != nil || x.State != "WITHDRAWN" || x.EstimatePaise != 12345 {
		t.Fatal(x, err)
	}
	var entries int
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM entries").Scan(&entries); err != nil || entries != 0 {
		t.Fatal("proposal changed the financial ledger", err)
	}
}

func TestNoticeAudienceFiltersCurrentMembershipAndNeverExposesPrivateQueue(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	committee := reviewLogin(t, s, "committee@demo.society")
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	for _, audience := range []string{"ALL_RESIDENTS", "OWNERS_ONLY", "TENANTS_ONLY", "COMMITTEE_ONLY", "BUILDING"} {
		in := proposal("NOTICE")
		in.Audience = audience
		in.Title = "Fictional " + audience + " notice"
		if audience == "BUILDING" {
			in.BuildingCode = "B"
		}
		id, err := s.SubmitReview(ctx, committee, "", in)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ReviewFor(ctx, owner, id, true); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("unapproved notice exposed", err)
		}
		if _, err = s.DecideReview(ctx, admin, id, decision(1, "APPROVED")); err != nil {
			t.Fatal(err)
		}
		if _, err = s.ReviewFor(ctx, owner, id, false); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("private review detail exposed", err)
		}
		for _, person := range []struct {
			token   string
			allowed bool
		}{{owner, audience == "ALL_RESIDENTS" || audience == "OWNERS_ONLY"}, {tenant, audience == "ALL_RESIDENTS" || audience == "TENANTS_ONLY"}} {
			x, err := s.ReviewFor(ctx, person.token, id, true)
			if person.allowed {
				if err != nil || len(x.Events) != 0 {
					t.Fatal("notice inaccessible or private review exposed", x, err)
				}
			} else if !errors.Is(err, sql.ErrNoRows) {
				t.Fatal("audience mismatch exposed", audience, err)
			}
		}
	}
	for _, token := range []string{owner, tenant} {
		page, err := s.ReviewsFor(ctx, token, "", "", 1, true)
		if err != nil || page.Total != 2 {
			t.Fatal("audience count leak", page, err)
		}
		page, err = s.ReviewsFor(ctx, token, "", "", 1, false)
		if err != nil || page.Total != 0 {
			t.Fatal("private queue exposed", page, err)
		}
	}
	if _, err := s.DB.Exec("UPDATE flat_memberships SET end_date='2026-01-01' WHERE resident_id='demo-tenant-A-103'"); err != nil {
		t.Fatal(err)
	}
	page, err := s.ReviewsFor(ctx, tenant, "", "", 1, true)
	if err != nil || page.Total != 0 {
		t.Fatal("departed resident retained current notices", page, err)
	}
}

func TestReviewsValidateScopesReplayIdentityAndCurrentReviewerPermission(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	in := proposal("REGISTRY_CHANGE")
	in.FlatID = "demo-flat-B-101"
	if _, err := s.SubmitReview(ctx, owner, "", in); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign home submission accepted", err)
	}
	in = proposal("NOTICE")
	in.Audience = "BUILDING"
	if _, err := s.SubmitReview(ctx, owner, "", in); !errors.Is(err, ErrInvalid) {
		t.Fatal("wing notice without target", err)
	}
	in = proposal("MAINTENANCE")
	id, err := s.SubmitReview(ctx, owner, "", in)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.SubmitReview(ctx, owner, "", in)
	if err != nil || replay != id {
		t.Fatal("duplicate submission", err)
	}
	in.Title = "Changed retry content"
	if _, err = s.SubmitReview(ctx, owner, "", in); !errors.Is(err, ErrConflict) {
		t.Fatal("changed retry accepted", err)
	}
	if _, err = s.DecideReview(ctx, owner, id, decision(1, "APPROVED")); !errors.Is(err, ErrForbidden) {
		t.Fatal("resident reviewer", err)
	}
	if _, err = s.DB.Exec("UPDATE sessions SET reauthenticated_at=1 WHERE token_hash=?", TokenHash(admin)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, admin, id, decision(1, "APPROVED")); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("stale privileged authentication accepted", err)
	}
}

func TestConcurrentReviewDecisionsHaveOneWinner(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	committee := reviewLogin(t, s, "committee@demo.society")
	owner := reviewLogin(t, s, "owner@demo.society")
	id, err := s.SubmitReview(ctx, owner, "", proposal("MAINTENANCE"))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, token := range []string{admin, committee} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			_, err := s.DecideReview(ctx, token, id, decision(1, "APPROVED"))
			results <- err
		}(token)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal("conflicting decisions both won", success, conflicts)
	}
	x, err := s.ReviewFor(ctx, owner, id, false)
	if err != nil || len(x.Events) != 2 {
		t.Fatal("duplicate decision events", x, err)
	}
}
