package database

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func contactInput() ContactInput {
	return ContactInput{OperationKey: randomToken(), Phone: "+91 (90000) 00101", Email: "Resident@EXAMPLE.TEST", PreferredChannel: "EMAIL", ContactPreferences: ContactPreferences{CommunityWhatsApp: true, CommunityEmail: true, FinanceEmail: true}, ConsentSource: "PRIVATE supplied resident permission reference C-01", Reason: "PRIVATE confirmed these supplied contact choices for independent verification", Confirmed: true}
}
func contactDetails(t *testing.T, s *Store, token, id string) ContactDetail {
	t.Helper()
	x, e := s.ContactFor(context.Background(), token, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func contactAction(x ContactDetail, action string) ContactAction {
	return ContactAction{OperationKey: randomToken(), Version: x.Version, Action: action, Reason: "PRIVATE independently checked the supplied identity and communication choices", Confirmed: true}
}
func TestContactSeparateVerificationExactConsentChangesAndImmutableHistory(t *testing.T) {
	s, a := adminFixture(t)
	o := reviewLogin(t, s, "owner@demo.society")
	b := reviewLogin(t, s, "committee@demo.society")
	ctx := context.Background()
	in := contactInput()
	id, e := s.RegisterContact(ctx, o, "me", in)
	if e != nil || id != "demo-owner-A-101" {
		t.Fatal(id, e)
	}
	if again, e := s.RegisterContact(ctx, o, "me", in); e != nil || again != id {
		t.Fatal("registration retry", again, e)
	}
	x := contactDetails(t, s, o, "me")
	if x.State != "PENDING" || x.Version != 1 || x.EventTotal != 1 || x.Phone != "+919000000101" || x.Email != "Resident@example.test" || x.Eligible != (ContactPreferences{}) {
		t.Fatal("pending destination is not eligible", x)
	}
	approve := contactAction(x, "VERIFIED")
	if _, e = s.ActOnContact(ctx, o, id, approve); !errors.Is(e, ErrForbidden) {
		t.Fatal("self verification accepted", e)
	}
	for i := 0; i < 2; i++ {
		if _, e = s.ActOnContact(ctx, b, id, approve); e != nil {
			t.Fatal("separate verification/retry", e)
		}
	}
	x = contactDetails(t, s, o, id)
	want := ContactPreferences{CommunityWhatsApp: true, CommunityEmail: true, FinanceEmail: true}
	if x.State != "VERIFIED" || x.Version != 2 || x.EventTotal != 2 || x.Eligible != want {
		t.Fatal("independently selected combinations", x)
	}
	stop := contactAction(x, "OPTED_OUT")
	stop.Channel, stop.Purpose = "EMAIL", "FINANCE"
	if _, e = s.ActOnContact(ctx, o, id, stop); e != nil {
		t.Fatal(e)
	}
	x = contactDetails(t, s, o, id)
	want.FinanceEmail = false
	if x.State != "VERIFIED" || x.Eligible != want || x.ReviewedBy != "demo-user-committee" || x.Version != 3 {
		t.Fatal("specific opt-out rewrote verification or wrong preference", x)
	}
	old := x.Events
	in.Version, in.OperationKey, in.Phone = x.Version, randomToken(), "+919000000202"
	if _, e = s.RegisterContact(ctx, o, id, in); e != nil {
		t.Fatal(e)
	}
	x = contactDetails(t, s, o, id)
	if x.State != "PENDING" || x.Version != 4 || x.Eligible != (ContactPreferences{}) || x.ReviewedBy != "" || !reflect.DeepEqual(old, x.Events[1:]) {
		t.Fatal("address change retained eligibility or rewrote snapshots", x)
	}
	if _, e = s.ActOnContact(ctx, a, id, contactAction(x, "DECLINED")); e != nil {
		t.Fatal(e)
	}
	x = contactDetails(t, s, o, id)
	if x.State != "DECLINED" || x.Eligible != (ContactPreferences{}) || x.EventTotal != 5 {
		t.Fatal(x)
	}
	for _, query := range []string{"DELETE FROM resident_contacts WHERE resident_id=?", "DELETE FROM contact_events WHERE resident_id=?", "UPDATE contact_events SET reason='changed' WHERE resident_id=?"} {
		if _, e = s.DB.Exec(query, id); e == nil {
			t.Fatal("retained history changed", query)
		}
	}
	for _, table := range []string{"entries", "receipts"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
	var audit string
	rows, e := s.DB.Query("SELECT reason||before_json||after_json FROM audit_events WHERE action LIKE 'CONTACT_%'")
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	for rows.Next() {
		if e = rows.Scan(&audit); e != nil || strings.Contains(audit, "PRIVATE") || strings.Contains(audit, "90000") || strings.Contains(audit, "example.test") {
			t.Fatal("broad audit leaks private contact data", audit, e)
		}
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
}
func TestContactCurrentPersonDirectorySameHomePrivacyAndFormerResidentStop(t *testing.T) {
	s, a := adminFixture(t)
	o := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	ctx := context.Background()
	for _, tc := range []struct {
		role, wing string
		count      int
	}{{"", "", 153}, {"OWNER", "", 118}, {"TENANT", "", 35}, {"OWNER", "A", 40}, {"TENANT", "A", 12}, {"OWNER", "C", 38}} {
		page, e := s.ContactsFor(ctx, a, "", tc.role, tc.wing, "", 1)
		if e != nil || page.Total != tc.count || len(page.Items) != 12 {
			t.Fatal("independent distinct-person count", tc, page, e)
		}
	}
	id, e := s.RegisterContact(ctx, o, "me", contactInput())
	if e != nil {
		t.Fatal(e)
	}
	x := contactDetails(t, s, a, id)
	if _, e = s.ActOnContact(ctx, a, id, contactAction(x, "VERIFIED")); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ContactFor(ctx, tenant, id, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("other person's private contact readable", e)
	}
	// Share a home while retaining independent person identities.
	accessExec(t, s, "INSERT INTO flat_memberships VALUES('contact-shared-home','demo-flat-A-101','demo-tenant-A-103','TENANT','2020-01-01',NULL,0,0)")
	if _, e = s.ContactFor(ctx, tenant, id, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("same-home contact readable", e)
	}
	page, e := s.ContactsFor(ctx, o, "", "", "", "", 1)
	if e != nil || page.Total != 1 || len(page.Items) != 1 || len(page.Items[0].Homes) != 2 {
		t.Fatal("own person is duplicated for two homes", page, e)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date='2026-01-01' WHERE resident_id=?", id)
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=0 WHERE token_hash=?", TokenHash(o))
	x = contactDetails(t, s, o, id)
	if x.Current || x.CanRegister || x.Eligible != (ContactPreferences{}) {
		t.Fatal("former resident retained delivery eligibility", x)
	}
	in := contactInput()
	in.Version = x.Version
	if _, e = s.RegisterContact(ctx, o, id, in); !errors.Is(e, ErrForbidden) {
		t.Fatal("former registration accepted", e)
	}
	stop := contactAction(x, "OPTED_OUT")
	stop.Channel, stop.Purpose = "ALL", "ALL"
	if _, e = s.ActOnContact(ctx, o, id, stop); e != nil {
		t.Fatal("own opt-out required new authentication/membership", e)
	}
	x = contactDetails(t, s, o, id)
	if x.ContactPreferences != (ContactPreferences{}) || x.EventTotal != 3 {
		t.Fatal("former opt-out not retained", x)
	}
}
func TestContactIndependentConcurrentDecisionStaleRetryAndCurrentAuthority(t *testing.T) {
	s, a := adminFixture(t)
	b := reviewLogin(t, s, "committee@demo.society")
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	in := contactInput()
	id, e := s.RegisterContact(ctx, a, "demo-owner-A-101", in)
	if e != nil {
		t.Fatal(e)
	}
	x := contactDetails(t, s, a, id)
	if _, e = s.ActOnContact(ctx, a, id, contactAction(x, "VERIFIED")); !errors.Is(e, ErrForbidden) {
		t.Fatal("officer verified own manual registration", e)
	}
	good := contactAction(x, "VERIFIED")
	bad := contactAction(x, "DECLINED")
	var wg sync.WaitGroup
	errorsOut := make(chan error, 2)
	for _, action := range []ContactAction{good, bad} {
		wg.Add(1)
		go func(in ContactAction) { defer wg.Done(); _, err := s.ActOnContact(ctx, b, id, in); errorsOut <- err }(action)
	}
	wg.Wait()
	close(errorsOut)
	accepted, rejected := 0, 0
	for e = range errorsOut {
		if e == nil {
			accepted++
		} else if errors.Is(e, ErrConflict) {
			rejected++
		} else {
			t.Fatal(e)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatal("concurrent review", accepted, rejected)
	}
	x = contactDetails(t, s, b, id)
	if x.EventTotal != 2 || x.Version != 2 {
		t.Fatal("decision duplicated", x)
	}
	in.OperationKey, in.Version = randomToken(), x.Version
	if _, e = s.RegisterContact(ctx, o, id, in); e != nil {
		t.Fatal(e)
	}
	x = contactDetails(t, s, b, id)
	good = contactAction(x, "VERIFIED")
	if _, e = s.ActOnContact(ctx, b, id, good); e != nil {
		t.Fatal(e)
	}
	changed := good
	changed.Reason = "Changed details under the same accepted retry identity"
	if _, e = s.ActOnContact(ctx, b, id, changed); !errors.Is(e, ErrConflict) {
		t.Fatal("changed replay", e)
	}
	accessExec(t, s, "UPDATE role_grants SET revoked_at=1 WHERE user_id='demo-user-committee'")
	if _, e = s.ActOnContact(ctx, b, id, good); e == nil {
		t.Fatal("revoked actor replayed private decision")
	}
	// A new eligible reviewer still needs a current factor for an accepted retry.
	x = contactDetails(t, s, a, id)
	in.OperationKey, in.Version = randomToken(), x.Version
	if _, e = s.RegisterContact(ctx, o, id, in); e != nil {
		t.Fatal(e)
	}
	x = contactDetails(t, s, a, id)
	good = contactAction(x, "VERIFIED")
	if _, e = s.ActOnContact(ctx, a, id, good); e != nil {
		t.Fatal(e)
	}
	accessExec(t, s, "DELETE FROM mfa_factors WHERE user_id='demo-user-admin'")
	if _, e = s.ActOnContact(ctx, a, id, good); !errors.Is(e, ErrMFARequired) {
		t.Fatal("lost-factor replay", e)
	}
}
func TestContactValidationReviewGuardsAndBoundedPrivateHistory(t *testing.T) {
	s, a := adminFixture(t)
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	for _, edit := range []func(*ContactInput){
		func(in *ContactInput) { in.Phone = "9000000101" },
		func(in *ContactInput) { in.Phone = "+0123456789" },
		func(in *ContactInput) { in.Phone = "+123" },
		func(in *ContactInput) { in.Email = "Name <person@example.test>" },
		func(in *ContactInput) { in.Email = "person@example.test\r\nBcc: secret@example.test" },
		func(in *ContactInput) { in.Email = "pérson@example.test" },
		func(in *ContactInput) { in.Phone = "" },
		func(in *ContactInput) { in.Email = "" },
		func(in *ContactInput) { in.Confirmed = false },
		func(in *ContactInput) { in.Version = -1 },
	} {
		in := contactInput()
		edit(&in)
		if _, e := s.RegisterContact(ctx, o, "me", in); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid contact accepted", in, e)
		}
	}
	id, e := s.RegisterContact(ctx, o, "me", contactInput())
	if e != nil {
		t.Fatal(e)
	}
	x := contactDetails(t, s, a, id)
	if _, e = s.ActOnContact(ctx, a, id, contactAction(x, "VERIFIED")); e != nil {
		t.Fatal(e)
	}
	for _, query := range []string{
		"UPDATE resident_contacts SET phone='+919000000303',version=version+1 WHERE resident_id=?",
		"UPDATE resident_contacts SET finance_whatsapp=1,version=version+1 WHERE resident_id=?",
		"UPDATE resident_contacts SET submitted_by='demo-user-admin',version=version+1 WHERE resident_id=?",
	} {
		if _, e = s.DB.Exec(query, id); e == nil {
			t.Fatal("verified destination/consent silently changed", query)
		}
	}
	for i := 0; i < 23; i++ {
		x = contactDetails(t, s, o, id)
		in := contactAction(x, "OPTED_OUT")
		in.Channel, in.Purpose = "ALL", "ALL"
		if _, e = s.ActOnContact(ctx, o, id, in); e != nil {
			t.Fatal(e)
		}
	}
	one := contactDetails(t, s, o, id)
	two, e := s.ContactFor(ctx, o, id, 2)
	if e != nil || one.EventTotal != 25 || len(one.Events) != 20 || len(two.Events) != 5 || one.Events[19].Version != 6 || two.Events[0].Version != 5 || two.Events[4].Snapshot.Phone != "+919000000101" {
		t.Fatal("bounded retained contact history", one, two, e)
	}
	if _, e = s.ContactFor(ctx, o, id, 10001); !errors.Is(e, ErrInvalid) {
		t.Fatal("unbounded history", e)
	}
	if _, e = s.ContactsFor(ctx, a, strings.Repeat("x", 101), "", "", "", 1); !errors.Is(e, ErrInvalid) {
		t.Fatal("unbounded search", e)
	}
}

func TestContactDirectoryAuthorityIsSeparateFromFinancialAppointments(t *testing.T) {
	s, a := adminFixture(t)
	o := reviewLogin(t, s, "owner@demo.society")
	ctx := context.Background()
	id, e := s.RegisterContact(ctx, o, "me", contactInput())
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.Authenticate(ctx, a)
	if e != nil || !p.CanReadContacts || !p.CanManageContacts || p.CanReadAllRecords || p.CanManageRecords {
		t.Fatal("registry appointment altered finance authority", p, e)
	}
	x := contactDetails(t, s, a, id)
	for _, role := range []string{"TREASURER", "AUDITOR"} {
		accessExec(t, s, "UPDATE role_grants SET role=? WHERE id='demo-grant-demo-user-admin'", role)
		p, e = s.Authenticate(ctx, a)
		if e != nil || p.CanReadContacts || p.CanManageContacts || !p.CanReadAllRecords || p.CanManageRecords != (role == "TREASURER") {
			t.Fatal("financial appointment acquired private contact authority", role, p, e)
		}
		page, e := s.ContactsFor(ctx, a, "", "", "", "", 1)
		if e != nil || page.Total != 0 || len(page.Items) != 0 {
			t.Fatal("unlinked financial operator acquired a directory", role, page, e)
		}
		if _, e = s.ContactFor(ctx, a, id, 1); !errors.Is(e, sql.ErrNoRows) {
			t.Fatal("financial operator read another person's destinations", role, e)
		}
		if _, e = s.ActOnContact(ctx, a, id, contactAction(x, "VERIFIED")); e == nil {
			t.Fatal("financial appointment verified private contact", role)
		}
	}
}
