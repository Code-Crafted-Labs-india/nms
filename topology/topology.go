package topology

import (
	"context"
	"log"
	"time"

	"nms-middleware/db"
)

// StartTopologyEngine runs the background polling loop until ctx is cancelled.
func StartTopologyEngine(ctx context.Context, database *db.DB, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[Topology] Context cancelled, stopping topology engine.")
				return
			case <-ticker.C:
				log.Println("[Topology] Scanning LLDP matrix for network discovery...")
				if err := syncTopology(database); err != nil {
					log.Printf("[Topology Error] Processing sync failed: %v", err)
				}
			}
		}
	}()
}

func syncTopology(database *db.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Resolve live LLDP-MIB rows to the local device inventory. The remote system
	// name is optional in LLDP, so use the advertised chassis ID as the neighbor
	// identifier when a remote system name is unavailable. Telegraf's index tag
	// encodes timeMark.localPortNum.remIndex; the second component is the local port.
	rows, err := database.Pool.Query(ctx, `
		SELECT
			COALESCE(src.hostname, lldp.agent_host::text) AS source_device,
			COALESCE(lldp.lldp_local_port_num::text, NULLIF(split_part(lldp."index", '.', 2), ''), 'unknown') AS source_port,
			COALESCE(dst.hostname, NULLIF(BTRIM(lldp.lldp_rem_sys_name), ''), NULLIF(BTRIM(lldp.lldp_rem_chassis_id), '')) AS target_device,
			COALESCE(NULLIF(BTRIM(lldp.lldp_rem_port_id), ''), 'unknown') AS target_port
		FROM snmp_lldp_topology lldp
		LEFT JOIN devices src ON src.ip_address = lldp.agent_host::inet
		LEFT JOIN LATERAL (
			SELECT d.hostname
			FROM devices d
			WHERE NULLIF(BTRIM(lldp.lldp_rem_sys_name), '') IS NOT NULL
			  AND (LOWER(d.hostname) = LOWER(BTRIM(lldp.lldp_rem_sys_name))
			       OR LOWER(d.hostname) = LOWER(SPLIT_PART(BTRIM(lldp.lldp_rem_sys_name), '.', 1)))
			ORDER BY CASE WHEN LOWER(d.hostname) = LOWER(BTRIM(lldp.lldp_rem_sys_name)) THEN 0 ELSE 1 END
			LIMIT 1
		) dst ON TRUE
		WHERE lldp.time > NOW() - INTERVAL '5 minutes'
		  AND lldp.agent_host IS NOT NULL
		  AND (NULLIF(BTRIM(lldp.lldp_rem_sys_name), '') IS NOT NULL
		       OR NULLIF(BTRIM(lldp.lldp_rem_chassis_id), '') IS NOT NULL)
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

		// Refresh the observation timestamp for a live edge. UPDATE + INSERT also
		// works with older installations whose topology table predates the unique
		// constraint used by current migrations.
		tag, err := database.Pool.Exec(ctx, `
			UPDATE network_topology
			SET time = NOW(), updated_at = NOW()
			WHERE source_device = $1
			  AND source_port IS NOT DISTINCT FROM $2
			  AND target_device = $3
			  AND target_port IS NOT DISTINCT FROM $4`,
			sourceDevice, sourcePort, targetDevice, targetPort,
		)
		if err != nil {
			log.Printf("[Topology DB Error] Edge refresh failed (%s -> %s): %v", sourceDevice, targetDevice, err)
			skippedCount++
			continue
		}
		if tag.RowsAffected() == 0 {
			_, err = database.Pool.Exec(ctx, `
				INSERT INTO network_topology (source_device, source_port, target_device, target_port, updated_at)
				VALUES ($1, $2, $3, $4, NOW())`,
				sourceDevice, sourcePort, targetDevice, targetPort,
			)
			if err != nil {
				log.Printf("[Topology DB Error] Edge insert failed (%s -> %s): %v", sourceDevice, targetDevice, err)
				skippedCount++
				continue
			}
		}
		committedCount++
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// network_topology is the current graph, not an event history. Expire links
	// that have not been observed in the raw LLDP table during its five-minute
	// freshness window.
	if _, err := database.Pool.Exec(ctx, `DELETE FROM network_topology WHERE updated_at < NOW() - INTERVAL '5 minutes'`); err != nil {
		return err
	}

	log.Printf("[Topology] Sync complete: %d edges committed, %d skipped", committedCount, skippedCount)
	return nil
}
