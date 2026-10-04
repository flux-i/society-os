package server

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"society.local/portal/internal/database"
)

const sessionCookie = "society_session"

type principalKey struct{}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}
func currentPrincipal(r *http.Request) database.Principal {
	p, _ := r.Context().Value(principalKey{}).(database.Principal)
	return p
}

func sameOrigin(r *http.Request) bool {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return r.Header.Get("Origin") == scheme+"://"+r.Host && r.Header.Get("Sec-Fetch-Site") != "cross-site"
}

func (s *Server) protected(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authenticate := s.Store.Authenticate
		if r.URL.Path == "/api/auth/me" {
			authenticate = s.Store.CheckSession
		}
		p, err := authenticate(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		if p.MFAPending && !strings.HasPrefix(r.URL.Path, "/api/auth/") {
			s.resultError(w, r, database.ErrMFARequired)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			csrf := r.Header.Get("X-CSRF-Token")
			if len(csrf) != 43 || subtle.ConstantTimeCompare([]byte(csrf), []byte(p.CSRF)) != 1 {
				respond(w, http.StatusForbidden, map[string]string{"error": "invalid_csrf"})
				return
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	}
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	return decodeLimit(w, r, value, 8192)
}

func decodeLimit(w http.ResponseWriter, r *http.Request, value any, limit int64) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		respond(w, 415, map[string]string{"error": "json_required"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		respond(w, 400, map[string]string{"error": "invalid_json"})
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		respond(w, 400, map[string]string{"error": "invalid_json"})
		return false
	}
	return true
}

type attempt struct {
	count int
	until time.Time
}
type loginGuard struct {
	mu        sync.Mutex
	attempts  map[string]attempt
	workers   chan struct{}
	dummyHash string
}

func newLoginGuard() *loginGuard {
	return &loginGuard{attempts: map[string]attempt{}, workers: make(chan struct{}, 2), dummyHash: database.HashPassword("fictional-dummy-" + time.Now().String())}
}
func (g *loginGuard) allow(key string, limit int) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	for k, a := range g.attempts {
		if now.After(a.until) {
			delete(g.attempts, k)
		}
	}
	a, exists := g.attempts[key]
	if (!exists && len(g.attempts) >= 4096) || a.count >= limit {
		return false
	}
	if !exists {
		a.until = now.Add(15 * time.Minute)
	}
	a.count++
	g.attempts[key] = a
	return true
}
func (g *loginGuard) clear(key string) { g.mu.Lock(); defer g.mu.Unlock(); delete(g.attempts, key) }

func (s *Server) login(guard *loginGuard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		if !decode(w, r, &input) {
			return
		}
		if len(input.Login) == 0 || len(input.Login) > 254 || len(input.Password) == 0 || len(input.Password) > 256 {
			respond(w, 400, map[string]string{"error": "invalid_login_input"})
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		key := "login-" + database.TokenHash(strings.ToLower(strings.TrimSpace(input.Login)))
		if !guard.allow("ip-"+ip, 60) || !guard.allow(key, 5) {
			w.Header().Set("Retry-After", "900")
			respond(w, 429, map[string]string{"error": "login_rate_limited"})
			return
		}
		select {
		case guard.workers <- struct{}{}:
			defer func() { <-guard.workers }()
		default:
			respond(w, 429, map[string]string{"error": "login_busy"})
			return
		}
		token, p, err := s.Store.Login(r.Context(), input.Login, input.Password, guard.dummyHash)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		guard.clear(key)
		// HttpOnly / Strict cookie. Secure is enabled under TLS; CLI only serves loopback HTTP.
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int(database.SessionAbsolute / time.Second)})
		respond(w, 200, p)
	}
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.Logout(r.Context(), sessionToken(r)); err != nil {
		s.resultError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	respond(w, 200, map[string]string{"status": "signed_out"})
}

func (s *Server) resultError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		respond(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(err, database.ErrMFARequired):
		respond(w, 403, map[string]string{"error": "mfa_required"})
	case errors.Is(err, database.ErrReauthRequired):
		respond(w, 403, map[string]string{"error": "reauthentication_required"})
	case errors.Is(err, database.ErrVerification):
		respond(w, 401, map[string]string{"error": "invalid_verification"})
	case errors.Is(err, database.ErrThrottled):
		w.Header().Set("Retry-After", "900")
		respond(w, 429, map[string]string{"error": "verification_rate_limited"})
	case errors.Is(err, database.ErrLink):
		respond(w, 400, map[string]string{"error": "invalid_link", "message": "This link has expired or has already been used. Ask the registry officer for a new one."})
	case errors.Is(err, database.ErrUnauthenticated):
		respond(w, 401, map[string]string{"error": "sign_in_required"})
	case errors.Is(err, database.ErrForbidden):
		respond(w, 403, map[string]string{"error": "permission_required"})
	case errors.Is(err, database.ErrConflict):
		respond(w, 409, map[string]string{"error": "record_changed"})
	case errors.Is(err, database.ErrInvalid):
		respond(w, 400, map[string]string{"error": "invalid_input", "message": strings.TrimPrefix(err.Error(), "invalid input: ")})
	default:
		s.failure(w, r)
	}
}
