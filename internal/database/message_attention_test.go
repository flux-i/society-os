package database

import (
	"context"
	"testing"
)

func TestMessageAttentionCountsOnlyApprovedQueueAndKeepsPendingReviewSeparate(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	id := messagePropose(t, s, a, messageInput(notice))
	assertPreparedOnly := func(id string) {
		t.Helper()
		x := messageDetail(t, s, a, id)
		if len(x.Outcomes) != 0 || x.DeliveryTotal != 0 || len(x.Deliveries) != 0 || x.Counts.Destinations != 2 {
			t.Fatal("unapproved proposal claimed delivery outcomes instead of prepared destinations", x.Outcomes, x.DeliveryTotal, x.Counts, x.Deliveries)
		}
		for _, r := range x.Recipients {
			if r.State == "QUEUED" {
				t.Fatal("unapproved recipient was described as queued", r)
			}
		}
		page, e := s.MessagesFor(ctx, a, "", x.State, "", 1)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, batch := range page.Items {
			if batch.ID == id {
				found = true
				if len(batch.Outcomes) != 0 || batch.Counts.Destinations != 2 {
					t.Fatal("proposal card claimed dispatched outcomes", batch.Outcomes, batch.Counts)
				}
			}
		}
		if !found {
			t.Fatal("prepared proposal missing from permitted register")
		}
	}
	assertPreparedOnly(id)
	summary, e := s.MessageSummaryFor(ctx, a)
	if e != nil || summary.Batches["PENDING"] != 1 || summary.Outcomes["QUEUED"] != 0 {
		t.Fatal("unapproved snapshots were described as approved queued delivery", summary, e)
	}
	attention, e := s.OverviewFor(ctx, a, "messages")
	if e != nil || attention.Counts["awaiting_review"] != 1 || attention.Counts["QUEUED"] != 0 || len(attention.Items) != 1 || attention.Items[0].Title != "Community message" {
		t.Fatal("overview conflated separate approval and queue", attention, e)
	}
	x := messageDetail(t, s, b, id)
	if _, e = s.ActOnMessage(ctx, b, id, messageDecision(x, "APPROVED")); e != nil {
		t.Fatal(e)
	}
	summary, e = s.MessageSummaryFor(ctx, a)
	if e != nil || summary.Batches["PENDING"] != 0 || summary.Outcomes["QUEUED"] != 2 {
		t.Fatal("approved unique destinations", summary, e)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["QUEUED"] != 2 || x.DeliveryTotal != 2 || len(x.Deliveries) != 2 {
		t.Fatal("approved batch lost its actual queued destinations", x)
	}
	if _, e = s.ActOnMessage(ctx, a, id, messageDecision(x, "CANCELLED")); e != nil {
		t.Fatal(e)
	}
	summary, e = s.MessageSummaryFor(ctx, a)
	if e != nil || summary.Outcomes["QUEUED"] != 0 || summary.Outcomes["CANCELLED"] != 2 {
		t.Fatal("cancelled queue still needs handoff", summary, e)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["CANCELLED"] != 2 || x.DeliveryTotal != 2 || len(x.Deliveries) != 2 {
		t.Fatal("previously approved cancellation lost its destination history", x)
	}
	for _, state := range []string{"DECLINED", "WITHDRAWN"} {
		id := messagePropose(t, s, a, messageInput(notice))
		token := b
		if state == "WITHDRAWN" {
			token = a
		}
		if _, e = s.ActOnMessage(ctx, token, id, messageDecision(messageDetail(t, s, token, id), state)); e != nil {
			t.Fatal(e)
		}
		assertPreparedOnly(id)
	}
}
