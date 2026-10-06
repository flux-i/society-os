package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func messageFixture(t *testing.T) (*Store, string, string, string) {
	t.Helper()
	s, a := recordFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	accessExec(t, s, `INSERT INTO users(id,login,display_name,password_hash,status,resident_id,is_demo,verified_at,auth_version,created_at) SELECT 'message-joint-account','joint@example.test','Demo Joint Owner A-101',password_hash,'ACTIVE','demo-joint-owner',0,verified_at,1,created_at FROM users WHERE id='demo-user-owner'`)
	for _, tc := range []struct{ id, phone, email string }{
		{"demo-owner-A-101", "+919000000101", "shared@example.test"},
		{"demo-joint-owner", "+919000000101", "shared@example.test"},
		{"demo-tenant-A-103", "+919000000103", "tenant@example.test"},
	} {
		in := contactInput()
		in.Phone, in.Email = tc.phone, tc.email
		in.ContactPreferences = ContactPreferences{true, true, true, true}
		if _, e := s.RegisterContact(context.Background(), a, tc.id, in); e != nil {
			t.Fatal(e)
		}
		x := contactDetails(t, s, b, tc.id)
		if _, e := s.ActOnContact(context.Background(), b, tc.id, contactAction(x, "VERIFIED")); e != nil {
			t.Fatal(e)
		}
	}
	in := proposal("NOTICE")
	in.Title = "Fictional public water service update"
	id, e := s.SubmitReview(context.Background(), a, "", in)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideReview(context.Background(), b, id, decision(1, "APPROVED")); e != nil {
		t.Fatal(e)
	}
	return s, a, b, id
}

