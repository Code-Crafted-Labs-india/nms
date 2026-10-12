// Package telegrafconfig renders Telegraf SNMP inputs from monitored inventory.
package telegrafconfig

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"nms-middleware/db"
)

type target struct {
	address   string
	version   string
	community string
}

type credentialGroup struct {
	version   int
	community string
	agents    []string
}

// Sync writes a valid Telegraf input fragment for the monitored inventory.
// The destination directory is shared with Telegraf as a read-only mount.
func Sync(ctx context.Context, database *db.DB, directory string, logger *slog.Logger) (bool, error) {
	rows, err := database.Pool.Query(ctx, `
		SELECT host(ip_address), snmp_version, snmp_community
		FROM devices d
		WHERE d.is_monitored = TRUE
		  AND d.device_type <> 'unknown_discovered'
		  AND ((SELECT setting_value FROM nms_runtime_settings WHERE setting_key = 'monitoring_mode') = 'test' AND LOWER(d.hostname) LIKE 'mock-%'
		    OR (SELECT setting_value FROM nms_runtime_settings WHERE setting_key = 'monitoring_mode') = 'production' AND LOWER(d.hostname) NOT LIKE 'mock-%')
		ORDER BY ip_address
	`)
	if err != nil {
		return false, fmt.Errorf("query monitored SNMP targets: %w", err)
	}
	defer rows.Close()

	var targets []target
	for rows.Next() {
		var item target
		if err := rows.Scan(&item.address, &item.version, &item.community); err != nil {
			return false, fmt.Errorf("read SNMP target: %w", err)
		}
		if item.address == "" || item.community == "" {
			logger.Warn("skipping Telegraf target without an address or community", "address", item.address)
			continue
		}
		if item.version != "v1" && item.version != "v2c" {
			logger.Warn("skipping Telegraf target with unsupported SNMP version", "address", item.address, "version", item.version)
			continue
		}
		targets = append(targets, item)
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate SNMP targets: %w", err)
	}

	contents := render(targets)
	path := filepath.Join(directory, "inventory-targets.conf")
	previous, err := os.ReadFile(path)
	if err == nil && string(previous) == contents {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read existing generated config: %w", err)
	}
	if err := os.MkdirAll(directory, 0750); err != nil {
		return false, fmt.Errorf("create generated config directory: %w", err)
	}
	tmp, err := os.CreateTemp(directory, ".inventory-targets-*.conf")
	if err != nil {
		return false, fmt.Errorf("create temporary generated config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0640); err != nil {
		tmp.Close()
		return false, fmt.Errorf("set generated config permissions: %w", err)
	}
	if _, err := tmp.WriteString(contents); err != nil {
		tmp.Close()
		return false, fmt.Errorf("write generated config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return false, fmt.Errorf("sync generated config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("close generated config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return false, fmt.Errorf("activate generated config: %w", err)
	}
	return true, nil
}

func render(targets []target) string {
	groupsByKey := make(map[string]*credentialGroup)
	for _, item := range targets {
		version := 2
		if item.version == "v1" {
			version = 1
		}
		key := fmt.Sprintf("%d\x00%s", version, item.community)
		group := groupsByKey[key]
		if group == nil {
			group = &credentialGroup{version: version, community: item.community}
			groupsByKey[key] = group
		}
		group.agents = append(group.agents, "udp://"+net.JoinHostPort(item.address, "161"))
	}

	groups := make([]*credentialGroup, 0, len(groupsByKey))
	for _, group := range groupsByKey {
		sort.Strings(group.agents)
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].version != groups[j].version {
			return groups[i].version < groups[j].version
		}
		return groups[i].community < groups[j].community
	})

	var b strings.Builder
	b.WriteString("# Generated from monitored device inventory. Do not edit by hand.\n")
	b.WriteString("[[inputs.internal]]\n\n")
	for _, group := range groups {
		writeAgentList(&b, "inputs.snmp", group)
		b.WriteString("  timeout = \"3s\"\n  retries = 2\n  name_override = \"snmp_device_health\"\n")
		b.WriteString("  [[inputs.snmp.field]]\n    name = \"sysName\"\n    oid = \"1.3.6.1.2.1.1.5.0\"\n    is_tag = true\n")
		b.WriteString("  [[inputs.snmp.field]]\n    name = \"sysUpTime\"\n    oid = \"1.3.6.1.2.1.1.3.0\"\n\n")

		writeAgentList(&b, "inputs.snmp", group)
		b.WriteString("  timeout = \"3s\"\n  retries = 2\n")
		b.WriteString("  [[inputs.snmp.field]]\n    name = \"sysName\"\n    oid = \"1.3.6.1.2.1.1.5.0\"\n    is_tag = true\n")
		b.WriteString("  [[inputs.snmp.table]]\n    name = \"interface_performance_metrics\"\n    inherit_tags = [\"sysName\"]\n")
		fields := []struct{ name, oid string }{
			{"ifIndex", "1.3.6.1.2.1.2.2.1.1"}, {"ifDescr", "1.3.6.1.2.1.2.2.1.2"},
			{"ifSpeed", "1.3.6.1.2.1.2.2.1.5"}, {"ifHighSpeed", "1.3.6.1.2.1.31.1.1.1.15"},
			{"ifAdminStatus", "1.3.6.1.2.1.2.2.1.7"}, {"ifOperStatus", "1.3.6.1.2.1.2.2.1.8"},
			{"ifInOctets", "1.3.6.1.2.1.2.2.1.10"}, {"ifHCInOctets", "1.3.6.1.2.1.31.1.1.1.6"},
			{"ifInDiscards", "1.3.6.1.2.1.2.2.1.13"}, {"ifInErrors", "1.3.6.1.2.1.2.2.1.14"},
			{"ifOutOctets", "1.3.6.1.2.1.2.2.1.16"}, {"ifHCOutOctets", "1.3.6.1.2.1.31.1.1.1.10"},
			{"ifOutDiscards", "1.3.6.1.2.1.2.2.1.19"}, {"ifOutErrors", "1.3.6.1.2.1.2.2.1.20"},
		}
		for _, field := range fields {
			b.WriteString("    [[inputs.snmp.table.field]]\n")
			b.WriteString("      name = " + strconv.Quote(field.name) + "\n")
			b.WriteString("      oid = " + strconv.Quote(field.oid) + "\n")
			if field.name == "ifIndex" || field.name == "ifDescr" {
				b.WriteString("      is_tag = true\n")
			}
		}
		b.WriteString("\n")

		writeAgentList(&b, "inputs.snmp", group)
		b.WriteString("  interval = \"60s\"\n  timeout = \"3s\"\n  retries = 2\n")
		b.WriteString("  [[inputs.snmp.table]]\n    name = \"snmp_lldp_topology\"\n    index_as_tag = true\n")
		lldpFields := []struct{ name, oid string }{
			{"lldp_rem_chassis_id", "1.0.8802.1.1.2.1.4.1.1.5"},
			{"lldp_rem_port_id", "1.0.8802.1.1.2.1.4.1.1.7"},
			{"lldp_rem_sys_name", "1.0.8802.1.1.2.1.4.1.1.9"},
		}
		for _, field := range lldpFields {
			b.WriteString("    [[inputs.snmp.table.field]]\n      name = " + strconv.Quote(field.name) + "\n      oid = " + strconv.Quote(field.oid) + "\n")
		}
		b.WriteString("\n")
	}

	if len(targets) > 0 {
		b.WriteString("[[inputs.ping]]\n  count = 2\n  timeout = 1.0\n  interval = \"15s\"\n  urls = [")
		for i, item := range targets {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(strconv.Quote(item.address))
		}
		b.WriteString("]\n")
	}
	return b.String()
}

func writeAgentList(b *strings.Builder, plugin string, group *credentialGroup) {
	b.WriteString("[[" + plugin + "]]\n  agents = [")
	for i, agent := range group.agents {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(agent))
	}
	b.WriteString("]\n")
	b.WriteString("  version = " + strconv.Itoa(group.version) + "\n")
	b.WriteString("  community = " + strconv.Quote(group.community) + "\n")
}

// Start periodically refreshes targets so inventory changes reach Telegraf.
func Start(ctx context.Context, database *db.DB, directory string, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		changed, err := Sync(ctx, database, directory, logger)
		if err != nil {
			// Keep the previous valid file on errors; the next pass retries.
			logger.Error("failed to synchronize Telegraf targets; keeping previous config", "error", err)
		} else if changed {
			logger.Info("synchronized Telegraf targets from inventory")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
