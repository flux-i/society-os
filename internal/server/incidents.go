package server

import (
	"io"
	"mime"
	"net/http"
	"net/url"
	"society.local/portal/internal/database"
	"strconv"
)

func incidentPage(r *http.Request, key string) (int, error) {
	if r.URL.Query().Get(key) == "" {
		return 1, nil
	}
	n, e := strconv.Atoi(r.URL.Query().Get(key))
	if e != nil || n < 1 || n > 100000 {
		return 0, database.ErrInvalid
	}
	return n, nil
}
func (s *Server) incidentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/rules", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, e := incidentPage(r, "page")
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		q := r.URL.Query()
		x, e := s.Store.RulesFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("state"), page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/rules/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, e := incidentPage(r, "event_page")
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		x, e := s.Store.RuleFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("POST /api/rules", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.RuleInput
		if !decodeLimit(w, r, &in, 32768) {
			return
		}
		id, e := s.Store.CreateRule(r.Context(), sessionToken(r), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/rules/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.RuleAction
		if !decodeLimit(w, r, &in, 16384) {
			return
		}
		id, e := s.Store.DecideRule(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/incidents/options", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, e := s.Store.IncidentReportOptions(r.Context(), sessionToken(r), r.URL.Query().Get("q"))
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/incidents", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, e := incidentPage(r, "page")
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		q := r.URL.Query()
		x, e := s.Store.IncidentsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("state"), page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/incidents/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, e := incidentPage(r, "event_page")
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		x, e := s.Store.IncidentFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, x)
	}))
	save := func(revision bool) http.HandlerFunc {
		return s.protected(func(w http.ResponseWriter, r *http.Request) {
			var in database.IncidentInput
			if !decodeLimit(w, r, &in, 32768) {
				return
			}
			id := ""
			if revision {
				id = r.PathValue("id")
			}
			id, e := s.Store.SaveIncident(r.Context(), sessionToken(r), id, in)
			if e != nil {
				s.resultError(w, r, e)
				return
			}
			respond(w, 200, map[string]string{"id": id})
		})
	}
	mux.HandleFunc("POST /api/incidents", save(false))
	mux.HandleFunc("POST /api/incidents/{id}/revision", save(true))
	mux.HandleFunc("POST /api/incidents/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.IncidentAction
		if !decodeLimit(w, r, &in, 32768) {
			return
		}
		id, e := s.Store.ActOnIncident(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/incident-notices", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, e := incidentPage(r, "page")
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		x, e := s.Store.IncidentNoticesFor(r.Context(), sessionToken(r), r.URL.Query().Get("q"), page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("GET /api/incident-notices/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, e := incidentPage(r, "response_page")
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		x, e := s.Store.IncidentNoticeFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, x)
	}))
	mux.HandleFunc("POST /api/incident-notices/{id}/responses", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.IncidentResponseInput
		if !decodeLimit(w, r, &in, 32768) {
			return
		}
		id, e := s.Store.RespondToIncident(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	slots := make(chan struct{}, 2)
	mux.HandleFunc("POST /api/incident-pictures", s.protected(func(w http.ResponseWriter, r *http.Request) {
		if _, e := s.Store.IncidentReportOptions(r.Context(), sessionToken(r), ""); e != nil {
			s.resultError(w, r, e)
			return
		}
		if r.Header.Get("Content-Type") != "application/octet-stream" {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			w.Header().Set("Retry-After", "2")
			respond(w, 429, map[string]string{"error": "uploads_busy", "message": "Two pictures are already being checked. Wait a moment and retry this picture."})
			return
		}
		data, e := io.ReadAll(http.MaxBytesReader(w, r.Body, database.MaxIncidentPictureBytes))
		if e != nil {
			respond(w, 400, map[string]string{"error": "invalid_picture", "message": "The picture was interrupted or exceeded 5 MiB. Choose a smaller readable original and retry."})
			return
		}
		filename, e := url.PathUnescape(r.Header.Get("X-Picture-Filename"))
		if e != nil {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		x, e := s.Store.UploadIncidentPicture(r.Context(), sessionToken(r), r.Header.Get("X-Operation-Key"), filename, data)
		if e != nil {
			s.resultError(w, r, e)
			return
		}
		respond(w, 200, x)
	}))
	picture := func(original bool) http.HandlerFunc {
		return s.protected(func(w http.ResponseWriter, r *http.Request) {
			x, e := s.Store.IncidentPictureFor(r.Context(), sessionToken(r), r.PathValue("id"), original)
			if e != nil {
				s.resultError(w, r, e)
				return
			}
			w.Header().Set("Content-Type", x.MediaType)
			disposition := "inline"
			filename := "incident-preview.png"
			if x.MediaType == "image/jpeg" {
				filename = "incident-preview.jpg"
			}
			if original {
				disposition = "attachment"
				filename = x.Filename
			}
			w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": filename}))
			w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
			w.Header().Set("Content-Length", strconv.Itoa(len(x.Bytes)))
			w.WriteHeader(200)
			_, _ = w.Write(x.Bytes)
		})
	}
	mux.HandleFunc("GET /api/incident-pictures/{id}/preview", picture(false))
	mux.HandleFunc("GET /api/incident-pictures/{id}/original", picture(true))
}
