package server

import (
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

func TestRecoveryReviewHTTPRequiresCurrentNominationRealMFASeparateActorAndCSRF(t *testing.T) {
	s, app, logs := handler(t)
	h := app.Handler()
	ctx := context.Background()
	copied := signIn(t, h, "admin@demo.society")
	now := time.Now().Unix()
	origin, err := s.QuarantineRecovery(ctx, database.RecoveryOrigin{FormatVersion: 1, SourceSHA256: strings.Repeat("a", 64), SourceBytes: 1585152, SourceSchema: database.SchemaVersion, SourceApplication: "0.33.0-dev", SourceCreatedAt: now - 60}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w := copied.request("GET", "/api/recovery/status", ""); w.Code != 423 {
		t.Fatal("Copied admin entered the current review desk", w.Code)
	}
	nomination := database.RecoveryNomination{FormatVersion: 1, OperationID: "http-independent-current-reviewers", EpochID: origin.ID, SourceSHA256: origin.SourceSHA256, ValidFrom: now - 1, ValidUntil: now + 86400,
		Custodians: []database.RecoveryAttestation{{IdentityRef: "supplied-current-custodian-1", EvidenceRef: "independent-current-authority-1", VerifiedAt: now - 2}, {IdentityRef: "supplied-current-custodian-2", EvidenceRef: "independent-current-authority-2", VerifiedAt: now - 2}},
		Reviewers:  []database.RecoveryNominee{{Email: "http.one@example.test", Name: "HTTP Current Reviewer One", IdentityRef: "separate-current-person-1", EvidenceRef: "supplied-current-person-check-1", VerifiedAt: now - 2}, {Email: "http.two@example.test", Name: "HTTP Current Reviewer Two", IdentityRef: "separate-current-person-2", EvidenceRef: "supplied-current-person-check-2", VerifiedAt: now - 2}}}
	password := "Private-HTTP-reviewer-2026!"
	if _, err = s.NominateRecovery(ctx, nomination, database.RecoveryPasswords{Passwords: []database.RecoveryPassword{{Email: nomination.Reviewers[0].Email, Password: password}, {Email: nomination.Reviewers[1].Email, Password: password}}}); err != nil {
		t.Fatal(err)
	}
	login := func(email string) (authClient, database.Principal) {
		body, _ := json.Marshal(map[string]string{"login": email, "password": password})
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/auth/login", strings.NewReader(string(body)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://127.0.0.1:8080")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var p database.Principal
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &p) != nil {
			t.Fatal("Current reviewer password login failed", w.Code)
		}
		client := authClient{cookie: w.Result().Cookies()[0], csrf: p.CSRF, handler: h}
		if !p.RecoveryReviewer || !p.MFAPending || p.CanReviewRecovery || len(p.Roles) != 0 {
			t.Fatal("Password appointment opened private/fresh authority", p)
		}
		if w := client.request("GET", "/api/recovery/status", ""); w.Code != 403 {
			t.Fatal("MFA-pending nominee entered the desk", w.Code)
		}
		if w := client.request("POST", "/api/auth/mfa/demo-code", `{}`); w.Code != 423 {
			t.Fatal("Nominee used a fictional factor bypass", w.Code)
		}
		setupResponse := client.request("POST", "/api/auth/mfa/setup", `{}`)
		var setup database.MFASetup
		if setupResponse.Code != 200 || json.Unmarshal(setupResponse.Body.Bytes(), &setup) != nil {
			t.Fatal("Narrow real factor setup failed", setupResponse.Code)
		}
		secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
		if err != nil {
			t.Fatal(err)
		}
		body, _ = json.Marshal(map[string]string{"code": security.Code(secret, time.Now().Unix()/30, 6)})
		confirmed := client.request("POST", "/api/auth/mfa/confirm", string(body))
		var result database.MFAVerified
		if confirmed.Code != 200 || json.Unmarshal(confirmed.Body.Bytes(), &result) != nil || !result.User.CanReviewRecovery || !result.User.Fresh {
			t.Fatal("Actual factor did not open only the recovery desk", confirmed.Code)
		}
		return client, result.User
	}
	one, p1 := login(nomination.Reviewers[0].Email)
	two, p2 := login(nomination.Reviewers[1].Email)
	for _, actor := range []authClient{one, two} {
		for _, path := range []string{"/api/flats", "/api/entries", "/api/admin/accounts", "/api/documents", "/api/messages", "/api/financial-statements"} {
			if w := actor.request("GET", path, ""); w.Code != 423 {
				t.Fatal("Narrow reviewer entered ordinary private records", path, w.Code)
			}
		}
		for _, path := range []string{"/api/recovery/status", "/api/recovery/accounts", "/api/recovery/work", "/api/recovery/homes", "/api/recovery/proposals"} {
			if w := actor.request("GET", path, ""); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("Narrow current review metadata was unavailable or cached", path, w.Code)
			}
		}
	}
	proposal := database.RecoveryProposalInput{OperationKey: "http-frozen-operation-17", ScopeKey: p1.ScopeKey, Kind: "ACCESS", Title: "Current supplied committee appointment", Reason: "Current independent society records checked for this supplied term.", Access: &database.RecoveryAccess{Email: "current.member@example.test", Name: "Independently Supplied Member", IdentityRef: "supplied-member-current-17", Role: "COMMITTEE", ValidFrom: now - 1, ValidUntil: now + 90*86400, VerifiedAt: now - 2, EvidenceRef: "supplied-current-committee-evidence-17"}}
	body, _ := json.Marshal(proposal)
	csrf := one.csrf
	one.csrf = ""
	if w := one.request("POST", "/api/recovery/proposals", string(body)); w.Code != 403 {
		t.Fatal("Current recovery write lacked CSRF", w.Code)
	}
	one.csrf = csrf
	w := one.request("POST", "/api/recovery/proposals", string(body))
	var saved database.RecoveryProposal
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &saved) != nil {
		t.Fatal("Frozen proposal was unavailable", w.Code, w.Body.String())
	}
	if w := two.request("GET", "/api/recovery/operations/"+proposal.OperationKey, ""); w.Code != 404 {
		t.Fatal("Other actor resolved someone else's write identity", w.Code)
	}
	decision := database.RecoveryDecisionInput{OperationKey: "http-separate-decision-17", ScopeKey: p1.ScopeKey, Version: 1, Decision: "APPROVED", Reason: "Another current person independently checked the supplied exact version.", Attestation: "I independently checked the frozen version and its evidence.", Confirmed: true}
	body, _ = json.Marshal(decision)
	if w := one.request("POST", "/api/recovery/proposals/"+saved.ID+"/decision", string(body)); w.Code != 403 {
		t.Fatal("Proposer self-approved their current appointment", w.Code)
	}
	decision.ScopeKey = p2.ScopeKey
	body, _ = json.Marshal(decision)
	for range 2 {
		if w := two.request("POST", "/api/recovery/proposals/"+saved.ID+"/decision", string(body)); w.Code != 200 {
			t.Fatal("Separate current decision/retry failed", w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/api/recovery/operations/" + decision.OperationKey, "/api/recovery/proposals/" + saved.ID + "/history?page=1"} {
		if w := two.request("GET", path, ""); w.Code != 200 {
			t.Fatal("Current saved result/history unavailable", path, w.Code)
		}
	}
	var count int
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM recovery_decisions").Scan(&count); err != nil || count != 1 {
		t.Fatal("HTTP lost-response retry duplicated a decision", err, count)
	}
	for _, secret := range []string{password, one.cookie.Value, two.cookie.Value, p1.CSRF, p2.CSRF, "supplied-current-committee-evidence-17"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("Private recovery data entered request logs")
		}
	}
	if w := one.request("POST", "/api/auth/logout", `{}`); w.Code != 200 {
		t.Fatal("Current nominee could not sign out", w.Code)
	}
	if w := one.request("GET", "/api/recovery/status", ""); w.Code != 401 {
		t.Fatal("Signed-out reviewer retained their bearer", w.Code)
	}
}
