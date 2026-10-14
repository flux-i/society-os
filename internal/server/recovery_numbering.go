package server

import (
	"net/http"
	"strconv"

	"society.local/portal/internal/database"
)

func (s *Server) recoveryNumberingRoutes(mux *http.ServeMux) {
	page := func(r *http.Request) int {
		if value := r.URL.Query().Get("page"); value != "" {
			x, err := strconv.Atoi(value)
			if err != nil {
				return 0
			}
			return x
		}
		return 1
	}
	mux.HandleFunc("GET /api/recovery/numbering/status", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryNumberingStatusFor(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/numbering/operations/{key}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryNumberingOperationFor(r.Context(), sessionToken(r), r.PathValue("key"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/numbering/policies/{id}/history", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryNumberingHistoryFor(r.Context(), sessionToken(r), r.PathValue("id"), page(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/numbering/policies", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryNumberingPoliciesFor(r.Context(), sessionToken(r), r.URL.Query().Get("state"), page(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/numbering/policies/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryNumberingPolicyFor(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("POST /api/recovery/numbering/policies", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.RecoveryNumberingInput
		if !decode(w, r, &in) {
			return
		}
		x, err := s.Store.ProposeRecoveryNumbering(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("POST /api/recovery/numbering/policies/{id}/decision", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.RecoveryDecisionInput
		if !decode(w, r, &in) {
			return
		}
		x, err := s.Store.DecideRecoveryNumbering(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
}
