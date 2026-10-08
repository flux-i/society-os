package database

import (
	"context"
	"fmt"
	"testing"
)

func TestReminderExceptionsIndependentFullCountsBoundedPagesExactDestinationLinksAndScope(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	rows, e := s.DB.Query("SELECT id FROM residents WHERE id LIKE 'demo-owner-B-%' ORDER BY id LIMIT 21")
	if e != nil {
		t.Fatal(e)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil || len(ids) != 21 {
		t.Fatal(ids, e)
	}
	for i, id := range ids {
		accessExec(t, s, `INSERT INTO users(id,login,display_name,password_hash,status,resident_id,is_demo,verified_at,auth_version,created_at) SELECT ?,?, ?,password_hash,'ACTIVE',?,0,verified_at,1,created_at FROM users WHERE id='demo-user-owner'`, fmt.Sprintf("reminder-account-%02d", i), fmt.Sprintf("reminder-%02d@example.test", i), "Fictional exception recipient", id)
		in := contactInput()
		in.Email = fmt.Sprintf("reminder-%02d@example.test", i)
		in.Phone = fmt.Sprintf("+919001%06d", i)
		in.ContactPreferences = ContactPreferences{true, true, true, true}
		if _, e = s.RegisterContact(ctx, a, id, in); e != nil {
			t.Fatal(e)
		}
		if _, e = s.ActOnContact(ctx, b, id, contactAction(contactDetails(t, s, b, id), "VERIFIED")); e != nil {
			t.Fatal(e)
		}
	}
	id := messageApproved(t, s, a, b, messageInput(notice))
	action := messageDecision(messageDetail(t, s, a, id), "DISPATCH")
	action.Outcome = "UNKNOWN"
	claims, _, e := s.ClaimMessageDispatch(ctx, a, id, action)
	if e != nil || len(claims) != 23 {
		t.Fatal("independent unique-destination count", len(claims), e)
	}
	for i, claim := range claims {
		if i == 22 {
			continue
		}
		outcome := "UNKNOWN"
		if i >= 15 {
			outcome = "REJECTED"
		}
		if _, e = s.SyntheticMessageHandoff(ctx, a, claim.ID, outcome); e != nil {
			t.Fatal(e)
		}
		if e = s.CompleteSyntheticMessage(ctx, claim.ID); e != nil {
			t.Fatal(e)
		}
	}
	first, e := s.MessageExceptionsFor(ctx, a, "", 1)
	if e != nil || first.Total != 23 || len(first.Items) != 12 || first.Counts["UNKNOWN"] != 15 || first.Counts["FAILED"] != 7 || first.Counts["CLAIMED"] != 1 {
		t.Fatal("full independent counts", first, e)
	}
	second, e := s.MessageExceptionsFor(ctx, a, "", 2)
	if e != nil || second.Total != 23 || len(second.Items) != 11 {
		t.Fatal("second bounded page", second, e)
	}
	unknown, e := s.MessageExceptionsFor(ctx, a, "UNKNOWN", 2)
	if e != nil || unknown.Total != 15 || len(unknown.Items) != 3 || unknown.Counts["FAILED"] != 7 {
		t.Fatal("filter lost full counts", unknown, e)
	}
	seen := map[string]bool{}
	laterPage := false
	for _, x := range append(first.Items, second.Items...) {
		if seen[x.DeliveryID] {
			t.Fatal("duplicate queue identity")
		}
		seen[x.DeliveryID] = true
		detail, e := s.MessageFor(ctx, a, x.MessageID, 1, x.DeliveryPage, 1)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, delivery := range detail.Deliveries {
			if delivery.ID == x.DeliveryID {
				found = true
			}
		}
		if !found {
			t.Fatal("exception link did not open exact destination page", x)
		}
		if x.DeliveryPage == 2 {
			laterPage = true
		}
		if x.CanReconcile != (x.State == "UNKNOWN") || x.CanRetry != (x.State == "FAILED") {
			t.Fatal("wrong permitted action", x)
		}
	}
	if !laterPage {
		t.Fatal("later destination page never exercised")
	}
	// Reconciliation is a deliberate existing-attempt action, not a claim retry.
	for _, x := range first.Items {
		if x.State == "UNKNOWN" {
			if _, e = s.ReconcileMessage(ctx, a, id, x.DeliveryID, messageDecision(messageDetail(t, s, a, id), "RECONCILE")); e != nil {
				t.Fatal(e)
			}
			break
		}
	}
	after, e := s.MessageExceptionsFor(ctx, a, "", 1)
	if e != nil || after.Total != 22 || after.Counts["UNKNOWN"] != 14 {
		t.Fatal("reconciled queue did not update", after, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 22)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempts", 23)
}
