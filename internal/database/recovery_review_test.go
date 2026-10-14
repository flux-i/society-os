package database

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"society.local/portal/internal/security"
)

const recoveryPasswordOne = "Supplied-reviewer-one-2026!"
const recoveryPasswordTwo = "Supplied-reviewer-two-2026!"

func recoveryAppointment(t *testing.T, s *Store, origin RecoveryOrigin) (RecoveryNomination, RecoveryPasswords) {
	t.Helper()
	now := time.Now().Unix()
	in := RecoveryNomination{FormatVersion: 1, OperationID: "independent-current-nomination", EpochID: origin.ID, SourceSHA256: origin.SourceSHA256, ValidFrom: now - 1, ValidUntil: now + 86400,
		Custodians: []RecoveryAttestation{{"supplied-person-custodian-1", "fictional-current-authority-check-1", now - 2}, {"supplied-person-custodian-2", "fictional-current-authority-check-2", now - 2}},
		Reviewers:  []RecoveryNominee{{Email: "reviewer.one@example.test", Name: "Current Reviewer One", IdentityRef: "supplied-person-reviewer-1", EvidenceRef: "independent-person-check-1", VerifiedAt: now - 2}, {Email: "reviewer.two@example.test", Name: "Current Reviewer Two", IdentityRef: "supplied-person-reviewer-2", EvidenceRef: "independent-person-check-2", VerifiedAt: now - 2}}}
	return in, RecoveryPasswords{[]RecoveryPassword{{in.Reviewers[0].Email, recoveryPasswordOne}, {in.Reviewers[1].Email, recoveryPasswordTwo}}}
}
func currentRecoveryReviewers(t *testing.T, s *Store) (string, string, RecoveryNomination) {
	t.Helper()
	s.MFA, _ = security.NewBox(make([]byte, 32))
	ctx := context.Background()
	origin, err := s.QuarantineRecovery(ctx, controlledRecoveryOrigin(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	in, passwords := recoveryAppointment(t, s, origin)
	if _, err = s.NominateRecovery(ctx, in, passwords); err != nil {
		t.Fatal(err)
	}
	login := func(email, password string) string {
		token, p := loginTest(t, s, email, password)
		if !p.RecoveryRequired || !p.RecoveryReviewer || !p.MFAPending || p.CanReviewRecovery || p.Fresh || len(p.Roles) != 0 {
			t.Fatal("Appointment alone established private/fresh authority", p)
		}
		if _, err = s.RecoverySummaryFor(ctx, token); !errors.Is(err, ErrMFARequired) {
			t.Fatal("Password-only appointment entered the desk", err)
		}
		setup, err := s.SetupMFA(ctx, token)
		if err != nil {
			t.Fatal(err)
		}
		secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
		if err != nil {
			t.Fatal(err)
		}
		result, err := s.ConfirmMFA(ctx, token, security.Code(secret, time.Now().Unix()/30, 6))
		if err != nil {
			t.Fatal(err)
		}
		if !result.User.CanReviewRecovery || !result.User.Fresh || result.User.MFAPending || result.User.IsDemo || len(result.User.Roles) != 0 {
			t.Fatal("Real personal factor did not establish only review authority", result.User)
		}
		return token
	}
	return login(in.Reviewers[0].Email, recoveryPasswordOne), login(in.Reviewers[1].Email, recoveryPasswordTwo), in
}
func recoveryAccessProposal(t *testing.T, s *Store, token string) RecoveryProposalInput {
	t.Helper()
	p, err := s.CheckSession(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	return RecoveryProposalInput{OperationKey: randomToken(), ScopeKey: p.ScopeKey, Kind: "ACCESS", Title: "Current supplied household access", Reason: "Current independent records checked for this supplied person and home.", Access: &RecoveryAccess{Email: "member.current@example.test", Name: "Supplied Current Member", IdentityRef: "independent-member-person-17", Role: "HOUSEHOLD", HomeID: "demo-flat-A-101", Relationship: "OWNER", Finance: false, ValidFrom: now - 1, ValidUntil: now + 90*86400, VerifiedAt: now - 2, EvidenceRef: "independent-current-register-sheet-17"}}
}

func TestRecoveryNominationStrictSourceAndExactRetryPreserveSessionsAndFactors(t *testing.T) {
	s, oldAdmin := adminFixture(t)
	ctx := context.Background()
	beforeFactor := moneyRows(t, s)["mfa_factors"]
	origin, err := s.QuarantineRecovery(ctx, controlledRecoveryOrigin(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	in, passwords := recoveryAppointment(t, s, origin)
	// Deliberately verified existing account: no password/factor replacement and
	// no historical administrator grant revival.
	var existing string
	if err = s.DB.QueryRow("SELECT id FROM users WHERE login='admin@demo.society'").Scan(&existing); err != nil {
		t.Fatal(err)
	}
	in.Reviewers[0].Email, in.Reviewers[0].AccountID = "admin@demo.society", existing
	passwords.Passwords = passwords.Passwords[1:]
	baseline := moneyRows(t, s)
	for _, change := range []struct {
		name   string
		change func(*RecoveryNomination)
	}{
		{"wrong source", func(x *RecoveryNomination) { x.SourceSHA256 = strings.Repeat("b", 64) }},
		{"wrong epoch", func(x *RecoveryNomination) { x.EpochID = strings.Repeat("X", 43) }},
		{"future appointment", func(x *RecoveryNomination) { x.ValidFrom = time.Now().Unix() + 60 }},
		{"expired appointment", func(x *RecoveryNomination) {
			x.ValidFrom = time.Now().Unix() - 120
			x.ValidUntil = time.Now().Unix() - 60
		}},
		{"same custodian", func(x *RecoveryNomination) {
			x.Custodians[1].IdentityRef = strings.ToUpper(x.Custodians[0].IdentityRef)
		}},
		{"same person", func(x *RecoveryNomination) { x.Reviewers[1].IdentityRef = strings.ToUpper(x.Reviewers[0].IdentityRef) }},
		{"later invalid existing identity", func(x *RecoveryNomination) { x.Reviewers[1].AccountID = "not-the-existing-identity" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			body, _ := json.Marshal(in)
			var bad RecoveryNomination
			json.Unmarshal(body, &bad)
			change.change(&bad)
			if _, err := s.NominateRecovery(ctx, bad, passwords); err == nil {
				t.Fatal("Bad supplied appointment was accepted")
			}
			if !reflect.DeepEqual(baseline, moneyRows(t, s)) {
				t.Fatal("Rejected appointment changed rows or credentials")
			}
		})
	}
	body, _ := json.Marshal(in)
	for _, bad := range []string{string(body[:len(body)-1]) + `,"operation_id":"ambiguous"}`, string(body[:len(body)-1]) + `,"unknown":true}`, string(body) + "{}", strings.Repeat(" ", 32769)} {
		if _, err = ParseRecoveryNomination([]byte(bad)); !errors.Is(err, ErrInvalid) {
			t.Fatal("Ambiguous nomination JSON was accepted", err)
		}
	}
	if _, err = s.NominateRecovery(ctx, in, passwords); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CheckSession(ctx, oldAdmin); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("Affected pre-nomination session remained valid", err)
	}
	if moneyRows(t, s)["mfa_factors"] != beforeFactor {
		t.Fatal("Nomination reset the supplied existing account's factor")
	}
	fresh, p := loginTest(t, s, "admin@demo.society", DemoPassword)
	if !p.RecoveryReviewer || p.CanManageRegistry || p.CanManageRecords || len(p.Roles) != 0 {
		t.Fatal("Old grants were substituted for current narrow nomination", p)
	}
	var version int
	s.DB.QueryRow("SELECT auth_version FROM users WHERE id=?", existing).Scan(&version)
	for range 2 {
		if _, err = s.NominateRecovery(ctx, in, passwords); err != nil {
			t.Fatal("Exact lost-response retry failed", err)
		}
	}
	var afterVersion int
	s.DB.QueryRow("SELECT auth_version FROM users WHERE id=?", existing).Scan(&afterVersion)
	if afterVersion != version {
		t.Fatal("Retry revoked credentials twice", version, afterVersion)
	}
	if _, err = s.CheckSession(ctx, fresh); err != nil {
		t.Fatal("Retry invalidated the newly signed-in device", err)
	}
	changed := in
	changed.OperationID = "competing-current-nomination"
	if _, err = s.NominateRecovery(ctx, changed, passwords); !errors.Is(err, ErrConflict) {
		t.Fatal("Conflicting nomination bypassed explicit predecessor", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_nominations", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_reviewers", 2)
}

func TestRecoveryReviewsFreezeEvidenceRequireSeparateFreshActorAndKeepMoneyAndWorkHeld(t *testing.T) {
	s, treasurer := recordFixture(t)
	ctx := context.Background()
	originalID := post(t, s, treasurer, received("400.00"))
	draft, err := s.CreateEntry(ctx, treasurer, received("400.01"))
	if err != nil {
		t.Fatal(err)
	}
	bindOffsite(t, s, time.Now().Add(-time.Hour))
	job, err := s.ClaimOffsite(ctx, testOffsiteBinding, "retained-original-capture", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	before := moneyRows(t, s)
	one, two, _ := currentRecoveryReviewers(t, s)
	in := recoveryAccessProposal(t, s, one)
	proposal, err := s.ProposeRecovery(ctx, one, in)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.State != "PENDING" || proposal.Version != 1 || proposal.Input.Access.Finance {
		t.Fatal("Frozen proposal invented financial entitlement", proposal)
	}
	same, err := s.ProposeRecovery(ctx, one, in)
	if err != nil || same.ID != proposal.ID {
		t.Fatal("Exact proposal retry created a second identity", err)
	}
	p1, _ := s.CheckSession(ctx, one)
	p2, _ := s.CheckSession(ctx, two)
	action := RecoveryDecisionInput{OperationKey: randomToken(), ScopeKey: p1.ScopeKey, Version: 1, Decision: "APPROVED", Reason: "Independent current source and supplied term checked by another person.", Attestation: "I independently checked the frozen proposal and its evidence.", Confirmed: true}
	if _, err = s.DecideRecovery(ctx, one, proposal.ID, action); !errors.Is(err, ErrForbidden) {
		t.Fatal("Proposer approved their own supplied access", err)
	}
	action.ScopeKey = p2.ScopeKey
	stale := action
	stale.Version = 2
	if _, err = s.DecideRecovery(ctx, two, proposal.ID, stale); !errors.Is(err, ErrConflict) {
		t.Fatal("Wrong frozen version was accepted", err)
	}
	if _, err = s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now')-301 WHERE token_hash=?", TokenHash(two)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideRecovery(ctx, two, proposal.ID, action); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("Stale password proof approved a proposal", err)
	}
	// Focused expiry injection only. Both reviewers enrolled with actual TOTP;
	// HTTP/browser separately exercise the real reauthentication controls.
	s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now') WHERE token_hash=?", TokenHash(two))
	decided, err := s.DecideRecovery(ctx, two, proposal.ID, action)
	if err != nil || decided.State != "APPROVED" || decided.Version != 2 {
		t.Fatal("Separate current review did not prepare one decision", err)
	}
	again, err := s.DecideRecovery(ctx, two, proposal.ID, action)
	if err != nil || again.ID != decided.ID || again.Version != 2 {
		t.Fatal("Lost-response decision retry duplicated its result", err)
	}
	changed := action
	changed.Reason = "Different evidence must not replace a retained review decision."
	if _, err = s.DecideRecovery(ctx, two, proposal.ID, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("Retry changed frozen decision evidence", err)
	}
	changed = action
	changed.OperationKey = randomToken()
	if _, err = s.DecideRecovery(ctx, two, proposal.ID, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("Another operation decided an already decided proposal", err)
	}
	in.OperationKey = randomToken()
	in.PredecessorID = proposal.ID
	in.PredecessorVersion = 2
	in.Access.Finance = true
	in.Access.Relationship = "FAMILY"
	successor, err := s.ProposeRecovery(ctx, one, in)
	if err != nil || successor.ID == proposal.ID || successor.Version != 3 {
		t.Fatal("Linked successor rewrote its original", err)
	}
	work := RecoveryProposalInput{OperationKey: randomToken(), ScopeKey: p1.ScopeKey, Kind: "EXTERNAL", Title: "Retain the uncertain original capture", Reason: "Independent evidence does not establish the external acknowledgement.", Work: &RecoveryWork{Kind: "OFFSITE", JobID: job.ID, OriginalState: "CAPTURING", Outcome: "RETAIN_UNKNOWN", EvidenceRef: "retained-custody-check-has-no-ack"}}
	workProposal, err := s.ProposeRecovery(ctx, one, work)
	if err != nil {
		t.Fatal(err)
	}
	action.OperationKey = randomToken()
	action.Version = 1
	if _, err = s.DecideRecovery(ctx, two, workProposal.ID, action); err != nil {
		t.Fatal(err)
	}
	summary, err := s.RecoverySummaryFor(ctx, one)
	if err != nil || summary.Pending != 1 || summary.Approved != 2 || summary.CanResume || summary.RetainedWork != 1 {
		t.Fatal("Review summary confused preparation with cutover", summary, err)
	}
	for _, table := range []string{"entries", "receipts", "receipt_counter", "entry_reversals", "role_grants", "flat_memberships", "offsite_backup_jobs", "offsite_backup_events"} {
		if before[table] != moneyRows(t, s)[table] {
			t.Fatal("Preparation altered preserved originals", table)
		}
	}
	for _, query := range []string{"UPDATE entries SET state='POSTED' WHERE id='" + draft + "'", "UPDATE receipt_counter SET next_number=next_number+1", "UPDATE offsite_backup_jobs SET state='FAILED'"} {
		if _, err = s.DB.Exec(query); !IsRecoveryRequired(err) {
			t.Fatal("Prepared review lifted an independent financial/external SQL hold", err)
		}
	}
	for _, token := range []string{one, two} {
		if _, err = s.EntryFor(ctx, token, originalID); !errors.Is(err, ErrRecoveryRequired) {
			t.Fatal("Recovery reviewer entered ordinary private finance", err)
		}
	}
	for _, query := range []string{"UPDATE recovery_proposals SET input_json='{}'", "DELETE FROM recovery_decisions", "UPDATE recovery_nominations SET valid_until=valid_until+1", "DELETE FROM recovery_reviewers", "UPDATE recovery_epoch_heads SET sequence=sequence+1"} {
		if _, err = s.DB.Exec(query); err == nil {
			t.Fatal("Retained recovery journal could be rewritten", query)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_decisions", 2)
}

func TestRecoveryCurrentEpochOverridesClockRollbackRevocationAndConcurrentDecision(t *testing.T) {
	s := fixture(t)
	one, two, nomination := currentRecoveryReviewers(t, s)
	ctx := context.Background()
	in := recoveryAccessProposal(t, s, one)
	proposal, err := s.ProposeRecovery(ctx, one, in)
	if err != nil {
		t.Fatal(err)
	}
	p2, _ := s.CheckSession(ctx, two)
	action := RecoveryDecisionInput{ScopeKey: p2.ScopeKey, Version: 1, Decision: "DECLINED", Reason: "The supplied evidence needs a separate current correction.", Attestation: "I independently reviewed and decline this frozen version.", Confirmed: true}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			current := action
			current.OperationKey = randomToken()
			_, e := s.DecideRecovery(ctx, two, proposal.ID, current)
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	passed, conflicted := 0, 0
	for e := range results {
		if e == nil {
			passed++
		} else if errors.Is(e, ErrConflict) {
			conflicted++
		} else {
			t.Fatal("Concurrent writer failed unexpectedly", e)
		}
	}
	if passed != 1 || conflicted != 1 {
		t.Fatal("Two competing operations decided one frozen proposal", passed, conflicted)
	}
	if _, err = s.QuarantineRecovery(ctx, controlledRecoveryOrigin(), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	p, err := s.CheckSession(ctx, one)
	if err != nil || p.RecoveryReviewer || p.CanReviewRecovery || p.ScopeKey == in.ScopeKey {
		t.Fatal("Rollback selected earlier current authority", p, err)
	}
	if _, err = s.RecoverySummaryFor(ctx, one); !errors.Is(err, ErrForbidden) {
		t.Fatal("Copied older nomination retained desk access", err)
	}
	if _, err = s.NominateRecovery(ctx, nomination, RecoveryPasswords{[]RecoveryPassword{{nomination.Reviewers[0].Email, recoveryPasswordOne}, {nomination.Reviewers[1].Email, recoveryPasswordTwo}}}); !errors.Is(err, ErrConflict) {
		t.Fatal("Old exact retry crossed a newer restore epoch", err)
	}
	origin, err := s.RecoveryOriginForCustody(ctx)
	if err != nil {
		t.Fatal(err)
	}
	replacement, passwords := recoveryAppointment(t, s, origin)
	replacement.OperationID = "new-epoch-current-reviewers"
	// Intentionally reuse verified existing accounts; passwords/factors remain.
	for i := range replacement.Reviewers {
		s.DB.QueryRow("SELECT id FROM users WHERE login=?", replacement.Reviewers[i].Email).Scan(&replacement.Reviewers[i].AccountID)
	}
	passwords.Passwords = nil
	if _, err = s.NominateRecovery(ctx, replacement, passwords); err != nil {
		t.Fatal(err)
	}
	fresh, p := loginTest(t, s, replacement.Reviewers[0].Email, recoveryPasswordOne)
	if !p.RecoveryReviewer {
		t.Fatal("Explicit new epoch nomination was unavailable")
	}
	revoke := RecoveryRevocation{FormatVersion: 1, OperationID: "current-authority-revoked", EpochID: origin.ID, SourceSHA256: origin.SourceSHA256, NominationID: replacement.OperationID, Reason: "Supplied custodians withdrew this current appointment.", Custodians: replacement.Custodians}
	if err = s.RevokeRecoveryNomination(ctx, revoke); err != nil {
		t.Fatal(err)
	}
	if err = s.RevokeRecoveryNomination(ctx, revoke); err != nil {
		t.Fatal("Exact revocation retry failed", err)
	}
	if _, err = s.CheckSession(ctx, fresh); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("Revoked nominee kept their earlier bearer", err)
	}
	_, p = loginTest(t, s, replacement.Reviewers[0].Email, recoveryPasswordOne)
	if p.RecoveryReviewer || p.CanReviewRecovery {
		t.Fatal("Revoked appointment resumed through password login")
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epochs", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epoch_heads", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_nomination_revocations", 1)
}
