package server

import (
	"net/http"
	"strconv"

	"society.local/portal/internal/database"
)

func (s *Server) offsiteRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/backups", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if raw := r.URL.Query().Get("page"); raw != "" {
			var err error
			page, err = strconv.Atoi(raw)
			if err != nil {
				s.resultError(w, r, database.ErrInvalid)
				return
			}
		}
		result, err := s.Store.OffsiteStatusFor(r.Context(), sessionToken(r), r.URL.Query().Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, result)
	}))
	mux.HandleFunc("GET /api/backups/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if raw := r.URL.Query().Get("page"); raw != "" {
			var err error
			page, err = strconv.Atoi(raw)
			if err != nil {
				s.resultError(w, r, database.ErrInvalid)
				return
			}
		}
		result, err := s.Store.OffsiteDetailFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, result)
	}))
}
