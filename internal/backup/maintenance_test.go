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

func TestMaintenanceRecoveryPreservesFrozenChargesReceiptLinksAndCheckpointCorrections(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	var err error
	s.MFA, err = security.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	login := func(store *database.Store, email string) string {
		t.Helper()
		token, p, e := store.Login(ctx, email, database.DemoPassword, database.HashPassword("dummy"))
		if e != nil {
			t.Fatal(e)
		}
		if p.MFAPending {
			if !p.MFAEnrolled {
				if _, e = store.SetupMFA(ctx, token); e != nil {
					t.Fatal(e)
				}
			}
			code, recovery, e := store.DemoVerificationCode(ctx, token)
			if e != nil {
				t.Fatal(e)
			}
			if p.MFAEnrolled {
				_, e = store.VerifyMFA(ctx, token, code, recovery)
			} else {
				_, e = store.ConfirmMFA(ctx, token, code)
			}
			if e != nil {
				t.Fatal(e)
			}
		}
		return token
	}
	admin := login(s, "admin@demo.society")
	if _, err = s.GrantAppointment(ctx, admin, "demo-user-committee", database.AppointmentInput{AccessChange: database.AccessChange{Version: 1, Confirmed: true, Reason: "Verified fictional separate maintenance reviewer before snapshot"}, Role: "TREASURER", TermDays: 30}); err != nil {
		t.Fatal(err)
	}
	reviewer := login(s, "committee@demo.society")
	proposal := database.MaintenanceInput{OperationKey: "recovery-cycle-create-12345", Title: "Checkpointed fictional maintenance", PeriodStart: "2026-01-01", PeriodEnd: "2026-01-31", DueDate: "2026-01-10", SourceReference: "Preserved private checkpoint source", Note: "Original checkpoint note", Lines: []database.MaintenanceLineInput{{FlatID: "demo-flat-A-101", Amount: "1000.00"}}, Confirmed: true}
	id, err := s.CreateMaintenanceCycle(ctx, admin, proposal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideMaintenanceCycle(ctx, reviewer, id, database.MaintenanceAction{OperationKey: "recovery-cycle-publish-1234", Version: 1, Decision: "PUBLISHED", Reason: "Separately checked supplied maintenance charges", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	cycle, err := s.MaintenanceCycleFor(ctx, admin, id, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	received, err := s.CreateEntry(ctx, admin, database.EntryInput{OperationKey: "recovery-received-create-123", FlatID: "demo-flat-A-101", Kind: "RECEIVED", Amount: "400.00", Date: "2026-01-01", Description: "Checkpointed fictional money already received", Payer: "Demo Owner A-101", Method: "CASH"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PostEntry(ctx, admin, received, database.EntryAction{OperationKey: "recovery-received-post-1234", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	entry, err := s.EntryFor(ctx, admin, received)
	if err != nil {
		t.Fatal(err)
	}
	allocation, err := s.AllocateCredit(ctx, admin, database.CreditAllocationInput{OperationKey: "recovery-allocation-123456", SourceID: received, ChargeID: cycle.Lines[0].EntryID, Amount: "300.00", Reason: "Matched checkpoint receipt to supplied charge", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	corrected, err := s.AllocateCredit(ctx, admin, database.CreditAllocationInput{OperationKey: "recovery-extra-link-123456", SourceID: received, ChargeID: cycle.Lines[0].EntryID, Amount: "50.00", Reason: "A fictional link to correct before checkpoint", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReverseAllocation(ctx, admin, corrected, database.AllocationCorrection{OperationKey: "recovery-correction-123456", Reason: "Corrected extra link before the checkpoint", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	want, err := s.HomeStatementFor(ctx, admin, "demo-flat-A-101", 1, 1, 1)
	if err != nil || want.OutstandingPaise != 70000 || want.UnallocatedPaise != 10000 || want.AllocationTotal != 2 {
		t.Fatal("unexpected checkpoint", want, err)
	}
	wantCycle, err := s.MaintenanceCycleFor(ctx, admin, id, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "maintenance-checkpoint")
	if _, err = Snapshot(ctx, s, bundle, "maintenance-test"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReverseAllocation(ctx, admin, allocation, database.AllocationCorrection{OperationKey: "recovery-later-correct-1234", Reason: "Later change must not cross the checkpoint", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReverseEntry(ctx, admin, received, database.EntryAction{OperationKey: "recovery-later-reversal-123", Reason: "Later receipt reversal must not cross checkpoint", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored-maintenance.db")
	if _, err = Restore(ctx, bundle, target); err != nil {
		t.Fatal(err)
	}
	recovered, err := database.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	recovered.MFA = s.MFA
	if err = recovered.VerifyMFAKey(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = recovered.CheckSession(ctx, admin); !errors.Is(err, database.ErrUnauthenticated) {
		t.Fatal("old treasury session restored", err)
	}
	current := login(recovered, "admin@demo.society")
	got, err := recovered.HomeStatementFor(ctx, current, "demo-flat-A-101", 1, 1, 1)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("statement checkpoint lost", got, want, err)
	}
	gotCycle, err := recovered.MaintenanceCycleFor(ctx, current, id, 1, 1)
	if err != nil || !reflect.DeepEqual(gotCycle, wantCycle) {
		t.Fatal("frozen proposal/decision lost", gotCycle, wantCycle, err)
	}
	gotEntry, err := recovered.EntryFor(ctx, current, received)
	if err != nil || gotEntry.ReceiptID != entry.ReceiptID || gotEntry.ReceiptNumber != entry.ReceiptNumber || gotEntry.State != "POSTED" || gotEntry.AmountPaise != 40000 {
		t.Fatal("immutable receipt checkpoint lost", gotEntry, err)
	}
	var total int
	if err = recovered.DB.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&total); err != nil || total != 1 {
		t.Fatal("receipt duplicated in restore", total, err)
	}
}
