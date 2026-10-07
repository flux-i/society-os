package messaging

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"society.local/portal/internal/database"
)

const WhatsAppWebhookLimit = 3 * 1024 * 1024
const whatsAppResponseLimit = 64 * 1024

type WhatsAppTemplate struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Language string `json:"language"`
}

type WhatsAppConfig struct {
	Origin             string                      `json:"origin"`
	APIVersion         string                      `json:"api_version"`
	AccountID          string                      `json:"account_id"`
	PhoneID            string                      `json:"phone_id"`
	MessagingAccountID string                      `json:"messaging_account_id"`
	AccessToken        string                      `json:"access_token"`
	AppSecret          string                      `json:"app_secret"`
	VerifyToken        string                      `json:"verify_token"`
	Templates          map[string]WhatsAppTemplate `json:"templates"`
}

type WhatsAppClient struct {
	config WhatsAppConfig
	mode   string
	http   *http.Client
}

// A fixture client accepts only numeric loopback origins. The production
// boundary accepts only the exact official HTTPS origin and is not enabled by
// the current preview CLI. Neither constructor sends or purchases anything.
func NewWhatsAppClient(config WhatsAppConfig, fixture bool) (*WhatsAppClient, error) {
	mode := "CLOUD"
	if fixture {
		mode = "CLOUD_FIXTURE"
	}
	probe := &database.MessageProvider{Mode: mode, Origin: config.Origin, APIVersion: config.APIVersion, AccountID: config.AccountID, PhoneID: config.PhoneID, MessagingAccountID: config.MessagingAccountID, TemplateID: "1", Name: "configuration_check", Language: "en", Category: "UTILITY", Body: "Your portal link: {{1}}"}
	if database.ValidateMessageProvider(probe) != nil || !privateHeader(config.AccessToken) || !privateHeader(config.AppSecret) || !privateHeader(config.VerifyToken) || len(config.Templates) == 0 || len(config.Templates) > 3 {
		return nil, errors.New("invalid separately held WhatsApp configuration")
	}
	templates := map[string]WhatsAppTemplate{}
	for kind, spec := range config.Templates {
		if kind != "NOTICE" && kind != "RECEIPT" && kind != "STATEMENT" {
			return nil, errors.New("unsupported WhatsApp source configuration")
		}
		probe.TemplateID, probe.Name, probe.Language = spec.ID, spec.Name, spec.Language
		if database.ValidateMessageProvider(probe) != nil {
			return nil, errors.New("invalid WhatsApp template configuration")
		}
		templates[kind] = spec
	}
	config.Templates = templates
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxIdleConns: 4, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &WhatsAppClient{config: config, mode: mode, http: client}, nil
}

func privateHeader(value string) bool {
	return len(value) >= 16 && len(value) <= 8192 && !strings.ContainsAny(value, "\r\n\x00") && strings.TrimSpace(value) == value
}

