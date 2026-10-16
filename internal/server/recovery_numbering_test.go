package server

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func numberingHTTPReviewers(t *testing.T, beforeRestore ...func(*database.Store, authClient)) (*database.Store, authClient, authClient, database.Principal, database.Principal, database.RecoveryNomination, *bytes.Buffer) {
	t.Helper()
	s, app, logs := handler(t)
	h := app.Handler()
	ctx, now := context.Background(), time.Now().Unix()
	copied := signIn(t, h, "admin@demo.society")
	for _, prepare := range beforeRestore {
		prepare(s, copied)
	}
	origin, err := s.QuarantineRecovery(ctx, database.RecoveryOrigin{FormatVersion: 1, SourceSHA256: strings.Repeat("d", 64), SourceBytes: 1691648, SourceSchema: database.SchemaVersion, SourceApplication: "0.35.0-dev", SourceCreatedAt: now - 60}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w := copied.request("GET", "/api/recovery/numbering/status", ""); w.Code != 423 {
		t.Fatal("Copied administrator entered numbering preparation", w.Code)
	}
	nomination := database.RecoveryNomination{FormatVersion: 1, OperationID: "http-numbering-current-reviewers", EpochID: origin.ID, SourceSHA256: origin.SourceSHA256, ValidFrom: now - 1, ValidUntil: now + 86400,
		Custodians: []database.RecoveryAttestation{{IdentityRef: "numbering-current-custodian-1", EvidenceRef: "independent-custodian-check-1", VerifiedAt: now - 2}, {IdentityRef: "numbering-current-custodian-2", EvidenceRef: "independent-custodian-check-2", VerifiedAt: now - 2}},
		Reviewers:  []database.RecoveryNominee{{Email: "numbering.one@example.test", Name: "Numbering Reviewer One", IdentityRef: "numbering-current-person-1", EvidenceRef: "supplied-numbering-person-check-1", VerifiedAt: now - 2}, {Email: "numbering.two@example.test", Name: "Numbering Reviewer Two", IdentityRef: "numbering-current-person-2", EvidenceRef: "supplied-numbering-person-check-2", VerifiedAt: now - 2}}}
	password := "Private-numbering-HTTP-2026!"
	if _, err = s.NominateRecovery(ctx, nomination, database.RecoveryPasswords{Passwords: []database.RecoveryPassword{{Email: nomination.Reviewers[0].Email, Password: password}, {Email: nomination.Reviewers[1].Email, Password: password}}}); err != nil {
		t.Fatal(err)
	}
	login := func(email string) (authClient, database.Principal) {
		body, _ := json.Marshal(map[string]string{"login": email, "password": password})
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/auth/login", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://127.0.0.1:8080")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var p database.Principal
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &p) != nil || !p.MFAPending || p.CanReviewRecovery {
			t.Fatal("Password-only nomination acquired numbering authority", w.Code)
		}
		client := authClient{cookie: w.Result().Cookies()[0], csrf: p.CSRF, handler: h}
		if response := client.request("GET", "/api/recovery/numbering/policies", ""); response.Code != 403 {
			t.Fatal("A password-only nominee read numbering policies", response.Code)
		}
		var setup database.MFASetup
		response := client.request("POST", "/api/auth/mfa/setup", `{}`)
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &setup) != nil {
			t.Fatal("Current nominee could not enrol a real factor", response.Code)
		}
		secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
		if err != nil {
			t.Fatal(err)
		}
		body, _ = json.Marshal(map[string]string{"code": security.Code(secret, time.Now().Unix()/30, 6)})
		response = client.request("POST", "/api/auth/mfa/confirm", string(body))
		var verified database.MFAVerified
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &verified) != nil || !verified.User.CanReviewRecovery || !verified.User.Fresh {
			t.Fatal("Real factor did not establish current narrow review", response.Code)
		}
		return client, verified.User
	}
	one, p1 := login(nomination.Reviewers[0].Email)
	two, p2 := login(nomination.Reviewers[1].Email)
	return s, one, two, p1, p2, nomination, logs
}

func httpNumberingInput(p database.Principal) database.RecoveryNumberingInput {
	return database.RecoveryNumberingInput{OperationKey: "http-numbering-frozen-preparation-17", ScopeKey: p.ScopeKey, Title: "Supplied independent receipt numbering", Reason: "Current supplied numbering and separate custody evidence checked.", Policy: database.RecoveryNumberingSpec{Prefix: "TRV-HTTP2026A", Digits: 6, First: 42, Last: 5000, Period: "Supplied fictional period 2026–27", AuthorityRef: "private-independent-numbering-evidence-17", NamespaceRef: "fictional-independent-series-namespace", VerifiedAt: time.Now().Add(-time.Second).Unix()}}
}

