package server

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"society.local/portal/internal/database"
)

func (s *Server) documentRoutes(mux *http.ServeMux) {
	uploadSlots := make(chan struct{}, 2)
	mux.HandleFunc("GET /api/documents", s.protected(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, err := documentPage(q.Get("page"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		data, err := s.Store.DocumentsFor(r.Context(), sessionToken(r), q.Get("q"), q.Get("category"), q.Get("state"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("GET /api/documents/subjects", s.protected(func(w http.ResponseWriter, r *http.Request) {
		data, err := s.Store.DocumentSubjects(r.Context(), sessionToken(r), r.URL.Query().Get("q"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/documents", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.DocumentInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ReserveDocument(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/documents/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		page, err := documentPage(r.URL.Query().Get("history_page"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		data, err := s.Store.DocumentFor(r.Context(), sessionToken(r), r.PathValue("id"), page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/documents/{id}/content", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, err := s.Store.DocumentFor(r.Context(), sessionToken(r), r.PathValue("id"), 1)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		if x.AuthorID != currentPrincipal(r).ID {
			s.resultError(w, r, database.ErrForbidden)
			return
		}
		if !x.CanUpload && !x.Uploaded {
			s.resultError(w, r, database.ErrConflict)
			return
		}
		select {
		case uploadSlots <- struct{}{}:
			defer func() { <-uploadSlots }()
		default:
			w.Header().Set("Retry-After", "2")
			respond(w, 429, map[string]string{"error": "uploads_busy", "message": "Two uploads are already in progress. Wait a moment, then retry the same file."})
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, database.MaxUploadBytes))
		if err != nil {
			respond(w, 400, map[string]string{"error": "invalid_upload", "message": "The file upload was interrupted or exceeded the 20 MiB limit. Retry the original file."})
			return
		}
		if err = s.Store.CompleteDocument(r.Context(), sessionToken(r), x.ID, data); err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": x.ID})
	}))
	mux.HandleFunc("POST /api/documents/{id}/actions", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.DocumentAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DecideDocument(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/documents/{id}/download", s.protected(func(w http.ResponseWriter, r *http.Request) {
		x, data, err := s.Store.DownloadDocument(r.Context(), sessionToken(r), r.PathValue("id"))
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
}
func documentPage(value string) (int, error) {
	if value == "" {
		return 1, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 10000 {
		return 0, database.ErrInvalid
	}
	return n, nil
}
