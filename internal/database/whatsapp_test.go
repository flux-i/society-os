package database

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func whatsappProvider() *MessageProvider {
	return &MessageProvider{Mode: "CLOUD_FIXTURE", Origin: "http://127.0.0.1:12345", APIVersion: "v26.0", AccountID: "111111", PhoneID: "222222", TemplateID: "444444", Name: "society_notice", Language: "en", Category: "UTILITY", Body: "An update is available.\n{{1}}"}
}
func whatsappApproved(t *testing.T, s *Store, a, b, notice string) string {
	t.Helper()
	in := messageInput(notice)
	in.Channel = "WHATSAPP"
	in.Provider = whatsappProvider()
	in.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}
	id := messagePropose(t, s, a, in)
	action := messageDecision(messageDetail(t, s, b, id), "APPROVED")
	action.Provider = in.Provider
	if _, err := s.ActOnMessage(context.Background(), b, id, action); err != nil {
		t.Fatal(err)
	}
	return id
}
func whatsappClaim(t *testing.T, s *Store, a, id string) MessageClaim {
	t.Helper()
	action := messageDecision(messageDetail(t, s, a, id), "DISPATCH")
	action.Provider = whatsappProvider()
	claims, _, err := s.ClaimMessageDispatch(context.Background(), a, id, action)
	if err != nil || len(claims) != 1 {
		t.Fatal(claims, err)
	}
	return claims[0]
}

