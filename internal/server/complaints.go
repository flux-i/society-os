package server

import (
	"net/http"
	"society.local/portal/internal/database"
	"strconv"
)

func complaintPage(r *http.Request, name string) (int, error) {
	if r.URL.Query().Get(name) == "" {
		return 1, nil
	}
	return strconv.Atoi(r.URL.Query().Get(name))
}
func (s *Server) complaintRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/complaints", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, err := complaintPage(r, "page")
		if err != nil {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		q := r.URL.Query()
		data, err := s.Store.ComplaintsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("status"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/complaints/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, err := complaintPage(r, "history_page")
		if err != nil {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		data, err := s.Store.ComplaintFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/complaints", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.ComplaintInput
		if !decodeLimit(w, r, &in, 32768) {
			return
		}
		id, err := s.Store.CreateComplaint(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/complaints/{id}/updates", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.ComplaintAction
		if !decodeLimit(w, r, &in, 16384) {
			return
		}
		id, err := s.Store.UpdateComplaint(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
}
