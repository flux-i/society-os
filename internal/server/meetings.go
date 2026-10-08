package server

import (
	"net/http"
	"society.local/portal/internal/database"
)

func (s *Server) meetingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/meetings", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		desk, ok := communityDesk(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		data, e := s.Store.MeetingsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("state"), desk, page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/meetings/options", s.protected(func(w http.ResponseWriter, r *http.Request) {
		data, e := s.Store.CommunityOptionsFor(r.Context(), sessionToken(r))
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/meetings/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		desk, ok := communityDesk(w, r)
		if !ok {
			return
		}
		data, e := s.Store.MeetingFor(r.Context(), sessionToken(r), r.PathValue("id"), desk, page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/meetings/{id}/responses", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		history, ok := queryPage(w, r, "history_page")
		if !ok {
			return
		}
		data, e := s.Store.MeetingResponsesFor(r.Context(), sessionToken(r), r.PathValue("id"), page, history)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	propose := s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.MeetingInput
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.ProposeMeeting(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	})
	mux.HandleFunc("POST /api/meetings", propose)
	mux.HandleFunc("POST /api/meetings/{id}/proposals", propose)
	mux.HandleFunc("POST /api/meetings/{id}/decisions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.MeetingAction
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.DecideMeeting(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/meetings/{id}/acknowledgements", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.MeetingAcknowledgementInput
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.AcknowledgeMeeting(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
}
