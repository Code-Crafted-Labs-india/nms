package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"nms-middleware/db"
)

type configurationPlanRequest struct {
	DeviceID     int    `json:"device_id"`
	Profile      string `json:"profile"`
	Operation    string `json:"operation"`
	VLANID       int    `json:"vlan_id"`
	VLANName     string `json:"vlan_name"`
	Interface    string `json:"interface"`
	PortMode     string `json:"port_mode"`
	AllowedVLANs []int  `json:"allowed_vlans"`
}

type configurationPlan struct {
	ID         int64           `json:"id"`
	DeviceID   int             `json:"device_id"`
	Hostname   string          `json:"hostname"`
	Profile    string          `json:"profile"`
	Operation  string          `json:"operation"`
	Parameters json.RawMessage `json:"parameters"`
	Commands   []string        `json:"commands"`
	Status     string          `json:"status"`
	CreatedBy  string          `json:"created_by"`
	CreatedAt  string          `json:"created_at"`
}

var configNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,32}$`)
var interfaceNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9./:-]{0,31}$`)
var atsConfigProfilePattern = regexp.MustCompile(`(?i)^ats-cli@\d+(?:\.\d+){1,3}(?:[-+][a-z0-9._-]+)?$`)

func isATSConfigurationProfile(profile string) bool {
	return profile == "ats-cli" || atsConfigProfilePattern.MatchString(profile)
}

func isATS3900ConfigurationProfile(profile string) bool {
	return profile == "ats3900-aos@3.3.1.1"
}

// ConfigurationPlansHandler persists reviewable CLI plans. It never connects
// to or sends commands to a managed device.
func ConfigurationPlansHandler(database *db.DB) http.HandlerFunc {
	return configurationPlansHandler(database.Pool)
}

