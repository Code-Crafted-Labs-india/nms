package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"nms-middleware/db"
)

type DevicePayload struct {
	Hostname      string `json:"hostname"`
	IPAddress     string `json:"ip_address"`
	DeviceType    string `json:"device_type"`
	SnmpCommunity string `json:"snmp_community"`
	SnmpVersion   string `json:"snmp_version"`
}

func AddDeviceHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload DevicePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Printf("[API Error] Failed to decode add device payload: %v", err)
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		if payload.Hostname == "" || payload.IPAddress == "" {
			http.Error(w, "Hostname and IPAddress are required", http.StatusBadRequest)
			return
		}

		_, err := database.Pool.Exec(r.Context(), `
			INSERT INTO devices (hostname, ip_address, device_type, snmp_community, snmp_version, is_monitored)
			VALUES ($1, $2, $3, $4, $5, true);
		`, payload.Hostname, payload.IPAddress, payload.DeviceType, payload.SnmpCommunity, payload.SnmpVersion)

		if err != nil {
			log.Printf("[API Error] Failed to insert device: %v", err)
			http.Error(w, "Failed to create device", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func EditDeviceHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "Device ID is required", http.StatusBadRequest)
			return
		}

		var payload DevicePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Printf("[API Error] Failed to decode edit device payload: %v", err)
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		_, err := database.Pool.Exec(r.Context(), `
			UPDATE devices 
			SET hostname = $1, ip_address = $2, device_type = $3, snmp_community = $4, snmp_version = $5 
			WHERE id = $6;
		`, payload.Hostname, payload.IPAddress, payload.DeviceType, payload.SnmpCommunity, payload.SnmpVersion, id)

		if err != nil {
			log.Printf("[API Error] Failed to update device: %v", err)
			http.Error(w, "Failed to update device", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func DeleteDeviceHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "Device ID is required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Before dropping the device, delete any connected graph edges in network_topology first
		_, err := database.Pool.Exec(ctx, `
			DELETE FROM network_topology 
			WHERE source_device = (SELECT hostname FROM devices WHERE id = $1)
			   OR target_device = (SELECT hostname FROM devices WHERE id = $1);
		`, id)
		if err != nil {
			log.Printf("[API Error] Failed to delete network topology edges for device: %v", err)
			http.Error(w, "Failed to delete device associations", http.StatusInternalServerError)
			return
		}

		// Device Dropping: Then execute the master deletion query
		_, err = database.Pool.Exec(ctx, `DELETE FROM devices WHERE id = $1;`, id)
		if err != nil {
			log.Printf("[API Error] Failed to delete device: %v", err)
			http.Error(w, "Failed to delete device", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
