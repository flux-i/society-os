package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"society.local/portal/internal/database"
)

func (s *Server) mutationResult(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		respond(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		s.resultError(w, r, err)
		return
	}
	respond(w, 200, map[string]string{"status": "saved"})
}
func (s *Server) changeOccupancy(w http.ResponseWriter, r *http.Request) {
	var input database.OccupancyChange
	if !decode(w, r, &input) {
		return
	}
	s.mutationResult(w, r, s.Store.ChangeOccupancy(r.Context(), sessionToken(r), r.PathValue("id"), input))
}
func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	var input database.AddMembership
	if !decode(w, r, &input) {
		return
	}
	s.mutationResult(w, r, s.Store.AddMembership(r.Context(), sessionToken(r), r.PathValue("id"), input))
}
func (s *Server) endMember(w http.ResponseWriter, r *http.Request) {
	var input database.EndMembership
	if !decode(w, r, &input) {
		return
	}
	s.mutationResult(w, r, s.Store.EndMembership(r.Context(), sessionToken(r), r.PathValue("id"), r.PathValue("membership"), input))
}
func (s *Server) people(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 100 {
		respond(w, 400, map[string]string{"error": "invalid_filter"})
		return
	}
	result, err := s.Store.PeopleFor(r.Context(), sessionToken(r), query)
	if err != nil {
		s.resultError(w, r, err)
		return
	}
	respond(w, 200, result)
}
func (s *Server) activity(w http.ResponseWriter, r *http.Request) {
	result, err := s.Store.ActivityFor(r.Context(), sessionToken(r), r.PathValue("id"))
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
