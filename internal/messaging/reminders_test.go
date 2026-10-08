package messaging

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReminderProviderConfigurationReadsThreeDistinctReviewedSourceTemplatesWithoutHandoff(t *testing.T) {
	gets, posts := 0, 0
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "GET" {
			posts++
			w.WriteHeader(400)
			return
		}
		gets++
		fmt.Fprintf(w, `{"id":"%s","name":"society_reminder","language":"en","status":"APPROVED","category":"UTILITY","components":[{"type":"BODY","text":"Please review your portal record. {{1}}"}]}`, r.URL.Path[len("/v26.0/"):])
	}))
	defer fixture.Close()
	config := whatsappTestConfig(fixture.URL)
	for i, kind := range []string{"RECEIPT", "STATEMENT", "MAINTENANCE_REMINDER", "FUND_REMINDER", "MEETING_REMINDER"} {
		config.Templates[kind] = WhatsAppTemplate{ID: fmt.Sprintf("55555%d", i), Name: "society_reminder", Language: "en"}
	}
	client, e := NewWhatsAppClient(config, true)
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"MAINTENANCE_REMINDER", "FUND_REMINDER", "MEETING_REMINDER"} {
		provider, e := client.ReadTemplate(context.Background(), kind)
		if e != nil || provider.TemplateID != config.Templates[kind].ID || provider.Body != "Please review your portal record. {{1}}" {
			t.Fatal(kind, provider, e)
		}
	}
	if gets != 3 || posts != 0 {
		t.Fatal("configuration lookup sent a message", gets, posts)
	}
	config.Templates["AUTOMATIC_BILLING"] = WhatsAppTemplate{ID: "666666", Name: "society_reminder", Language: "en"}
	if _, e = NewWhatsAppClient(config, true); e == nil {
		t.Fatal("unsupported automatic source accepted")
	}
}
