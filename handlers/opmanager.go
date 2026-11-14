package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"nms-middleware/opmanager"
)

// OpManagerDevicesHandler proxies a bounded, read-only page of the
// commercial NMS inventory. The integration API key is never returned.
func OpManagerDevicesHandler(client *opmanager.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if client == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "OpManager integration is not configured",
			})
			return
		}

		page := parsePositiveInt(r.URL.Query().Get("page"), 1)
		pageSize := parsePositiveInt(r.URL.Query().Get("page_size"), 50)
		if pageSize > 100 {
			pageSize = 100
		}

		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()

		result, err := client.ListDevices(ctx, page, pageSize)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				writeJSON(w, http.StatusGatewayTimeout, map[string]string{
					"error": "OpManager request timed out",
				})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": "OpManager inventory is unavailable",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"devices":   result.Rows,
			"total":     result.Total,
			"page":      result.Page,
			"page_size": pageSize,
		})
	}
}
