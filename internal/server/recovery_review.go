package server

import (
	"net/http"
	"strconv"

	"society.local/portal/internal/database"
)

func (s *Server) recoveryReviewRoutes(mux *http.ServeMux) {
	s.recoveryNumberingRoutes(mux)
	s.recoveryGapRoutes(mux)
	page := func(r *http.Request) int {
		value := r.URL.Query().Get("page")
		if value == "" {
			return 1
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return 0
		}
		return n
	}
	mux.HandleFunc("GET /api/recovery/operations/{key}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryOperationFor(r.Context(), sessionToken(r), r.PathValue("key"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/proposals/{id}/history", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryHistoryFor(r.Context(), sessionToken(r), r.PathValue("id"), page(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))

	mux.HandleFunc("GET /api/recovery/status", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoverySummaryFor(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/accounts", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryCandidatesFor(r.Context(), sessionToken(r), r.URL.Query().Get("q"), page(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/work", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryWorkFor(r.Context(), sessionToken(r), page(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/homes", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryHomesFor(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]any{"items": x, "limit": 200})
	}))
	mux.HandleFunc("GET /api/recovery/proposals", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryProposalsFor(r.Context(), sessionToken(r), r.URL.Query().Get("kind"), r.URL.Query().Get("state"), page(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/proposals/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryProposalFor(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("POST /api/recovery/proposals", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.RecoveryProposalInput
		if !decode(w, r, &in) {
			return
		}
		x, err := s.Store.ProposeRecovery(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("POST /api/recovery/proposals/{id}/decision", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.RecoveryDecisionInput
		if !decode(w, r, &in) {
			return
		}
		x, err := s.Store.DecideRecovery(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
}