func TestRecoveryNumberingHTTPRequiresExactSeparateCurrentFreshActorAndCSRF(t *testing.T) {
	s, one, two, p1, p2, nomination, logs := numberingHTTPReviewers(t)
	in := httpNumberingInput(p1)
	body, _ := json.Marshal(in)
	csrf := one.csrf
	one.csrf = ""
	if w := one.request("POST", "/api/recovery/numbering/policies", string(body)); w.Code != 403 {
		t.Fatal("Numbering write lacked CSRF", w.Code)
	}
	one.csrf = csrf
	if w := one.request("POST", "/api/recovery/numbering/policies", `{"unsupported_private_input":"must-not-be-retained"}`); w.Code != 400 {
		t.Fatal("Numbering API accepted unsupported custody inputs", w.Code)
	}
	var policy database.RecoveryNumberingPolicy
	for range 2 {
		w := one.request("POST", "/api/recovery/numbering/policies", string(body))
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &policy) != nil || policy.FirstExample != "TRV-HTTP2026A-000042" || policy.LastExample != "TRV-HTTP2026A-005000" || policy.State != "PENDING" {
			t.Fatal("Frozen policy retry changed supplied numbering", w.Code)
		}
	}
	if w := two.request("GET", "/api/recovery/numbering/operations/"+in.OperationKey, ""); w.Code != 404 {
		t.Fatal("Another actor resolved a private retained operation", w.Code)
	}
	decision := database.RecoveryDecisionInput{OperationKey: "http-numbering-separate-decision-17", ScopeKey: p1.ScopeKey, Version: 1, Decision: "APPROVED", Reason: "A separate person checked the supplied format and independent custody evidence.", Attestation: "I checked this frozen supplied version independently.", Confirmed: true}
	body, _ = json.Marshal(decision)
	path := "/api/recovery/numbering/policies/" + policy.ID + "/decision"
	if w := one.request("POST", path, string(body)); w.Code != 403 {
		t.Fatal("Author approved their own receipt numbering", w.Code)
	}
	decision.ScopeKey = p2.ScopeKey
	decision.Version = 2
	body, _ = json.Marshal(decision)
	if w := two.request("POST", path, string(body)); w.Code != 409 {
		t.Fatal("A different frozen policy version was approved", w.Code)
	}
	decision.Version = 1
	body, _ = json.Marshal(decision)
	if _, err := s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now')-301 WHERE user_id=?", p2.ID); err != nil {
		t.Fatal(err)
	}
	if w := two.request("POST", path, string(body)); w.Code != 403 || !strings.Contains(w.Body.String(), "reauthentication_required") {
		t.Fatal("Expired freshness approved numbering", w.Code)
	}
	if _, err := s.DB.Exec("UPDATE sessions SET reauthenticated_at=unixepoch('now') WHERE user_id=?", p2.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if w := two.request("POST", path, string(body)); w.Code != 200 {
			t.Fatal("Exact second-person decision retry failed", w.Code)
		}
	}
	for _, path := range []string{"/api/recovery/numbering/status", "/api/recovery/numbering/policies?page=1&state=APPROVED", "/api/recovery/numbering/policies/" + policy.ID, "/api/recovery/numbering/policies/" + policy.ID + "/history?page=1", "/api/recovery/numbering/operations/" + decision.OperationKey} {
		if w := two.request("GET", path, ""); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("Current numbering result/history was inaccessible or cached", path, w.Code)
		}
	}
	for _, query := range []string{"?page=zero", "?page=10001", "?state=UNKNOWN"} {
		if w := one.request("GET", "/api/recovery/numbering/policies"+query, ""); w.Code != 400 {
			t.Fatal("Unbounded numbering query was accepted", query, w.Code)
		}
	}
	var count int
	if err := s.DB.QueryRow("SELECT (SELECT COUNT(*) FROM recovery_numbering_policies)+(SELECT COUNT(*) FROM recovery_numbering_decisions)").Scan(&count); err != nil || count != 2 {
		t.Fatal("Retried HTTP writes duplicated retained decisions", count, err)
	}
	for _, secret := range []string{"Private-numbering-HTTP-2026!", one.cookie.Value, two.cookie.Value, p1.CSRF, p2.CSRF, in.Policy.AuthorityRef, nomination.Custodians[0].EvidenceRef} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("Private numbering data entered request logs")
		}
	}
	revoke := database.RecoveryRevocation{FormatVersion: 1, OperationID: "http-numbering-revocation-17", EpochID: nomination.EpochID, SourceSHA256: nomination.SourceSHA256, NominationID: nomination.OperationID, Custodians: nomination.Custodians, Reason: "Independent current custodians ended this supplied review appointment."}
	if err := s.RevokeRecoveryNomination(context.Background(), revoke); err != nil {
		t.Fatal(err)
	}
	for _, actor := range []authClient{one, two} {
		if w := actor.request("GET", "/api/recovery/numbering/policies", ""); w.Code != 401 {
			t.Fatal("Revoked nominee retained numbering access", w.Code)
		}
	}
}

