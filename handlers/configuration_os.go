package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"nms-middleware/db"
)

type configurationQueries interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type configurationOSDetection struct {
	Status      string   `json:"status"`
	Family      string   `json:"family"`
	Version     string   `json:"version"`
	Profile     string   `json:"profile"`
	ProfileName string   `json:"profileName"`
	Confidence  string   `json:"confidence"`
	Reason      string   `json:"reason"`
	Evidence    []string `json:"evidence"`
	ObservedAt  string   `json:"observedAt"`
}

type configurationDevice struct {
	ID          int                      `json:"id"`
	Hostname    string                   `json:"hostname"`
	IPAddress   string                   `json:"ipAddress"`
	DeviceType  string                   `json:"deviceType"`
	OSDetection configurationOSDetection `json:"osDetection"`
}

type deviceIdentity struct {
	Hostname   string
	DeviceType string
	SysDescr   string
	SysObject  string
	ObservedAt string
}

var configurationVersionPattern = regexp.MustCompile(`(?i)\b(?:software(?:\s+version)?|version|release|firmware|os|aos|v)\s*[:=]?\s*(?:aos\s*)?v?(\d+(?:\.\d+){1,3}(?:[-+][a-z0-9._-]+)?)\b`)
var atsIdentityPattern = regexp.MustCompile(`(?i)(?:\bATS\b|Allied\s+Telesis)`)
var ats3900IdentityPattern = regexp.MustCompile(`(?i)\bATS[- ]?3900[- ]?24P6X\b`)
var atsAOSIdentityPattern = regexp.MustCompile(`(?i)\bAOS\b`)

// detectConfigurationOS only returns an executable profile for device/version
// combinations covered by a command reference. sysObjectID is retained as
// evidence until a tested model-to-profile mapping exists for that identifier.
func detectConfigurationOS(identity deviceIdentity) configurationOSDetection {
	description := strings.TrimSpace(identity.SysDescr)
	model := strings.TrimSpace(identity.DeviceType)
	evidence := make([]string, 0, 3)
	if model != "" {
		evidence = append(evidence, "inventory model: "+model)
	}
	if description != "" {
		evidence = append(evidence, "SNMP sysDescr: "+description)
	}
	if strings.TrimSpace(identity.SysObject) != "" {
		evidence = append(evidence, "SNMP sysObjectID: "+strings.TrimSpace(identity.SysObject))
	}
	result := configurationOSDetection{
		Status:     "unidentified",
		Confidence: "none",
		Reason:     "No recent SNMP system description is available. Confirm SNMP access and poll the device before preparing a configuration.",
		Evidence:   evidence,
		ObservedAt: identity.ObservedAt,
	}
	if description == "" {
		return result
	}

	result.Version = extractConfigurationVersion(description)
	descriptionLower := strings.ToLower(description)
	modelLower := strings.ToLower(model)
	ats3900 := ats3900IdentityPattern.MatchString(model) || ats3900IdentityPattern.MatchString(description)

	if ats3900 {
		result.Family = "ATS-3900-24P6X · AOS"
		if !ats3900IdentityPattern.MatchString(description) && !atsAOSIdentityPattern.MatchString(description) {
			result.Status = "identity_unconfirmed"
			result.Confidence = "low"
			result.Reason = "The inventory identifies an ATS-3900-24P6X, but sysDescr does not confirm the model or AOS. No profile will be selected."
			return result
		}
		if result.Version == "" {
			result.Status = "version_unreported"
			result.Confidence = "medium"
			result.Reason = "The ATS-3900-24P6X model is identified, but AOS version was not found in sysDescr. The Type 3 command profile requires a version match."
			return result
		}
		if result.Version != "3.3.1.1" {
			result.Status = "unsupported_version"
			result.Confidence = "high"
			result.Reason = fmt.Sprintf("ATS-3900-24P6X AOS version %s has no qualified command profile. The supplied device evidence covers AOS 3.3.1.1.", result.Version)
			return result
		}
		result.Status = "supported"
		result.Profile = "ats3900-aos@3.3.1.1"
		result.ProfileName = "ATS-3900-24P6X · AOS 3.3.1.1"
		result.Confidence = "medium"
		result.Reason = "The inventory model and reported AOS version match the Type 3 VLAN and interface command references."
		if ats3900IdentityPattern.MatchString(description) {
			result.Confidence = "high"
			result.Reason = "The reported model and AOS version match the Type 3 VLAN and interface command references."
		}
		return result
	}

	if strings.Contains(descriptionLower, "routing switch") {
		result.Family = "Routing Switch"
		if result.Version == "" {
			result.Status = "version_unreported"
			result.Confidence = "medium"
			result.Reason = "A Routing Switch was identified, but its software version was not found in sysDescr. Configuration commands require a version match."
			return result
		}
		if result.Version != "3.3.1" {
			result.Status = "unsupported_version"
			result.Confidence = "high"
			result.Reason = fmt.Sprintf("Routing Switch version %s has no qualified command profile. The available reference covers V3.3.1.", result.Version)
			return result
		}
		result.Status = "supported"
		result.Profile = "routing-v3.3.1"
		result.ProfileName = "Routing Switch CLI · V3.3.1"
		result.Confidence = "high"
		result.Reason = "The detected family and software version match the qualified Routing Switch CLI reference."
		return result
	}

	if atsIdentityPattern.MatchString(description) {
		result.Family = "ATS"
		if result.Version == "" {
			result.Status = "version_unreported"
			result.Confidence = "medium"
			result.Reason = "ATS was identified, but the software version was not found in sysDescr. Confirm the version before preparing a configuration."
			return result
		}
		result.Status = "supported"
		result.Profile = "ats-cli@" + result.Version
		result.ProfileName = "ATS CLI · " + result.Version
		result.Confidence = "medium"
		result.Reason = "The device reports an ATS identity and software version. Confirm the exact model/firmware entry in the supported-device matrix before applying changes."
		return result
	}
	if ats3900IdentityPattern.MatchString(model) || atsIdentityPattern.MatchString(model) || strings.Contains(modelLower, "routing switch") {
		result.Family = model
		result.Status = "identity_unconfirmed"
		result.Confidence = "low"
		result.Reason = "The inventory model suggests a known switch family, but sysDescr does not confirm it. No command profile will be selected."
		return result
	}

	result.Status = "unsupported"
	result.Confidence = "medium"
	result.Reason = "The device reported a software description, but it does not match a qualified command profile. No profile-specific commands can be prepared."
	return result
}

