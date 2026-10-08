package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMoveChecklistIndependentCountsPagesUnicodeAndOriginalHistoryStayBounded(t *testing.T) {
	s, a, _, o := moveFixture(t)
	ctx := context.Background()
	ids := []string{}
	for i := range 13 {
		in := moveInput()
		in.Note = fmt.Sprintf("PRIVATE_PAGED_%02d supplied personal checklist reference", i)
		id, e := s.SubmitMoveChecklist(ctx, o, in)
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	first, e := s.MoveChecklistsFor(ctx, a, "", "", 1)
	if e != nil || len(first.Items) != 12 || first.Total != 13 || first.Counts["checking"] != 13 {
		t.Fatal(first, e)
	}
	second, e := s.MoveChecklistsFor(ctx, o, "", "", 2)
	if e != nil || len(second.Items) != 1 || second.Total != 13 || second.Counts["checking"] != 13 {
		t.Fatal(second, e)
	}
	filtered, e := s.MoveChecklistsFor(ctx, o, "PRIVATE_PAGED_07", "", 1)
	if e != nil || filtered.Total != 1 || filtered.Counts["checking"] != 13 {
		t.Fatal("filtered full counts", filtered, e)
	}
	if _, e = s.MoveChecklistsFor(ctx, o, strings.Repeat("आ", 34), "", 1); e != nil {
		t.Fatal("bounded Unicode search rejected", e)
	}
	if _, e = s.MoveChecklistsFor(ctx, o, "", "", 10001); e == nil {
		t.Fatal("unbounded page accepted")
	}
	if _, e = s.MoveChecklistsFor(ctx, o, "", "UNKNOWN", 1); e == nil {
		t.Fatal("unknown state accepted")
	}
	before := moveDetail(t, s, o, ids[0])
	time.Sleep(1100 * time.Millisecond)
	if after := moveDetail(t, s, o, ids[0]); after.CurrentKey != before.CurrentKey {
		t.Fatal("observation time changed source key")
	}
	for version := 1; version < 25; version++ {
		x := moveDetail(t, s, o, ids[0])
		in := moveAction(x, "REVISE")
		in.EffectiveDate = x.Snapshot.EffectiveDate
		in.Note = fmt.Sprintf("PRIVATE historical supplied personal revision number%02d", version)
		moveAct(t, s, o, ids[0], in)
	}
	page, e := s.MoveChecklistFor(ctx, o, ids[0], 3)
	if e != nil || page.EventTotal != 25 || len(page.Events) != 1 || page.Events[0].Version != 1 || page.Events[0].Snapshot.Note != "PRIVATE_PAGED_00 supplied personal checklist reference" {
		t.Fatal(page, e)
	}
}
