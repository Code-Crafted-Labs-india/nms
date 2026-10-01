package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"nms-middleware/db"
)

// ============================================================================
// INVENTORY HANDLER — GET /api/v1/devices
// Server-paginated, filterable, sortable device fleet list.
// Scope: §2 "Fleet inventory" (P0) — supports status, type, and text filters.
// ============================================================================

type InventoryDevice struct {
	ID            int      `json:"id"`
	Hostname      string   `json:"hostname"`
	IPAddress     string   `json:"ipAddress"`
	DeviceType    string   `json:"deviceType"`
	Location      string   `json:"location"`
	IsMonitored   bool     `json:"isMonitored"`
	SnmpVersion   string   `json:"snmpVersion"`
	CreatedAt     string   `json:"createdAt"`
	Status        string   `json:"status"`        // online | offline | unknown
	RTTMs         *float64 `json:"rttMs"`
	PacketLoss    *float64 `json:"packetLoss"`
	OpenAlarms    int      `json:"openAlarms"`
	LastPoll      *string  `json:"lastPoll"`
	UpInterfaces  int      `json:"upInterfaces"`
	DownInterfaces int     `json:"downInterfaces"`
}

type InventoryResponse struct {
	Devices    []InventoryDevice `json:"devices"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"pageSize"`
	TotalPages int               `json:"totalPages"`
}

func InventoryHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := r.URL.Query()

		// --- Pagination ---
		page, _ := strconv.Atoi(q.Get("page"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(q.Get("page_size"))
		if pageSize < 1 || pageSize > 200 {
			pageSize = 50
		}
		offset := (page - 1) * pageSize

		// --- Filters ---
		statusFilter := q.Get("status")   // "online" | "offline" | "" (all)
		typeFilter := q.Get("type")       // free-text device_type match
		searchText := q.Get("q")          // hostname / IP free-text search
		monitoredOnly := q.Get("monitored") != "false"

		// Build dynamic WHERE clause.
		args := []any{}
		where := "WHERE 1=1"

		if monitoredOnly {
			where += " AND d.is_monitored = TRUE"
		}
		if typeFilter != "" {
			args = append(args, "%"+typeFilter+"%")
			where += fmt.Sprintf(" AND d.device_type ILIKE $%d", len(args))
		}
		if searchText != "" {
			args = append(args, "%"+searchText+"%", "%"+searchText+"%")
			where += fmt.Sprintf(" AND (d.hostname ILIKE $%d OR d.ip_address::text ILIKE $%d)", len(args)-1, len(args))
		}

		// Count total matching rows for pagination metadata.
		var total int
		countQuery := "SELECT COUNT(*) FROM devices d " + where
		if err := database.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "inventory count query failed"})
			return
		}

		// Main query: join latest ICMP status and open alarm count.
		args = append(args, pageSize, offset)
		pLimit := len(args) - 1
		pOffset := len(args)

		dataQuery := fmt.Sprintf(`
			SELECT
				d.id,
				d.hostname,
				d.ip_address::text,
				d.device_type,
				COALESCE(d.location, 'Unassigned'),
				d.is_monitored,
				d.snmp_version,
				to_char(d.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
				CASE
					WHEN m.icmp_status = 1 THEN 'online'
					WHEN m.icmp_status = 0 THEN 'offline'
					ELSE 'unknown'
				END AS status,
				m.icmp_rtt_ms,
				m.icmp_packet_loss,
				COALESCE(alm.open_count, 0),
				to_char(m.time AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
				COALESCE(ifs.up_count, 0),
				COALESCE(ifs.down_count, 0)
			FROM devices d
			LEFT JOIN LATERAL (
				SELECT icmp_status, icmp_rtt_ms, icmp_packet_loss, time
				FROM device_health_metrics
				WHERE device_id = d.id
				ORDER BY time DESC LIMIT 1
			) m ON TRUE
			LEFT JOIN LATERAL (
				SELECT COUNT(*) AS open_count
				FROM network_events
				WHERE device_id = d.id AND resolved = FALSE
			) alm ON TRUE
			LEFT JOIN LATERAL (
				SELECT
					COUNT(*) FILTER (WHERE if_oper_status = 1) AS up_count,
					COUNT(*) FILTER (WHERE if_oper_status = 2) AS down_count
				FROM (
					SELECT DISTINCT ON (interface_id) interface_id, if_oper_status
					FROM interface_performance_metrics ipm
					JOIN interfaces i ON i.id = ipm.interface_id
					WHERE i.device_id = d.id AND ipm.time >= NOW() - INTERVAL '5 minutes'
					ORDER BY interface_id, ipm.time DESC
				) latest_if
			) ifs ON TRUE
			%s
			ORDER BY d.hostname
			LIMIT $%d OFFSET $%d
		`, where, pLimit, pOffset)

		rows, err := database.Pool.Query(ctx, dataQuery, args...)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "inventory query failed"})
			return
		}
		defer rows.Close()

		devices := []InventoryDevice{}
		for rows.Next() {
			var item InventoryDevice
			var openAlarms int
			if err := rows.Scan(
				&item.ID, &item.Hostname, &item.IPAddress, &item.DeviceType,
				&item.Location, &item.IsMonitored, &item.SnmpVersion, &item.CreatedAt,
				&item.Status, &item.RTTMs, &item.PacketLoss, &openAlarms,
				&item.LastPoll, &item.UpInterfaces, &item.DownInterfaces,
			); err != nil {
				rows.Close()
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "row scan failed"})
				return
			}
			item.OpenAlarms = openAlarms

			// Apply post-query status filter (cannot easily do in SQL without subquery ordering issues).
			if statusFilter != "" && item.Status != statusFilter {
				continue
			}
			devices = append(devices, item)
		}

		totalPages := (total + pageSize - 1) / pageSize

		writeJSON(w, http.StatusOK, InventoryResponse{
			Devices:    devices,
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		})
	}
}

