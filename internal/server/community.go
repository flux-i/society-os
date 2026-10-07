package server

import (
	"net/http"
	"society.local/portal/internal/database"
)

func communityDesk(w http.ResponseWriter, r *http.Request) (bool, bool) {
	value := r.URL.Query().Get("desk")
	if value != "" && value != "true" && value != "false" {
		respond(w, 400, map[string]string{"error": "invalid_request"})
		return false, false
	}
	return value == "true", true
}
func (s *Server) communityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/community", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		desk, ok := communityDesk(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		data, e := s.Store.CommunityFor(r.Context(), sessionToken(r), q.Get("kind"), q.Get("q"), q.Get("state"), desk, page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/community/options", s.protected(func(w http.ResponseWriter, r *http.Request) {
		data, e := s.Store.CommunityOptionsFor(r.Context(), sessionToken(r))
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/community/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		desk, ok := communityDesk(w, r)
		if !ok {
			return
		}
		data, e := s.Store.CommunityResourceFor(r.Context(), sessionToken(r), r.PathValue("id"), desk, page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	propose := s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.CommunityInput
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.ProposeCommunity(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	})
	mux.HandleFunc("POST /api/community", propose)
	mux.HandleFunc("POST /api/community/{id}/proposals", propose)
	mux.HandleFunc("POST /api/community/{id}/decisions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.CommunityAction
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.DecideCommunity(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
}
