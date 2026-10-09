package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"society.local/portal/internal/database"
)

func (s *Server) householdFinance(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page := 1
	if value := r.URL.Query().Get("page"); value != "" {
		var err error
		page, err = strconv.Atoi(value)
		if err != nil {
			respond(w, 400, map[string]string{"error": "invalid_filter"})
			return
		}
	}
	result, err := s.Store.HouseholdFinanceFor(r.Context(), sessionToken(r), r.PathValue("id"), query, page)
	if errors.Is(err, sql.ErrNoRows) {
		respond(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		s.resultError(w, r, err)
		return
	}
	respond(w, 200, result)
}

func (s *Server) changeHouseholdFinance(w http.ResponseWriter, r *http.Request) {
	var input database.HouseholdFinanceInput
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Store.ChangeHouseholdFinance(r.Context(), sessionToken(r), r.PathValue("id"), input)
	if errors.Is(err, sql.ErrNoRows) {
		respond(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		s.resultError(w, r, err)
		return
	}
	respond(w, 200, result)
}

func (s *Server) householdFinanceAction(w http.ResponseWriter, r *http.Request) {
	result, err := s.Store.HouseholdFinanceActionFor(r.Context(), sessionToken(r), r.PathValue("id"), r.PathValue("key"))
	if errors.Is(err, sql.ErrNoRows) {
		respond(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		s.resultError(w, r, err)
		return
	}
	respond(w, 200, result)
}