func configurationPlansHandler(database configurationQueries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if setupCORS(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			rows, err := database.Query(r.Context(), `
				SELECT c.id, c.device_id, d.hostname, c.profile, c.operation,
				       c.parameters, c.commands, c.status, c.created_by, c.created_at
				FROM configuration_changes c JOIN devices d ON d.id=c.device_id
				ORDER BY c.created_at DESC LIMIT 100`)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "configuration plans unavailable"})
				return
			}
			defer rows.Close()
			plans := make([]configurationPlan, 0)
			for rows.Next() {
				var p configurationPlan
				if err := rows.Scan(&p.ID, &p.DeviceID, &p.Hostname, &p.Profile, &p.Operation, &p.Parameters, &p.Commands, &p.Status, &p.CreatedBy, &p.CreatedAt); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "configuration plans unavailable"})
					return
				}
				plans = append(plans, p)
			}
			if err := rows.Err(); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "configuration plans unavailable"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
		case http.MethodPost:
			var request configurationPlanRequest
			if err := decodeJSON(w, r, &request); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
				return
			}
			if request.DeviceID < 1 {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "select a valid device"})
				return
			}
			identity, detection, err := loadConfigurationProfileFrom(r.Context(), database, request.DeviceID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
					return
				}
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not identify the device operating system"})
				return
			}
			if detection.Status != "supported" || detection.Profile == "" {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": detection.Reason, "osDetection": detection})
				return
			}
			if request.Profile != "" && request.Profile != detection.Profile {
				writeJSON(w, http.StatusConflict, map[string]any{
					"error":       "the selected command profile does not match the device's current detected operating system",
					"osDetection": detection,
				})
				return
			}
			request.Profile = detection.Profile
			commands, params, err := renderConfigurationCommands(request)
			if err != nil {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
				return
			}
			params["detected_os"] = map[string]any{
				"family": detection.Family, "version": detection.Version,
				"profile": detection.Profile, "observed_at": detection.ObservedAt,
			}
			encoded, err := json.Marshal(params)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save configuration plan"})
				return
			}
			hostname := identity.Hostname
			actor := "unknown"
			if sess, ok := r.Context().Value(sessionContextKey{}).(session); ok && sess.Actor != "" {
				actor = sess.Actor
			}
			var planID int64
			err = database.QueryRow(r.Context(), `
				INSERT INTO configuration_changes (device_id, profile, operation, parameters, commands, created_by)
				VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, request.DeviceID, request.Profile, request.Operation, encoded, commands, actor).Scan(&planID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save configuration plan"})
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"id": planID, "hostname": hostname, "profile": request.Profile, "osDetection": detection, "operation": request.Operation, "commands": commands, "status": "draft"})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	}
}

func renderConfigurationCommands(r configurationPlanRequest) ([]string, map[string]any, error) {
	if r.DeviceID < 1 {
		return nil, nil, fmt.Errorf("select a valid device")
	}
	atsProfile := isATSConfigurationProfile(r.Profile)
	ats3900Profile := isATS3900ConfigurationProfile(r.Profile)
	if !atsProfile && !ats3900Profile && r.Profile != "routing-v3.3.1" {
		return nil, nil, fmt.Errorf("this command profile is not enabled")
	}
	if r.VLANID < 2 || r.VLANID > 4094 {
		return nil, nil, fmt.Errorf("VLAN ID must be between 2 and 4094")
	}
	if r.VLANName == "" {
		r.VLANName = fmt.Sprintf("VLAN%d", r.VLANID)
	}
	if !configNamePattern.MatchString(r.VLANName) {
		return nil, nil, fmt.Errorf("VLAN name may contain letters, numbers, dot, underscore, and hyphen (max 32 characters)")
	}
	params := map[string]any{"vlan_id": r.VLANID, "vlan_name": r.VLANName}
	if r.Operation == "create_vlan" {
		if ats3900Profile {
			return []string{"config", fmt.Sprintf("vlan-view %d", r.VLANID), "name " + r.VLANName, "quit"}, params, nil
		}
		if atsProfile {
			return []string{"configure terminal", fmt.Sprintf("vlan %d", r.VLANID), "name " + r.VLANName, "exit"}, params, nil
		}
		return []string{"configure terminal", "vlan database", fmt.Sprintf("vlan %d name %s state enable", r.VLANID, r.VLANName), "exit"}, params, nil
	}
	if r.Operation != "configure_port" {
		return nil, nil, fmt.Errorf("choose VLAN creation or port VLAN membership")
	}
	if !interfaceNamePattern.MatchString(r.Interface) {
		return nil, nil, fmt.Errorf("enter a valid interface name without spaces or command separators")
	}
	params["interface"] = r.Interface
	if r.PortMode != "access" && r.PortMode != "trunk" {
		return nil, nil, fmt.Errorf("port mode must be access or trunk")
	}
	globalConfig := "configure terminal"
	if ats3900Profile {
		globalConfig = "config"
	}
	commands := []string{globalConfig, "interface " + r.Interface}
	if r.PortMode == "access" {
		commands = append(commands, "switchport mode access")
		if atsProfile || ats3900Profile {
			commands = append(commands, fmt.Sprintf("switchport pvid %d", r.VLANID))
		} else {
			commands = append(commands, fmt.Sprintf("switchport access vlan %d", r.VLANID))
		}
	} else {
		if len(r.AllowedVLANs) == 0 {
			return nil, nil, fmt.Errorf("add at least one allowed VLAN for a trunk port")
		}
		seen := make(map[int]bool)
		values := make([]string, 0, len(r.AllowedVLANs))
		for _, vlan := range r.AllowedVLANs {
			if vlan < 1 || vlan > 4094 || seen[vlan] {
				return nil, nil, fmt.Errorf("allowed VLAN IDs must be unique values from 1 to 4094")
			}
			seen[vlan] = true
			values = append(values, fmt.Sprint(vlan))
		}
		list := strings.Join(values, ",")
		params["allowed_vlans"] = r.AllowedVLANs
		commands = append(commands, "switchport mode trunk")
		if ats3900Profile {
			commands = append(commands, "switchport trunk vlan-allowed "+list)
		} else if atsProfile {
			commands = append(commands, "switchport trunk vlan-allowed "+list)
		} else {
			commands = append(commands, "switchport trunk allowed vlan add "+list)
		}
	}
	return commands, params, nil
}
