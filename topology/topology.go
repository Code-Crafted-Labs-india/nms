package topology

import (
	"context"
	"log"
	"time"

	"nms-middleware/db"
)

type Edge struct {
	SourcePort   string
	TargetDevice string
	TargetPort   string
}

// NetworkGraph maps a source hostname to its discovered neighbors
type NetworkGraph map[string][]Edge

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

	// 1. Fetch live metrics stream captured via Telegraf SNMP
	// Note: Explicitly targets 'snmp_lldp_topology' to align with our custom Telegraf table
	rows, err := database.Pool.Query(ctx, `
		SELECT 
			agent_host, 
			COALESCE(target_device, 'unknown') as target,
			COALESCE(target_port, 'unknown') as port
		FROM snmp_lldp_topology
		WHERE time > NOW() - INTERVAL '5 minutes'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	graph := make(NetworkGraph)
	for rows.Next() {
		var source, target, targetPort string
		if err := rows.Scan(&source, &target, &targetPort); err != nil {
			continue
		}

		graph[source] = append(graph[source], Edge{
			SourcePort:   "eth0", // Mock standard uplink interface fallback
			TargetDevice: target,
			TargetPort:   targetPort,
		})
	}

	// 2. Commit distinct resolved node edges back to database topology log
	for source, edges := range graph {
		for _, edge := range edges {
			_, err := database.Pool.Exec(ctx, `
				INSERT INTO network_topology (source_device, source_port, target_device, target_port, updated_at)
				VALUES ($1, $2, $3, $4, NOW())
				ON CONFLICT DO NOTHING`,
				source, edge.SourcePort, edge.TargetDevice, edge.TargetPort,
			)
			if err != nil {
				log.Printf("[Topology DB Error] Edge execution failed: %v", err)
			}
		}
	}
	return nil
}
