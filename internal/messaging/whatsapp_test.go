package messaging

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"society.local/portal/internal/database"
)

func whatsappTestConfig(origin string) WhatsAppConfig {
	return WhatsAppConfig{Origin: origin, APIVersion: "v26.0", AccountID: "111111", PhoneID: "222222", MessagingAccountID: "333333", AccessToken: "FICTIONAL_ACCESS_TOKEN_123456789", AppSecret: "FICTIONAL_APPLICATION_SECRET_123456789", VerifyToken: "FICTIONAL_VERIFICATION_TOKEN_123456789", Templates: map[string]WhatsAppTemplate{"NOTICE": {ID: "444444", Name: "society_notice", Language: "en"}}}
}

func whatsappTestSignature(body []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWhatsAppOfficialTransportFreezesApprovedComponentsAndExactRequest(t *testing.T) {
	var requestBody map[string]any
	posts := 0
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer FICTIONAL_ACCESS_TOKEN_123456789" {
			t.Error("credential missing from server-side header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /v26.0/444444":
			if r.URL.Query().Get("fields") != "id,name,language,status,category,components" {
				t.Error("actual content fields not requested")
			}
			fmt.Fprint(w, `{"id":"444444","name":"society_notice","language":"en","status":"APPROVED","category":"UTILITY","components":[{"type":"HEADER","format":"TEXT","text":"Your society"},{"type":"BODY","text":"An update is available.\n{{1}}"},{"type":"FOOTER","text":"Open with your portal account"}]}`)
		case "POST /v26.0/222222/messages":
			posts++
			if json.NewDecoder(r.Body).Decode(&requestBody) != nil {
				t.Error("invalid send body")
			}
			fmt.Fprint(w, `{"messaging_product":"whatsapp","contacts":[{"input":"+919000000101","wa_id":"919000000101"}],"messages":[{"id":"wamid.FICTIONAL_EXACT_001"}]}`)
		default:
			t.Error("unexpected endpoint", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer fixture.Close()
	client, err := NewWhatsAppClient(whatsappTestConfig(fixture.URL), true)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := client.ReadTemplate(context.Background(), "NOTICE")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Body != "An update is available.\n{{1}}" || provider.Header != "Your society" || provider.Footer != "Open with your portal account" || provider.Mode != "CLOUD_FIXTURE" {
		t.Fatal(provider)
	}
	result := client.Send(context.Background(), database.WhatsAppAttempt{ID: "attempt-fixture-exact-12345", Destination: "+919000000101", Link: "http://127.0.0.1:8080/#community?notice=original-123", Provider: provider})
	if result.State != "ACCEPTED" || result.ProviderID != "wamid.FICTIONAL_EXACT_001" || posts != 1 {
		t.Fatal(result, posts)
	}
	var expected map[string]any
	json.Unmarshal([]byte(`{"messaging_product":"whatsapp","recipient_type":"individual","to":"+919000000101","type":"template","messaging_account_id":"333333","biz_opaque_callback_data":"attempt-fixture-exact-12345","template":{"name":"society_notice","language":{"code":"en"},"components":[{"type":"body","parameters":[{"type":"text","text":"http://127.0.0.1:8080/#community?notice=original-123"}]}]}}`), &expected)
	if !reflect.DeepEqual(expected, requestBody) {
		t.Fatal("unexpected request", requestBody)
	}
	if strings.Contains(fmt.Sprint(requestBody), "ACCESS_TOKEN") || strings.Contains(fmt.Sprint(requestBody), "APPLICATION_SECRET") {
		t.Fatal("credential included in message")
	}
}

func TestWhatsAppTransportDefiniteRejectsAndUncertainResponsesDoNotInventDelivery(t *testing.T) {
	for _, tc := range []struct {
		name                                    string
		code                                    int
		body, contentType, state, reason, retry string
	}{
		{"accepted", 200, `{"messaging_product":"whatsapp","messages":[{"id":"wamid.fixture"}]}`, "application/json", "ACCEPTED", "WHATSAPP_ACCEPTED", ""},
		{"rejected", 400, `{"error":{"code":131026,"message":"PRIVATE_PROVIDER_ERROR"}}`, "application/json", "FAILED", "WHATSAPP_REJECTED", ""},
		{"rate", 429, `{"error":{"code":130429}}`, "application/json", "FAILED", "WHATSAPP_RATE_LIMITED", "9000"},
		{"server", 500, `{"error":{"code":2}}`, "application/json", "UNKNOWN", "WHATSAPP_UNCERTAIN", ""},
		{"timeout", 408, `{"error":{"code":2}}`, "application/json", "UNKNOWN", "WHATSAPP_UNCERTAIN", ""},
		{"malformed", 200, `{"messages":`, "application/json", "UNKNOWN", "WHATSAPP_UNCERTAIN", ""},
		{"oversized", 200, strings.Repeat("x", whatsAppResponseLimit+1), "application/json", "UNKNOWN", "WHATSAPP_UNCERTAIN", ""},
		{"html", 400, "PRIVATE_PROXY_PAGE", "text/html", "UNKNOWN", "WHATSAPP_UNCERTAIN", ""},
		{"redirect", 302, `{"messages":[{"id":"no"}]}`, "application/json", "UNKNOWN", "WHATSAPP_UNCERTAIN", ""},
		{"ambiguous", 400, `{"error":{"code":2},"messages":[{"id":"wamid.ambiguous"}]}`, "application/json", "UNKNOWN", "WHATSAPP_UNCERTAIN", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", tc.contentType)
				w.Header().Set("Retry-After", tc.retry)
				w.Header().Set("Location", "http://127.0.0.1:1/credential-leak")
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer fixture.Close()
			client, err := NewWhatsAppClient(whatsappTestConfig(fixture.URL), true)
			if err != nil {
				t.Fatal(err)
			}
			provider := &database.MessageProvider{Mode: "CLOUD_FIXTURE", Origin: fixture.URL, Name: "society_notice", Language: "en"}
			result := client.Send(context.Background(), database.WhatsAppAttempt{ID: "attempt-rejection-fixture", Destination: "+919000000101", Link: "http://127.0.0.1:8080/#community", Provider: provider})
			if result.State != tc.state || result.Reason != tc.reason || calls != 1 || strings.Contains(fmt.Sprint(result), "PRIVATE") {
				t.Fatal(result, calls)
			}
			if tc.name == "rate" && result.RetryAfter != 3600 {
				t.Fatal("unbounded retry", result)
			}
		})
	}
	if retryDelay("nonsense", time.Now()) != 30 || retryDelay("0", time.Now()) != 1 {
		t.Fatal("invalid retry fallback")
	}
}

func TestWhatsAppSignaturesDelayedStatusesChallengeAndEntireBatchBounds(t *testing.T) {
	config := whatsappTestConfig("http://127.0.0.1:12345")
	client, err := NewWhatsAppClient(config, true)
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"hub.mode": {"subscribe"}, "hub.verify_token": {config.VerifyToken}, "hub.challenge": {"1234567890"}}
	if proof, err := client.VerifyChallenge(query); err != nil || proof != "1234567890" {
		t.Fatal(proof, err)
	}
	query["hub.verify_token"] = []string{"wrong"}
	if _, err = client.VerifyChallenge(query); err == nil {
		t.Fatal("bad verify token accepted")
	}
	now := time.Now()
	at := now.Add(-8 * 24 * time.Hour).Unix()
	body := []byte(fmt.Sprintf(`{"object":"whatsapp_business_account","entry":[{"id":"111111","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","metadata":{"phone_number_id":"222222"},"statuses":[{"id":"wamid.delayed","status":"read","timestamp":"%d","recipient_id":"919000000101","biz_opaque_callback_data":"original-attempt-12345","pricing":{"category":"utility","billable":true}}]}}]}]}`, at))
	statuses, err := client.SignedStatuses(body, whatsappTestSignature(body, config.AppSecret), now)
	if err != nil || len(statuses) != 1 || statuses[0].At != at || statuses[0].State != "READ" || statuses[0].Destination != "+919000000101" || statuses[0].AttemptID != "original-attempt-12345" || !strings.Contains(statuses[0].Metadata, "pricing") {
		t.Fatal(statuses, err)
	}
	for _, bad := range [][]byte{append(append([]byte{}, body...), byte('x')), []byte(strings.ReplaceAll(string(body), "222222", "invalid_phone_identity")), []byte(strings.ReplaceAll(string(body), `"read"`, `"made_up"`)), []byte(strings.Repeat(" ", WhatsAppWebhookLimit+1))} {
		if _, err = client.SignedStatuses(bad, whatsappTestSignature(bad, config.AppSecret), now); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	if _, err = client.SignedStatuses(body, "sha256="+strings.Repeat("0", 64), now); err == nil {
		t.Fatal("invalid signature accepted")
	}
	changed := config
	changed.PhoneID = "999999"
	changed.AccountID = "888888"
	reconfigured, err := NewWhatsAppClient(changed, true)
	if err != nil {
		t.Fatal(err)
	}
	late, err := reconfigured.SignedStatuses(body, whatsappTestSignature(body, config.AppSecret), now)
	if err != nil || len(late) != 1 || late[0].PhoneID != "222222" || late[0].AccountID != "111111" || late[0].EventHash != statuses[0].EventHash {
		t.Fatal("configuration change lost original identity or duplicate hash", late, err)
	}
}

func TestWhatsAppConfigurationCustodyOriginsAndUnsupportedTemplates(t *testing.T) {
	for _, origin := range []string{"https://graph.facebook.com", "http://localhost:1234", "http://10.0.0.1:1234", "http://127.0.0.1:1/path", "http://user:password@127.0.0.1:1", "http://127.0.0.1:1?token=private"} {
		if _, err := NewWhatsAppClient(whatsappTestConfig(origin), true); err == nil {
			t.Fatal("fixture accepted unsafe origin", origin)
		}
	}
	if _, err := NewWhatsAppClient(whatsappTestConfig("https://graph.facebook.com"), false); err != nil {
		t.Fatal("official production boundary unavailable", err)
	}
	if _, err := NewWhatsAppClient(whatsappTestConfig("https://graph.facebook.com.attacker.test"), false); err == nil {
		t.Fatal("non-official production origin accepted")
	}
	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "provider.json")
	body, _ := json.Marshal(whatsappTestConfig("http://127.0.0.1:12345"))
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWhatsAppConfig(file); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "symlink.json")
	os.Symlink(file, link)
	if _, err := LoadWhatsAppConfig(link); err == nil {
		t.Fatal("symlink credential accepted")
	}
	os.Chmod(file, 0644)
	if _, err := LoadWhatsAppConfig(file); err == nil {
		t.Fatal("public credentials accepted")
	}
	os.Chmod(file, 0600)
	os.WriteFile(file, append(body, []byte(`{}`)...), 0600)
	if _, err := LoadWhatsAppConfig(file); err == nil {
		t.Fatal("trailing configuration accepted")
	}
}

