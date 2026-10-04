package server

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"society.local/portal/internal/database"
)

func (s *Server) accountRoutes(mux *http.ServeMux, guard *loginGuard) {
	mux.HandleFunc("POST /api/auth/mfa/setup", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input struct{}
		if !decode(w, r, &input) {
			return
		}
		value, err := s.Store.SetupMFA(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, value)
	}))
	mux.HandleFunc("POST /api/auth/mfa/confirm", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Code string `json:"code"`
		}
		if !decode(w, r, &input) {
			return
		}
		if len(input.Code) > 64 {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		value, err := s.Store.ConfirmMFA(r.Context(), sessionToken(r), input.Code)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, value)
	}))
	mux.HandleFunc("POST /api/auth/mfa/verify", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Code     string `json:"code"`
			Recovery bool   `json:"recovery"`
		}
		if !decode(w, r, &input) {
			return
		}
		if len(input.Code) > 64 {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		value, err := s.Store.VerifyMFA(r.Context(), sessionToken(r), input.Code, input.Recovery)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, value)
	}))
	mux.HandleFunc("POST /api/auth/mfa/demo-code", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input struct{}
		if !decode(w, r, &input) {
			return
		}
		code, recovery, err := s.Store.DemoVerificationCode(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]any{"code": code, "recovery": recovery})
	}))
	mux.HandleFunc("POST /api/auth/reauthenticate", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Password string `json:"password"`
			Code     string `json:"code"`
			Recovery bool   `json:"recovery"`
		}
		if !decode(w, r, &input) {
			return
		}
		if len(input.Password) == 0 || len(input.Password) > 256 || len(input.Code) > 64 {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		done, ok := s.passwordWork(w, r, guard, currentPrincipal(r).ID)
		if !ok {
			return
		}
		defer done()
		value, err := s.Store.Reauthenticate(r.Context(), sessionToken(r), input.Password, input.Code, input.Recovery)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		guard.clear("password-" + database.TokenHash(currentPrincipal(r).ID))
		respond(w, 200, value)
	}))
	mux.HandleFunc("POST /api/auth/recovery-codes", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input struct{}
		if !decode(w, r, &input) {
			return
		}
		codes, err := s.Store.RegenerateRecovery(r.Context(), sessionToken(r))
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, map[string]any{"recovery_codes": codes})
	}))
	mux.HandleFunc("POST /api/auth/change-password", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Current string `json:"current_password"`
			New     string `json:"new_password"`
		}
		if !decode(w, r, &input) {
			return
		}
		if len(input.Current) == 0 || len(input.Current) > 256 || len(input.New) > 256 {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		done, ok := s.passwordWork(w, r, guard, currentPrincipal(r).ID)
		if !ok {
			return
		}
		defer done()
		if err := s.Store.ChangePassword(r.Context(), sessionToken(r), input.Current, input.New); err != nil {
			s.resultError(w, r, err)
			return
		}
		guard.clear("password-" + database.TokenHash(currentPrincipal(r).ID))
		clearCookie(w, r)
		respond(w, 200, map[string]string{"status": "password_changed"})
	}))
	mux.HandleFunc("GET /api/admin/accounts", s.protected(s.accounts))
	mux.HandleFunc("POST /api/admin/invitations", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input database.Invitation
		if !decode(w, r, &input) {
			return
		}
		value, err := s.Store.Invite(r.Context(), sessionToken(r), input)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 201, value)
	}))
	mux.HandleFunc("POST /api/admin/accounts/{id}/link", s.protected(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Purpose  string `json:"purpose"`
			Verified bool   `json:"identity_verified"`
			Note     string `json:"note"`
		}
		if !decode(w, r, &input) {
			return
		}
		value, err := s.Store.IssueAccountLink(r.Context(), sessionToken(r), r.PathValue("id"), input.Purpose, input.Verified, input.Note)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 201, value)
	}))
	mux.HandleFunc("POST /api/auth/link", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Token string `json:"token"`
		}
		if !decode(w, r, &input) {
			return
		}
		value, err := s.Store.InspectAccountLink(r.Context(), input.Token)
		if err != nil {
			s.resultError(w, r, err)
			return
		}
		respond(w, 200, value)
	})
	mux.HandleFunc("POST /api/auth/link/complete", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if !decode(w, r, &input) {
			return
		}
		if len(input.Token) != 43 {
			s.resultError(w, r, database.ErrLink)
			return
		}
		if len(input.Password) > 256 {
			s.resultError(w, r, database.ErrInvalid)
			return
		}
		done, ok := s.passwordWork(w, r, guard, input.Token)
		if !ok {
			return
		}
		defer done()
		if err := s.Store.CompleteAccountLink(r.Context(), input.Token, input.Password); err != nil {
			s.resultError(w, r, err)
			return
		}
		// Do not clear an unrelated account's cookie when a link is redeemed on this browser.
		respond(w, 200, map[string]string{"status": "password_saved"})
	})
}

func clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func (s *Server) passwordWork(w http.ResponseWriter, r *http.Request, guard *loginGuard, subject string) (func(), bool) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !guard.allow("ip-"+ip, 60) || !guard.allow("password-"+database.TokenHash(subject), 5) {
		w.Header().Set("Retry-After", "900")
		respond(w, 429, map[string]string{"error": "verification_rate_limited"})
		return nil, false
	}
	select {
	case guard.workers <- struct{}{}:
		return func() { <-guard.workers }, true
	default:
		respond(w, 429, map[string]string{"error": "login_busy"})
		return nil, false
	}
}
func (s *Server) accounts(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, size := 1, 12
	if len(q) > 100 {
		s.resultError(w, r, database.ErrInvalid)
		return
	}
	for key, target := range map[string]*int{"page": &page, "page_size": &size} {
		if value := r.URL.Query().Get(key); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || (key == "page_size" && n > 50) || (key == "page" && n > 10000) {
				s.resultError(w, r, database.ErrInvalid)
				return
			}
			*target = n
		}
	}
	value, err := s.Store.AccountsFor(r.Context(), sessionToken(r), q, page, size)
	if err != nil {
		s.resultError(w, r, err)
		return
	}
	respond(w, 200, value)
}
