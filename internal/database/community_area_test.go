package database

import (
	"context"
	"errors"
	"testing"
)

func TestCommunityChangedAreaOptionsRequireReviewButAcceptedRetryRetainsOriginal(t *testing.T) {
	s, a := adminFixture(t)
	ctx := context.Background()
	in := communityTestInput(t, s, a, "CONTACT")
	id := communityPropose(t, s, a, "", in)
	accessExec(t, s, "UPDATE flats SET flat_number='999' WHERE id='demo-flat-A-104'")
	if again, e := s.ProposeCommunity(ctx, a, "", in); e != nil || again != id {
		t.Fatal("accepted retry after unrelated registry change", again, e)
	}
	in.OperationKey = randomToken()
	if _, e := s.ProposeCommunity(ctx, a, "", in); !errors.Is(e, ErrConflict) {
		t.Fatal("stale reviewed home options expanded/changed", e)
	}
	in = communityTestInput(t, s, a, "CONTACT")
	created := communityPropose(t, s, a, "", in)
	x := communityDetail(t, s, a, created, true)
	found := false
	for _, h := range x.Snapshot.Homes {
		if h.ID == "demo-flat-A-104" {
			found = true
			if h.Label != "A-999" {
				t.Fatal("wrong frozen current label", h)
			}
		}
	}
	if !found {
		t.Fatal("updated options lost home")
	}
}