func TestWhatsAppFrozenTemplateSeparateApprovalReplayAndImmutableSnapshots(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	in := messageInput(notice)
	in.Channel = "WHATSAPP"
	in.Provider = whatsappProvider()
	id := messagePropose(t, s, a, in)
	x := messageDetail(t, s, b, id)
	if x.Provider == nil || strings.Contains(x.Envelope, "Fictional public water") || !strings.Contains(x.Envelope, "An update is available.\nhttp://127.0.0.1:8080/") {
		t.Fatal("template envelope", x)
	}
	action := messageDecision(x, "APPROVED")
	action.Provider = whatsappProvider()
	if _, err := s.ActOnMessage(ctx, a, id, action); !errors.Is(err, ErrForbidden) {
		t.Fatal("self approval", err)
	}
	changed := *action.Provider
	changed.Body = "A different update. {{1}}"
	action.Provider = &changed
	if _, err := s.ActOnMessage(ctx, b, id, action); !errors.Is(err, ErrConflict) {
		t.Fatal("changed template approved", err)
	}
	refresh := messageDecision(messageDetail(t, s, a, id), "REFRESH")
	refresh.Provider = &changed
	if _, err := s.ActOnMessage(ctx, a, id, refresh); err != nil {
		t.Fatal(err)
	}
	x = messageDetail(t, s, b, id)
	if x.SnapshotVersion != 2 || x.Provider.Body != changed.Body || len(x.Events) != 2 || x.Events[0].Version != 2 || x.Events[0].Snapshot.Provider.Body != changed.Body || x.Events[1].Version != 1 || x.Events[1].Snapshot.Provider.Body != whatsappProvider().Body {
		t.Fatal("old or current binding lost", x)
	}
	approve := messageDecision(x, "APPROVED")
	approve.Provider = &changed
	if _, err := s.ActOnMessage(ctx, b, id, approve); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ActOnMessage(ctx, b, id, approve); err != nil {
		t.Fatal("accepted action replay", err)
	}
	for _, query := range []string{"UPDATE message_provider_bindings SET provider_json='{}'", "DELETE FROM message_provider_bindings"} {
		if _, err := s.DB.Exec(query); err == nil {
			t.Fatal("immutable binding changed")
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_provider_bindings", 2)
}

func TestWhatsAppUncertainStartReconciliationAndOpaqueSignedProofCannotResend(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	id := whatsappApproved(t, s, a, b, notice)
	claim := whatsappClaim(t, s, a, id)
	if _, err := s.SyntheticMessageHandoff(ctx, a, claim.ID, "READ"); !errors.Is(err, ErrForbidden) {
		t.Fatal("synthetic proof mixed into provider job", err)
	}
	attempt, err := s.PrepareWhatsAppHandoff(ctx, a, claim.ID, whatsappProvider())
	if err != nil || attempt.ID != claim.ID || attempt.Destination != "+919000000101" {
		t.Fatal(attempt, err)
	}
	if err = s.CompleteWhatsAppHandoff(ctx, claim.ID, WhatsAppResult{State: "UNKNOWN", Reason: "WHATSAPP_UNCERTAIN"}); err != nil {
		t.Fatal(err)
	}
	x := messageDetail(t, s, a, id)
	if x.Outcomes["UNKNOWN"] != 1 || x.CanDispatch {
		t.Fatal("uncertainty retryable", x)
	}
	reconcile := messageDecision(x, "RECONCILE")
	if _, err = s.ReconcileMessage(ctx, a, id, claim.DeliveryID, reconcile); err != nil {
		t.Fatal(err)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["UNKNOWN"] != 1 || x.CanDispatch || x.Deliveries[0].Attempts != 1 {
		t.Fatal("reconcile invented nonacceptance", x)
	}
	proof := WhatsAppStatus{EventHash: strings.Repeat("a", 64), AccountID: "111111", PhoneID: "222222", AttemptID: claim.ID, ProviderID: "wamid.original.001", Destination: "+919000000101", State: "READ", At: time.Now().Unix(), Metadata: `{"id":"wamid.original.001","status":"read"}`}
	if err = s.ReceiveWhatsAppStatuses(ctx, []WhatsAppStatus{proof}); err != nil {
		t.Fatal(err)
	}
	if err = s.ReceiveWhatsAppStatuses(ctx, []WhatsAppStatus{proof}); err != nil {
		t.Fatal("duplicate callback", err)
	}
	proof.EventHash, proof.State, proof.Metadata = strings.Repeat("b", 64), "FAILED", `{"id":"wamid.original.001","status":"failed"}`
	if err = s.ReceiveWhatsAppStatuses(ctx, []WhatsAppStatus{proof}); err != nil {
		t.Fatal(err)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["READ"] != 1 || x.Deliveries[0].ReadAt == 0 || x.Deliveries[0].DeliveredAt != 0 || x.Deliveries[0].Attempts != 1 {
		t.Fatal("proof regressed or fabricated explicit delivery", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM whatsapp_handoffs", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM whatsapp_status_events", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
}

func TestWhatsAppAtomicUnknownIdentityCallbacksAndCallbackBeforeHTTPResponse(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	id := whatsappApproved(t, s, a, b, notice)
	claim := whatsappClaim(t, s, a, id)
	if _, err := s.PrepareWhatsAppHandoff(ctx, a, claim.ID, whatsappProvider()); err != nil {
		t.Fatal(err)
	}
	proof := WhatsAppStatus{EventHash: strings.Repeat("c", 64), AccountID: "111111", PhoneID: "222222", AttemptID: claim.ID, ProviderID: "wamid.early.001", Destination: "+919000000101", State: "DELIVERED", At: time.Now().Unix(), Metadata: `{"status":"delivered"}`}
	bad := proof
	bad.AttemptID = "unknown-attempt"
	bad.EventHash = strings.Repeat("d", 64)
	if err := s.ReceiveWhatsAppStatuses(ctx, []WhatsAppStatus{proof, bad}); !errors.Is(err, ErrForbidden) {
		t.Fatal("unknown identity batch", err)
	}
	for _, wrong := range []WhatsAppStatus{func() WhatsAppStatus { v := proof; v.PhoneID = "999999"; return v }(), func() WhatsAppStatus { v := proof; v.AccountID = "999999"; return v }(), func() WhatsAppStatus { v := proof; v.Destination = "+919000000999"; return v }()} {
		if err := s.ReceiveWhatsAppStatuses(ctx, []WhatsAppStatus{wrong}); !errors.Is(err, ErrForbidden) {
			t.Fatal("status bypassed the frozen provider or destination identity", err)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM whatsapp_status_events", 0)
	x := messageDetail(t, s, a, id)
	if x.Outcomes["CLAIMED"] != 1 || x.Deliveries[0].ProviderID != "" {
		t.Fatal("partially applied callback batch", x)
	}
	if err := s.ReceiveWhatsAppStatuses(ctx, []WhatsAppStatus{proof}); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteWhatsAppHandoff(ctx, claim.ID, WhatsAppResult{State: "UNKNOWN", Reason: "WHATSAPP_UNCERTAIN"}); err != nil {
		t.Fatal(err)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["DELIVERED"] != 1 || x.Deliveries[0].ProviderID != "wamid.early.001" {
		t.Fatal("lost HTTP response regressed signed evidence", x)
	}
}

func TestWhatsAppCurrentConsentRaceAndKnownUnsentRestartAllowOnlyOneNewHandoff(t *testing.T) {
	s, a, b, notice := messageFixture(t)
	ctx := context.Background()
	id := whatsappApproved(t, s, a, b, notice)
	claim := whatsappClaim(t, s, a, id)
	if err := s.RecoverMessageClaims(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReconcileMessage(ctx, a, id, claim.DeliveryID, messageDecision(messageDetail(t, s, a, id), "RECONCILE")); err != nil {
		t.Fatal(err)
	}
	x := messageDetail(t, s, a, id)
	if x.Outcomes["FAILED"] != 1 || !x.CanDispatch {
		t.Fatal("unstarted claim not proved unsent", x)
	}
	claim = whatsappClaim(t, s, a, id)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.PrepareWhatsAppHandoff(ctx, a, claim.ID, whatsappProvider())
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	ok, conflicts := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatal("two provider starts", ok, conflicts)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM whatsapp_handoffs", 1)
	if err := s.CompleteWhatsAppHandoff(ctx, claim.ID, WhatsAppResult{State: "FAILED", Reason: "WHATSAPP_RATE_LIMITED", RetryAfter: 60}); err != nil {
		t.Fatal(err)
	}
	blocked := messageDecision(messageDetail(t, s, a, id), "DISPATCH")
	blocked.Provider = whatsappProvider()
	if _, _, err := s.ClaimMessageDispatch(ctx, a, id, blocked); !errors.Is(err, ErrConflict) {
		t.Fatal("retry delay ignored", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempts", 2)
	other := whatsappApproved(t, s, a, b, notice)
	next := whatsappClaim(t, s, a, other)
	owner := reviewLogin(t, s, "owner@demo.society")
	stop := contactAction(contactDetails(t, s, owner, "me"), "OPTED_OUT")
	stop.Channel, stop.Purpose = "WHATSAPP", "COMMUNITY"
	if _, err := s.ActOnContact(ctx, owner, "me", stop); err != nil {
		t.Fatal(err)
	}
	if attempt, err := s.PrepareWhatsAppHandoff(ctx, a, next.ID, whatsappProvider()); err != nil || attempt.ID != "" {
		t.Fatal("withdrawn consent sent", attempt, err)
	}
	if messageDetail(t, s, a, other).Outcomes["OPTED_OUT"] != 1 {
		t.Fatal("current opt-out not recorded")
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM whatsapp_handoffs", 1)
}

func TestWhatsAppAcceptedProposalRetryCanonicalisesSelectedPeopleBeforeProviderReads(t *testing.T) {
	s, a, _, notice := messageFixture(t)
	ctx := context.Background()
	in := messageInput(notice)
	in.Channel = "WHATSAPP"
	in.Provider = whatsappProvider()
	in.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101", "demo-joint-owner"}}
	preview, err := s.MessagePreviewFor(ctx, a, in, 1)
	if err != nil {
		t.Fatal(err)
	}
	in.OperationKey, in.PreviewHash, in.Reason, in.Confirmed = randomToken(), preview.PreviewHash, "Reviewed the exact template and intentionally unsorted fictional selection.", true
	id, err := s.ProposeMessage(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := s.CheckMessageWrite(ctx, a, "", "MESSAGE_PROPOSE", in, in.SourceKind)
	if err != nil || replayed != id {
		t.Fatal("same unsorted request rejected before accepted replay", replayed, err)
	}
	in.Reason = "A different reviewed reason cannot reuse the old operation identity."
	if _, err = s.CheckMessageWrite(ctx, a, "", "MESSAGE_PROPOSE", in, in.SourceKind); !errors.Is(err, ErrConflict) {
		t.Fatal("changed replay accepted", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_batches", 1)
}
