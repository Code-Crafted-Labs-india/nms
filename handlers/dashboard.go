package handlers

import (
	"context"
	"net/http"
	"time"

	"nms-middleware/db"
)

type Overview struct {
	Monitored  int `json:"monitored"`
	Online     int `json:"online"`
	Offline    int `json:"offline"`
	OpenAlerts int `json:"openAlerts"`
}

type DeviceStatus struct {
	ID         int      `json:"id"`
	Hostname   string   `json:"hostname"`
	IPAddress  string   `json:"ipAddress"`
	Type       string   `json:"type"`
	Location   string   `json:"location"`
	Status     string   `json:"status"`
	RTT        *float64 `json:"rttMs"`
	LastUpdate *string  `json:"lastUpdate"`
}

type Event struct {
	Time     string `json:"time"`
	Device   string `json:"device"`
	Severity string `json:"severity"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

type MetricPoint struct {
	Time   string  `json:"time"`
	Device string  `json:"device"`
	Value  float64 `json:"value"`
}

type TopologyEdge struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	SourcePort string `json:"sourcePort"`
	TargetPort string `json:"targetPort"`
}

type Dashboard struct {
	GeneratedAt string         `json:"generatedAt"`
	Overview    Overview       `json:"overview"`
	Devices     []DeviceStatus `json:"devices"`
	Events      []Event        `json:"events"`
	Latency     []MetricPoint  `json:"latency"`
	CPU         []MetricPoint  `json:"cpu"`
	Memory      []MetricPoint  `json:"memory"`
	Topology    []TopologyEdge `json:"topology"`
}

func DashboardHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		payload := Dashboard{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Devices:     []DeviceStatus{}, Events: []Event{}, Latency: []MetricPoint{}, CPU: []MetricPoint{}, Memory: []MetricPoint{}, Topology: []TopologyEdge{},
		}

		err := database.Pool.QueryRow(ctx, `
			WITH latest AS (
				SELECT DISTINCT ON (device_id) device_id, icmp_status
				FROM device_health_metrics WHERE time > NOW() - INTERVAL '5 minutes'
				ORDER BY device_id, time DESC
			)
			SELECT COUNT(*) FILTER (WHERE d.is_monitored),
			       COUNT(*) FILTER (WHERE d.is_monitored AND l.icmp_status = 1),
			       COUNT(*) FILTER (WHERE d.is_monitored AND COALESCE(l.icmp_status, 0) = 0),
			       (SELECT COUNT(*) FROM network_events WHERE resolved = FALSE)
			FROM devices d LEFT JOIN latest l ON l.device_id = d.id
		`).Scan(&payload.Overview.Monitored, &payload.Overview.Online, &payload.Overview.Offline, &payload.Overview.OpenAlerts)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "dashboard query failed"})
			return
		}

		rows, err := database.Pool.Query(ctx, `
			SELECT d.id, d.hostname, d.ip_address::text, d.device_type, COALESCE(d.location, 'Unassigned'),
			       CASE WHEN m.icmp_status = 1 THEN 'online' ELSE 'offline' END,
			       m.icmp_rtt_ms, to_char(m.time AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
			FROM devices d
			LEFT JOIN LATERAL (
				SELECT icmp_status, icmp_rtt_ms, time FROM device_health_metrics
				WHERE device_id = d.id ORDER BY time DESC LIMIT 1
			) m ON TRUE WHERE d.is_monitored ORDER BY d.hostname
		`)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "device query failed"})
			return
		}
		for rows.Next() {
			var item DeviceStatus
			if err := rows.Scan(&item.ID, &item.Hostname, &item.IPAddress, &item.Type, &item.Location, &item.Status, &item.RTT, &item.LastUpdate); err != nil {
				rows.Close()
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "device query failed"})
				return
			}
			payload.Devices = append(payload.Devices, item)
		}
		rows.Close()

		eventRows, err := database.Pool.Query(ctx, `SELECT e.time, COALESCE(d.hostname, 'System'), e.severity, e.event_type, COALESCE(e.message, '') FROM network_events e LEFT JOIN devices d ON d.id=e.device_id WHERE e.resolved=FALSE ORDER BY e.time DESC LIMIT 20`)
		if err == nil {
			for eventRows.Next() {
				var e Event
				var at time.Time
				if eventRows.Scan(&at, &e.Device, &e.Severity, &e.Type, &e.Message) == nil {
					e.Time = at.UTC().Format(time.RFC3339)
					payload.Events = append(payload.Events, e)
				}
			}
			eventRows.Close()
		}

		payload.Latency = metricSeries(ctx, database, `SELECT m.time, d.hostname, COALESCE(m.icmp_rtt_ms, 0) FROM device_health_metrics m JOIN devices d ON d.id=m.device_id WHERE m.time > NOW()-INTERVAL '30 minutes' ORDER BY m.time`)
		payload.CPU = metricSeries(ctx, database, `SELECT h.time, COALESCE(d.hostname,h.agent_host,h."sysName",'Unknown'), COALESCE(100.0-h.cpu_idle, LEAST(h.cpu_load_1*25.0,100.0),0) FROM snmp_device_health h LEFT JOIN devices d ON d.ip_address=NULLIF(h.agent_host,'')::inet WHERE h.time > NOW()-INTERVAL '30 minutes' ORDER BY h.time`)
		payload.Memory = metricSeries(ctx, database, `SELECT h.time, COALESCE(d.hostname,h.agent_host,h."sysName",'Unknown'), COALESCE(100.0*(h.mem_total_kb-h.mem_available_kb)/NULLIF(h.mem_total_kb,0),0) FROM snmp_device_health h LEFT JOIN devices d ON d.ip_address=NULLIF(h.agent_host,'')::inet WHERE h.time > NOW()-INTERVAL '30 minutes' ORDER BY h.time`)

		topologyRows, err := database.Pool.Query(ctx, `SELECT id::text, source_device, target_device, COALESCE(source_port,''), COALESCE(target_port,'') FROM network_topology ORDER BY source_device, target_device`)
		if err == nil {
			for topologyRows.Next() {
				var edge TopologyEdge
				if topologyRows.Scan(&edge.ID, &edge.Source, &edge.Target, &edge.SourcePort, &edge.TargetPort) == nil {
					payload.Topology = append(payload.Topology, edge)
				}
			}
			topologyRows.Close()
		}

		writeJSON(w, http.StatusOK, payload)
	}
}

func metricSeries(ctx context.Context, database *db.DB, query string) []MetricPoint {
	rows, err := database.Pool.Query(ctx, query)
	if err != nil {
		return []MetricPoint{}
	}
	defer rows.Close()
	result := []MetricPoint{}
	for rows.Next() {
		var p MetricPoint
		var at time.Time
		if rows.Scan(&at, &p.Device, &p.Value) == nil {
			p.Time = at.UTC().Format(time.RFC3339)
			result = append(result, p)
		}
	}
	return result
}
