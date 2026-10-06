package server

import (
	"io"
	"mime"
	"net/http"
	"time"

	"society.local/portal/internal/database"
)

func messageOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
func (s *Server) messageConfigured(w http.ResponseWriter, r *http.Request) bool {
	if s.Messages == nil {
		respond(w, 503, map[string]string{"error": "message_provider_unavailable", "message": "No delivery provider is configured. Nothing was queued or sent."})
		return false
	}
	return true
}
func (s *Server) messageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/messages/config", s.protected(func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"simulation_enabled": s.Messages != nil, "whatsapp_live": false, "email_live": false, "attempt_limit": 3, "dispatch_limit": 25})
	}))
	mux.HandleFunc("GET /api/messages/summary", s.protected(func(w http.ResponseWriter, r *http.Request) {
		out, err := s.Store.MessageSummaryFor(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/messages/sources", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.MessageSourcesFor(r.Context(), sessionToken(r), q.Get("kind"), q.Get("q"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/messages/targets", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.MessageTargetsFor(r.Context(), sessionToken(r), q.Get("source_kind"), q.Get("source_id"), q.Get("kind"), q.Get("q"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/messages", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.MessagesFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("state"), q.Get("kind"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/messages/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		people, ok := queryPage(w, r, "recipient_page")
		if !ok {
			return
		}
		deliveries, ok := queryPage(w, r, "delivery_page")
		if !ok {
			return
		}
		events, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		out, err := s.Store.MessageFor(r.Context(), sessionToken(r), r.PathValue("id"), people, deliveries, events)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/messages/preview", s.protected(func(w http.ResponseWriter, r *http.Request) {
		if !s.messageConfigured(w, r) {
			return
		}
		var in database.MessageInput
		if !decodeLimit(w, r, &in, 32768) {
			return
		}
		in.PortalOrigin = messageOrigin(r)
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		out, err := s.Store.MessagePreviewFor(r.Context(), sessionToken(r), in, page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/messages", s.protected(func(w http.ResponseWriter, r *http.Request) {
		if !s.messageConfigured(w, r) {
			return
		}
		var in database.MessageInput
		if !decodeLimit(w, r, &in, 32768) {
			return
		}
		in.PortalOrigin = messageOrigin(r)
		id, err := s.Store.ProposeMessage(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/messages/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.MessageAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ActOnMessage(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/messages/{id}/dispatch", s.protected(func(w http.ResponseWriter, r *http.Request) {
		if !s.messageConfigured(w, r) {
			return
		}
		var in database.MessageAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Messages.Dispatch(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/messages/{id}/deliveries/{delivery}/reconcile", s.protected(func(w http.ResponseWriter, r *http.Request) {
		if !s.messageConfigured(w, r) {
			return
		}
		var in database.MessageAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ReconcileMessage(r.Context(), sessionToken(r), r.PathValue("id"), r.PathValue("delivery"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	// This loopback-only proof endpoint uses its independent signing key rather
	// than a browser session/CSRF token. It is explicitly a local simulation.
	mux.HandleFunc("POST /simulation/message-events", func(w http.ResponseWriter, r *http.Request) {
		if !s.messageConfigured(w, r) {
			return
		}
		kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || kind != "application/json" {
			respond(w, 415, map[string]string{"error": "json_required"})
			return
		}
		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
		if err != nil {
			respond(w, 400, map[string]string{"error": "invalid_json"})
			return
		}
		if err = s.Messages.Callback(r.Context(), payload, r.Header.Get("X-Simulation-Signature"), time.Now()); err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]any{"recorded": true, "simulation": true})
	})
}
