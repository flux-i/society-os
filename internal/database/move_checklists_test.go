package database

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func moveFixture(t *testing.T) (*Store, string, string, string) {
	t.Helper()
	s, a := adminFixture(t)
	grantAppointment(t, s, a, "demo-user-committee", "ADMINISTRATOR", 30)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	return s, a, b, o
}
func moveInput() MoveChecklistInput {
	return MoveChecklistInput{OperationKey: randomToken(), FlatID: "demo-flat-A-101", Kind: "CONTACT_REVIEW", EffectiveDate: "2026-10-10", Note: "PRIVATE deliberately supplied review of current registry and contact preferences.", Reason: "PRIVATE confirm this exact current personal checklist submission.", Confirmed: true}
}
func moveDetail(t *testing.T, s *Store, a, id string) MoveChecklistDetail {
	t.Helper()
	x, e := s.MoveChecklistFor(context.Background(), a, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func moveAction(x MoveChecklistDetail, action string) MoveChecklistAction {
	return MoveChecklistAction{OperationKey: randomToken(), Version: x.Version, Action: action, Reason: "PRIVATE deliberately review the exact checklist source and requested effect.", Confirmed: true}
}
func moveAct(t *testing.T, s *Store, a, id string, in MoveChecklistAction) {
	t.Helper()
	if _, e := s.ActOnMoveChecklist(context.Background(), a, id, in); e != nil {
		t.Fatal(in.Action, in.CheckKind, e)
	}
}
func moveCheck(t *testing.T, s *Store, a, id, kind string) {
	t.Helper()
	x := moveDetail(t, s, a, id)
	in := moveAction(x, "CHECK")
	in.CheckKind = kind
	in.CheckState = "CHECKED"
	in.Reference = "PRIVATE independently supplied source reference for " + kind
	in.SourceKey = x.Source.Key
	moveAct(t, s, a, id, in)
}
func moveReady(t *testing.T, s *Store, a, id string) {
	t.Helper()
	x := moveDetail(t, s, a, id)
	in := moveAction(x, "READY")
	in.SourceKey = x.Source.Key
	moveAct(t, s, a, id, in)
}

func TestMoveChecklistIndependentChecksSeparateCompletionRetriesAndRetainedCorrection(t *testing.T) {
	s, a, b, o := moveFixture(t)
	ctx := context.Background()
	before, e := s.Flat(ctx, "demo-flat-A-101")
	if e != nil {
		t.Fatal(e)
	}
	input := moveInput()
	id, e := s.SubmitMoveChecklist(ctx, o, input)
	if e != nil {
		t.Fatal(e)
	}
	original := moveDetail(t, s, a, id)
	if original.Version != 1 || original.Checked != 0 || original.EventTotal != 1 || original.Phase != "CHECKING" {
		t.Fatal(original)
	}
	for _, kind := range []string{"IDENTITY", "REGISTRY", "DOCUMENTS", "HANDOVER"} {
		moveCheck(t, s, a, id, kind)
	}
	x := moveDetail(t, s, a, id)
	in := moveAction(x, "READY")
	in.SourceKey = x.Source.Key
	if _, e = s.ActOnMoveChecklist(ctx, a, id, in); !errors.Is(e, ErrConflict) {
		t.Fatal("four checks cannot replace missing CONTACT", e)
	}
	if moveDetail(t, s, a, id).Version != 5 {
		t.Fatal("rejected readiness changed history")
	}
	moveCheck(t, s, a, id, "CONTACT")
	moveReady(t, s, a, id)
	x = moveDetail(t, s, a, id)
	in = moveAction(x, "APPROVED")
	in.SourceKey = x.Source.Key
	if x.Version != 7 || !moveDetail(t, s, b, id).CanComplete {
		t.Fatal("independent ready proposal", x)
	}
	if _, e = s.ActOnMoveChecklist(ctx, a, id, in); !errors.Is(e, ErrForbidden) {
		t.Fatal("checking submitter cannot approve", e)
	}
	if _, e = s.ActOnMoveChecklist(ctx, o, id, in); !errors.Is(e, ErrForbidden) {
		t.Fatal("resident author cannot approve", e)
	}
	moveAct(t, s, b, id, in)
	accepted := moveDetail(t, s, o, id)
	if accepted.Version != 8 || accepted.EventTotal != 8 || accepted.Approved == nil || accepted.Approved.Version != 8 || accepted.State != "COMPLETED" || accepted.Pending {
		t.Fatal(accepted)
	}
	if retried, e := s.ActOnMoveChecklist(ctx, b, id, in); e != nil || retried != id {
		t.Fatal("same approval retry", retried, e)
	}
	changed := in
	changed.Reason = "PRIVATE different reviewed reason must conflict with this accepted retry identity."
	if _, e = s.ActOnMoveChecklist(ctx, b, id, changed); !errors.Is(e, ErrConflict) {
		t.Fatal("changed retry accepted", e)
	}
	if moveDetail(t, s, b, id).EventTotal != 8 {
		t.Fatal("approval retry duplicated an event")
	}
	correction := moveAction(accepted, "CORRECTION")
	correction.EffectiveDate = "2026-10-11"
	correction.Note = "PRIVATE correct the supplied review date without altering the accepted original."
	moveAct(t, s, o, id, correction)
	pending := moveDetail(t, s, o, id)
	if pending.Version != 9 || !pending.Pending || pending.Checked != 0 || pending.State != "COMPLETED" || !reflect.DeepEqual(accepted.Approved, pending.Approved) {
		t.Fatal("pending correction displaced accepted original", pending)
	}
	cancel := moveAction(pending, "CANCELLED")
	moveAct(t, s, o, id, cancel)
	retained := moveDetail(t, s, o, id)
	if retained.Version != 10 || retained.Pending || retained.State != "COMPLETED" || !reflect.DeepEqual(accepted.Approved, retained.Approved) {
		t.Fatal("withdrawal changed original", retained)
	}
	after, e := s.Flat(ctx, "demo-flat-A-101")
	if e != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("checklist changed registry", after, e)
	}
	for _, table := range []string{"entries", "receipts", "resident_contacts"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
	if retained.Events[len(retained.Events)-1].Version != 1 || !reflect.DeepEqual(original.Snapshot, retained.Events[len(retained.Events)-1].Snapshot) {
		t.Fatal("original submission changed")
	}
}

func TestMoveChecklistSourceChangesPreserveNotesAndRequireCurrentChecksBeforeCompletion(t *testing.T) {
	s, a, b, o := moveFixture(t)
	ctx := context.Background()
	id, e := s.SubmitMoveChecklist(ctx, o, moveInput())
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range moveCheckKinds {
		moveCheck(t, s, a, id, kind)
	}
	moveReady(t, s, a, id)
	ready := moveDetail(t, s, b, id)
	approval := moveAction(ready, "APPROVED")
	approval.SourceKey = ready.Source.Key
	contact := contactInput()
	if _, e = s.RegisterContact(ctx, o, "me", contact); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnMoveChecklist(ctx, b, id, approval); !errors.Is(e, ErrConflict) {
		t.Fatal("changed contact accepted", e)
	}
	stale := moveDetail(t, s, a, id)
	if stale.Version != 7 || stale.StaleChecks != 5 || stale.CanComplete || !reflect.DeepEqual(stale.Snapshot.Checks, ready.Snapshot.Checks) {
		t.Fatal("stale contact erased checks", stale)
	}
	moveAct(t, s, a, id, moveAction(stale, "RETURN"))
	moveCheck(t, s, a, id, "CONTACT")
	partial := moveDetail(t, s, a, id)
	if partial.StaleChecks != 4 || partial.CanReady {
		t.Fatal("one refreshed item made others current", partial)
	}
	for _, kind := range []string{"IDENTITY", "REGISTRY", "DOCUMENTS", "HANDOVER"} {
		moveCheck(t, s, a, id, kind)
	}
	moveReady(t, s, a, id)
	ready = moveDetail(t, s, b, id)
	if ready.Version != 14 {
		t.Fatal("independent event sequence", ready.Version)
	}
	approval = moveAction(ready, "APPROVED")
	approval.SourceKey = ready.Source.Key
	if e = s.ChangeOccupancy(ctx, a, "demo-flat-A-101", OccupancyChange{RegistryChange: RegistryChange{Version: 1, Reason: "PRIVATE supplied occupancy correction for current source checks"}, Status: "RENTED"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActOnMoveChecklist(ctx, b, id, approval); !errors.Is(e, ErrConflict) {
		t.Fatal("changed registry accepted", e)
	}
	stale = moveDetail(t, s, a, id)
	if stale.StaleChecks != 5 || stale.Version != 14 || stale.Snapshot.Note != moveInput().Note {
		t.Fatal(stale)
	}
	moveAct(t, s, a, id, moveAction(stale, "RETURN"))
	for _, kind := range moveCheckKinds {
		moveCheck(t, s, a, id, kind)
	}
	moveReady(t, s, a, id)
	ready = moveDetail(t, s, b, id)
	approval = moveAction(ready, "APPROVED")
	approval.SourceKey = ready.Source.Key
	moveAct(t, s, b, id, approval)
	if got := moveDetail(t, s, o, id); got.Version != 22 || got.Approved == nil || got.Approved.Source.FlatVersion != 2 || got.Approved.Source.ContactVersion != 1 {
		t.Fatal(got)
	}
	profile, e := s.ContactFor(ctx, o, "me", 1)
	if e != nil || profile.State != "PENDING" || profile.Eligible != (ContactPreferences{}) {
		t.Fatal("completion invented verified consent", profile, e)
	}
}
