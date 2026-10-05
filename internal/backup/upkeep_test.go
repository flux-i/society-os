package backup

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"testing"
)

func upkeepRecoveryLogin(t *testing.T, s *database.Store, email string) string {
	t.Helper()
	ctx := context.Background()
	token, p, err := s.Login(ctx, email, database.DemoPassword, database.HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	if p.MFAPending {
		if !p.MFAEnrolled {
			if _, err = s.SetupMFA(ctx, token); err != nil {
				t.Fatal(err)
			}
		}
		code, recovery, e := s.DemoVerificationCode(ctx, token)
		if e != nil {
			t.Fatal(e)
		}
		if p.MFAEnrolled {
			_, err = s.VerifyMFA(ctx, token, code, recovery)
		} else {
			_, err = s.ConfirmMFA(ctx, token, code)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return token
}
func TestUpkeepRecoveryPreservesPrivateRegisterSeparateCheckFrozenPublicSnapshotAndActivity(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	var err error
	s.MFA, err = security.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	admin := upkeepRecoveryLogin(t, s, "admin@demo.society")
	committee := upkeepRecoveryLogin(t, s, "committee@demo.society")
	owner := upkeepRecoveryLogin(t, s, "owner@demo.society")
	vin := database.UpkeepRegisterInput{OperationKey: "recovery-vendor-create-123", Name: "Checkpoint supplier", Category: "Water", Contact: "Private checkpoint contact", Phone: "+91 98765 43210", Email: "checkpoint@example.test", SourceReference: "Private checkpoint supplier source", State: "ACTIVE", Reason: "Checked the fictional checkpoint supplier record", Confirmed: true}
	vendor, err := s.SaveUpkeepRegister(ctx, admin, "VENDOR", "", vin)
	if err != nil {
		t.Fatal(err)
	}
	ain := database.UpkeepRegisterInput{OperationKey: "recovery-asset-create-1234", Name: "Checkpoint pump", Category: "Water", Location: "Private checkpoint pump room", VendorID: vendor, AMCStart: "2026-01-01", AMCEnd: "2026-12-31", InspectionDate: "2026-11-01", SourceReference: "Private checkpoint asset contract", State: "ACTIVE", Reason: "Checked fictional supplied contract and inspection dates", Confirmed: true}
	asset, err := s.SaveUpkeepRegister(ctx, admin, "ASSET", "", ain)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateUpkeepTask(ctx, admin, database.UpkeepTaskInput{OperationKey: "recovery-work-create-12345", Title: "Private checkpoint work", Body: "Private checkpoint scope and original work evidence", Category: "WATER", Priority: "HIGH", DueDate: "2026-10-10", VisitDate: "2026-10-09", AssetID: asset, VendorID: vendor, AssignedTo: "demo-user-admin", RepeatDays: 30, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	action := func(store *database.Store, token, kind string, extra database.UpkeepAction) {
		t.Helper()
		x, e := store.UpkeepTaskFor(ctx, token, id, 1)
		if e != nil {
			t.Fatal(e)
		}
		extra.OperationKey = "recovery-work-" + kind + "-123456"
		extra.Version = x.Version
		extra.Action = kind
		extra.Reason = "Private checkpoint evidence retained before this work action"
		extra.Confirmed = true
		if _, e = store.UpdateUpkeepTask(ctx, token, id, extra); e != nil {
			t.Fatal(kind, e)
		}
	}
	action(s, admin, "PUBLISH", database.UpkeepAction{Audience: "BUILDING", BuildingCode: "A", PublicTitle: "Checkpoint resident update", PublicBody: "A deliberately shared fictional work plan for current Wing A residents."})
	action(s, admin, "START", database.UpkeepAction{})
	action(s, admin, "SUBMIT_CHECK", database.UpkeepAction{})
	action(s, committee, "CONFIRM_DONE", database.UpkeepAction{})
	vin.OperationKey, vin.Version, vin.State = "recovery-vendor-retire-123", 1, "INACTIVE"
	if _, err = s.SaveUpkeepRegister(ctx, admin, "VENDOR", vendor, vin); err != nil {
		t.Fatal(err)
	}
	wantWork, err := s.UpkeepTaskFor(ctx, admin, id, 1)
	if err != nil || wantWork.State != "DONE" || wantWork.ReadyBy == wantWork.CheckedBy || wantWork.EventTotal != 5 {
		t.Fatal(wantWork, err)
	}
	wantPublic, err := s.UpkeepTaskFor(ctx, owner, id, 1)
	if err != nil || wantPublic.State != "PLANNED" || wantPublic.Version != 1 || len(wantPublic.Events) != 0 {
		t.Fatal("public checkpoint snapshot", wantPublic, err)
	}
	wantVendor, err := s.UpkeepRegisterDetailFor(ctx, admin, "VENDOR", vendor, 1)
	if err != nil {
		t.Fatal(err)
	}
	wantAsset, err := s.UpkeepRegisterDetailFor(ctx, admin, "ASSET", asset, 1)
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "upkeep-checkpoint")
	if _, err = Snapshot(ctx, s, bundle, "upkeep-test"); err != nil {
		t.Fatal(err)
	}
	action(s, admin, "UNPUBLISH", database.UpkeepAction{})
	action(s, admin, "REOPEN", database.UpkeepAction{})
	target := filepath.Join(root, "restored-upkeep.db")
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
		t.Fatal("old work session revived", err)
	}
	current := upkeepRecoveryLogin(t, restored, "admin@demo.society")
	got, err := restored.UpkeepTaskFor(ctx, current, id, 1)
	if err != nil || !reflect.DeepEqual(got, wantWork) {
		t.Fatal("private work checkpoint lost", got, err)
	}
	resident := upkeepRecoveryLogin(t, restored, "owner@demo.society")
	public, err := restored.UpkeepTaskFor(ctx, resident, id, 1)
	if err != nil || !reflect.DeepEqual(public, wantPublic) {
		t.Fatal("public checkpoint changed", public, err)
	}
	gotVendor, err := restored.UpkeepRegisterDetailFor(ctx, current, "VENDOR", vendor, 1)
	if err != nil || !reflect.DeepEqual(gotVendor, wantVendor) {
		t.Fatal("retired vendor lost", gotVendor, err)
	}
	gotAsset, err := restored.UpkeepRegisterDetailFor(ctx, current, "ASSET", asset, 1)
	if err != nil || !reflect.DeepEqual(gotAsset, wantAsset) {
		t.Fatal("asset contract/deadline link lost", gotAsset, err)
	}
	if _, err = restored.UpkeepRegisterFor(ctx, resident, "ASSET", "", "", 1); !errors.Is(err, database.ErrForbidden) {
		t.Fatal("private directory restored to resident", err)
	}
	var receipts, entries int
	if err = restored.DB.QueryRow("SELECT (SELECT COUNT(*) FROM receipts),(SELECT COUNT(*) FROM entries)").Scan(&receipts, &entries); err != nil || receipts != 0 || entries != 0 {
		t.Fatal("operational restore posted money", receipts, entries, err)
	}
}
