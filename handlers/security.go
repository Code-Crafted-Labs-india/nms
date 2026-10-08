package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

type sessionContextKey struct{}

type session struct {
	CSRF      string
	Actor     string
	ExpiresAt time.Time
}

type AuditRecorder func(context.Context, string, string, string, string, string, int) error

// SessionStore keeps short-lived, opaque sessions in process memory. A restart
// invalidates every session, which is a safe failure mode for an operator UI.
type SessionStore struct {
	mu        sync.RWMutex
	sessions  map[string]session
	tokenHash [32]byte
	ttl       time.Duration
	audit     AuditRecorder
}

func NewSessionStore(bootstrapToken string) (*SessionStore, error) {
	if len(bootstrapToken) < 32 {
		return nil, errors.New("NMS_BOOTSTRAP_TOKEN must contain at least 32 characters")
	}
	return &SessionStore{
		sessions:  make(map[string]session),
		tokenHash: sha256.Sum256([]byte(bootstrapToken)),
		ttl:       8 * time.Hour,
	}, nil
}

func (s *SessionStore) SetAuditRecorder(recorder AuditRecorder) {
	s.audit = recorder
}

func (s *SessionStore) recordAudit(actor, action, outcome, method, path string, status int) {
	if s.audit == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.audit(ctx, actor, action, outcome, method, path, status)
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *SessionStore) HandleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if !sameOrigin(r) {
			s.recordAudit("anonymous", "auth.login", "failure", r.Method, r.URL.Path, http.StatusForbidden)
			jsonResponse(w, http.StatusForbidden, map[string]string{"error": "origin rejected"})
			return
		}
		var credentials struct {
			Token string `json:"token"`
		}
		if err := decodeJSON(w, r, &credentials); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		candidate := sha256.Sum256([]byte(credentials.Token))
		if subtle.ConstantTimeCompare(candidate[:], s.tokenHash[:]) != 1 {
			s.recordAudit("anonymous", "auth.login", "failure", r.Method, r.URL.Path, http.StatusUnauthorized)
			time.Sleep(250 * time.Millisecond)
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		id, err := randomToken()
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "session unavailable"})
			return
		}
		csrf, err := randomToken()
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "session unavailable"})
			return
		}
		expires := time.Now().Add(s.ttl)
		s.mu.Lock()
		s.sessions[id] = session{CSRF: csrf, Actor: "bootstrap-admin", ExpiresAt: expires}
		s.mu.Unlock()
		s.recordAudit("bootstrap-admin", "auth.login", "success", r.Method, r.URL.Path, http.StatusOK)
		http.SetCookie(w, &http.Cookie{
			Name: cookieName(r), Value: id, Path: "/", HttpOnly: true,
			Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode,
			Expires: expires, MaxAge: int(s.ttl.Seconds()),
		})
		writeJSON(w, http.StatusOK, map[string]string{"csrfToken": csrf, "role": "administrator"})
	case http.MethodGet:
		sess, ok := s.current(r)
		if !ok {
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"csrfToken": sess.CSRF, "role": "administrator"})
	case http.MethodDelete:
		sess, ok := s.current(r)
		provided := r.Header.Get("X-CSRF-Token")
		if !ok || !sameOrigin(r) || subtle.ConstantTimeCompare([]byte(provided), []byte(sess.CSRF)) != 1 {
			actor := "anonymous"
			if ok {
				actor = sess.Actor
			}
			s.recordAudit(actor, "auth.logout", "failure", r.Method, r.URL.Path, http.StatusForbidden)
			jsonResponse(w, http.StatusForbidden, map[string]string{"error": "request verification failed"})
			return
		}
		cookie, _ := r.Cookie(cookieName(r))
		if cookie != nil {
			s.mu.Lock()
			delete(s.sessions, cookie.Value)
			s.mu.Unlock()
		}
		s.recordAudit(sess.Actor, "auth.logout", "success", r.Method, r.URL.Path, http.StatusNoContent)
		http.SetCookie(w, &http.Cookie{Name: cookieName(r), Value: "", Path: "/", HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode, MaxAge: -1})
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *SessionStore) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.current(r)
		if !ok {
			s.recordAudit("anonymous", "auth.request", "failure", r.Method, r.URL.Path, http.StatusUnauthorized)
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			provided := r.Header.Get("X-CSRF-Token")
			if !sameOrigin(r) || subtle.ConstantTimeCompare([]byte(provided), []byte(sess.CSRF)) != 1 {
				s.recordAudit(sess.Actor, "auth.csrf_rejected", "failure", r.Method, r.URL.Path, http.StatusForbidden)
				jsonResponse(w, http.StatusForbidden, map[string]string{"error": "request verification failed"})
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, sess)))
	})
}

func (s *SessionStore) current(r *http.Request) (session, bool) {
	cookie, err := r.Cookie(cookieName(r))
	if err != nil || cookie.Value == "" {
		return session{}, false
	}
	s.mu.RLock()
	sess, ok := s.sessions[cookie.Value]
	s.mu.RUnlock()
	if !ok || time.Now().After(sess.ExpiresAt) {
		if ok {
			s.mu.Lock()
			delete(s.sessions, cookie.Value)
			s.mu.Unlock()
		}
		return session{}, false
	}
	return sess, true
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if r.Header.Get("Content-Type") != "application/json" {
		return errors.New("content type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	return strings.EqualFold(origin, scheme+"://"+r.Host)
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func cookieName(r *http.Request) string {
	if isHTTPS(r) {
		return "__Host-nms_session"
	}
	return "nms_session_dev"
}
