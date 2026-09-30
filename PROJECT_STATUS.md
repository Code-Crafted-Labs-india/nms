# 📑 PROJECT_STATUS.md

## 1. 🚀 System Architecture Overview
The NMS ecosystem ("NetPulse Core") is wired together as a decoupled, multi-tier data pipeline designed to prevent blocking bottlenecks, guarantee processing throughput, and safely scale.

The data flows through 4 distinct layers:
`Simulated Hardware (SNMPd Alpine Containers) ──> Ingestion Agent (Telegraf) ──> Storage Layer (TimescaleDB/PostgreSQL) ──> Processing Middleware (Go Backend) ──> Presentation Layer (React Command Center)`.

1. **Simulated Hardware**: Containerized network endpoints exposing SNMP and responding to ICMP.
2. **Ingestion Agent**: Telegraf polls SNMP metrics from the devices and dumps raw telemetry directly into TimescaleDB.
3. **Storage Layer**: PostgreSQL equipped with TimescaleDB extension maintains strict relational inventory while ingesting high-velocity time-series metrics into hypertables.
4. **Processing Middleware**: A concurrent Go backend executes high-speed ICMP sweeps, processes raw telemetry into actionable relational graphs (LLDP topology), provisions unmanaged devices (Auto-Discovery), and exposes a REST API for management.
5. **Presentation Layer**: The secured Go API supplies the Bun/React command center with fleet health, time-series, alarm, inventory, and topology data. Grafana remains an optional loopback-only engineering aid during feature parity and troubleshooting.

## 2. 🗄️ Relational Database & Table Schema Map
Based on our verified PostgreSQL schema, here is the relational and time-series data map:

| Table | Type | Purpose | Primary/Foreign Keys & Connections |
|---|---|---|---|
| `devices` | Relational | Inventory catalog of managed elements. | `id` (PK). Connected to `network_topology`, `device_health_metrics`, and `interfaces`. |
| `network_topology` | Relational | Graph adjacency list tracking source/target device physical connections. | `id` (PK), `source_device`, `target_device`. |
| `device_health_metrics` | Hypertable | TimescaleDB hypertable tracking node availability (ICMP) and core resource utilization (CPU/Mem). | Partitioned by `time`. Connected to `devices` via `device_id` (FK). |
| `interface_performance_metrics` | Hypertable | TimescaleDB hypertable tracking port speeds, states, and bandwidth deltas. | Partitioned by `time`. Connected to `interfaces` via `interface_id` (FK). |
| `snmp` | Raw Telemetry | Telegraf SNMP sink for standard MIB-II system polling. | Written purely by Telegraf. |
| `snmp_interface` | Raw Telemetry | Telegraf SNMP sink for interface metrics. Scanned by Go `discovery` engine. | Written purely by Telegraf. |
| `snmp_lldp_topology` | Raw Telemetry | Telegraf SNMP sink for `LLDP-MIB::lldpRemTable`. Scanned by Go `topology` engine. | Written purely by Telegraf. |

## 📦 3. Modular Go Package Inventory & State Tracking

| Package / Component | Responsibility | Current State |
|---|---|---|
| `db/` | Database connection pooling implementation utilizing `pgx`/`pgxpool` for thread-safe concurrent transactions. | ✅ Stable |
| `topology/` | Layer 2 live graph sync worker. Background routine continuously scans `snmp_lldp_topology` for neighbors and writes to `network_topology`. | ✅ Stable |
| `discovery/` | Automated asset scanner. Goroutine loop querying `snmp_interface` for unknown reporting IPs and provisioning them into `devices`. | ✅ Stable |
| `handlers/` | HTTP REST API handling full CRUD operations for device inventory management via native `http.ServeMux`. | ✅ Stable |
| `main.go` | The master orchestration layer. Bootstraps environment configs, db pools, background worker loops, and the API multiplexer. | ✅ Stable |

## 🔄 4. The Live Topology Graph Data Lifecycle
The physical link discovery and visualization lifecycle operates completely automatically:

1. **Telegraf Queries**: Telegraf utilizes `inputs.snmp.table` to query `LLDP-MIB::lldpRemTable` across all monitored devices.
2. **Raw Storage**: Telegraf saves the resulting MIB payload natively into the `snmp_lldp_topology` table.
3. **Go Processing**: The Go `topology` engine sweeps `snmp_lldp_topology` every 30 seconds. It parses the `agent_host` and `target_device` (mapping missing edge adjacencies) and commits them as discrete edge pairs into the relational `network_topology` table.
4. **React Presentation**: The authenticated dashboard API returns the normalized nodes, live device status, and topology edges. The React topology view renders the graph and marks online, down, and unknown devices without exposing PostgreSQL to the browser.

**Underlying node query:**
```sql
SELECT 
  hostname AS id,
  hostname AS title,
  device_type AS subTitle,
  ip_address::text AS mainStat
FROM devices;
```

**Underlying edge query:**
```sql
SELECT 
  id::text AS id,
  source_device AS source,
  target_device AS target
FROM network_topology;
```

## 🐛 5. Known Constraints, Discovered Caveats & Potential Bugs

### The Identifier/Naming Mismatch Warning
If Telegraf writes a bare IP address to `agent_host` in the telemetry tables, but our Go topology package relies on writing raw string hostnames into `network_topology`, the Grafana Node Graph rendering engine will break. The Node Graph **requires absolute character string matching** between `Edges.source`/`Edges.target` and `Nodes.id`. We must ensure hostnames are cleanly resolved and identically mapped across all pipelines.

### The Overwrite Trap
Specifying a `command:` block in `docker-compose.yml` completely suppresses the native `CMD` string instructed inside `Dockerfile.snmpd`. Because Alpine Linux behaves uniquely with the net-snmp daemon, we must explicitly include the runtime flags `-I -smux,mteTrigger,mteTriggerConf` in our `docker-compose.yml` command arrays to keep the bare Alpine daemons alive without crashing natively on boot.

### Database Deadlocks
Concurrent writes executing continuously from the topology sync loop (`network_topology`) and the auto-discovery loops (`devices`) can cause locking contentions or race conditions at the database layer. We successfully mitigate this by aggressively enforcing `ON CONFLICT DO NOTHING` statements in our PostgreSQL `INSERT` logic and delegating deduplication constraints directly to the database engine.
