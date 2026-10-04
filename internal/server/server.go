package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"society.local/portal/internal/database"
	"society.local/portal/internal/documents"
)

type Server struct {
	Store     *database.Store
	Logger    *slog.Logger
	Version   string
	Web       fs.FS
	Documents *documents.Store
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	guard := newLoginGuard()
	s.accountRoutes(mux, guard)
	s.recordRoutes(mux)
	s.reviewRoutes(mux)
	s.complaintRoutes(mux)
	s.documentRoutes(mux)
	s.overviewRoutes(mux)
	mux.HandleFunc("POST /api/auth/login", s.login(guard))
	mux.HandleFunc("GET /api/auth/me", s.protected(func(w http.ResponseWriter, r *http.Request) { respond(w, 200, currentPrincipal(r)) }))
	mux.HandleFunc("POST /api/auth/logout", s.protected(s.logout))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := s.Store.Ready(ctx); err != nil {
			respond(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		if err := s.Store.VerifyMFAKey(ctx); err != nil {
			respond(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		respond(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/system", s.protected(func(w http.ResponseWriter, r *http.Request) {
		if !currentPrincipal(r).CanManageRegistry {
			s.resultError(w, r, database.ErrForbidden)
			return
		}
		engine, err := s.Store.Engine(r.Context())
		if err != nil {
			s.failure(w, r)
			return
		}
		respond(w, http.StatusOK, map[string]any{"application_version": s.Version, "schema_version": database.SchemaVersion, "engine": engine, "mode": "synthetic-preview"})
	}))
	mux.HandleFunc("GET /api/registry/summary", s.protected(func(w http.ResponseWriter, r *http.Request) {
		summary, err := s.Store.SummaryFor(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, summary)
	}))
	mux.HandleFunc("GET /api/flats", s.protected(s.flats))
	mux.HandleFunc("GET /api/flats/{id}", s.protected(func(w http.ResponseWriter, r *http.Request) {
		flat, err := s.Store.FlatFor(r.Context(), r.PathValue("id"), sessionToken(r))
		if errors.Is(err, sql.ErrNoRows) {
			respond(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, http.StatusOK, flat)
	}))
	mux.HandleFunc("PATCH /api/flats/{id}", s.protected(s.changeOccupancy))
	mux.HandleFunc("POST /api/flats/{id}/members", s.protected(s.addMember))
	mux.HandleFunc("POST /api/flats/{id}/members/{membership}/end", s.protected(s.endMember))
	mux.HandleFunc("GET /api/flats/{id}/activity", s.protected(s.activity))
	mux.HandleFunc("GET /api/people", s.protected(s.people))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		respond(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	})
	files := http.FileServerFS(s.Web)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		files.ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		if !localHost(r.Host) {
			respond(wrapped, http.StatusForbidden, map[string]string{"error": "local_preview_only"})
		} else if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOrigin(r) {
			respond(wrapped, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		} else {
			mux.ServeHTTP(wrapped, r)
		}
		// Only log the route template, never query terms, names or opaque record IDs.
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		s.Logger.Info("http_request", "method", r.Method, "route", route, "status", wrapped.status, "duration_ms", float64(time.Since(start).Microseconds())/1000)
	})
}

func localHost(hostport string) bool {
	host := hostport
	if value, _, err := net.SplitHostPort(hostport); err == nil {
		host = value
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func ValidateAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return errors.New("address must include a loopback IP and port")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("synthetic preview must bind to a loopback IP")
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return errors.New("invalid port")
	}
	return nil
}

func (s *Server) failure(w http.ResponseWriter, r *http.Request) {
	s.Logger.Warn("database_request_failed", "route", r.Pattern)
	respond(w, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
}

func (s *Server) flats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := database.FlatFilter{Query: strings.TrimSpace(q.Get("q")), Building: q.Get("building"), Status: q.Get("status"), Page: 1, PageSize: 12}
	if len(filter.Query) > 100 || (filter.Building != "" && filter.Building != "A" && filter.Building != "B" && filter.Building != "C") ||
		(filter.Status != "" && filter.Status != "OWNER_OCCUPIED" && filter.Status != "RENTED" && filter.Status != "VACANT") {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
		return
	}
	for key, target := range map[string]*int{"page": &filter.Page, "page_size": &filter.PageSize} {
		if value := q.Get(key); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || key == "page_size" && n > 50 || key == "page" && n > 10000 {
				respond(w, http.StatusBadRequest, map[string]string{"error": "invalid_pagination"})
				return
			}
			*target = n
		}
	}
	result, err := s.Store.FlatsFor(r.Context(), filter, sessionToken(r))
	if err != nil {
		s.resultError(w, r, err)
		return
	}
	respond(w, http.StatusOK, result)
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
