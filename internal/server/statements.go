package server

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"society.local/portal/internal/database"
)

func (s *Server) statementRoutes(mux *http.ServeMux) {
	uploadSlots := make(chan struct{}, 2)
	mux.HandleFunc("GET /api/financial-statements/targets", s.protected(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, err := documentPage(q.Get("page"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		data, err := s.Store.StatementTargetsFor(r.Context(), sessionToken(r), q.Get("target"), q.Get("q"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/financial-statements", s.protected(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, err := documentPage(q.Get("page"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		data, err := s.Store.StatementsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("kind"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/financial-statements", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.StatementInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ReserveStatement(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/financial-statements/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		pages := make([]int, 3)
		for i, key := range []string{"event_page", "version_page", "publication_page"} {
			page, err := documentPage(r.URL.Query().Get(key))
			if err != nil {
				s.resultError(w, r, err)
				return
			}
			pages[i] = page
		}
		data, err := s.Store.StatementFor(r.Context(), sessionToken(r), r.PathValue("id"), pages[0], pages[1], pages[2])
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/financial-statements/{id}/content", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.StatementFor(r.Context(), sessionToken(r), r.PathValue("id"), 1, 1, 1)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		if x.UploaderID != currentPrincipal(r).ID {
			s.resultError(w, r, database.ErrForbidden)
			return
		}
		if !x.CanUpload && x.UploadedAt == 0 {
			s.resultError(w, r, database.ErrConflict)
			return
		}
		select {
		case uploadSlots <- struct{}{}:
			defer func() { <-uploadSlots }()
		default:
			w.Header().Set("Retry-After", "2")
			respond(w, 429, map[string]string{"error": "uploads_busy", "message": "Two uploads are in progress. Retry the unchanged original shortly."})
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10*1024*1024))
		if err != nil {
			respond(w, 400, map[string]string{"error": "invalid_upload", "message": "The upload was interrupted or exceeded 10 MiB. Retry the unchanged original."})
			return
		}
		if err = s.Store.CompleteStatement(r.Context(), sessionToken(r), x.ID, data); err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": x.ID})
	}))
	mux.HandleFunc("POST /api/financial-statements/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.StatementAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideStatement(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/financial-statements/publication-preview", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.StatementPublicationInput
		if !decodeLimit(w, r, &in, 32768) {
			return
		}
		data, err := s.Store.PreviewStatementPublication(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/financial-statements/publications", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.StatementPublicationInput
		if !decodeLimit(w, r, &in, 32768) {
			return
		}
		id, err := s.Store.ProposeStatementPublication(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/financial-statements/publications/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.StatementAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideStatementPublication(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/financial-statements/{id}/download", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, data, err := s.Store.DownloadStatement(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", x.ContentType)
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": x.Filename}))
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
}