func TestMessageSharedDestinationOptOutBetweenClaimAndHandoffPreservesEachPerson(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	in := messageInput(notice)
	in.Target = MessageTarget{Kind: "OWNERS"}
	id := messageApproved(t, s, a, b, in)
	action := messageDecision(messageDetail(t, s, a, id), "DISPATCH")
	action.Outcome = "ACCEPTED"
	claims, _, e := s.ClaimMessageDispatch(ctx, a, id, action)
	if e != nil || len(claims) != 1 {
		t.Fatal(claims, e)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	stop := contactAction(contactDetails(t, s, owner, "me"), "OPTED_OUT")
	stop.Channel, stop.Purpose = "EMAIL", "COMMUNITY"
	if _, e = s.ActOnContact(ctx, owner, "me", stop); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "ACCEPTED"); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteSyntheticMessage(ctx, claims[0].ID); e != nil {
		t.Fatal(e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE state='HANDED_OFF' AND resident_id='demo-joint-owner'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE state='OPTED_OUT' AND resident_id='demo-owner-A-101'", 1)
	x := messageDetail(t, s, owner, id)
	if x.Outcomes["OPTED_OUT"] != 1 || x.Outcomes["ACCEPTED"] != 0 || x.RecipientTotal != 1 {
		t.Fatal("a shared envelope fabricated an opted-out person's delivery", x)
	}
	personal, e := s.MessageSummaryFor(ctx, owner)
	if e != nil || personal.Outcomes["OPTED_OUT"] != 1 || personal.Outcomes["ACCEPTED"] != 0 {
		t.Fatal("own summary credited a shared handoff", personal, e)
	}
	staff, e := s.MessageSummaryFor(ctx, a)
	if e != nil || staff.Outcomes["ACCEPTED"] != 1 {
		t.Fatal("staff envelope summary", staff, e)
	}
	attention, e := s.OverviewFor(ctx, owner, "messages")
	if e != nil || attention.Counts["OPTED_OUT"] != 1 || attention.Counts["ACCEPTED"] != 0 || len(attention.Items) != 0 {
		t.Fatal("own overview broadened scope", attention, e)
	}
	for _, q := range []string{"UPDATE message_attempt_recipients SET state='HANDED_OFF' WHERE resident_id='demo-owner-A-101'", "DELETE FROM message_attempt_recipients"} {
		if _, e = s.DB.Exec(q); e == nil {
			t.Fatal("attempt recipient proof changed", q)
		}
	}
}

func TestMessageQueuedRechecksContactAccountMembershipSourceAndApprovedAuthority(t *testing.T) {
	for _, problem := range []string{"CONTACT_CHANGED", "NO_ACTIVE_ACCOUNT", "NO_CURRENT_HOME", "NO_SOURCE_ACCESS", "SOURCE_UNAVAILABLE", "APPROVAL_AUTHORITY_ENDED"} {
		t.Run(problem, func(t *testing.T) {
			s, a, b, notice := messageFixture(t)
			ctx := context.Background()
			in := messageInput(notice)
			in.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}
			if problem == "NO_SOURCE_ACCESS" {
				entry := post(t, s, a, received("123.45"))
				x, e := s.EntryFor(ctx, a, entry)
				if e != nil {
					t.Fatal(e)
				}
				in.SourceKind, in.SourceID = "RECEIPT", x.ReceiptID
				b = maintenanceReviewer(t, s, a)
			}
			id := messageApproved(t, s, a, b, in)
			switch problem {
			case "CONTACT_CHANGED":
				contact := contactDetails(t, s, a, "demo-owner-A-101")
				change := contactInput()
				change.Version = contact.Version
				change.Phone, change.Email = "+919000000202", "changed@example.test"
				change.ContactPreferences = ContactPreferences{true, true, true, true}
				if _, e := s.RegisterContact(ctx, a, contact.ID, change); e != nil {
					t.Fatal(e)
				}
				if _, e := s.ActOnContact(ctx, b, contact.ID, contactAction(contactDetails(t, s, b, contact.ID), "VERIFIED")); e != nil {
					t.Fatal(e)
				}
			case "NO_ACTIVE_ACCOUNT":
				accessExec(t, s, "UPDATE users SET suspended_at=1 WHERE id='demo-user-owner'")
			case "NO_CURRENT_HOME":
				accessExec(t, s, "UPDATE flat_memberships SET end_date=?,is_primary_contact=0 WHERE resident_id='demo-owner-A-101'", today())
			case "NO_SOURCE_ACCESS":
				accessExec(t, s, "UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101'")
			case "SOURCE_UNAVAILABLE":
				if _, e := s.DecideReview(ctx, b, notice, decision(2, "ARCHIVED")); e != nil {
					t.Fatal(e)
				}
			case "APPROVAL_AUTHORITY_ENDED":
				accessExec(t, s, "UPDATE role_grants SET revoked_at=1 WHERE user_id='demo-user-committee'")
			}
			action := messageDecision(messageDetail(t, s, a, id), "DISPATCH")
			action.Outcome = "ACCEPTED"
			claims, _, e := s.ClaimMessageDispatch(ctx, a, id, action)
			if e != nil || len(claims) != 0 {
				t.Fatal("ineligible recipient acquired a handoff", claims, e)
			}
			x := messageDetail(t, s, a, id)
			if x.Outcomes["SKIPPED"] != 1 || x.Recipients[0].Reason != problem || x.Deliveries[0].Destination != "shared@example.test" {
				t.Fatal("frozen destination/suppression", x)
			}
			maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 0)
			maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempts", 0)
		})
	}
}

