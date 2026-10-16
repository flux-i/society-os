package server

import (
	"net/http"
	"strconv"

	"society.local/portal/internal/database"
)

func (s *Server) recoveryGapRoutes(mux *http.ServeMux) {
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
	mux.HandleFunc("GET /api/recovery/financial-gap/status", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryGapStatusFor(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/financial-gap/balance/{home}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryGapBalanceFor(r.Context(), sessionToken(r), r.PathValue("home"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/financial-gap/operations/{key}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryGapOperationFor(r.Context(), sessionToken(r), r.PathValue("key"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/financial-gap/preparations", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryGapPreparationsFor(r.Context(), sessionToken(r), r.URL.Query().Get("state"), page(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/financial-gap/preparations/{id}/history", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryGapHistoryFor(r.Context(), sessionToken(r), r.PathValue("id"), page(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/recovery/financial-gap/preparations/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.RecoveryGapFor(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("POST /api/recovery/financial-gap/preparations", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.RecoveryGapInput
		if !decode(w, r, &in) {
			return
		}
		x, err := s.Store.ProposeRecoveryGap(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("POST /api/recovery/financial-gap/preparations/{id}/decision", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.RecoveryDecisionInput
		if !decode(w, r, &in) {
			return
		}
		x, err := s.Store.DecideRecoveryGap(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, x)
	}))
}
