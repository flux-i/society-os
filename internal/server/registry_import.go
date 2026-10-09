package server

import (
	"net/http"
	"society.local/portal/internal/database"
)

func (s *Server) registryImportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/workspace", func(w http.ResponseWriter, r *http.Request) {
		info, err := s.Store.WorkspaceInfo(r.Context())
		if err != nil {
			s.failure(w, r)
			return
		}
		respond(w, 200, info)
	})
	mux.HandleFunc("GET /api/registry/import", s.protected(func(w http.ResponseWriter, r *http.Request) {
		status, err := s.Store.RegistryImportStatus(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, status)
	}))
	mux.HandleFunc("POST /api/registry/import/preview", s.protected(func(w http.ResponseWriter, r *http.Request) {
		if !currentPrincipal(r).CanManageRegistry {
			s.resultError(w, r, database.ErrForbidden)
			return
		}
		var input struct {
			InputText string `json:"input_text"`
		}
		if !decodeLimit(w, r, &input, 8*1024*1024) {
			return
		}
		preview, err := s.Store.PreviewRegistryImport(r.Context(), sessionToken(r), input.InputText)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, preview)
	}))
	mux.HandleFunc("POST /api/registry/import/apply", s.protected(func(w http.ResponseWriter, r *http.Request) {
		if !currentPrincipal(r).CanManageRegistry {
			s.resultError(w, r, database.ErrForbidden)
			return
		}
		var input database.ApplyRegistryImport
		if !decodeLimit(w, r, &input, 8*1024*1024) {
			return
		}
		result, err := s.Store.ApplyRegistryImport(r.Context(), sessionToken(r), input)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, result)
	}))
}
