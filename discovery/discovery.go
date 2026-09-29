package discovery

import (
	"context"
	"log"
	"time"

	"nms-middleware/db"
)

// StartDiscoveryEngine runs a loop checking for unmanaged reporting agents until ctx is cancelled.
func StartDiscoveryEngine(ctx context.Context, database *db.DB, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[Discovery] Context cancelled, stopping discovery engine.")
				return
			case <-ticker.C:
				log.Println("[Discovery] Scanning metric pools for new network assets...")
				if err := discoverNewDevices(database); err != nil {
					log.Printf("[Discovery Error] Auto-discovery scan failed: %v", err)
				}
			}
		}
	}()
}

func discoverNewDevices(database *db.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Find any agent_host from raw telemetry that isn't registered in the devices inventory yet
	rows, err := database.Pool.Query(ctx, `
		SELECT DISTINCT agent_host 
		FROM snmp_interface 
		WHERE time > NOW() - INTERVAL '10 minutes'
		AND agent_host NOT IN (SELECT ip_address::text FROM devices);
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var newIP string
		if err := rows.Scan(&newIP); err != nil {
			continue
		}

		log.Printf("[Discovery Alert] Found new unmanaged reporting host: %s. Provisioning asset record...", newIP)

		// Automatically seed the device inventory table with a generic fallback hostname
		_, err = database.Pool.Exec(ctx, `
			INSERT INTO devices (hostname, ip_address, device_type, snmp_version, snmp_community, is_monitored)
			VALUES ($1, $2, 'unknown_discovered', 'v2c', 'public', true)
			ON CONFLICT DO NOTHING;
		`, "discovered-"+newIP, newIP)
		if err != nil {
			log.Printf("[Discovery Error] Failed to auto-provision host %s: %v", newIP, err)
		}
	}
	return nil
}