func providerIdentity(value string) bool {
	if len(value) == 0 || len(value) > 32 {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (c *WhatsAppClient) request(ctx context.Context, method, path string, body []byte) (*http.Response, []byte, error) {
	r, err := http.NewRequestWithContext(ctx, method, c.config.Origin+"/"+c.config.APIVersion+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, errors.New("WhatsApp request unavailable")
	}
	r.Header.Set("Authorization", "Bearer "+c.config.AccessToken)
	r.Header.Set("Accept", "application/json")
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(r)
	if err != nil {
		return nil, nil, errors.New("WhatsApp response interrupted")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, whatsAppResponseLimit+1))
	kind, _, typeErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || len(data) > whatsAppResponseLimit || typeErr != nil || kind != "application/json" {
		return response, nil, errors.New("WhatsApp response could not be verified")
	}
	return response, data, nil
}

func decodeProviderJSON(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if d.Decode(target) != nil || d.Decode(new(any)) != io.EOF {
		return database.ErrInvalid
	}
	return nil
}

// ReadTemplate reads the provider's actual approved content. Configured names
// cannot assert approval and unsupported components are never silently omitted.
func (c *WhatsAppClient) ReadTemplate(ctx context.Context, kind string) (*database.MessageProvider, error) {
	spec, exists := c.config.Templates[kind]
	if !exists {
		return nil, database.InvalidMessageProvider("No WhatsApp template is configured for this source.")
	}
	response, data, err := c.request(ctx, http.MethodGet, "/"+spec.ID+"?fields=id,name,language,status,category,components", nil)
	if err != nil || response.StatusCode != 200 {
		return nil, database.InvalidMessageProvider("The WhatsApp template could not be verified. Retry after provider configuration is available.")
	}
	var template struct {
		ID, Name, Language, Status, Category string
		Components                           []struct {
			Type, Format, Text string
		}
	}
	if decodeProviderJSON(data, &template) != nil || template.ID != spec.ID || template.Name != spec.Name || template.Language != spec.Language || template.Status != "APPROVED" {
		return nil, database.InvalidMessageProvider("The configured WhatsApp template is not currently approved with the expected identity and language.")
	}
	p := &database.MessageProvider{Mode: c.mode, Origin: c.config.Origin, APIVersion: c.config.APIVersion, AccountID: c.config.AccountID, PhoneID: c.config.PhoneID, MessagingAccountID: c.config.MessagingAccountID, TemplateID: template.ID, Name: template.Name, Language: template.Language, Category: template.Category}
	seen := map[string]bool{}
	for _, component := range template.Components {
		if seen[component.Type] {
			return nil, database.InvalidMessageProvider("The WhatsApp template has unsupported repeated components.")
		}
		seen[component.Type] = true
		switch component.Type {
		case "BODY":
			p.Body = component.Text
		case "HEADER":
			if component.Format != "TEXT" {
				return nil, database.InvalidMessageProvider("Only static text headers are supported.")
			}
			p.Header = component.Text
		case "FOOTER":
			p.Footer = component.Text
		default:
			return nil, database.InvalidMessageProvider("This WhatsApp template needs an unsupported component workflow.")
		}
	}
	if err = database.ValidateMessageProvider(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (c *WhatsAppClient) Send(ctx context.Context, attempt database.WhatsAppAttempt) database.WhatsAppResult {
	unknown := database.WhatsAppResult{State: "UNKNOWN", Reason: "WHATSAPP_UNCERTAIN"}
	if attempt.Provider == nil || attempt.Provider.Origin != c.config.Origin || attempt.Provider.Mode != c.mode || !strings.HasPrefix(attempt.Destination, "+") || len(attempt.Destination) < 9 || len(attempt.Destination) > 16 {
		return unknown
	}
	for _, ch := range attempt.Destination[1:] {
		if ch < '0' || ch > '9' {
			return unknown
		}
	}
	body, _ := json.Marshal(map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": attempt.Destination, "type": "template", "template": map[string]any{"name": attempt.Provider.Name, "language": map[string]string{"code": attempt.Provider.Language}, "components": []any{map[string]any{"type": "body", "parameters": []any{map[string]string{"type": "text", "text": attempt.Link}}}}}, "biz_opaque_callback_data": attempt.ID})
	if c.config.MessagingAccountID != "" {
		var values map[string]any
		json.Unmarshal(body, &values)
		values["messaging_account_id"] = c.config.MessagingAccountID
		body, _ = json.Marshal(values)
	}
	response, data, err := c.request(ctx, http.MethodPost, "/"+c.config.PhoneID+"/messages", body)
	if err != nil {
		return unknown
	}
	var out struct {
		MessagingProduct string                `json:"messaging_product"`
		Messages         []struct{ ID string } `json:"messages"`
		Error            *struct{ Code int }   `json:"error"`
	}
	if decodeProviderJSON(data, &out) != nil {
		return unknown
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 && out.Error == nil && out.MessagingProduct == "whatsapp" && len(out.Messages) == 1 && len(out.Messages[0].ID) > 0 && len(out.Messages[0].ID) <= 512 {
		return database.WhatsAppResult{State: "ACCEPTED", ProviderID: out.Messages[0].ID, Reason: "WHATSAPP_ACCEPTED"}
	}
	// Generic 5xx/timeout/malformed responses do not prove non-acceptance. Only
	// a structured definitive client rejection permits a deliberate retry.
	if response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != 408 && out.Error != nil && out.Error.Code > 0 && len(out.Messages) == 0 {
		result := database.WhatsAppResult{State: "FAILED", Reason: "WHATSAPP_REJECTED"}
		if response.StatusCode == 429 {
			result.Reason, result.RetryAfter = "WHATSAPP_RATE_LIMITED", retryDelay(response.Header.Get("Retry-After"), time.Now())
		}
		return result
	}
	return unknown
}

func retryDelay(value string, now time.Time) int64 {
	delay, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		if at, failure := http.ParseTime(value); failure == nil {
			delay = int64(at.Sub(now).Seconds())
		} else {
			delay = 30
		}
	}
	if delay < 1 {
		delay = 1
	}
	if delay > 3600 {
		delay = 3600
	}
	return delay
}

func (c *WhatsAppClient) VerifyChallenge(query url.Values) (string, error) {
	challenge := query.Get("hub.challenge")
	if len(query["hub.mode"]) != 1 || len(query["hub.verify_token"]) != 1 || len(query["hub.challenge"]) != 1 || query.Get("hub.mode") != "subscribe" || !hmac.Equal([]byte(query.Get("hub.verify_token")), []byte(c.config.VerifyToken)) || len(challenge) == 0 || len(challenge) > 128 {
		return "", database.ErrForbidden
	}
	return challenge, nil
}

func (c *WhatsAppClient) SignedStatuses(payload []byte, signature string, now time.Time) ([]database.WhatsAppStatus, error) {
	if len(payload) == 0 || len(payload) > WhatsAppWebhookLimit || len(signature) != 71 || !strings.HasPrefix(signature, "sha256=") {
		return nil, database.ErrForbidden
	}
	mac := hmac.New(sha256.New, []byte(c.config.AppSecret))
	mac.Write(payload)
	actual, err := hex.DecodeString(signature[7:])
	if err != nil || !hmac.Equal(mac.Sum(nil), actual) {
		return nil, database.ErrForbidden
	}
	var packet struct {
		Object string
		Entry  []struct {
			ID      string
			Changes []struct {
				Field string
				Value struct {
					MessagingProduct string `json:"messaging_product"`
					Metadata         struct {
						PhoneID string `json:"phone_number_id"`
					}
					Statuses []json.RawMessage
					Messages []json.RawMessage
				}
			}
		}
	}
	if decodeProviderJSON(payload, &packet) != nil || packet.Object != "whatsapp_business_account" || len(packet.Entry) == 0 || len(packet.Entry) > 100 {
		return nil, database.ErrInvalid
	}
	statuses := []database.WhatsAppStatus{}
	for _, entry := range packet.Entry {
		// The held secret belongs to the application. Configuration can change
		// phones while old accepted jobs still await callbacks. Validate each
		// account/phone against its frozen attempt in the atomic store boundary.
		if !providerIdentity(entry.ID) || len(entry.Changes) == 0 || len(entry.Changes) > 100 {
			return nil, database.ErrForbidden
		}
		for _, change := range entry.Changes {
			if change.Field != "messages" || change.Value.MessagingProduct != "whatsapp" || !providerIdentity(change.Value.Metadata.PhoneID) || len(change.Value.Messages) != 0 || len(change.Value.Statuses) == 0 {
				return nil, database.ErrInvalid
			}
			for _, raw := range change.Value.Statuses {
				var status struct {
					ID, Status, Timestamp string
					RecipientID           string `json:"recipient_id"`
					AttemptID             string `json:"biz_opaque_callback_data"`
				}
				var metadata map[string]any
				if decodeProviderJSON(raw, &status) != nil || decodeProviderJSON(raw, &metadata) != nil {
					return nil, database.ErrInvalid
				}
				state := map[string]string{"sent": "ACCEPTED", "delivered": "DELIVERED", "read": "READ", "failed": "FAILED"}[status.Status]
				at, failure := strconv.ParseInt(status.Timestamp, 10, 64)
				if failure != nil || at <= 0 || at > now.Add(5*time.Minute).Unix() || state == "" || status.ID == "" || len(status.ID) > 512 || status.RecipientID == "" || len(status.RecipientID) > 32 || len(status.AttemptID) > 100 {
					return nil, database.ErrInvalid
				}
				canonical, _ := json.Marshal(metadata)
				if len(canonical) > 16384 {
					return nil, database.ErrInvalid
				}
				hash := sha256.Sum256(append([]byte(entry.ID+":"+change.Value.Metadata.PhoneID+":"), canonical...))
				statuses = append(statuses, database.WhatsAppStatus{EventHash: hex.EncodeToString(hash[:]), AccountID: entry.ID, PhoneID: change.Value.Metadata.PhoneID, ProviderID: status.ID, AttemptID: status.AttemptID, Destination: "+" + strings.TrimPrefix(status.RecipientID, "+"), State: state, At: at, Metadata: string(canonical)})
				if len(statuses) > 200 {
					return nil, database.ErrInvalid
				}
			}
		}
	}
	return statuses, nil
}
