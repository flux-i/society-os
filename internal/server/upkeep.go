package server

import (
	"net/http"
	"society.local/portal/internal/database"
)

func (s *Server) upkeepRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/upkeep", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		data, err := s.Store.UpkeepTasksFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/upkeep/options", s.protected(func(w http.ResponseWriter, r *http.Request) {
		data, err := s.Store.UpkeepOptionsFor(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/upkeep/tasks", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.UpkeepTaskInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.CreateUpkeepTask(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/upkeep/tasks/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		data, err := s.Store.UpkeepTaskFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/upkeep/tasks/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.UpkeepAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.UpdateUpkeepTask(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/upkeep/register/{kind}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "page")
		if !ok {
			return
		}
		q := r.URL.Query()
		data, err := s.Store.UpkeepRegisterFor(r.Context(), sessionToken(r), r.PathValue("kind"), q.Get("q"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/upkeep/register/{kind}/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, ok := queryPage(w, r, "event_page")
		if !ok {
			return
		}
		data, err := s.Store.UpkeepRegisterDetailFor(r.Context(), sessionToken(r), r.PathValue("kind"), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	save := s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.UpkeepRegisterInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.SaveUpkeepRegister(r.Context(), sessionToken(r), r.PathValue("kind"), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	})
	mux.HandleFunc("POST /api/upkeep/register/{kind}", save)
	mux.HandleFunc("POST /api/upkeep/register/{kind}/{id}", save)
}
