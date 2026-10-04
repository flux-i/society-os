package backup

import (
	"context"
	"errors"
	"path/filepath"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"strings"
	"testing"
)

func TestComplaintRecoveryPreservesPrivateAndPublicHistoryWithSessionsInvalidated(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	login := func(store *database.Store, email string) string {
		token, p, err := store.Login(ctx, email, database.DemoPassword, database.HashPassword("dummy"))
		if err != nil {
			t.Fatal(err)
		}
		if p.MFAPending {
			if !p.MFAEnrolled {
				if _, err = store.SetupMFA(ctx, token); err != nil {
					t.Fatal(err)
				}
			}
			code, recovery, e := store.DemoVerificationCode(ctx, token)
			if e != nil {
				t.Fatal(e)
			}
			if p.MFAEnrolled {
				_, err = store.VerifyMFA(ctx, token, code, recovery)
			} else {
				_, err = store.ConfirmMFA(ctx, token, code)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		return token
	}
	owner, admin := login(s, "owner@demo.society"), login(s, "admin@demo.society")
	id, err := s.CreateComplaint(ctx, owner, database.ComplaintInput{OperationKey: "recovery-complaint-create-001", FlatID: "demo-flat-A-101", Category: "WATER", Subject: "Fictional recovery inspection", Description: "The reported description and private coordination must survive recovery.", Priority: "NORMAL"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateComplaint(ctx, admin, id, database.ComplaintAction{OperationKey: "recovery-complaint-private-001", Version: 1, Action: "COMMENT", Visibility: "STAFF_ONLY", Message: "RECOVERY_STAFF_ONLY_Keep this coordination private"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateComplaint(ctx, owner, id, database.ComplaintAction{OperationKey: "recovery-complaint-public-001", Version: 1, Action: "COMMENT", Visibility: "RESIDENT_VISIBLE", Message: "The author added public inspection details"}); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "complaint-checkpoint")
	if _, err = Snapshot(ctx, s, bundle, "complaint-test"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored-complaints.db")
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
		t.Fatal("old session revived", err)
	}
	newOwner, newAdmin := login(recovered, "owner@demo.society"), login(recovered, "admin@demo.society")
	public, err := recovered.ComplaintFor(ctx, newOwner, id, 1)
	if err != nil || public.Version != 2 || public.HistoryTotal != 2 {
		t.Fatal("public recovery changed", public, err)
	}
	for _, x := range public.Updates {
		if strings.Contains(x.Message, "RECOVERY_STAFF_ONLY") {
			t.Fatal("private recovery note exposed")
		}
	}
	private, err := recovered.ComplaintFor(ctx, newAdmin, id, 1)
	if err != nil || private.Version != 3 || private.HistoryTotal != 3 || private.Updates[1].Visibility != "STAFF_ONLY" {
		t.Fatal("private recovery history lost", private, err)
	}
	if public.Number != private.Number || public.Description != private.Description {
		t.Fatal("reported identity changed")
	}
}
