package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MessageProvider is public, frozen content metadata. Credentials never belong
// in proposals, operation payloads, event snapshots or browser responses.
type MessageProvider struct {
	Mode               string `json:"mode"`
	Origin             string `json:"origin"`
	APIVersion         string `json:"api_version"`
	AccountID          string `json:"account_id"`
	PhoneID            string `json:"phone_id"`
	MessagingAccountID string `json:"messaging_account_id,omitempty"`
	TemplateID         string `json:"template_id"`
	Name               string `json:"name"`
	Language           string `json:"language"`
	Category           string `json:"category"`
	Body               string `json:"body"`
	Header             string `json:"header,omitempty"`
	Footer             string `json:"footer,omitempty"`
}

var providerNumber = regexp.MustCompile(`^[0-9]{1,32}$`)
var providerVersion = regexp.MustCompile(`^v[0-9]{1,3}\.0$`)
var providerTemplateName = regexp.MustCompile(`^[a-z0-9_]{1,512}$`)
var providerLanguage = regexp.MustCompile(`^[a-z]{2,3}(?:_[A-Z]{2})?$`)

func ValidateMessageProvider(p *MessageProvider) error {
	if p == nil {
		return nil
	}
	u, err := url.Parse(p.Origin)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return ErrInvalid
	}
	if p.Mode == "CLOUD_FIXTURE" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || ip == nil || !ip.IsLoopback() || u.Port() == "" {
			return ErrInvalid
		}
	} else if p.Mode != "CLOUD" || p.Origin != "https://graph.facebook.com" {
		return ErrInvalid
	}
	if !providerVersion.MatchString(p.APIVersion) || !providerNumber.MatchString(p.AccountID) || !providerNumber.MatchString(p.PhoneID) || !providerNumber.MatchString(p.TemplateID) || (p.MessagingAccountID != "" && !providerNumber.MatchString(p.MessagingAccountID)) || !providerTemplateName.MatchString(p.Name) || !providerLanguage.MatchString(p.Language) {
		return ErrInvalid
	}
	if p.Category != "UTILITY" && p.Category != "MARKETING" {
		return invalid("This delivery workflow supports approved utility and marketing templates only.")
	}
	if !providerText(p.Body, 5, 1024) || !providerText(p.Header, 0, 120) || !providerText(p.Footer, 0, 120) || strings.Count(p.Body, "{{1}}") != 1 || strings.Contains(strings.ReplaceAll(p.Body, "{{1}}", ""), "{{") || strings.Contains(p.Header+p.Footer, "{{") {
		return invalid("Use a template with one portal-link body parameter and optional static text header/footer.")
	}
	// Rune limits alone do not bound JSON: escaping HTML characters can expand
	// the retained content. Reject before preview if it cannot be frozen intact.
	blob, err := json.Marshal(p)
	if err != nil || len(blob) > 8192 {
		return invalid("This template is too large to retain safely. Choose a shorter approved template.")
	}
	return nil
}

func providerText(value string, min, max int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) < min || utf8.RuneCountInString(value) > max || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' {
			return false
		}
	}
	return true
}

func storeMessageProvider(ctx context.Context, tx *sql.Tx, x MessageBatch) error {
	if x.Provider == nil {
		return nil
	}
	if err := ValidateMessageProvider(x.Provider); err != nil {
		return err
	}
	blob, err := json.Marshal(x.Provider)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO message_provider_bindings(batch_id,snapshot_version,provider_json) VALUES(?,?,?)", x.ID, x.SnapshotVersion, string(blob))
	return err
}

func SameMessageProvider(a, b *MessageProvider) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func providerEnvelope(p *MessageProvider, link string) string {
	parts := []string{}
	if p.Header != "" {
		parts = append(parts, p.Header)
	}
	parts = append(parts, strings.ReplaceAll(p.Body, "{{1}}", link))
	if p.Footer != "" {
		parts = append(parts, p.Footer)
	}
	return strings.Join(parts, "\n")
}

// InvalidMessageProvider exposes the same bounded validation error as other
// domain workflows without embedding a provider body, token or destination.
func InvalidMessageProvider(message string) error { return invalid(message) }

type WhatsAppAttempt struct {
	ID          string
	Destination string
	Link        string
	Provider    *MessageProvider
}

type WhatsAppResult struct {
	State      string `json:"state"`
	ProviderID string `json:"provider_id"`
	Reason     string `json:"reason"`
	RetryAfter int64  `json:"retry_after"`
}

type WhatsAppStatus struct {
	EventHash, AccountID, PhoneID, ProviderID, AttemptID, Destination, State, Metadata string
	At                                                                                 int64
}
