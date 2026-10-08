package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"nms-middleware/db"
)

// apiLogger is a package-level structured logger for all HTTP handler events.
var apiLogger = slog.New(slog.NewTextHandler(os.Stdout, nil))

// DevicePayload is the JSON body contract for device create/update operations.
type DevicePayload struct {
	Hostname      string `json:"hostname"`
	IPAddress     string `json:"ip_address"`
	DeviceType    string `json:"device_type"`
	Location      string `json:"location"`
	SnmpCommunity string `json:"snmp_community"`
	SnmpVersion   string `json:"snmp_version"`
	IsMonitored   *bool  `json:"is_monitored"` // nil = default true on create
}

// jsonResponse writes a consistent JSON envelope and sets Content-Type.
// Must be called before WriteHeader to allow header mutation.
func jsonResponse(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// setupCORS deliberately supports same-origin requests only. The UI reverse
// proxies /api, so cross-origin browser access is unnecessary.
func setupCORS(w http.ResponseWriter, r *http.Request) bool {
	if !sameOrigin(r) {
		jsonResponse(w, http.StatusForbidden, map[string]string{"error": "origin rejected"})
		return true
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// PreflightHandler returns a bare 200 OK for explicit OPTIONS routes registered
// in main.go. This ensures preflight requests are answered even before the
// method-specific handler is matched by the ServeMux.
func PreflightHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setupCORS(w, r)
	}
}

// RequestLogger wraps a handler and logs the method and API path without
// retaining the operator's source address. Attach this in main.go via mux.Handle(...).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiLogger.Info("[API] Inbound request",
			"method", r.Method,
			"path", r.URL.Path,
		)
		next.ServeHTTP(w, r)
	})
}

// ============================================================================
// CRUD HANDLERS
// ============================================================================

func AddDeviceHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if setupCORS(w, r) {
			return
		}

		var payload DevicePayload
		if err := decodeJSON(w, r, &payload); err != nil {
			apiLogger.Error("[API] Failed to decode add device payload", "err", err)
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
			return
		}

		if payload.Hostname == "" || payload.IPAddress == "" || payload.DeviceType == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "hostname, ip_address, and device_type are required"})
			return
		}
		if payload.SnmpCommunity == "" {
			payload.SnmpCommunity = "public"
		}
		if payload.SnmpVersion == "" {
			payload.SnmpVersion = "v2c"
		}
		monitored := true
		if payload.IsMonitored != nil {
			monitored = *payload.IsMonitored
		}

		// Verify the device_type is in the supported catalog before inserting.
		var catalogCount int
		err := database.Pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM device_models WHERE model_name = $1`, payload.DeviceType,
		).Scan(&catalogCount)
		if err != nil || catalogCount == 0 {
			jsonResponse(w, http.StatusUnprocessableEntity, map[string]string{
				"error": "device_type '" + payload.DeviceType + "' is not in the supported hardware catalog",
			})
			return
		}

		_, err = database.Pool.Exec(r.Context(), `
			INSERT INTO devices (hostname, ip_address, device_type, location, snmp_community, snmp_version, is_monitored)
			VALUES ($1::text, $2::inet, $3::text, NULLIF($4,''), $5::text, $6::text, $7)
		`, payload.Hostname, payload.IPAddress, payload.DeviceType, payload.Location,
			payload.SnmpCommunity, payload.SnmpVersion, monitored)

		if err != nil {
			errStr := err.Error()
			apiLogger.Error("[API] Failed to insert device", "err", err)
			switch {
			case containsAny(errStr, "devices_hostname_key", "unique constraint", "hostname"):
				jsonResponse(w, http.StatusConflict, map[string]string{"error": "A device with that hostname already exists"})
			case containsAny(errStr, "devices_ip_address_key", "ip_address", "inet"):
				jsonResponse(w, http.StatusConflict, map[string]string{"error": "A device with that IP address already exists"})
			default:
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create device"})
			}
			return
		}

		jsonResponse(w, http.StatusCreated, map[string]string{"status": "success", "message": "Device enrolled successfully"})
	}
}

// containsAny returns true if s contains any of the provided substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func EditDeviceHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if setupCORS(w, r) {
			return
		}

		id := r.PathValue("id")
		if id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Device ID is required"})
			return
		}

		var payload DevicePayload
		if err := decodeJSON(w, r, &payload); err != nil {
			apiLogger.Error("[API] Failed to decode edit device payload", "err", err)
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
			return
		}

		if payload.Hostname == "" || payload.IPAddress == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "hostname and ip_address are required"})
			return
		}

		monitored := true
		if payload.IsMonitored != nil {
			monitored = *payload.IsMonitored
		}

		tag, err := database.Pool.Exec(r.Context(), `
			UPDATE devices
			SET hostname       = $1,
			    ip_address     = $2::inet,
			    device_type    = $3,
			    location       = NULLIF($4, ''),
			    snmp_community = COALESCE(NULLIF($5, ''), snmp_community),
			    snmp_version   = $6,
			    is_monitored   = $7
			WHERE id = $8
		`, payload.Hostname, payload.IPAddress, payload.DeviceType, payload.Location,
			payload.SnmpCommunity, payload.SnmpVersion, monitored, id)

		if err != nil {
			errStr := err.Error()
			apiLogger.Error("[API] Failed to update device", "id", id, "err", err)
			switch {
			case containsAny(errStr, "devices_hostname_key", "hostname"):
				jsonResponse(w, http.StatusConflict, map[string]string{"error": "Hostname already in use by another device"})
			case containsAny(errStr, "devices_ip_address_key", "ip_address"):
				jsonResponse(w, http.StatusConflict, map[string]string{"error": "IP address already in use by another device"})
			default:
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update device"})
			}
			return
		}

		if tag.RowsAffected() == 0 {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Device not found"})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]string{"status": "success", "message": "Device updated successfully"})
	}
}

func DeleteDeviceHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if setupCORS(w, r) {
			return
		}

		id := r.PathValue("id")
		if id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Device ID is required"})
			return
		}

		ctx := r.Context()

		// Purge topology edges referencing this device before dropping the parent row
		// to avoid FK constraint violations and orphaned graph edges.
		_, err := database.Pool.Exec(ctx, `
			DELETE FROM network_topology
			WHERE source_device = (SELECT hostname FROM devices WHERE id = $1)
			   OR target_device = (SELECT hostname FROM devices WHERE id = $1)
		`, id)
		if err != nil {
			apiLogger.Error("[API] Failed to delete topology edges", "id", id, "err", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete device associations"})
			return
		}

		tag, err := database.Pool.Exec(ctx, `DELETE FROM devices WHERE id = $1`, id)
		if err != nil {
			apiLogger.Error("[API] Failed to delete device", "id", id, "err", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete device"})
			return
		}

		if tag.RowsAffected() == 0 {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Device not found"})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]string{"status": "success", "message": "Device deleted successfully"})
	}
}
