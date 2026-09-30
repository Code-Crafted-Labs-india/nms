package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testBootstrapToken = "test-only-bootstrap-token-with-32-characters"

func TestSessionAuthenticationAndCSRF(t *testing.T) {
	store, err := NewSessionStore(testBootstrapToken)
	if err != nil {
		t.Fatal(err)
	}

	login := httptest.NewRequest(http.MethodPost, "http://console.local/api/session", strings.NewReader(`{"token":"`+testBootstrapToken+`"}`))
	login.Host = "console.local"
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("Origin", "http://console.local")
	recorder := httptest.NewRecorder()
	store.HandleSession(recorder, login)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login returned %d: %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie is missing required protections: %#v", cookies)
	}

	protected := store.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodDelete, "http://console.local/api/devices/1", nil)
	request.Host = "console.local"
	request.AddCookie(cookies[0])
	denied := httptest.NewRecorder()
	protected.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("mutation without CSRF token returned %d", denied.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://console.local/api/v1/dashboard", nil))

	for _, header := range []string{"Content-Security-Policy", "Cross-Origin-Opener-Policy", "X-Content-Type-Options", "X-Frame-Options", "Permissions-Policy"} {
		if recorder.Header().Get(header) == "" {
			t.Errorf("missing security header %s", header)
		}
	}
}

func TestRejectsCrossOriginLogin(t *testing.T) {
	store, _ := NewSessionStore(testBootstrapToken)
	request := httptest.NewRequest(http.MethodPost, "http://console.local/api/session", strings.NewReader(`{"token":"`+testBootstrapToken+`"}`))
	request.Host = "console.local"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://attacker.invalid")
	recorder := httptest.NewRecorder()
	store.HandleSession(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin login returned %d", recorder.Code)
	}
}
