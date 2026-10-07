package server

import (
	"net/http"
	"strconv"
	"strings"

	"society.local/portal/internal/database"
)

func (s *Server) financeExportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/finance-exports/choices", s.protected(func(w http.ResponseWriter, r *http.Request) {
		out, err := s.Store.FinanceExportChoicesFor(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/finance-exports/preview", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FinanceExportFilter
		if !decode(w, r, &in) {
			return
		}
		out, err := s.Store.PreviewFinanceExport(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/finance-exports", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FinanceExportInput
		if !decode(w, r, &in) {
			return
		}
		out, err := s.Store.CreateFinanceExport(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/finance-exports", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if value := r.URL.Query().Get("page"); value != "" {
			var err error
			page, err = strconv.Atoi(value)
			if err != nil {
				s.resultError(w, r, database.ErrInvalid)
				return
			}
		}
		out, err := s.Store.FinanceExportsFor(r.Context(), sessionToken(r), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/finance-exports/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		out, err := s.Store.FinanceExportFor(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/finance-exports/{id}/download", s.protected(func(w http.ResponseWriter, r *http.Request) {
		out, body, err := s.Store.DownloadFinanceExport(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="society-`+strings.ToLower(out.Report)+`-`+out.From+`-`+out.To+`.csv"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Header().Set("X-Export-SHA256", out.SHA256)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
}