func extractConfigurationVersion(description string) string {
	for _, match := range configurationVersionPattern.FindAllStringSubmatchIndex(description, -1) {
		versionEnd := match[3]
		if versionEnd+1 < len(description) && description[versionEnd] == '.' && description[versionEnd+1] >= '0' && description[versionEnd+1] <= '9' {
			continue
		}
		return description[match[2]:match[3]]
	}
	return ""
}

func readConfigurationDeviceFrom(ctx context.Context, database configurationQueries, deviceID int) (deviceIdentity, error) {
	var identity deviceIdentity
	err := database.QueryRow(ctx, `
		SELECT d.hostname, d.device_type, COALESCE(h."sysDescr", ''),
		       COALESCE(h."sysObjectID", ''),
		       COALESCE(to_char(h.time AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')
		FROM devices d
		LEFT JOIN LATERAL (
			SELECT "sysDescr", "sysObjectID", time
			FROM snmp_device_health
			WHERE agent_host = host(d.ip_address)
			  AND time >= NOW() - INTERVAL '15 minutes'
			ORDER BY time DESC LIMIT 1
		) h ON TRUE
		WHERE d.id = $1
		  AND d.is_monitored = TRUE
		  AND `+deviceModePredicate("d"), deviceID).Scan(&identity.Hostname, &identity.DeviceType, &identity.SysDescr, &identity.SysObject, &identity.ObservedAt)
	return identity, err
}

// ConfigurationDevicesHandler reports the current device identity evidence and
// the profile (if any) that is safe to use for a configuration draft.
func ConfigurationDevicesHandler(database *db.DB) http.HandlerFunc {
	return configurationDevicesHandler(database.Pool)
}

func configurationDevicesHandler(database configurationQueries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if setupCORS(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		rows, err := database.Query(r.Context(), `
			SELECT d.id, d.hostname, host(d.ip_address), d.device_type,
			       COALESCE(h."sysDescr", ''), COALESCE(h."sysObjectID", ''),
			       COALESCE(to_char(h.time AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')
			FROM devices d
			LEFT JOIN LATERAL (
				SELECT "sysDescr", "sysObjectID", time
				FROM snmp_device_health
				WHERE agent_host = host(d.ip_address)
				  AND time >= NOW() - INTERVAL '15 minutes'
				ORDER BY time DESC LIMIT 1
			) h ON TRUE
			WHERE d.is_monitored = TRUE
			  AND d.device_type <> 'unknown_discovered'
			  AND `+deviceModePredicate("d")+`
			ORDER BY d.hostname`)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "configuration devices unavailable"})
			return
		}
		defer rows.Close()
		devices := make([]configurationDevice, 0)
		for rows.Next() {
			var device configurationDevice
			var identity deviceIdentity
			if err := rows.Scan(&device.ID, &identity.Hostname, &device.IPAddress, &identity.DeviceType, &identity.SysDescr, &identity.SysObject, &identity.ObservedAt); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "configuration devices unavailable"})
				return
			}
			device.Hostname = identity.Hostname
			device.DeviceType = identity.DeviceType
			device.OSDetection = detectConfigurationOS(identity)
			devices = append(devices, device)
		}
		if err := rows.Err(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "configuration devices unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
	}
}

func loadConfigurationProfileFrom(ctx context.Context, database configurationQueries, deviceID int) (deviceIdentity, configurationOSDetection, error) {
	identity, err := readConfigurationDeviceFrom(ctx, database, deviceID)
	if err != nil {
		return identity, configurationOSDetection{}, err
	}
	return identity, detectConfigurationOS(identity), nil
}
