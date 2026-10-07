package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func publishedMessageStatement(t *testing.T, s *Store, a, b, target string) (string, string) {
	t.Helper()
	data := []byte("Description,Amount\nFictional supplied figure,999999.99\n")
	id := approvedStatement(t, s, a, b, statementInput(data), data)
	return id, publishStatement(t, s, a, b, id, target)
}
func statementMessageInput(pub string) MessageInput {
	x := messageInput(pub)
	x.SourceKind = "STATEMENT"
	return x
}
func statementMessageDispatch(x MessageDetail) MessageAction {
	a := messageDecision(x, "DISPATCH")
	a.Outcome = "ACCEPTED"
	return a
}
func revokeMessageStatement(t *testing.T, s *Store, token, file, pub string) {
	t.Helper()
	x := statementDetail(t, s, token, file)
	for _, p := range x.Publications {
		if p.ID == pub {
			_, err := s.DecideStatementPublication(context.Background(), token, pub, StatementAction{OperationKey: randomToken(), Version: p.Version, Action: "REVOKED", Confirmed: true, Reason: "Intentionally removed this fictional published original"})
			if err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("publication missing")
}

func TestStatementMessageFinanceConsentExactTenantAudienceSeparateReviewAndNoMoney(t *testing.T) {
	s, a, committee, _ := messageFixture(t)
	ctx := context.Background()
	if _, err := s.MessageSourcesFor(ctx, committee, "STATEMENT", "", 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("community authority became finance", err)
	}
	b := maintenanceReviewer(t, s, a)
	entry := post(t, s, a, received("432.19"))
	original, err := s.EntryFor(ctx, a, entry)
	if err != nil {
		t.Fatal(err)
	}
	file, pub := publishedMessageStatement(t, s, a, b, "TENANTS")
	in := statementMessageInput(pub)
	preview, err := s.MessagePreviewFor(ctx, a, in, 1)
	if err != nil || preview.Purpose != "FINANCE" || preview.Counts.TargetPeople != 153 || preview.Counts.SourcePeople != 35 || preview.Counts.ConsentedPeople != 1 || preview.Counts.EligiblePeople != 1 || preview.Counts.Destinations != 1 || preview.Counts.OmittedPeople != 152 {
		t.Fatal("independent tenant/consent intersection", preview, err)
	}
	if preview.Source.ID != pub || preview.Source.Link != "/#statements?statement="+file || strings.Contains(preview.Envelope, "999999") || strings.Contains(preview.Envelope, preview.Source.Title) {
		t.Fatal("private figures or wrong frozen link", preview)
	}
	owners := in
	owners.Target = MessageTarget{Kind: "OWNERS"}
	x, err := s.MessagePreviewFor(ctx, a, owners, 1)
	if err != nil || x.Counts.TargetPeople != 118 || x.Counts.SourcePeople != 0 || x.Counts.Destinations != 0 {
		t.Fatal("owner target widened tenant publication", x, err)
	}
	tenant := reviewLogin(t, s, "tenant@demo.society")
	stop := contactAction(contactDetails(t, s, tenant, "me"), "OPTED_OUT")
	stop.Channel, stop.Purpose = "EMAIL", "FINANCE"
	if _, err = s.ActOnContact(ctx, tenant, "me", stop); err != nil {
		t.Fatal(err)
	}
	x, err = s.MessagePreviewFor(ctx, a, in, 1)
	if err != nil || x.Counts.SourcePeople != 35 || x.Counts.ConsentedPeople != 0 || x.Counts.Destinations != 0 || x.Counts.Reasons["OPTED_OUT"] != 1 {
		t.Fatal("community consent substituted for finance", x, err)
	}
	preferences := contactInput()
	preferences.Phone, preferences.Email = "+919000000103", "tenant@example.test"
	preferences.Version = contactDetails(t, s, tenant, "me").Version
	preferences.ContactPreferences = ContactPreferences{true, true, true, true}
	if _, err = s.RegisterContact(ctx, tenant, "me", preferences); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ActOnContact(ctx, b, "demo-tenant-A-103", contactAction(contactDetails(t, s, b, "demo-tenant-A-103"), "VERIFIED")); err != nil {
		t.Fatal(err)
	}
	id := messagePropose(t, s, a, in)
	if _, err = s.ActOnMessage(ctx, a, id, messageDecision(messageDetail(t, s, a, id), "APPROVED")); !errors.Is(err, ErrForbidden) {
		t.Fatal("delivery self review", err)
	}
	if _, err = s.ActOnMessage(ctx, b, id, messageDecision(messageDetail(t, s, b, id), "APPROVED")); err != nil {
		t.Fatal(err)
	}
	resident := messageDetail(t, s, tenant, id)
	if resident.Source.Title != "Published financial statement" || resident.Source.PublicationTarget != nil || resident.Source.Version != "" || resident.Source.Link != "/#statements?statement="+file || resident.Counts.TargetPeople != 1 {
		t.Fatal("resident private publication metadata", resident)
	}
	if _, err = s.HomeStatementFor(ctx, tenant, "demo-flat-A-103", 1, 1, 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("statement sharing grants household finance", err)
	}
	got, err := s.EntryFor(ctx, a, entry)
	if err != nil || !reflect.DeepEqual(got, original) || got.AmountPaise != 43219 {
		t.Fatal("message changed original money", got, err)
	}
}

func TestStatementMessagePublicationRevocationAndSupersessionSuppressNewHandoffs(t *testing.T) {
	for _, afterClaim := range []bool{false, true} {
		t.Run(map[bool]string{false: "before claim", true: "between claim and handoff"}[afterClaim], func(t *testing.T) {
			s, a, _, _ := messageFixture(t)
			ctx := context.Background()
			b := maintenanceReviewer(t, s, a)
			file, pub := publishedMessageStatement(t, s, a, b, "TENANTS")
			id := messageApproved(t, s, a, b, statementMessageInput(pub))
			var claims []MessageClaim
			var err error
			if afterClaim {
				claims, _, err = s.ClaimMessageDispatch(ctx, a, id, statementMessageDispatch(messageDetail(t, s, a, id)))
				if err != nil || len(claims) != 1 {
					t.Fatal(claims, err)
				}
			}
			revokeMessageStatement(t, s, a, file, pub)
			if afterClaim {
				_, err = s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "ACCEPTED")
			} else {
				claims, _, err = s.ClaimMessageDispatch(ctx, a, id, statementMessageDispatch(messageDetail(t, s, a, id)))
			}
			if err != nil {
				t.Fatal(err)
			}
			maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 0)
			x := messageDetail(t, s, a, id)
			if x.Outcomes["SKIPPED"] != 1 || x.Outcomes["ACCEPTED"] != 0 {
				t.Fatal("revoked source handed off", x)
			}
			tenant := reviewLogin(t, s, "tenant@demo.society")
			own := messageDetail(t, s, tenant, id)
			if own.Source.Link != "" || own.Source.ID != "" || own.Source.PublicationTarget != nil {
				t.Fatal("revoked original remains linked", own)
			}
		})
	}
	s, a, _, _ := messageFixture(t)
	ctx := context.Background()
	b := maintenanceReviewer(t, s, a)
	file, pub := publishedMessageStatement(t, s, a, b, "ALL")
	id := messageApproved(t, s, a, b, statementMessageInput(pub))
	data := []byte("Description,Amount\nReplacement supplied original,0.01\n")
	in := statementInput(data)
	in.Replaces = file
	in.Version = statementDetail(t, s, a, file).Version
	replacement := approvedStatement(t, s, a, b, in, data)
	if _, err := s.MessagePreviewFor(ctx, a, statementMessageInput(pub), 1); err != nil {
		t.Fatal("unpublished replacement hid old publication", err)
	}
	publishStatement(t, s, a, b, replacement, "ALL")
	if _, err := s.MessagePreviewFor(ctx, a, statementMessageInput(pub), 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("superseded publication reused", err)
	}
	if _, _, err := s.ClaimMessageDispatch(ctx, a, id, statementMessageDispatch(messageDetail(t, s, a, id))); err != nil {
		t.Fatal(err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 0)
	if x := messageDetail(t, s, a, id); x.Outcomes["SKIPPED"] != 2 || x.Outcomes["ACCEPTED"] != 0 {
		t.Fatal("replacement silently substituted", x)
	}
}

func TestStatementMessageSharedDestinationRetainsSeparateFinanceConsentProof(t *testing.T) {
	s, a, _, _ := messageFixture(t)
	ctx := context.Background()
	b := maintenanceReviewer(t, s, a)
	_, pub := publishedMessageStatement(t, s, a, b, "OWNERS")
	in := statementMessageInput(pub)
	in.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101", "demo-joint-owner"}}
	preview, err := s.MessagePreviewFor(ctx, a, in, 1)
	if err != nil || preview.Counts.TargetPeople != 2 || preview.Counts.SourcePeople != 2 || preview.Counts.EligiblePeople != 2 || preview.Counts.Destinations != 1 {
		t.Fatal("shared destination decisions", preview, err)
	}
	id := messageApproved(t, s, a, b, in)
	claims, _, err := s.ClaimMessageDispatch(ctx, a, id, statementMessageDispatch(messageDetail(t, s, a, id)))
	if err != nil || len(claims) != 1 {
		t.Fatal(claims, err)
	}
	owner := reviewLogin(t, s, "owner@demo.society")
	stop := contactAction(contactDetails(t, s, owner, "me"), "OPTED_OUT")
	stop.Channel, stop.Purpose = "EMAIL", "FINANCE"
	if _, err = s.ActOnContact(ctx, owner, "me", stop); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "ACCEPTED"); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteSyntheticMessage(ctx, claims[0].ID); err != nil {
		t.Fatal(err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE state='HANDED_OFF' AND resident_id='demo-joint-owner'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM message_attempt_recipients WHERE state='OPTED_OUT' AND resident_id='demo-owner-A-101'", 1)
	if x := messageDetail(t, s, owner, id); x.Outcomes["OPTED_OUT"] != 1 || x.Outcomes["ACCEPTED"] != 0 {
		t.Fatal("another person's acceptance attributed to opted out owner", x)
	}
}

func TestStatementMessageUnknownProofSurvivesPublicationRemovalAndNoBlindResend(t *testing.T) {
	s, a, _, _ := messageFixture(t)
	ctx := context.Background()
	b := maintenanceReviewer(t, s, a)
	file, pub := publishedMessageStatement(t, s, a, b, "TENANTS")
	id := messageApproved(t, s, a, b, statementMessageInput(pub))
	claims, _, err := s.ClaimMessageDispatch(ctx, a, id, statementMessageDispatch(messageDetail(t, s, a, id)))
	if err != nil || len(claims) != 1 {
		t.Fatal(claims, err)
	}
	h, err := s.SyntheticMessageHandoff(ctx, a, claims[0].ID, "UNKNOWN")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteSyntheticMessage(ctx, claims[0].ID); err != nil {
		t.Fatal(err)
	}
	revokeMessageStatement(t, s, a, file, pub)
	x := messageDetail(t, s, a, id)
	if x.Outcomes["UNKNOWN"] != 1 || x.CanDispatch {
		t.Fatal("unknown source removal invented outcome", x)
	}
	if _, _, err = s.ClaimMessageDispatch(ctx, a, id, statementMessageDispatch(x)); !errors.Is(err, ErrConflict) {
		t.Fatal("blind resend", err)
	}
	if _, err = s.ReconcileMessage(ctx, a, id, claims[0].DeliveryID, messageDecision(x, "RECONCILE")); err != nil {
		t.Fatal(err)
	}
	x = messageDetail(t, s, a, id)
	if x.Outcomes["ACCEPTED"] != 1 || x.Deliveries[0].ProviderID != h.ProviderID || x.Deliveries[0].DeliveredAt != 0 || x.Deliveries[0].ReadAt != 0 {
		t.Fatal("retained acceptance proof invents delivered/read", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM simulation_messages", 1)
}

func TestStatementMessageSelectedPublicationChoicesAndResidentSearchDoNotRevealPrivateTarget(t *testing.T) {
	s, a, _, _ := messageFixture(t)
	ctx := context.Background()
	b := maintenanceReviewer(t, s, a)
	data := []byte("Description,Amount\nPrivate supplied figure,1.00\n")
	file := approvedStatement(t, s, a, b, statementInput(data), data)
	in := publicationInput(t, s, a, file, "ALL")
	in.Target = MessageTarget{Kind: "HOMES", IDs: []string{"demo-flat-A-101"}}
	preview, err := s.PreviewStatementPublication(ctx, a, in)
	if err != nil || preview.TargetPeople != 2 {
		t.Fatal(preview, err)
	}
	in.PreviewHash = preview.PreviewHash
	pub, err := s.ProposeStatementPublication(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideStatementPublication(ctx, b, pub, StatementAction{OperationKey: randomToken(), Version: 1, Action: "PUBLISHED", Confirmed: true, Reason: "Reviewed the exact single-home publication"}); err != nil {
		t.Fatal(err)
	}
	choices, err := s.MessageTargetsFor(ctx, a, "STATEMENT", pub, "HOMES", "", 1)
	if err != nil || choices.Total != 1 || len(choices.Items) != 1 || choices.Items[0].ID != "demo-flat-A-101" {
		t.Fatal("multi-home person broadens home picker", choices, err)
	}
	messages := statementMessageInput(pub)
	messages.Target = MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}
	id := messageApproved(t, s, a, b, messages)
	owner := reviewLogin(t, s, "owner@demo.society")
	own := messageDetail(t, s, owner, id)
	body, _ := json.Marshal(own)
	if own.Source.PublicationTarget != nil || strings.Contains(string(body), "demo-flat-A-101") || strings.Contains(string(body), "999999") {
		t.Fatal("resident sees frozen private target", string(body))
	}
	for _, query := range []string{statementInput(data).Title, "Fictional externally prepared"} {
		page, err := s.MessagesFor(ctx, owner, query, "", "STATEMENT", 1)
		if err != nil || page.Total != 0 {
			t.Fatal("private original title is a search oracle", page, err)
		}
	}
	page, err := s.MessagesFor(ctx, owner, "Published financial statement", "", "STATEMENT", 1)
	if err != nil || page.Total != 1 {
		t.Fatal("public generic history search", page, err)
	}
}