func TestWhatsAppTemplateAPIRejectsUnsupportedContentAndPausedOrChangedIdentities(t *testing.T) {
	oversized, _ := json.Marshal(map[string]any{"id": "444444", "name": strings.Repeat("a", 512), "language": "en", "status": "APPROVED", "category": "UTILITY", "components": []map[string]any{{"type": "BODY", "text": strings.Repeat("<", 1019) + "{{1}}"}, {"type": "HEADER", "format": "TEXT", "text": strings.Repeat("<", 120)}, {"type": "FOOTER", "text": strings.Repeat("<", 120)}}})
	for _, tc := range []struct{ name, body string }{
		{"encoded_snapshot_too_large", string(oversized)},
		{"paused", `{"id":"444444","name":"society_notice","language":"en","status":"PAUSED","category":"UTILITY","components":[{"type":"BODY","text":"Your update: {{1}}"}]}`},
		{"identity", `{"id":"999999","name":"society_notice","language":"en","status":"APPROVED","category":"UTILITY","components":[{"type":"BODY","text":"Your update: {{1}}"}]}`},
		{"language", `{"id":"444444","name":"society_notice","language":"hi","status":"APPROVED","category":"UTILITY","components":[{"type":"BODY","text":"Your update: {{1}}"}]}`},
		{"authentication", `{"id":"444444","name":"society_notice","language":"en","status":"APPROVED","category":"AUTHENTICATION","components":[{"type":"BODY","text":"Your code: {{1}}"}]}`},
		{"multiple_variables", `{"id":"444444","name":"society_notice","language":"en","status":"APPROVED","category":"UTILITY","components":[{"type":"BODY","text":"Your update: {{1}}. Hello {{2}}"}]}`},
		{"buttons", `{"id":"444444","name":"society_notice","language":"en","status":"APPROVED","category":"UTILITY","components":[{"type":"BODY","text":"Your update: {{1}}"},{"type":"BUTTONS","buttons":[]}]}`},
		{"media_header", `{"id":"444444","name":"society_notice","language":"en","status":"APPROVED","category":"UTILITY","components":[{"type":"BODY","text":"Your update: {{1}}"},{"type":"HEADER","format":"IMAGE"}]}`},
		{"duplicate_body", `{"id":"444444","name":"society_notice","language":"en","status":"APPROVED","category":"UTILITY","components":[{"type":"BODY","text":"Your update: {{1}}"},{"type":"BODY","text":"Other update: {{1}}"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.body)
			}))
			defer fixture.Close()
			client, err := NewWhatsAppClient(whatsappTestConfig(fixture.URL), true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = client.ReadTemplate(context.Background(), "NOTICE"); err == nil {
				t.Fatal("unsupported approved content silently accepted")
			}
			if _, err = client.ReadTemplate(context.Background(), "RECEIPT"); err == nil {
				t.Fatal("unconfigured source fell back to another template")
			}
		})
	}
}

func TestWhatsAppTimeoutBeforeAndAfterFixtureAcceptanceRemainsUnknownWithoutAutomaticRetry(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(fmt.Sprint(accepted), func(t *testing.T) {
			var calls atomic.Int32
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// The HTTP server monitors disconnect after the complete request
				// body has been consumed; otherwise Close can wait indefinitely.
				io.Copy(io.Discard, r.Body)
				if accepted {
					calls.Add(1)
				}
				<-r.Context().Done()
			}))
			defer fixture.Close()
			client, err := NewWhatsAppClient(whatsappTestConfig(fixture.URL), true)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			provider := &database.MessageProvider{Mode: "CLOUD_FIXTURE", Origin: fixture.URL, Name: "society_notice", Language: "en"}
			result := client.Send(ctx, database.WhatsAppAttempt{ID: "attempt-timeout-12345", Destination: "+919000000101", Link: "http://127.0.0.1:8080/#community", Provider: provider})
			if result.State != "UNKNOWN" || result.ProviderID != "" || calls.Load() > 1 {
				t.Fatal("timeout manufactured delivery or retried", result, calls.Load())
			}
			if accepted && calls.Load() != 1 {
				t.Fatal("accepted fixture case never reached the independent acceptance boundary")
			}
		})
	}
}
