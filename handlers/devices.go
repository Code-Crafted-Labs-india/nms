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
	SnmpCommunity string `json:"snmp_community"`
	SnmpVersion   string `json:"snmp_version"`
}

// jsonResponse writes a consistent JSON envelope and sets Content-Type.
// Must be called before WriteHeader to allow header mutation.
func jsonResponse(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// setupCORS writes the CORS headers required for Grafana Business Forms to reach
// the middleware from a separate Docker container. Returns true if this was a
// preflight OPTIONS request (caller should return immediately after).
func setupCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
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

// RequestLogger wraps any HandlerFunc and logs method, path, and remote addr
// for every inbound request. Attach this in main.go via mux.Handle(...).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiLogger.Info("[API] Inbound request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
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
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			apiLogger.Error("[API] Failed to decode add device payload", "err", err)
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
			return
		}

		if payload.Hostname == "" || payload.IPAddress == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "hostname and ip_address are required"})
			return
		}

		// Insert is gated by the device_models catalog: device_type must match a
		// registered model_name. ON CONFLICT DO NOTHING prevents duplicate hostnames.
		tag, err := database.Pool.Exec(r.Context(), `
    INSERT INTO devices (hostname, ip_address, device_type, snmp_community, snmp_version, is_monitored)
    SELECT $1::text, $2::inet, $3::text, $4::text, $5::text, true
    WHERE EXISTS (SELECT 1 FROM device_models WHERE model_name = $3::text)
    ON CONFLICT DO NOTHING;
`, payload.Hostname, payload.IPAddress, payload.DeviceType, payload.SnmpCommunity, payload.SnmpVersion)

		if err != nil {
			apiLogger.Error("[API] Failed to insert device", "err", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create device"})
			return
		}

		if tag.RowsAffected() == 0 {
			// Either a duplicate hostname, or device_type is not in the supported catalog.
			jsonResponse(w, http.StatusConflict, map[string]string{
				"error": "Device not created — hostname may already exist, or device_type is not in the supported device_models catalog",
			})
			return
		}

		jsonResponse(w, http.StatusCreated, map[string]string{"status": "success", "message": "Device created successfully"})
	}
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
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			apiLogger.Error("[API] Failed to decode edit device payload", "err", err)
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
			return
		}

		tag, err := database.Pool.Exec(r.Context(), `
			UPDATE devices
			SET hostname       = $1,
			    ip_address     = $2::inet,
			    device_type    = $3,
			    snmp_community = $4,
			    snmp_version   = $5
			WHERE id = $6
		`, payload.Hostname, payload.IPAddress, payload.DeviceType, payload.SnmpCommunity, payload.SnmpVersion, id)

		if err != nil {
			apiLogger.Error("[API] Failed to update device", "id", id, "err", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update device"})
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
