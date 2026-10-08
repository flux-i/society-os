package server

import (
	"net/http"
	"society.local/portal/internal/database"
)

func (s *Server) moveChecklistRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/move-checklists", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := budgetPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		data, e := s.Store.MoveChecklistsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("state"), page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/move-checklists/options", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := budgetPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		data, e := s.Store.MoveChecklistOptionsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("home"), page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/move-checklists/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := budgetPage(w, r, "event_page")
		if !ok {
			return
		}
		data, e := s.Store.MoveChecklistFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/move-checklists", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.MoveChecklistInput
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.SubmitMoveChecklist(r.Context(), sessionToken(r), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/move-checklists/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.MoveChecklistAction
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.ActOnMoveChecklist(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
}