func TestRecoveryNumberingHTTPProgressIsBoundedWithoutPrivateStorageOrMutationRoutes(t *testing.T) {
	s, one, two, p1, p2, nomination, _ := numberingHTTPReviewers(t)
	ctx := context.Background()
	in := httpNumberingInput(p1)
	policy, err := s.ProposeRecoveryNumbering(ctx, one.cookie.Value, in)
	if err != nil {
		t.Fatal(err)
	}
	policy, err = s.DecideRecoveryNumbering(ctx, two.cookie.Value, policy.ID, database.RecoveryDecisionInput{OperationKey: "http-private-storage-decision-17", ScopeKey: p2.ScopeKey, Version: 1, Decision: "APPROVED", Reason: "Another person checked the exact supplied series and custody evidence.", Attestation: "I checked these independent supplied bounds.", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	request := database.RecoverySeriesRequest{FormatVersion: 1, OperationID: "http-private-storage-intent-17", AttemptKey: "http-private-storage-handoff-17", EpochID: nomination.EpochID, SourceSHA256: nomination.SourceSHA256, NominationID: nomination.OperationID, PolicyID: policy.ID, PolicyVersion: 2, Custodians: nomination.Custodians}
	// A deterministic store-level fake proves DTO secrecy only. The separate
	// backup and browser tests exercise real age encryption and signed storage.
	intent, err := s.RetainRecoverySeries(ctx, request, strings.Repeat("b", 64), "private-custody/series", func([]byte) ([]byte, error) { return bytes.Repeat([]byte("private-original-ciphertext"), 20), nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.StartRecoverySeriesHandoff(ctx, intent.OperationID, request.AttemptKey, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.FinishRecoverySeriesHandoff(ctx, intent.OperationID, request.AttemptKey, "VERIFIED", "STORAGE_CONFIRMED", "private-independent-object-version"); err != nil {
		t.Fatal(err)
	}
	w := one.request("GET", "/api/recovery/numbering/status", "")
	var status map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &status) != nil || len(status) != 6 || status["verified"] != float64(1) || status["approved"] != float64(1) || status["can_resume"] != false {
		t.Fatal("Bounded verification status fabricated release or exposed private fields", w.Code, status)
	}
	for _, path := range []string{"/api/recovery/numbering/policies", "/api/recovery/numbering/policies/" + policy.ID, "/api/recovery/numbering/policies/" + policy.ID + "/history"} {
		w := one.request("GET", path, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"checksum_verified":true`) {
			t.Fatal("Public reservation progress was unavailable", w.Code)
		}
		for _, private := range []string{intent.ConfigHash, intent.ObjectKey, intent.Version, "private-independent-object-version", "private-original-ciphertext", `"ciphertext"`, `"manifest_json"`, `"object_version"`, `"config_hash"`} {
			if private != "" && strings.Contains(w.Body.String(), private) {
				t.Fatal("Public progress exposed private storage/custody material", private)
			}
		}
	}
	for _, path := range []string{"/api/recovery/numbering/reserve", "/api/recovery/numbering/reconcile", "/api/recovery/numbering/retry", "/api/recovery/numbering/release", "/api/recovery/numbering/policies/" + policy.ID + "/reserve"} {
		// The shared GET fallback makes unsupported POSTs method-not-allowed.
		if w := one.request("POST", path, `{}`); w.Code != 405 {
			t.Fatal("A public route crossed private custody or cutover", path, w.Code)
		}
	}
	for _, path := range []string{"/api/entries", "/api/receipts/" + strings.Repeat("A", 43) + "/download", "/api/admin/accounts", "/api/messages"} {
		if w := one.request("GET", path, ""); w.Code != 423 {
			t.Fatal("Verified series preparation released ordinary private actions", path, w.Code)
		}
	}
}
