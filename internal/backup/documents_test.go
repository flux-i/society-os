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
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func TestDocumentSnapshotRestoresOriginalBytesAndSeparateReviewWithoutRevivingSessions(t *testing.T) {
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
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	data := out.Bytes()
	sum := sha256.Sum256(data)
	id, err := s.ReserveDocument(ctx, owner, database.DocumentInput{OperationKey: "document-recovery-reserve-001", Title: "Fictional original recovery record", Filename: "original.png", Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Category: "RESIDENT_DOCUMENT", Visibility: "RESIDENT_SPECIFIC"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteDocument(ctx, owner, id, data); err != nil {
		t.Fatal(err)
	}
	job, err := s.ClaimDocumentCheck(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishDocumentCheck(ctx, job, "image/png", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	doc, err := s.DocumentFor(ctx, admin, id, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideDocument(ctx, admin, id, database.DocumentAction{OperationKey: "document-recovery-approve-001", Version: doc.Version, Action: "APPROVED", Reason: "A different reviewer checked the fictional file", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "document-checkpoint")
	if _, err = Snapshot(ctx, s, bundle, "document-test"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored-documents.db")
	if _, err = Restore(ctx, bundle, target); err != nil {
		t.Fatal(err)
	}
	restored, err := database.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restored.MFA = s.MFA
	if _, err = restored.CheckSession(ctx, owner); !errors.Is(err, database.ErrUnauthenticated) {
		t.Fatal("snapshot revived a session", err)
	}
	newOwner := login(restored, "owner@demo.society")
	doc, got, err := restored.DownloadDocument(ctx, newOwner, id)
	if err != nil || !bytes.Equal(got, data) || doc.Revision != 1 || doc.State != "APPROVED" || doc.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("original or approval lost", doc, err)
	}
}
