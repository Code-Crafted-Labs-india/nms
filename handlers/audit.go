package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"nms-middleware/db"
)

type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *auditResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// AuditActivity records authenticated API request metadata after the handler
// completes. It deliberately excludes request bodies, query strings and client
// addresses. It is activity logging, not a tamper-proof compliance ledger.
func AuditActivity(database *db.DB, logger *slog.Logger, action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured := &auditResponseWriter{ResponseWriter: w}
		next.ServeHTTP(captured, r)
		status := captured.status
		if status == 0 {
			status = http.StatusOK
		}
		actor := "unknown"
		if sess, ok := r.Context().Value(sessionContextKey{}).(session); ok && sess.Actor != "" {
			actor = sess.Actor
		}
		outcome := "success"
		if status >= http.StatusBadRequest {
			outcome = "failure"
		}
		if err := database.RecordAuditEvent(r.Context(), actor, action, outcome, r.Method, r.URL.Path, status); err != nil {
			logger.Error("failed to persist API audit event", "action", action, "status", status, "error", err)
		}
	})
}

type auditEventView struct {
	ID       int64     `json:"id"`
	Occurred time.Time `json:"occurred_at"`
	Actor    string    `json:"actor"`
	Action   string    `json:"action"`
	Outcome  string    `json:"outcome"`
	Method   string    `json:"method"`
	Path     string    `json:"path"`
	Status   int       `json:"status_code"`
}

// AuditEventsHandler returns a bounded, newest-first page of audit activity.
func AuditEventsHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := parsePositiveInt(r.URL.Query().Get("page"), 1)
		pageSize := parsePositiveInt(r.URL.Query().Get("page_size"), 50)
		if pageSize > 100 {
			pageSize = 100
		}
		offset := (page - 1) * pageSize
		rows, err := database.Pool.Query(r.Context(), `
			SELECT id, occurred_at, actor, action, outcome, method, path, status_code
			FROM audit_events ORDER BY occurred_at DESC, id DESC LIMIT $1 OFFSET $2
		`, pageSize, offset)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "audit events unavailable"})
			return
		}
		defer rows.Close()
		events := make([]auditEventView, 0, pageSize)
		for rows.Next() {
			var event auditEventView
			if err := rows.Scan(&event.ID, &event.Occurred, &event.Actor, &event.Action, &event.Outcome, &event.Method, &event.Path, &event.Status); err != nil {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "audit events unavailable"})
				return
			}
			events = append(events, event)
		}
		if err := rows.Err(); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "audit events unavailable"})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"events": events, "page": page, "page_size": pageSize})
	}
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
