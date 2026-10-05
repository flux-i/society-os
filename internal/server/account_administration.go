package server

import (
	"net/http"
	"strconv"

	"society.local/portal/internal/database"
)

func (s *Server) accountAdministrationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/accounts/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page := func(key string) (int, error) {
			value := r.URL.Query().Get(key)
			if value == "" {
				return 1, nil
			}
			return strconv.Atoi(value)
		}
		grantPage, err := page("grant_page")
		historyPage, historyErr := page("history_page")
		if err != nil || historyErr != nil {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		value, err := s.Store.AccountDetailsFor(r.Context(), sessionToken(r), r.PathValue("id"), grantPage, historyPage)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, value)
	}))
	mux.HandleFunc("POST /api/admin/accounts/{id}/roles", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input database.AppointmentInput
		if !decode(w, r, &input) {
			return
		}
		value, err := s.Store.GrantAppointment(r.Context(), sessionToken(r), r.PathValue("id"), input)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 201, value)
	}))
	mux.HandleFunc("POST /api/admin/accounts/{id}/roles/{grant}/revoke", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input database.AccessChange
		if !decode(w, r, &input) {
			return
		}
		value, err := s.Store.RevokeAppointment(r.Context(), sessionToken(r), r.PathValue("id"), r.PathValue("grant"), input)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, value)
	}))
	mux.HandleFunc("POST /api/admin/accounts/{id}/status", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input database.AccountStatusInput
		if !decode(w, r, &input) {
			return
		}
		value, err := s.Store.ChangeAccountStatus(r.Context(), sessionToken(r), r.PathValue("id"), input)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, value)
	}))
}