func TestMessageConcurrentClaimsRestartAndCurrentAuthorityBeforeReplay(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	in := messageInput(notice)
	in.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}
	id := messageApproved(t, s, a, b, in)
	x := messageDetail(t, s, a, id)
	var wg sync.WaitGroup
	results := make(chan []MessageClaim, 2)
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		action := messageDecision(x, "DISPATCH")
		action.Outcome = "ACCEPTED"
		wg.Add(1)
		go func() {
			defer wg.Done()
			claims, _, e := s.ClaimMessageDispatch(ctx, a, id, action)
			if e != nil {
				failures <- e
			} else {
				results <- claims
			}
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	var claim MessageClaim
	success, conflicts := 0, 0
	for claims := range results {
		if len(claims) != 1 {
			t.Fatal(claims)
		}
		claim = claims[0]
		success++
	}
	for e := range failures {
		if !errors.Is(e, ErrConflict) {
			t.Fatal(e)
		}
		conflicts++
	}
	if success != 1 || conflicts != 1 {
		t.Fatal(success, conflicts)
	}
	// Durable provider acceptance precedes portal completion; startup cannot resend.
	h, e := s.SyntheticMessageHandoff(ctx, a, claim.ID, "ACCEPTED")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.RecoverMessageClaims(ctx); e != nil {
		t.Fatal(e)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["UNKNOWN"] != 1 || x.CanDispatch {
		t.Fatal(x)
	}
	reconcile := messageDecision(x, "RECONCILE")
	if _, e = s.ReconcileMessage(ctx, a, id, claim.DeliveryID, reconcile); e != nil {
		t.Fatal(e)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["ACCEPTED"] != 1 || x.Deliveries[0].ProviderID != h.ProviderID || x.Deliveries[0].Attempts != 1 {
		t.Fatal(x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 1)
	// A process may stop before the provider ever sees the claim. Startup still
	// marks that uncertainty, and only reconciliation can prove a retry is safe.
	noHandoff := messageApproved(t, s, a, b, in)
	noProofAction := messageDecision(messageDetail(t, s, a, noHandoff), "DISPATCH")
	noProofAction.Outcome = "ACCEPTED"
	claims, _, e := s.ClaimMessageDispatch(ctx, a, noHandoff, noProofAction)
	if e != nil || len(claims) != 1 {
		t.Fatal(claims, e)
	}
	if e = s.RecoverMessageClaims(ctx); e != nil {
		t.Fatal(e)
	}
	unresolved := messageDetail(t, s, a, noHandoff)
	if unresolved.Outcomes["UNKNOWN"] != 1 || unresolved.CanDispatch {
		t.Fatal(unresolved)
	}
	noProofDecision := messageDecision(unresolved, "RECONCILE")
	if _, e = s.ReconcileMessage(ctx, a, noHandoff, claims[0].DeliveryID, noProofDecision); e != nil {
		t.Fatal(e)
	}
	proven := messageDetail(t, s, a, noHandoff)
	if proven.Outcomes["FAILED"] != 1 || !proven.CanDispatch || proven.RetryableDeliveries != 1 || proven.Deliveries[0].Attempts != 1 || proven.Deliveries[0].ProviderID != "" || proven.Deliveries[0].AcceptedAt != 0 {
		t.Fatal(proven)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE attempt_id='"+claims[0].ID+"' AND state='SKIPPED'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 1)
	retryAction := messageDecision(proven, "DISPATCH")
	retryAction.Outcome = "ACCEPTED"
	retryClaims, _, e := s.ClaimMessageDispatch(ctx, a, noHandoff, retryAction)
	if e != nil || len(retryClaims) != 1 || retryClaims[0].ID == claims[0].ID {
		t.Fatal(retryClaims, e)
	}
	if _, e = s.SyntheticMessageHandoff(ctx, a, retryClaims[0].ID, "ACCEPTED"); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteSyntheticMessage(ctx, retryClaims[0].ID); e != nil {
		t.Fatal(e)
	}
	proven = messageDetail(t, s, a, noHandoff)
	if proven.Outcomes["ACCEPTED"] != 1 || proven.Deliveries[0].Attempts != 2 || proven.Deliveries[0].DeliveredAt != 0 || proven.Deliveries[0].ReadAt != 0 {
		t.Fatal(proven)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 2)
	accessExec(t, s, "UPDATE role_grants SET revoked_at=1 WHERE user_id='demo-user-admin'")
	if _, e = s.ReconcileMessage(ctx, a, id, claim.DeliveryID, reconcile); !errors.Is(e, ErrForbidden) {
		t.Fatal("accepted operation bypassed current revoked authority", e)
	}
}

func TestMessageCallbacksRetainActualReadProofBounceRetryAndOldAttemptEvents(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	in := messageInput(notice)
	in.Target = MessageTarget{Kind: "OWNERS"}
	id := messageApproved(t, s, a, b, in)
	handoff := func() (MessageClaim, SimulationHandoff) {
		t.Helper()
		action := messageDecision(messageDetail(t, s, a, id), "DISPATCH")
		action.Outcome = "ACCEPTED"
		claims, _, e := s.ClaimMessageDispatch(ctx, a, id, action)
		if e != nil || len(claims) != 1 {
			t.Fatal(claims, e)
		}
		h, e := s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "ACCEPTED")
		if e != nil {
			t.Fatal(e)
		}
		if e = s.CompleteSyntheticMessage(ctx, claims[0].ID); e != nil {
			t.Fatal(e)
		}
		return claims[0], h
	}
	callback := func(h SimulationHandoff, state string) MessageCallback {
		t.Helper()
		proof := MessageCallback{randomToken(), h.ProviderID, state, time.Now().Unix()}
		data, _ := json.Marshal(proof)
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		if e := s.ReceiveMessageCallback(ctx, proof, hash); e != nil {
			t.Fatal(e)
		}
		if e := s.ReceiveMessageCallback(ctx, proof, hash); e != nil {
			t.Fatal("duplicate callback", e)
		}
		if e := s.ReceiveMessageCallback(ctx, proof, strings.Repeat("a", 64)); !errors.Is(e, ErrConflict) {
			t.Fatal("changed replay callback", e)
		}
		return proof
	}
	first, h1 := handoff()
	callback(h1, "FAILED")
	owner := reviewLogin(t, s, "owner@demo.society")
	stop := contactAction(contactDetails(t, s, owner, "me"), "OPTED_OUT")
	stop.Channel, stop.Purpose = "EMAIL", "COMMUNITY"
	if _, e := s.ActOnContact(ctx, owner, "me", stop); e != nil {
		t.Fatal(e)
	}
	second, h2 := handoff()
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE attempt_id='"+first.ID+"' AND state='HANDED_OFF'", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE attempt_id='"+second.ID+"' AND state='HANDED_OFF'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE attempt_id='"+second.ID+"' AND resident_id='demo-owner-A-101'", 0)
	callback(h1, "READ") // Preserve old proof without promoting the current attempt.
	x := messageDetail(t, s, a, id)
	if x.Outcomes["ACCEPTED"] != 1 || x.Deliveries[0].ReadAt != 0 {
		t.Fatal("earlier attempt promoted current delivery", x)
	}
	read := callback(h2, "READ")
	x = messageDetail(t, s, a, id)
	if x.Outcomes["READ"] != 1 || x.Deliveries[0].ReadAt != read.At || x.Deliveries[0].DeliveredAt != 0 {
		t.Fatal("read manufactured an unreported delivery timestamp", x)
	}
	callback(h2, "ACCEPTED")
	callback(h2, "FAILED")
	x = messageDetail(t, s, a, id)
	if x.Outcomes["READ"] != 1 || x.Deliveries[0].DeliveredAt != 0 {
		t.Fatal("out of order regressed known proof", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_callbacks", 5)
	if x := messageDetail(t, s, owner, id); x.Outcomes["OPTED_OUT"] != 1 || x.Outcomes["READ"] != 0 {
		t.Fatal("read attributed to opted-out person", x)
	}
}
func messageInput(id string) MessageInput {
	return MessageInput{SourceKind: "NOTICE", SourceID: id, Channel: "EMAIL", Target: MessageTarget{Kind: "ALL", IDs: []string{}}, PortalOrigin: "http://127.0.0.1:8080"}
}
func messageDetail(t *testing.T, s *Store, token, id string) MessageDetail {
	t.Helper()
	x, e := s.MessageFor(context.Background(), token, id, 1, 1, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func messageDecision(x MessageDetail, action string) MessageAction {
	return MessageAction{OperationKey: randomToken(), Version: x.Version, Action: action, Reason: "Reviewed the fictional frozen recipients and delivery effect.", Confirmed: true}
}
func messagePropose(t *testing.T, s *Store, token string, in MessageInput) string {
	t.Helper()
	preview, e := s.MessagePreviewFor(context.Background(), token, in, 1)
	if e != nil {
		t.Fatal(e)
	}
	in.OperationKey, in.PreviewHash, in.Reason, in.Confirmed = randomToken(), preview.PreviewHash, "Reviewed the exact fictional content and recipients.", true
	id, e := s.ProposeMessage(context.Background(), token, in)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.ProposeMessage(context.Background(), token, in)
	if e != nil || again != id {
		t.Fatal("proposal retry", again, e)
	}
	return id
}
func messageApproved(t *testing.T, s *Store, a, b string, in MessageInput) string {
	t.Helper()
	id := messagePropose(t, s, a, in)
	x := messageDetail(t, s, b, id)
	if _, e := s.ActOnMessage(context.Background(), b, id, messageDecision(x, "APPROVED")); e != nil {
		t.Fatal(e)
	}
	return id
}
func TestMessageExactTargetsSharedDestinationsAndIndependentSourceIntersection(t *testing.T) {
	s, a, _, notice := messageFixture(t)
	ctx := context.Background()
	in := messageInput(notice)
	for _, tc := range []struct {
		target                                            MessageTarget
		people, source, consented, eligible, destinations int
	}{
		{MessageTarget{Kind: "ALL"}, 153, 153, 3, 3, 2},
		{MessageTarget{Kind: "OWNERS"}, 118, 118, 2, 2, 1},
		{MessageTarget{Kind: "TENANTS"}, 35, 35, 1, 1, 1},
		{MessageTarget{Kind: "WING", Wing: "A"}, 52, 52, 3, 3, 2},
		{MessageTarget{Kind: "HOMES", IDs: []string{"demo-flat-A-101", "demo-flat-A-102"}}, 2, 2, 2, 2, 1},
		{MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101", "demo-tenant-A-103"}}, 2, 2, 2, 2, 2},
	} {
		in.Target = tc.target
		x, e := s.MessagePreviewFor(ctx, a, in, 1)
		if e != nil || x.Counts.TargetPeople != tc.people || x.Counts.SourcePeople != tc.source || x.Counts.ConsentedPeople != tc.consented || x.Counts.EligiblePeople != tc.eligible || x.Counts.Destinations != tc.destinations || x.Counts.OmittedPeople != tc.people-tc.eligible || len(x.Recipients) > 20 {
			t.Fatal("independent target counts", tc, x.Counts, e)
		}
	}
	b := reviewLogin(t, s, "committee@demo.society")
	tenantOnly := proposal("NOTICE")
	tenantOnly.Audience = "TENANTS_ONLY"
	tenantOnly.Title = "Fictional tenant-only water update"
	id, e := s.SubmitReview(ctx, a, "", tenantOnly)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DecideReview(ctx, b, id, decision(1, "APPROVED")); e != nil {
		t.Fatal(e)
	}
	in = messageInput(id)
	x, e := s.MessagePreviewFor(ctx, a, in, 1)
	if e != nil || x.Counts.TargetPeople != 153 || x.Counts.SourcePeople != 35 || x.Counts.EligiblePeople != 1 || x.Counts.Destinations != 1 || x.Counts.Reasons["NO_SOURCE_ACCESS"] != 118 {
		t.Fatal("tenant source broadened", x.Counts, e)
	}
	in.Target.Kind = "OWNERS"
	x, e = s.MessagePreviewFor(ctx, a, in, 1)
	if e != nil || x.Counts.SourcePeople != 0 || x.Counts.Destinations != 0 || x.Counts.OmittedPeople != 118 {
		t.Fatal(x, e)
	}
	in.OperationKey, in.PreviewHash, in.Reason, in.Confirmed = randomToken(), x.PreviewHash, "Reviewed the empty intersection; it cannot send.", true
	if _, e = s.ProposeMessage(ctx, a, in); !errors.Is(e, ErrInvalid) {
		t.Fatal("zero recipients accepted", e)
	}
	// A verified shared destination does not substitute for an active portal identity.
	accessExec(t, s, "UPDATE users SET suspended_at=1 WHERE id='message-joint-account'")
	in = messageInput(notice)
	x, e = s.MessagePreviewFor(ctx, a, in, 1)
	if e != nil || x.Counts.ConsentedPeople != 3 || x.Counts.EligiblePeople != 2 || x.Counts.Reasons["NO_ACTIVE_ACCOUNT"] != 1 || x.Counts.Destinations != 2 {
		t.Fatal("missing identity omitted incorrectly", x.Counts, e)
	}
	for _, table := range []string{"entries", "receipts", "message_batches", "simulation_messages"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
}
func TestMessageSeparateReviewRefreshPrivateReceiptAndNoMoneyChanges(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	id := messagePropose(t, s, a, messageInput(notice))
	x := messageDetail(t, s, a, id)
	if x.State != "PENDING" || x.Version != 1 || x.CanApprove || !x.CanRefresh || x.Counts.Destinations != 2 {
		t.Fatal(x)
	}
	if _, e := s.ActOnMessage(ctx, a, id, messageDecision(x, "APPROVED")); !errors.Is(e, ErrForbidden) {
		t.Fatal("self approval", e)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	c := contactDetails(t, s, owner, "me")
	stop := contactAction(c, "OPTED_OUT")
	stop.Channel, stop.Purpose = "EMAIL", "COMMUNITY"
	if _, e := s.ActOnContact(ctx, owner, "me", stop); e != nil {
		t.Fatal(e)
	}
	reviewer := messageDetail(t, s, b, id)
	if reviewer.CanApprove || reviewer.ReviewProblem != "PREVIEW_CHANGED" {
		t.Fatal("stale recipient approval enabled", reviewer)
	}
	if _, e := s.ActOnMessage(ctx, b, id, messageDecision(reviewer, "APPROVED")); !errors.Is(e, ErrConflict) {
		t.Fatal("stale review accepted", e)
	}
	if _, e := s.ActOnMessage(ctx, a, id, messageDecision(x, "REFRESH")); e != nil {
		t.Fatal(e)
	}
	x = messageDetail(t, s, b, id)
	if x.Version != 2 || x.SnapshotVersion != 2 || !x.CanApprove || x.Counts.EligiblePeople != 2 || x.Counts.Destinations != 2 || x.Counts.Reasons["OPTED_OUT"] != 1 || x.EventTotal != 2 {
		t.Fatal("refresh did not retain correct snapshot", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_recipients WHERE batch_id='"+id+"'", 306)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_deliveries WHERE batch_id='"+id+"' AND snapshot_version=1 AND state='CANCELLED'", 2)
	approve := messageDecision(x, "APPROVED")
	if _, e := s.ActOnMessage(ctx, b, id, approve); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ActOnMessage(ctx, b, id, approve); e != nil {
		t.Fatal("decision replay", e)
	}
	tenant := reviewLogin(t, s, "tenant@demo.society")
	personal := messageDetail(t, s, tenant, id)
	if personal.Staff || personal.Counts.TargetPeople != 1 || personal.RecipientTotal != 1 || personal.EventTotal != 0 || personal.DeliveryTotal != 0 || personal.ProposedBy != "" || strings.Contains(personal.Envelope, "shared@example") {
		t.Fatal("personal projection leaked", personal)
	}
	entry := post(t, s, a, received("432.19"))
	money, e := s.EntryFor(ctx, a, entry)
	if e != nil {
		t.Fatal(e)
	}
	finance := messageInput(money.ReceiptID)
	finance.SourceKind = "RECEIPT"
	preview, e := s.MessagePreviewFor(ctx, a, finance, 1)
	if e != nil || preview.Counts.TargetPeople != 153 || preview.Counts.SourcePeople != 2 || preview.Counts.EligiblePeople != 2 || preview.Counts.Destinations != 1 || strings.Contains(preview.Envelope, "432") || strings.Contains(preview.Envelope, "A-101") || strings.Contains(preview.Envelope, "Demo Owner") {
		t.Fatal("private receipt eligibility/envelope", preview, e)
	}
	if _, e = s.MessagePreviewFor(ctx, b, finance, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("committee acquired treasury sharing", e)
	}
	if _, e = s.MessagePreviewFor(ctx, owner, finance, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("owner acquired mass sharing", e)
	}
	receiptProposal := messagePropose(t, s, a, finance)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	own, e := s.MessagesFor(ctx, owner, "", "", "", 1)
	if e != nil || own.Total != 0 {
		t.Fatal("private pending/omitted proposal visible", own, e)
	}
	for _, query := range []string{"DELETE FROM message_batches WHERE id=?", "UPDATE message_events SET reason='changed' WHERE batch_id=?", "DELETE FROM message_recipients WHERE batch_id=?"} {
		if _, e = s.DB.Exec(query, receiptProposal); e == nil {
			t.Fatal("immutable messaging history changed", query)
		}
	}
}
func TestMessageUnknownReconciliationAndDefiniteRetryCapsPreserveHandoffIdentity(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	in := messageInput(notice)
	in.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}
	id := messageApproved(t, s, a, b, in)
	x := messageDetail(t, s, a, id)
	dispatch := messageDecision(x, "DISPATCH")
	dispatch.Outcome = "UNKNOWN"
	claims, result, e := s.ClaimMessageDispatch(ctx, a, id, dispatch)
	if e != nil || result != id || len(claims) != 1 {
		t.Fatal(claims, result, e)
	}
	if duplicate, result, e := s.ClaimMessageDispatch(ctx, a, id, dispatch); e != nil || result != id || len(duplicate) != 0 {
		t.Fatal("replayed claim", duplicate, result, e)
	}
	h, e := s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "UNKNOWN")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteSyntheticMessage(ctx, claims[0].ID); e != nil {
		t.Fatal(e)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["UNKNOWN"] != 1 || x.Deliveries[0].ProviderID != "" || x.Deliveries[0].AcceptedAt != 0 || x.CanDispatch {
		t.Fatal("unknown pretended accepted or allowed resend", x)
	}
	again := messageDecision(x, "DISPATCH")
	again.Outcome = "ACCEPTED"
	if _, _, e = s.ClaimMessageDispatch(ctx, a, id, again); !errors.Is(e, ErrConflict) {
		t.Fatal("blind resend of unknown", e)
	}
	reconcile := messageDecision(x, "RECONCILE")
	if _, e = s.ReconcileMessage(ctx, a, id, claims[0].DeliveryID, reconcile); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReconcileMessage(ctx, a, id, claims[0].DeliveryID, reconcile); e != nil {
		t.Fatal("reconcile replay", e)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["ACCEPTED"] != 1 || x.Deliveries[0].ProviderID != h.ProviderID || x.Deliveries[0].AcceptedAt == 0 || x.Deliveries[0].DeliveredAt != 0 || x.Deliveries[0].ReadAt != 0 {
		t.Fatal("reconciliation invented delivery/read", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempts", 1)
	failed := messageApproved(t, s, a, b, in)
	for n := 1; n <= 3; n++ {
		x = messageDetail(t, s, a, failed)
		action := messageDecision(x, "DISPATCH")
		action.Outcome = "REJECTED"
		claims, _, e = s.ClaimMessageDispatch(ctx, a, failed, action)
		if e != nil || len(claims) != 1 {
			t.Fatal(n, claims, e)
		}
		if _, e = s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "REJECTED"); e != nil {
			t.Fatal(e)
		}
		if e = s.CompleteSyntheticMessage(ctx, claims[0].ID); e != nil {
			t.Fatal(e)
		}
		x = messageDetail(t, s, a, failed)
		if x.Outcomes["FAILED"] != 1 || x.Deliveries[0].Attempts != n || x.Deliveries[0].AcceptedAt != 0 || x.CanDispatch != (n < 3) {
			t.Fatal("definite retry bound", n, x)
		}
	}
	fourth := messageDecision(x, "DISPATCH")
	fourth.Outcome = "ACCEPTED"
	if _, _, e = s.ClaimMessageDispatch(ctx, a, failed, fourth); !errors.Is(e, ErrConflict) {
		t.Fatal("fourth attempt accepted", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 4)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}
