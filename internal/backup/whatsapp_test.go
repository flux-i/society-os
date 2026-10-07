package backup

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/messaging"
	"society.local/portal/internal/security"
)

func TestWhatsAppRecoveryRetainsUnknownExternalStartBindingsProofAndHeldSecret(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	for _, seed := range []func(context.Context) error{s.SeedDemoAccounts, s.SeedDemoTreasury} {
		if err := seed(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	s.MFA, err = security.LoadKey(filepath.Join(root, "keys", "mfa.key"), true)
	if err != nil {
		t.Fatal(err)
	}
	a, b, o := upkeepRecoveryLogin(t, s, "admin@demo.society"), upkeepRecoveryLogin(t, s, "committee@demo.society"), upkeepRecoveryLogin(t, s, "owner@demo.society")
	contact := database.ContactInput{OperationKey: "whatsapp-recovery-contact-12345", Phone: "+919000000101", PreferredChannel: "WHATSAPP", ConsentSource: "Fictional separately supplied communication choice", Reason: "Fictional deliberately supplied registered destination", Confirmed: true, ContactPreferences: database.ContactPreferences{CommunityWhatsApp: true}}
	if _, err = s.RegisterContact(ctx, o, "me", contact); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ActOnContact(ctx, b, "demo-owner-A-101", database.ContactAction{OperationKey: "whatsapp-recovery-contact-review-12345", Version: 1, Action: "VERIFIED", Reason: "Independently verified the fictional destination and person", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	notice, err := s.SubmitReview(ctx, a, "", database.ReviewInput{OperationKey: "whatsapp-recovery-notice-12345", Kind: "NOTICE", Title: "Fictional recovery provider notice", Body: "Fictional source retained inside the authenticated portal audience.", Audience: "ALL_RESIDENTS"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideReview(ctx, b, notice, database.ReviewAction{OperationKey: "whatsapp-recovery-notice-review-12345", Version: 1, Decision: "APPROVED", Reason: "Independently reviewed the fictional notice and audience", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	provider := &database.MessageProvider{Mode: "CLOUD_FIXTURE", Origin: "http://127.0.0.1:12345", APIVersion: "v26.0", AccountID: "111111", PhoneID: "222222", TemplateID: "444444", Name: "society_notice", Language: "en", Category: "MARKETING", Body: "Open the society update securely: {{1}}"}
	config := messaging.WhatsAppConfig{Origin: provider.Origin, APIVersion: provider.APIVersion, AccountID: provider.AccountID, PhoneID: provider.PhoneID, AccessToken: "FICTIONAL_RECOVERY_ACCESS_TOKEN_123456789", AppSecret: "FICTIONAL_RECOVERY_APP_SECRET_123456789", VerifyToken: "FICTIONAL_RECOVERY_VERIFY_TOKEN_123456789", Templates: map[string]messaging.WhatsAppTemplate{"NOTICE": {ID: "444444", Name: "society_notice", Language: "en"}}}
	client, err := messaging.NewWhatsAppClient(config, true)
	if err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{61}, 32)
	engine, err := messaging.New(s, key)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.WithWhatsAppFixture(client); err != nil {
		t.Fatal(err)
	}
	if err = engine.VerifyKey(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err = engine.VerifyWhatsAppKey(ctx); err != nil {
		t.Fatal(err)
	}
	in := database.MessageInput{SourceKind: "NOTICE", SourceID: notice, Channel: "WHATSAPP", Target: database.MessageTarget{Kind: "PEOPLE", IDs: []string{"demo-owner-A-101"}}, PortalOrigin: "http://127.0.0.1:8080", Provider: provider}
	preview, err := s.MessagePreviewFor(ctx, a, in, 1)
	if err != nil {
		t.Fatal(err)
	}
	in.OperationKey, in.PreviewHash, in.Reason, in.Confirmed = "whatsapp-recovery-proposal-12345", preview.PreviewHash, "Reviewed the actual frozen template and fictional recipient", true
	id, err := s.ProposeMessage(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ActOnMessage(ctx, b, id, database.MessageAction{OperationKey: "whatsapp-recovery-approve-12345", Version: 1, Action: "APPROVED", Reason: "Independently reviewed the frozen content and recipient", Confirmed: true, Provider: provider}); err != nil {
		t.Fatal(err)
	}
	claims, _, err := s.ClaimMessageDispatch(ctx, a, id, database.MessageAction{OperationKey: "whatsapp-recovery-dispatch-12345", Version: 2, Action: "DISPATCH", Reason: "Deliberately requested the approved provider handoff", Confirmed: true, Provider: provider})
	if err != nil || len(claims) != 1 {
		t.Fatal(claims, err)
	}
	if _, err = s.PrepareWhatsAppHandoff(ctx, a, claims[0].ID, provider); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "provider-checkpoint")
	if _, err = Snapshot(ctx, s, bundle, "0.20.0-dev"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored", "society.db")
	if _, err = Restore(ctx, bundle, target); err != nil {
		t.Fatal(err)
	}
	r, err := database.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.MFA = s.MFA
	if _, err = r.CheckSession(ctx, a); !errors.Is(err, database.ErrUnauthenticated) {
		t.Fatal("restored old session", err)
	}
	restored, err := messaging.New(r, key)
	if err != nil {
		t.Fatal(err)
	}
	if err = restored.WithWhatsAppFixture(client); err != nil {
		t.Fatal(err)
	}
	if err = restored.VerifyKey(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err = restored.VerifyWhatsAppKey(ctx); err != nil {
		t.Fatal(err)
	}
	wrongConfig := config
	wrongConfig.AppSecret = "FICTIONAL_DIFFERENT_APP_SECRET_123456789"
	wrongClient, _ := messaging.NewWhatsAppClient(wrongConfig, true)
	wrong, _ := messaging.New(r, key)
	wrong.WithWhatsAppFixture(wrongClient)
	if err = wrong.VerifyWhatsAppKey(ctx); err == nil {
		t.Fatal("application secret rebound after restore")
	}
	if err = r.RecoverMessageClaims(ctx); err != nil {
		t.Fatal(err)
	}
	fresh := upkeepRecoveryLogin(t, r, "admin@demo.society")
	x, err := r.MessageFor(ctx, fresh, id, 1, 1, 1)
	if err != nil || x.Outcomes["UNKNOWN"] != 1 || x.CanDispatch || !reflect.DeepEqual(x.Provider, provider) {
		t.Fatal(x, err)
	}
	if _, err = r.ReconcileMessage(ctx, fresh, id, claims[0].DeliveryID, database.MessageAction{OperationKey: "whatsapp-recovery-reconcile-12345", Version: 3, Action: "RECONCILE", Reason: "Reviewed the retained unresolved external provider start", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	x, err = r.MessageFor(ctx, fresh, id, 1, 1, 1)
	if err != nil || x.Outcomes["UNKNOWN"] != 1 || x.Deliveries[0].Attempts != 1 || x.CanDispatch {
		t.Fatal("restore caused a duplicate or fabricated resolution", x, err)
	}
	proof := database.WhatsAppStatus{EventHash: strings.Repeat("e", 64), AccountID: "111111", PhoneID: "222222", AttemptID: claims[0].ID, ProviderID: "wamid.RECOVERED_FICTIONAL_001", Destination: "+919000000101", State: "READ", At: time.Now().Unix(), Metadata: `{"status":"read","id":"wamid.RECOVERED_FICTIONAL_001"}`}
	if err = r.ReceiveWhatsAppStatuses(ctx, []database.WhatsAppStatus{proof}); err != nil {
		t.Fatal(err)
	}
	x, err = r.MessageFor(ctx, fresh, id, 1, 1, 1)
	if err != nil || x.Outcomes["READ"] != 1 || x.Deliveries[0].Attempts != 1 || x.Deliveries[0].DeliveredAt != 0 {
		t.Fatal("late evidence lost or delivery timestamp invented", x, err)
	}
	var count int
	if err = r.DB.QueryRow("SELECT COUNT(*) FROM whatsapp_handoffs").Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate retained handoff", count, err)
	}
}
