package server

import (
	"net/http"

	"society.local/portal/internal/database"
)

func (s *Server) contactRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/contacts", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.ContactsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("relationship"), q.Get("building"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/contacts/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		out, err := s.Store.ContactFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/contacts/{id}/register", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.ContactInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.RegisterContact(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/contacts/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.ContactAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ActOnContact(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
}
