package server

import (
	"net/http"
	"society.local/portal/internal/database"
	"strconv"
)

func queryPage(w http.ResponseWriter, r *http.Request, key string) (int, bool) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 1, true
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 || page > 100000 {
		respond(w, 400, map[string]string{"error": "invalid_input", "message": "Use a positive page number, at most 100000."})
		return 0, false
	}
	return page, true
}

func (s *Server) maintenanceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/maintenance", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		data, err := s.Store.MaintenanceCyclesFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("home"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/maintenance/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		lines, ok := queryPage(w, r, "line_page")
		if !ok {
			return
		}
		events, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		data, err := s.Store.MaintenanceCycleFor(r.Context(), sessionToken(r), r.PathValue("id"), lines, events)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/maintenance", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.MaintenanceInput
		if !decodeLimit(w, r, &in, 98304) {
			return
		}
		id, err := s.Store.CreateMaintenanceCycle(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/maintenance/{id}/decision", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.MaintenanceAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideMaintenanceCycle(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/statements/{home}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		charges, ok := queryPage(w, r, "charge_page")
		if !ok {
			return
		}
		credits, ok := queryPage(w, r, "credit_page")
		if !ok {
			return
		}
		allocations, ok := queryPage(w, r, "allocation_page")
		if !ok {
			return
		}
		data, err := s.Store.HomeStatementFor(r.Context(), sessionToken(r), r.PathValue("home"), charges, credits, allocations)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/allocations", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.CreditAllocationInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.AllocateCredit(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/allocations/{id}/reverse", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.AllocationCorrection
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ReverseAllocation(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
}
