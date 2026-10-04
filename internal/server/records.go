package server

import (
	"net/http"
	"strconv"

	"society.local/portal/internal/database"
)

func (s *Server) recordRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/entries/{id}/discard", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.EntryAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.DiscardDraft(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/entries", s.protected(func(w http.ResponseWriter, r *http.Request) {
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
		data, err := s.Store.EntriesFor(r.Context(), sessionToken(r), q.Get("home"), q.Get("q"), q.Get("state"), q.Get("receipts") == "true", page)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/entries", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.EntryInput
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.CreateEntry(r.Context(), sessionToken(r), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("GET /api/entries/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		data, err := s.Store.EntryFor(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, data)
	}))
	mux.HandleFunc("POST /api/entries/{id}/post", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.EntryAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.PostEntry(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/entries/{id}/reverse", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in database.EntryAction
		if !decode(w, r, &in) {
			return
		}
		id, err := s.Store.ReverseEntry(r.Context(), sessionToken(r), r.PathValue("id"), in)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"id": id})
	}))
	mux.HandleFunc("POST /api/receipts/{id}/retry", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var in struct{}
		if !decode(w, r, &in) {
			return
		}
		err := s.Store.RetryReceipt(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]bool{"queued": true})
	}))
	mux.HandleFunc("GET /api/receipts/{id}/download", s.protected(func(w http.ResponseWriter, r *http.Request) {
		entry, err := s.Store.ReceiptEntryFor(r.Context(), sessionToken(r), r.PathValue("id"))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		if entry.PDFState != "READY" || s.Documents == nil {
			respond(w, 409, map[string]string{"error": "receipt_pending", "message": "This receipt is still being prepared. Check again in a moment."})
			return
		}
		bytes, err := s.Documents.Read(entry.FileHash)
		if err != nil {
			s.failure(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+entry.ReceiptNumber+`.pdf"`)
		w.Header().Set("X-Receipt-Status", entry.State)
		w.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
		w.WriteHeader(200)
		_, _ = w.Write(bytes)
	}))
}
