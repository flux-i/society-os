package backup

import (
	"context"
	"errors"
	"path/filepath"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"testing"
)

func TestReviewRecoveryRetainsApprovedContentAndDecisionsWithoutRevivingSessions(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	admin, _, err := s.Login(ctx, "admin@demo.society", database.DemoPassword, database.HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetupMFA(ctx, admin); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.DemoVerificationCode(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmMFA(ctx, admin, code); err != nil {
		t.Fatal(err)
	}
	owner, _, err := s.Login(ctx, "owner@demo.society", database.DemoPassword, database.HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.SubmitReview(ctx, owner, "", database.ReviewInput{OperationKey: "review-recovery-submission-001", Kind: "NOTICE", Title: "Fictional recovery notice", Body: "An approved notice whose exact submitted content must survive a restore.", Audience: "OWNERS_ONLY"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, admin, id, database.ReviewAction{OperationKey: "review-recovery-approval-001", Version: 1, Decision: "APPROVED", Reason: "Reviewed and approved before the snapshot", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "review-checkpoint")
	if _, err = Snapshot(ctx, s, bundle, "review-test"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, admin, id, database.ReviewAction{OperationKey: "review-recovery-archive-001", Version: 2, Decision: "ARCHIVED", Reason: "Changed only after the recovery checkpoint", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored-review.db")
	if _, err = Restore(ctx, bundle, target); err != nil {
		t.Fatal(err)
	}
	recovered, err := database.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	recovered.MFA = s.MFA
	if _, err = recovered.CheckSession(ctx, owner); !errors.Is(err, database.ErrUnauthenticated) {
		t.Fatal("restored session revived", err)
	}
	newOwner, _, err := recovered.Login(ctx, "owner@demo.society", database.DemoPassword, database.HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	x, err := recovered.ReviewFor(ctx, newOwner, id, false)
	if err != nil || x.State != "APPROVED" || x.Version != 2 || len(x.Events) != 2 || x.Events[1].Reason != "Reviewed and approved before the snapshot" {
		t.Fatal("review history changed in recovery", x, err)
	}
	notice, err := recovered.ReviewFor(ctx, newOwner, id, true)
	if err != nil || notice.Body != x.Body || len(notice.Events) != 0 {
		t.Fatal("published version lost or private events leaked", notice, err)
	}
}
