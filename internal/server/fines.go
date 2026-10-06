package server

import (
	"mime"
	"net/http"
	"strconv"

	"society.local/portal/internal/database"
)

func (s *Server) fineRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/fines", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.FinesFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("GET /api/fine-sources", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		out, err := s.Store.FineSourcesFor(r.Context(), sessionToken(r), r.URL.Query().Get("q"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("GET /api/fine-sources/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "response_page")
		if !ok {
			return
		}
		out, err := s.Store.FineSourceFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST /api/fines", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FineInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.CreateFine(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fines/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		events, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		responses, ok := queryPage(w, r, "response_page")
		if !ok {
			return
		}
		out, err := s.Store.FineFor(r.Context(), sessionToken(r), r.PathValue("id"), events, responses)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST /api/fines/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FineAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ActOnFine(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fines/{id}/evidence-options", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		out, err := s.Store.FineEvidenceFor(r.Context(), sessionToken(r), r.PathValue("id"), r.URL.Query().Get("q"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("GET /api/fine-notices", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		out, err := s.Store.FineNoticesFor(r.Context(), sessionToken(r), r.URL.Query().Get("q"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("GET /api/fine-notices/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "response_page")
		if !ok {
			return
		}
		out, err := s.Store.FineNoticeFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST /api/fine-notices/{id}/responses", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.IncidentResponseInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.RespondToFineNotice(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fine-reports", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.FineReportsFor(r.Context(), sessionToken(r), q.Get("fine"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST /api/fine-reports", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FineReportInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.SaveFineReport(r.Context(), sessionToken(r), "", in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fine-reports/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		out, err := s.Store.FineReportFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST /api/fine-reports/{id}/revision", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FineReportInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.SaveFineReport(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/fine-reports/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FundReportDecision
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideFineReport(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fine-reports/{id}/options", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		out, err := s.Store.FineConfirmOptionsFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("GET /api/fine-reports/{id}/evidence", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, data, err := s.Store.FineReportEvidenceFor(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", x.Type)
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": x.Filename}))
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	mux.HandleFunc("GET /api/fine-appeals", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		out, err := s.Store.FineAppealsFor(r.Context(), sessionToken(r), r.URL.Query().Get("fine"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST /api/fine-appeals", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FineAppealInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.CreateFineAppeal(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fine-appeals/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		out, err := s.Store.FineAppealFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST /api/fine-appeals/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FineChildAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ActOnFineAppeal(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fine-waivers", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		out, err := s.Store.FineWaiversFor(r.Context(), sessionToken(r), r.URL.Query().Get("fine"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST /api/fine-waivers", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FineWaiverInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ProposeFineWaiver(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fine-waivers/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		out, err := s.Store.FineWaiverFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST /api/fine-waivers/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FineChildAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideFineWaiver(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, map[string]string{"id": id})
	}))
}
