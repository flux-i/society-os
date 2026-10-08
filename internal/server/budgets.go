package server

import (
	"errors"
	"net/http"
	"society.local/portal/internal/database"
	"strconv"
)

func (s *Server) budgetError(w http.ResponseWriter, r *http.Request, e error) {
	if errors.Is(e, database.ErrDuplicatePaidExpense) {
		respond(w, http.StatusConflict, map[string]string{"error": "duplicate_paid_expense"})
		return
	}
	s.resultError(w, r, e)
}
func budgetPage(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 1, true
	}
	page, e := strconv.Atoi(value)
	if e != nil || page < 1 || page > 10000 {
		respond(w, 400, map[string]string{"error": "invalid_input"})
		return 0, false
	}
	return page, true
}
func (s *Server) budgetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/budgets", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := budgetPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		data, e := s.Store.BudgetsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("state"), page)
		if e != nil {
			s.budgetError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/budgets/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := budgetPage(w, r, "event_page")
		if !ok {
			return
		}
		data, e := s.Store.BudgetFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if e != nil {
			s.budgetError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/budgets/{id}/comparison", s.protected(func(w http.ResponseWriter, r *http.Request) {
		version := 0
		if raw := r.URL.Query().Get("version"); raw != "" {
			var e error
			version, e = strconv.Atoi(raw)
			if e != nil || version < 1 || version > 1000000 {
				respond(w, 400, map[string]string{"error": "invalid_input"})
				return
			}
		}
		data, e := s.Store.BudgetComparisonFor(r.Context(), sessionToken(r), r.PathValue("id"), version)
		if e != nil {
			s.budgetError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	propose := s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.BudgetInput
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.ProposeBudget(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.budgetError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	})
	mux.HandleFunc("POST /api/budgets", propose)
	mux.HandleFunc("POST /api/budgets/{id}/proposals", propose)
	mux.HandleFunc("POST /api/budgets/{id}/decisions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.BudgetAction
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.DecideBudget(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.budgetError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/paid-expenses", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := budgetPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		data, e := s.Store.PaidExpensesFor(r.Context(), sessionToken(r), q.Get("budget_id"), q.Get("q"), q.Get("state"), page)
		if e != nil {
			s.budgetError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/paid-expenses/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := budgetPage(w, r, "event_page")
		if !ok {
			return
		}
		data, e := s.Store.PaidExpenseFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if e != nil {
			s.budgetError(w, r, e)
			return
		}
		respond(w, 200, data)
	}))
	expense := s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.PaidExpenseInput
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.ProposePaidExpense(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.budgetError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	})
	mux.HandleFunc("POST /api/paid-expenses", expense)
	mux.HandleFunc("POST /api/paid-expenses/{id}/proposals", expense)
	mux.HandleFunc("POST /api/paid-expenses/{id}/decisions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.BudgetAction
		if !decode(w, r, &in) {
			return
		}
		id, e := s.Store.DecidePaidExpense(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.budgetError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
}
