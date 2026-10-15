package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"nms-middleware/db"
)

func configurationHandlerDB() *db.DB {
	return &db.DB{Pool: &pgxpool.Pool{}}
}

func TestConfigurationHandlersRejectInvalidHTTPRequests(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		method      string
		body        string
		contentType string
		origin      string
		want        int
	}{
		{
			name:    "configuration device endpoint rejects post",
			handler: ConfigurationDevicesHandler(configurationHandlerDB()),
			method:  http.MethodPost,
			want:    http.StatusMethodNotAllowed,
		},
		{
			name:    "configuration plan endpoint rejects unsupported method",
			handler: ConfigurationPlansHandler(configurationHandlerDB()),
			method:  http.MethodPut,
			want:    http.StatusMethodNotAllowed,
		},
		{
			name:    "plan endpoint rejects malformed JSON before database access",
			handler: ConfigurationPlansHandler(configurationHandlerDB()),
			method:  http.MethodPost,
			body:    "{",
			want:    http.StatusBadRequest,
		},
		{
			name:        "plan endpoint rejects non JSON content type",
			handler:     ConfigurationPlansHandler(configurationHandlerDB()),
			method:      http.MethodPost,
			body:        `{}`,
			contentType: "text/plain",
			want:        http.StatusBadRequest,
		},
		{
			name:    "plan endpoint rejects unknown JSON field",
			handler: ConfigurationPlansHandler(configurationHandlerDB()),
			method:  http.MethodPost,
			body:    `{"device_id":1,"unexpected":true}`,
			want:    http.StatusBadRequest,
		},
		{
			name:    "plan endpoint rejects trailing JSON value",
			handler: ConfigurationPlansHandler(configurationHandlerDB()),
			method:  http.MethodPost,
			body:    `{"device_id":1} {"device_id":2}`,
			want:    http.StatusBadRequest,
		},
		{
			name:    "plan endpoint rejects missing device before database access",
			handler: ConfigurationPlansHandler(configurationHandlerDB()),
			method:  http.MethodPost,
			body:    `{"device_id":0}`,
			want:    http.StatusUnprocessableEntity,
		},
		{
			name:    "configuration endpoint rejects cross-origin request",
			handler: ConfigurationDevicesHandler(configurationHandlerDB()),
			method:  http.MethodPost,
			origin:  "https://attacker.invalid",
			want:    http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, "http://console.local/api/v1/configuration", strings.NewReader(tt.body))
			request.Host = "console.local"
			request.Header.Set("Origin", "http://console.local")
			if tt.body != "" {
				contentType := tt.contentType
				if contentType == "" {
					contentType = "application/json"
				}
				request.Header.Set("Content-Type", contentType)
			}
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}
			recorder := httptest.NewRecorder()
			tt.handler(recorder, request)
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d; body: %s", recorder.Code, tt.want, recorder.Body.String())
			}
		})
	}
}

func TestConfigurationPlanRejectsOversizedRequestBody(t *testing.T) {
	body := `{"device_id":1,"padding":"` + strings.Repeat("x", 64<<10) + `"}`
	request, recorder := configurationHTTPRequest(http.MethodPost, body)
	configurationPlansHandler(nil).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("oversized body status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestConfigurationHandlersAcceptSameOriginPreflight(t *testing.T) {
	handlers := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "devices", handler: ConfigurationDevicesHandler(configurationHandlerDB())},
		{name: "plans", handler: ConfigurationPlansHandler(configurationHandlerDB())},
	}
	for _, entry := range handlers {
		t.Run(entry.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, "http://console.local/api/v1/configuration", nil)
			request.Host = "console.local"
			request.Header.Set("Origin", "http://console.local")
			recorder := httptest.NewRecorder()
			entry.handler(recorder, request)
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("preflight status = %d, want %d", recorder.Code, http.StatusNoContent)
			}
		})
	}
}
