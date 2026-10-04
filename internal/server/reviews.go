package server

import (
	"net/http"
	"society.local/portal/internal/database"
	"strconv"
)

func (s *Server) reviewRoutes(mux *http.ServeMux) {
	for _, base := range []string{"reviews", "notices"} {
		notices := base == "notices"
		mux.HandleFunc("GET /api/"+base, s.protected(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			page := 1
			if q.Get("page") != "" {
				var err error
				page, err = strconv.Atoi(q.Get("page"))
				if err != nil {
					s.resultError(w, r, database.ErrInvalid)
					return
				}
			}
			data, err := s.Store.ReviewsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("state"), page, notices)
			if err != nil {
				s.resultError(w, r, err)
				return
			}
			respond(w, 200, data)
		}))
		mux.HandleFunc("GET /api/"+base+"/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
			data, err := s.Store.ReviewFor(r.Context(), sessionToken(r), r.PathValue("id"), notices)
			if err != nil {
				s.resultError(w, r, err)
				return
			}
			respond(w, 200, data)
		}))
	}
	for _, path := range []string{"POST /api/reviews", "POST /api/reviews/{id}/resubmit"} {
		mux.HandleFunc(path, s.protected(func(w http.ResponseWriter, r *http.Request) {
			var in database.ReviewInput
			if !decodeLimit(w, r, &in, 32768) {
				return
			}
			id, err := s.Store.SubmitReview(r.Context(), sessionToken(r), r.PathValue("id"), in)
			if err != nil {
				s.resultError(w, r, err)
				return
			}
			respond(w, 200, map[string]string{"id": id})
		}))
	}
	mux.HandleFunc("POST /api/reviews/{id}/decision", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.ReviewAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideReview(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
}
