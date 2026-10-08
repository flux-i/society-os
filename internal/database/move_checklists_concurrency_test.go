package database

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestMoveChecklistConcurrentIdenticalOperationsAndCompetingCorrectionsKeepOneResult(t *testing.T) {
	s, a, b, o := moveFixture(t)
	ctx := context.Background()
	in := moveInput()
	const workers = 4
	ids := make([]string, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range workers {
		go func(i int) { defer wg.Done(); ids[i], errs[i] = s.SubmitMoveChecklist(ctx, o, in) }(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil || ids[i] != ids[0] {
			t.Fatal("concurrent submission", ids, errs)
		}
	}
	id := ids[0]
	maintenanceCount(t, s, "SELECT COUNT(*) FROM move_checklist_resources", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM move_checklist_events", 1)
	for _, kind := range moveCheckKinds {
		moveCheck(t, s, a, id, kind)
	}
	moveReady(t, s, a, id)
	ready := moveDetail(t, s, b, id)
	approval := moveAction(ready, "APPROVED")
	approval.SourceKey = ready.Source.Key
	wg.Add(workers)
	for i := range workers {
		go func(i int) { defer wg.Done(); ids[i], errs[i] = s.ActOnMoveChecklist(ctx, b, id, approval) }(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil || ids[i] != id {
			t.Fatal("concurrent separate approval", ids, errs)
		}
	}
	accepted := moveDetail(t, s, o, id)
	if accepted.Version != 8 || accepted.EventTotal != 8 {
		t.Fatal(accepted)
	}
	corrections := []MoveChecklistAction{moveAction(accepted, "CORRECTION"), moveAction(accepted, "CORRECTION")}
	for i := range corrections {
		corrections[i].EffectiveDate = "2026-10-11"
		corrections[i].Note = "PRIVATE independently supplied linked correction and retained original."
	}
	corrections[1].EffectiveDate = "2026-10-12"
	wg.Add(2)
	for i := range 2 {
		go func(i int) { defer wg.Done(); ids[i], errs[i] = s.ActOnMoveChecklist(ctx, o, id, corrections[i]) }(i)
	}
	wg.Wait()
	success, conflict := 0, 0
	for _, e := range errs[:2] {
		if e == nil {
			success++
		} else if errors.Is(e, ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("competing current versions", errs[:2])
	}
	pending := moveDetail(t, s, o, id)
	if pending.Version != 9 || pending.EventTotal != 9 || pending.Approved.Version != 8 || !pending.Pending {
		t.Fatal(pending)
	}
}
