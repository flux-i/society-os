package server

import (
	"mime"
	"net/http"
	"society.local/portal/internal/database"
	"strconv"
)

func (s *Server) collectionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/collections", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.FundCampaignsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("home"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/collections/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		lines, ok := queryPage(w, r, "line_page")
		if !ok {
			return
		}
		events, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		out, err := s.Store.FundCampaignFor(r.Context(), sessionToken(r), r.PathValue("id"), lines, events)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/collections", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FundInput
		if !decodeLimit(w, r, &in, 98304) {
			return
		}
		id, err := s.Store.CreateFundCampaign(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/collections/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FundAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideFundCampaign(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/payment-reports", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.FundReportsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("campaign"), q.Get("home"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/payment-reports", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FundReportInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.SaveFundReport(r.Context(), sessionToken(r), "", in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/payment-reports/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		out, err := s.Store.FundReportFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/payment-reports/{id}/revision", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FundReportInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.SaveFundReport(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/payment-reports/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FundReportDecision
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideFundReport(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/payment-reports/{id}/options", s.protected(func(w http.ResponseWriter, r *http.Request) {
		out, err := s.Store.FundConfirmOptionsFor(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("GET /api/payment-reports/{id}/evidence", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, data, err := s.Store.FundReportEvidenceFor(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", x.Type)
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": x.Filename}))
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	mux.HandleFunc("GET /api/fund-waivers", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.FundWaiversFor(r.Context(), sessionToken(r), q.Get("campaign"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/fund-waivers", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FundWaiverInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ProposeFundWaiver(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fund-waivers/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		out, err := s.Store.FundWaiverFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/fund-waivers/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FundAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideFundWaiver(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/fund-contributions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := s.Store.FundContributionsFor(r.Context(), sessionToken(r), q.Get("campaign"), q.Get("home"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, out)
	}))
	mux.HandleFunc("POST /api/fund-contributions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.FundAttributionInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.AttributeFundCredit(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/fund-contributions/{id}/reverse", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.AllocationCorrection
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.CorrectFundContribution(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
}
