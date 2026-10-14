package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"society.local/portal/internal/testfixtures"
)

const seriesTestPassword = "Private-fictional-series-review-2026!"

type seriesFixture struct {
	store                *database.Store
	root, writer, reader string
	peer                 *testfixtures.S3
	nomination           database.RecoveryNomination
	policy               database.RecoveryNumberingPolicy
	request              database.RecoverySeriesRequest
	identities           []*age.X25519Identity
	secrets              [2]string
}

func seriesPersonalLogin(t *testing.T, s *database.Store, email string, retainedSecret string) (string, string) {
	t.Helper()
	ctx := context.Background()
	token, p, err := s.Login(ctx, email, seriesTestPassword, database.HashPassword("unmatched-dummy-series-password"))
	if err != nil || !p.RecoveryReviewer || !p.MFAPending {
		t.Fatal("Password must not establish recovery authority", err)
	}
	if retainedSecret != "" {
		secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(retainedSecret)
		if err != nil {
			t.Fatal(err)
		}
		var lastStep int64
		if err := s.DB.QueryRow("SELECT last_step FROM mfa_factors WHERE user_id=(SELECT id FROM users WHERE login=?)", email).Scan(&lastStep); err != nil {
			t.Fatal(err)
		}
		if lastStep >= time.Now().Unix()/30 {
			timer := time.NewTimer(time.Until(time.Unix((lastStep+1)*30+1, 0)))
			defer timer.Stop()
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-timer.C:
			}
		}
		result, err := s.VerifyMFA(ctx, token, security.Code(secret, time.Now().Unix()/30, 6), false)
		if err != nil || !result.CanReviewRecovery || !result.Fresh {
			t.Fatal("Actual new authenticator period failed", err)
		}
		return token, retainedSecret
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
	if err != nil || !result.User.CanReviewRecovery || !result.User.Fresh || result.User.IsDemo {
		t.Fatal("Real personal authenticator did not establish narrow review", err)
	}
	return token, setup.Secret
}