// ============================================================================
// DEVICE DETAIL HANDLER — GET /api/v1/devices/{id}
// Returns full device diagnostics with Overview, Interfaces, Events tabs data.
// Scope: §2 "Device detail page" (P0).
// ============================================================================

type DeviceInterface struct {
	ID            int     `json:"id"`
	IfIndex       int     `json:"ifIndex"`
	IfDescr       string  `json:"ifDescr"`
	Speed         *int64  `json:"speedBps"`
	OperStatus    int     `json:"operStatus"`
	AdminStatus   int     `json:"adminStatus"`
	RxBytesDelta  *int64  `json:"rxBytesDelta"`
	TxBytesDelta  *int64  `json:"txBytesDelta"`
	RxErrors      *int64  `json:"rxErrors"`
	TxErrors      *int64  `json:"txErrors"`
	RxDiscards    *int64  `json:"rxDiscards"`
	TxDiscards    *int64  `json:"txDiscards"`
	OpticalRxDbm  *float64 `json:"opticalRxDbm"`
	OpticalTxDbm  *float64 `json:"opticalTxDbm"`
	LastSeen      *string  `json:"lastSeen"`
}

type DeviceAlarm struct {
	ID        int    `json:"id"`
	Time      string `json:"time"`
	Severity  string `json:"severity"`
	EventType string `json:"eventType"`
	Message   string `json:"message"`
	Resolved  bool   `json:"resolved"`
	ResolvedAt *string `json:"resolvedAt"`
}

type DeviceDetail struct {
	// Identity
	ID          int    `json:"id"`
	Hostname    string `json:"hostname"`
	IPAddress   string `json:"ipAddress"`
	DeviceType  string `json:"deviceType"`
	Location    string `json:"location"`
	IsMonitored bool   `json:"isMonitored"`
	SnmpVersion string `json:"snmpVersion"`
	CreatedAt   string `json:"createdAt"`

	// Reachability
	Status      string   `json:"status"`
	RTTMs       *float64 `json:"rttMs"`
	PacketLoss  *float64 `json:"packetLoss"`
	LastPoll    *string  `json:"lastPoll"`

	// Resource summary (latest values)
	CPUUtilization    *float64 `json:"cpuUtilization"`
	MemoryUtilization *float64 `json:"memoryUtilization"`

	// Current alarms (unresolved)
	OpenAlarmCount int           `json:"openAlarmCount"`
	Alarms         []DeviceAlarm `json:"alarms"`

	// Interfaces (latest state)
	Interfaces []DeviceInterface `json:"interfaces"`
}

func DeviceDetailHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		idStr := r.PathValue("id")
		if idStr == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "device ID is required"})
			return
		}

		var detail DeviceDetail

		// --- Identity row ---
		err := database.Pool.QueryRow(ctx, `
			SELECT id, hostname, ip_address::text, device_type,
			       COALESCE(location, 'Unassigned'), is_monitored, snmp_version,
			       to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
			FROM devices WHERE id = $1
		`, idStr).Scan(
			&detail.ID, &detail.Hostname, &detail.IPAddress, &detail.DeviceType,
			&detail.Location, &detail.IsMonitored, &detail.SnmpVersion, &detail.CreatedAt,
		)
		if err != nil {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}

		// --- Latest reachability ---
		var icmpStatus *int
		_ = database.Pool.QueryRow(ctx, `
			SELECT icmp_status, icmp_rtt_ms, icmp_packet_loss,
			       to_char(time AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
			FROM device_health_metrics
			WHERE device_id = $1
			ORDER BY time DESC LIMIT 1
		`, detail.ID).Scan(&icmpStatus, &detail.RTTMs, &detail.PacketLoss, &detail.LastPoll)

		if icmpStatus == nil {
			detail.Status = "unknown"
		} else if *icmpStatus == 1 {
			detail.Status = "online"
		} else {
			detail.Status = "offline"
		}

		// --- CPU / Memory (latest from snmp_device_health) ---
		_ = database.Pool.QueryRow(ctx, `
			SELECT
				COALESCE(100.0 - cpu_idle, cpu_load_1 * 25.0),
				COALESCE(100.0 * (mem_total_kb - mem_available_kb) / NULLIF(mem_total_kb, 0))
			FROM snmp_device_health h
			JOIN devices d ON d.ip_address = NULLIF(h.agent_host, '')::inet
			WHERE d.id = $1
			ORDER BY h.time DESC LIMIT 1
		`, detail.ID).Scan(&detail.CPUUtilization, &detail.MemoryUtilization)

		// --- Open alarm count ---
		_ = database.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM network_events WHERE device_id = $1 AND resolved = FALSE
		`, detail.ID).Scan(&detail.OpenAlarmCount)

		// --- Recent alarms (last 50, including resolved) ---
		alarmRows, err := database.Pool.Query(ctx, `
			SELECT id, time, severity, event_type, COALESCE(message, ''), resolved,
			       to_char(resolved_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
			FROM network_events
			WHERE device_id = $1
			ORDER BY time DESC LIMIT 50
		`, detail.ID)
		if err == nil {
			defer alarmRows.Close()
			for alarmRows.Next() {
				var a DeviceAlarm
				var t time.Time
				if err := alarmRows.Scan(&a.ID, &t, &a.Severity, &a.EventType, &a.Message, &a.Resolved, &a.ResolvedAt); err == nil {
					a.Time = t.UTC().Format(time.RFC3339)
					detail.Alarms = append(detail.Alarms, a)
				}
			}
		}
		if detail.Alarms == nil {
			detail.Alarms = []DeviceAlarm{}
		}

		// --- Interfaces (latest per interface) ---
		ifRows, err := database.Pool.Query(ctx, `
			SELECT
				i.id, i.if_index, i.if_descr, i.speed,
				ipm.if_oper_status, ipm.if_admin_status,
				ipm.rx_bytes_delta, ipm.tx_bytes_delta,
				ipm.rx_errors_delta, ipm.tx_errors_delta,
				ipm.rx_discards_delta, ipm.tx_discards_delta,
				ipm.optical_rx_dbm, ipm.optical_tx_dbm,
				to_char(ipm.time AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
			FROM interfaces i
			JOIN LATERAL (
				SELECT * FROM interface_performance_metrics
				WHERE interface_id = i.id
				ORDER BY time DESC LIMIT 1
			) ipm ON TRUE
			WHERE i.device_id = $1
			ORDER BY i.if_index
		`, detail.ID)
		if err == nil {
			defer ifRows.Close()
			for ifRows.Next() {
				var iface DeviceInterface
				if err := ifRows.Scan(
					&iface.ID, &iface.IfIndex, &iface.IfDescr, &iface.Speed,
					&iface.OperStatus, &iface.AdminStatus,
					&iface.RxBytesDelta, &iface.TxBytesDelta,
					&iface.RxErrors, &iface.TxErrors,
					&iface.RxDiscards, &iface.TxDiscards,
					&iface.OpticalRxDbm, &iface.OpticalTxDbm,
					&iface.LastSeen,
				); err == nil {
					detail.Interfaces = append(detail.Interfaces, iface)
				}
			}
		}
		if detail.Interfaces == nil {
			detail.Interfaces = []DeviceInterface{}
		}

		writeJSON(w, http.StatusOK, detail)
	}
}

// ============================================================================
// RESOLVE ALARM HANDLER — POST /api/v1/alarms/{id}/resolve
// Manually resolves a specific alarm instance. Scope: §6 "Alarm lifecycle" (P1).
// ============================================================================

func ResolveAlarmHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		idStr := r.PathValue("id")
		if idStr == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "alarm ID required"})
			return
		}

		// Parse optional resolution note from body.
		var body struct {
			Note string `json:"note"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		tag, err := database.Pool.Exec(ctx, `
			UPDATE network_events
			SET resolved = TRUE, resolved_at = NOW()
			WHERE id = $1 AND resolved = FALSE
		`, idStr)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "resolve query failed"})
			return
		}
		if tag.RowsAffected() == 0 {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "alarm not found or already resolved"})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]string{"status": "resolved"})
	}
}


