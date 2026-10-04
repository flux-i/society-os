package server

import "net/http"

func (s *Server) overviewRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/overview/{section}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		data, err := s.Store.OverviewFor(r.Context(), sessionToken(r), r.PathValue("section"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, data)
	}))
}