func seriesPolicy(t *testing.T, s *database.Store, one, two string) database.RecoveryNumberingPolicy {
	t.Helper()
	ctx := context.Background()
	p1, err := s.CheckSession(ctx, one)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.CheckSession(ctx, two)
	if err != nil {
		t.Fatal(err)
	}
	in := database.RecoveryNumberingInput{OperationKey: "fictional-numbering-preparation-0001", ScopeKey: p1.ScopeKey, Title: "Supplied fictional recovery receipt series", Reason: "The independent fictional numbering and custody records were checked.", Policy: database.RecoveryNumberingSpec{Prefix: "TRV-R2026A", Digits: 6, First: 1, Last: 999999, Period: "Supplied fictional period 2026–27", AuthorityRef: "fictional-current-numbering-evidence", NamespaceRef: "fictional-retained-series-namespace", VerifiedAt: time.Now().Add(-time.Second).Unix()}}
	policy, err := s.ProposeRecoveryNumbering(ctx, one, in)
	if err != nil {
		t.Fatal(err)
	}
	policy, err = s.DecideRecoveryNumbering(ctx, two, policy.ID, database.RecoveryDecisionInput{OperationKey: "fictional-numbering-second-person-0001", ScopeKey: p2.ScopeKey, Version: 1, Decision: "APPROVED", Reason: "Another current person independently checked these exact bounds.", Attestation: "I checked the exact supplied version and its current evidence.", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func configuredSeriesFixture(t *testing.T, mode string) *seriesFixture {
	t.Helper()
	ctx := context.Background()
	s, root := setup(t)
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	now := time.Now().Unix()
	origin, err := s.QuarantineRecovery(ctx, database.RecoveryOrigin{FormatVersion: 1, SourceSHA256: strings.Repeat("a", 64), SourceBytes: 100000, SourceSchema: database.SchemaVersion, SourceApplication: "0.34.0-dev", SourceCreatedAt: now - 60}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	n := database.RecoveryNomination{FormatVersion: 1, OperationID: "supplied-current-series-reviewers", EpochID: origin.ID, SourceSHA256: origin.SourceSHA256, ValidFrom: now - 1, ValidUntil: now + 86400,
		Custodians: []database.RecoveryAttestation{{IdentityRef: "independent-series-custodian-1", EvidenceRef: "fictional-no-delete-namespace-check-1", VerifiedAt: now - 1}, {IdentityRef: "independent-series-custodian-2", EvidenceRef: "fictional-no-delete-namespace-check-2", VerifiedAt: now - 1}},
		Reviewers:  []database.RecoveryNominee{{Email: "series.one@example.test", Name: "Supplied Series Reviewer One", IdentityRef: "independent-series-reviewer-1", EvidenceRef: "fictional-person-check-1", VerifiedAt: now - 1}, {Email: "series.two@example.test", Name: "Supplied Series Reviewer Two", IdentityRef: "independent-series-reviewer-2", EvidenceRef: "fictional-person-check-2", VerifiedAt: now - 1}}}
	if _, err := s.NominateRecovery(ctx, n, database.RecoveryPasswords{Passwords: []database.RecoveryPassword{{Email: n.Reviewers[0].Email, Password: seriesTestPassword}, {Email: n.Reviewers[1].Email, Password: seriesTestPassword}}}); err != nil {
		t.Fatal(err)
	}
	one, secretOne := seriesPersonalLogin(t, s, n.Reviewers[0].Email, "")
	two, secretTwo := seriesPersonalLogin(t, s, n.Reviewers[1].Email, "")
	policy := seriesPolicy(t, s, one, two)
	peer := testfixtures.NewS3(t)
	peer.SetMode(mode)
	_, identities := custodyPair(t)
	public := []string{identities[0].Recipient().String(), identities[1].Recipient().String()}
	paths := make(map[string]string)
	for _, role := range []string{"writer", "reader"} {
		path := filepath.Join(root, role+"-series-config.json")
		body, _ := json.Marshal(OffsiteConfig{FormatVersion: 1, Storage: peer.Config(role), Recipients: public})
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		paths[role] = path
	}
	request := database.RecoverySeriesRequest{FormatVersion: 1, OperationID: "fictional-series-reservation-0001", AttemptKey: "fictional-series-first-handoff-0001", EpochID: origin.ID, SourceSHA256: origin.SourceSHA256, NominationID: n.OperationID, PolicyID: policy.ID, PolicyVersion: policy.Version, Custodians: n.Custodians}
	return &seriesFixture{store: s, root: root, writer: paths["writer"], reader: paths["reader"], peer: peer, nomination: n, policy: policy, request: request, identities: identities, secrets: [2]string{secretOne, secretTwo}}
}

func TestRecoverySeriesRealEncryptionSignedCreateOnlyAndEitherCustodian(t *testing.T) {
	f := configuredSeriesFixture(t, "")
	ctx := context.Background()
	result, err := ReserveRecoverySeries(ctx, f.store, f.writer, true, f.request)
	if err != nil || result.State != "VERIFIED" || result.Attempts != 1 {
		t.Fatal("One acknowledged original was not retained", result.State, err)
	}
	objects := f.peer.Objects()
	if len(objects) != 1 {
		t.Fatal("Unexpected independent reservation objects", len(objects))
	}
	object := objects[0]
	societyHash := sha256.Sum256([]byte("fictional-demo-housing_demo_v1"))
	prefixHash := sha256.Sum256([]byte("TRV-R2026A"))
	wantKey := "recovery/society/receipt-series/" + hex.EncodeToString(societyHash[:]) + "/" + hex.EncodeToString(prefixHash[:]) + ".age"
	if object.Key != wantKey {
		t.Fatal("The collision key depends on a restored/random identity", object.Key)
	}
	sum := sha256.Sum256(object.Bytes)
	if hex.EncodeToString(sum[:]) != result.CipherSHA256 || !bytes.Equal(object.Bytes, result.Ciphertext) || object.Version != result.Version {
		t.Fatal("Independent bytes/checksum/version differ from retained original")
	}
	for _, identity := range f.identities {
		reader, err := age.Decrypt(bytes.NewReader(object.Bytes), identity)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		var manifest map[string]any
		if json.Unmarshal(body, &manifest) != nil {
			t.Fatal("Custodied manifest is not valid JSON")
		}
		request := manifest["request"].(map[string]any)
		if manifest["society_key"] != "fictional-demo-housing_demo_v1" || request["epoch_id"] != f.request.EpochID || request["policy_id"] != f.policy.ID || request["operation_id"] != f.request.OperationID {
			t.Fatal("Encrypted original lost its exact supplied source/policy/operation")
		}
		if _, err := ReconcileRecoverySeries(ctx, f.store, f.reader, true, result.OperationID, object.Version, []age.Identity{identity}); err != nil {
			t.Fatal("Either independent custodian must decrypt the same original", err)
		}
	}
	again, err := ReserveRecoverySeries(ctx, f.store, f.writer, true, f.request)
	if err != nil || !bytes.Equal(again.Ciphertext, result.Ciphertext) {
		t.Fatal("Exact acknowledged retry changed ciphertext", err)
	}
	puts, gets := f.peer.Counts()
	if puts != 1 || gets != 2 {
		t.Fatal("Exact retries or independent retrieval performed unexpected requests", puts, gets)
	}
	if required, err := f.store.RecoveryRequired(ctx); err != nil || !required {
		t.Fatal("Reservation released recovery", err)
	}
}

func TestRecoverySeriesLostReplyRestartNeverResendsAndRequiresExactVersionDecrypt(t *testing.T) {
	f := configuredSeriesFixture(t, "LOST_REPLY")
	ctx := context.Background()
	unknown, err := ReserveRecoverySeries(ctx, f.store, f.writer, true, f.request)
	if err != nil || unknown.State != "UNKNOWN" {
		t.Fatal("Lost actual HTTP reply fabricated acknowledgement", unknown.State, err)
	}
	objects := f.peer.Objects()
	if len(objects) != 1 {
		t.Fatal("Lost reply must still retain the actual object")
	}
	original := objects[0]
	reopened, err := database.Open(ctx, f.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	f.peer.SetMode("")
	again, err := ReserveRecoverySeries(ctx, reopened, f.writer, true, f.request)
	if err != nil || again.State != "UNKNOWN" || !bytes.Equal(again.Ciphertext, unknown.Ciphertext) {
		t.Fatal("Restart changed uncertainty/original", err)
	}
	retry := database.RecoverySeriesRetry{FormatVersion: 1, OperationID: f.request.OperationID, AttemptKey: "fictional-uncertain-forbidden-retry", Custodians: f.nomination.Custodians}
	if _, err := RetryRecoverySeries(ctx, reopened, f.writer, true, retry); !errors.Is(err, database.ErrConflict) {
		t.Fatal("Unknown network handoff was resent", err)
	}
	if puts, _ := f.peer.Counts(); puts != 1 {
		t.Fatal("Uncertain retries performed another PUT", puts)
	}
	wrong, _ := age.GenerateX25519Identity()
	if _, err := ReconcileRecoverySeries(ctx, reopened, f.reader, true, f.request.OperationID, original.Version, []age.Identity{wrong}); err == nil {
		t.Fatal("Wrong custody identity adopted the reservation")
	}
	for _, mode := range []string{"CORRUPT_GET", "WRONG_GET_VERSION"} {
		f.peer.SetMode(mode)
		if _, err := ReconcileRecoverySeries(ctx, reopened, f.reader, true, f.request.OperationID, original.Version, []age.Identity{f.identities[0]}); err == nil {
			t.Fatal("Mismatched independent retrieval accepted", mode)
		}
	}
	f.peer.SetMode("")
	verified, err := ReconcileRecoverySeries(ctx, reopened, f.reader, true, f.request.OperationID, original.Version, []age.Identity{f.identities[1]})
	if err != nil || verified.State != "VERIFIED" || verified.Version != original.Version || !bytes.Equal(verified.Ciphertext, unknown.Ciphertext) {
		t.Fatal("Exact independent custody did not resolve one original", err)
	}
	if puts, _ := f.peer.Counts(); puts != 1 {
		t.Fatal("Custody reconciliation uploaded instead of retrieving", puts)
	}
}

func TestRecoverySeriesOlderOperationalRestoreCannotReuseIndependentPrefix(t *testing.T) {
	f := configuredSeriesFixture(t, "")
	ctx := context.Background()
	bundle := filepath.Join(f.root, "before-series-handoff")
	if _, err := Snapshot(ctx, f.store, bundle, "0.34.0-dev"); err != nil {
		t.Fatal(err)
	}
	first, err := ReserveRecoverySeries(ctx, f.store, f.writer, true, f.request)
	if err != nil || first.State != "VERIFIED" {
		t.Fatal(err)
	}
	object := f.peer.Objects()[0]
	destination := filepath.Join(f.root, "older-restored", "society.db")
	if _, err := Restore(ctx, bundle, destination); err != nil {
		t.Fatal(err)
	}
	restored, err := database.Open(ctx, destination)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restored.MFA = f.store.MFA
	var copiedCodes int
	if err := restored.DB.QueryRow("SELECT COUNT(*) FROM mfa_recovery_codes").Scan(&copiedCodes); err != nil || copiedCodes != 0 {
		t.Fatal("Operational restore retained copied recovery codes", copiedCodes, err)
	}
	origin, err := restored.RecoveryOriginForCustody(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := f.nomination
	n.OperationID = "supplied-new-host-current-reviewers"
	n.EpochID = origin.ID
	n.SourceSHA256 = origin.SourceSHA256
	n.Reviewers = append([]database.RecoveryNominee(nil), n.Reviewers...)
	for i := range n.Reviewers {
		if err := restored.DB.QueryRow("SELECT id FROM users WHERE login=?", n.Reviewers[i].Email).Scan(&n.Reviewers[i].AccountID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := restored.NominateRecovery(ctx, n, database.RecoveryPasswords{Passwords: []database.RecoveryPassword{}}); err != nil {
		t.Fatal(err)
	}
	one, _ := seriesPersonalLogin(t, restored, n.Reviewers[0].Email, f.secrets[0])
	two, _ := seriesPersonalLogin(t, restored, n.Reviewers[1].Email, f.secrets[1])
	p1, _ := restored.CheckSession(ctx, one)
	p2, _ := restored.CheckSession(ctx, two)
	in := f.policy.Input
	in.OperationKey = "fictional-older-host-new-policy-0001"
	in.ScopeKey = p1.ScopeKey
	policy, err := restored.ProposeRecoveryNumbering(ctx, one, in)
	if err != nil {
		t.Fatal(err)
	}
	policy, err = restored.DecideRecoveryNumbering(ctx, two, policy.ID, database.RecoveryDecisionInput{OperationKey: "fictional-older-host-separate-check-0001", ScopeKey: p2.ScopeKey, Version: 1, Decision: "APPROVED", Reason: "Current independent people checked this restored host's exact policy.", Attestation: "I independently checked this supplied version and its evidence.", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	request := f.request
	request.OperationID = "fictional-older-host-series-attempt-0001"
	request.AttemptKey = "fictional-older-host-first-handoff-0001"
	request.EpochID = origin.ID
	request.SourceSHA256 = origin.SourceSHA256
	request.NominationID = n.OperationID
	request.PolicyID = policy.ID
	request.PolicyVersion = policy.Version
	conflict, err := ReserveRecoverySeries(ctx, restored, f.writer, true, request)
	if err != nil || conflict.State != "UNKNOWN" || conflict.ObjectKey != first.ObjectKey || conflict.CipherSHA256 == first.CipherSHA256 {
		t.Fatal("Older counter/source reused or concealed the independent prefix", conflict.State, err)
	}
	if _, err := ReconcileRecoverySeries(ctx, restored, f.reader, true, request.OperationID, object.Version, []age.Identity{f.identities[0]}); err == nil {
		t.Fatal("Older host adopted another request's actual reservation")
	}
	after := f.peer.Objects()
	if len(after) != 1 || after[0].Version != object.Version || !bytes.Equal(after[0].Bytes, object.Bytes) {
		t.Fatal("The independently retained original was overwritten")
	}
	puts, _ := f.peer.Counts()
	if puts != 2 {
		t.Fatal("Conflict request count differs", puts)
	}
	if required, err := restored.RecoveryRequired(ctx); err != nil || !required {
		t.Fatal("Conflicting series released quarantine", err)
	}
}
