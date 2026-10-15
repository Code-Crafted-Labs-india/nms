package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"nms-middleware/db"
)

// deviceModePredicate returns a SQL scope for an internal, fixed alias. Test
// mode selects mock-* hostnames; production mode selects all other devices.
func deviceModePredicate(alias string) string {
	return fmt.Sprintf(`%[1]s.device_type <> 'unknown_discovered' AND ((SELECT setting_value FROM nms_runtime_settings WHERE setting_key = 'monitoring_mode') = 'test' AND LEFT(LOWER(%[1]s.hostname), 5) = 'mock-'
		OR (SELECT setting_value FROM nms_runtime_settings WHERE setting_key = 'monitoring_mode') = 'production' AND LEFT(LOWER(%[1]s.hostname), 5) <> 'mock-')`, alias)
}

type monitoringModePayload struct {
	Mode             string `json:"mode"`
	TestDeviceCount  int    `json:"testDeviceCount"`
	LiveDeviceCount  int    `json:"liveDeviceCount"`
}

func currentMonitoringMode(database *db.DB, r *http.Request) (monitoringModePayload, error) {
	var result monitoringModePayload
	err := database.Pool.QueryRow(r.Context(), `
		SELECT setting_value,
		       (SELECT COUNT(*) FROM devices WHERE is_monitored AND device_type <> 'unknown_discovered' AND LEFT(LOWER(hostname), 5) = 'mock-'),
		       (SELECT COUNT(*) FROM devices WHERE is_monitored AND device_type <> 'unknown_discovered' AND LEFT(LOWER(hostname), 5) <> 'mock-')
		FROM nms_runtime_settings WHERE setting_key = 'monitoring_mode'
	`).Scan(&result.Mode, &result.TestDeviceCount, &result.LiveDeviceCount)
	return result, err
}

// MonitoringModeHandler reads or changes the global device scope used by the
// polling workers. The setting is persisted so restarts keep the chosen scope.
func MonitoringModeHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if setupCORS(w, r) {
			return
		}

		if r.Method == http.MethodGet {
			result, err := currentMonitoringMode(database, r)
			if err != nil {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "monitoring mode unavailable"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}

		var payload monitoringModePayload
		if err := decodeJSON(w, r, &payload); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
			return
		}
		if payload.Mode != "test" && payload.Mode != "production" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "mode must be 'test' or 'production'"})
			return
		}

		var result monitoringModePayload
		err := database.Pool.QueryRow(r.Context(), `
			WITH updated AS (
				UPDATE nms_runtime_settings
				SET setting_value = $1, updated_at = NOW()
				WHERE setting_key = 'monitoring_mode'
				RETURNING setting_value
			)
			SELECT updated.setting_value,
			       (SELECT COUNT(*) FROM devices WHERE is_monitored AND device_type <> 'unknown_discovered' AND LEFT(LOWER(hostname), 5) = 'mock-'),
			       (SELECT COUNT(*) FROM devices WHERE is_monitored AND device_type <> 'unknown_discovered' AND LEFT(LOWER(hostname), 5) <> 'mock-')
			FROM updated
		`, payload.Mode).Scan(&result.Mode, &result.TestDeviceCount, &result.LiveDeviceCount)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "unable to change monitoring mode"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}
