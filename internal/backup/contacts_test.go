package backup

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func TestContactRecoveryPreservesVerifiedDestinationsConsentOptOutHistoryAndNoTemporaryCredentials(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if e := s.SeedDemoAccounts(ctx); e != nil {
		t.Fatal(e)
	}
	var e error
	s.MFA, e = security.NewBox(make([]byte, 32))
	if e != nil {
		t.Fatal(e)
	}
	o, a := upkeepRecoveryLogin(t, s, "owner@demo.society"), upkeepRecoveryLogin(t, s, "admin@demo.society")
	in := database.ContactInput{OperationKey: "recovery-contact-register-12345", Phone: "+919000000101", Email: "recovery-contact@example.test", PreferredChannel: "EMAIL", ConsentSource: "PRIVATE recorded resident identity and channel permission", Reason: "PRIVATE original supplied contact choices to preserve", ContactPreferences: database.ContactPreferences{CommunityWhatsApp: true, CommunityEmail: true, FinanceEmail: true}, Confirmed: true}
	id, e := s.RegisterContact(ctx, o, "me", in)
	if e != nil {
		t.Fatal(e)
	}
	approve := database.ContactAction{OperationKey: "recovery-contact-verify-12345", Version: 1, Action: "VERIFIED", Reason: "PRIVATE independently verified the supplied destination and permission", Confirmed: true}
	if _, e = s.ActOnContact(ctx, a, id, approve); e != nil {
		t.Fatal(e)
	}
	stop := database.ContactAction{OperationKey: "recovery-contact-stop-12345", Version: 2, Action: "OPTED_OUT", Channel: "EMAIL", Purpose: "FINANCE", Reason: "PRIVATE stop only the selected financial messages", Confirmed: true}
	if _, e = s.ActOnContact(ctx, o, id, stop); e != nil {
		t.Fatal(e)
	}
	want, e := s.ContactFor(ctx, o, id, 1)
	if e != nil || want.State != "VERIFIED" || want.Eligible != (database.ContactPreferences{CommunityWhatsApp: true, CommunityEmail: true}) || want.EventTotal != 3 {
		t.Fatal(want, e)
	}
	bundle := filepath.Join(root, "contacts-snapshot")
	if _, e = Snapshot(ctx, s, bundle, "contact-test"); e != nil {
		t.Fatal(e)
	}
	in.Version, in.OperationKey, in.Phone = 3, "recovery-contact-later-12345", "+919000000202"
	if _, e = s.RegisterContact(ctx, o, id, in); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(root, "restored-contacts", "society.db")
	if _, e = Restore(ctx, bundle, target); e != nil {
		t.Fatal(e)
	}
	r, e := database.Open(ctx, target)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	r.MFA = s.MFA
	if _, e = r.CheckSession(ctx, o); !errors.Is(e, database.ErrUnauthenticated) {
		t.Fatal("old session survived restore", e)
	}
	fresh := upkeepRecoveryLogin(t, r, "owner@demo.society")
	got, e := r.ContactFor(ctx, fresh, id, 1)
	if e != nil || !reflect.DeepEqual(want, got) {
		t.Fatal("contact/consent checkpoint changed", got, e)
	}
	for _, table := range []string{"account_tokens", "mfa_recovery_codes", "mfa_pending"} {
		var count int
		if e = r.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); e != nil || count != 0 {
			t.Fatal("temporary credential restored", table, count, e)
		}
	}
	if e = r.VerifyMFAKey(ctx); e != nil {
		t.Fatal(e)
	}
}
