package topology

import (
	"context"
	"log"
	"time"

	"nms-middleware/db"
)

// StartTopologyEngine runs the background polling loop
func StartTopologyEngine(database *db.DB, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("[Topology] Scanning LLDP matrix for network discovery...")
			if err := syncTopology(database); err != nil {
				log.Printf("[Topology Error] Processing sync failed: %v", err)
			}
		}
	}()
}

func syncTopology(database *db.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Single resolved query: JOIN snmp_lldp_topology against the devices inventory
	// to resolve raw IP agent_host values into registered hostnames.
	// Falls back to the raw IP string if no match found so we never silently drop edges.
	// Uses canonical Telegraf column names: lldp_rem_sys_name (remote hostname),
	// lldp_rem_port_id (remote port). The lldpRemTable index encodes the local port number
	// as the third component of the OID index (lldp_local_port_num).
	rows, err := database.Pool.Query(ctx, `
		SELECT
			COALESCE(src.hostname, lldp.agent_host::text)  AS source_device,
			lldp.lldp_local_port_num::text                 AS source_port,
			COALESCE(NULLIF(lldp.lldp_rem_sys_name, ''), lldp.agent_host::text) AS target_device,
			COALESCE(NULLIF(lldp.lldp_rem_port_id, ''), 'unknown')              AS target_port
		FROM snmp_lldp_topology lldp
		LEFT JOIN devices src ON src.ip_address = lldp.agent_host
		WHERE lldp.time > NOW() - INTERVAL '5 minutes'
		  AND lldp.lldp_rem_sys_name IS NOT NULL
		  AND lldp.lldp_rem_sys_name <> ''
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	committedCount := 0
	skippedCount := 0
	for rows.Next() {
		var sourceDevice, sourcePort, targetDevice, targetPort string
		if err := rows.Scan(&sourceDevice, &sourcePort, &targetDevice, &targetPort); err != nil {
			log.Printf("[Topology Error] Failed to scan resolved edge row: %v", err)
			skippedCount++
			continue
		}

		// Guard: do not write edges where the target resolved to a bare IP or 'unknown'
		if targetDevice == "unknown" {
			log.Printf("[Topology Warning] Skipping unresolved edge from source '%s' — target_device could not be resolved", sourceDevice)
			skippedCount++
			continue
		}

		_, err := database.Pool.Exec(ctx, `
			INSERT INTO network_topology (source_device, source_port, target_device, target_port, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT DO NOTHING`,
			sourceDevice, sourcePort, targetDevice, targetPort,
		)
		if err != nil {
			log.Printf("[Topology DB Error] Edge upsert failed (%s -> %s): %v", sourceDevice, targetDevice, err)
		} else {
			committedCount++
		}
	}

	log.Printf("[Topology] Sync complete: %d edges committed, %d skipped", committedCount, skippedCount)
	return nil
}
