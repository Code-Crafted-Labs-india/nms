# 📑 PROJECT_STATUS.md

> Last updated: 2026-10-01. Scope source: [`docs/product-future-scope.md`](docs/product-future-scope.md)

---

## 1. 🚀 System Architecture Overview

The NMS ecosystem ("NetPulse Core") is wired together as a decoupled, multi-tier data pipeline designed to prevent blocking bottlenecks, guarantee processing throughput, and safely scale.

The data flows through 4 distinct layers:
`Simulated Hardware (SNMPd Alpine Containers) ──> Ingestion Agent (Telegraf) ──> Storage Layer (TimescaleDB/PostgreSQL) ──> Processing Middleware (Go Backend) ──> Presentation Layer (React Command Center)`.

1. **Simulated Hardware**: Containerized network endpoints exposing SNMP and responding to ICMP.
2. **Ingestion Agent**: Telegraf polls SNMP metrics from the devices and dumps raw telemetry directly into TimescaleDB.
3. **Storage Layer**: PostgreSQL equipped with TimescaleDB extension maintains strict relational inventory while ingesting high-velocity time-series metrics into hypertables.
4. **Processing Middleware**: A concurrent Go backend executes high-speed ICMP sweeps, processes raw telemetry into actionable relational graphs (LLDP topology), provisions unmanaged devices (Auto-Discovery), evaluates 6 alarm strategies, and exposes a REST API for management.
5. **Presentation Layer**: The secured Go API supplies the Bun/React command center with fleet health, time-series, alarm, inventory, topology, and device-detail data. Grafana remains an optional loopback-only engineering aid.

---

## 2. 🗄️ Relational Database & Table Schema Map

| Table | Type | Purpose | Primary/Foreign Keys & Connections |
|---|---|---|---|
| `devices` | Relational | Inventory catalog of managed elements. | `id` (PK). Connected to `network_topology`, `device_health_metrics`, and `interfaces`. |
| `device_models` | Relational | Supported hardware SKU catalog. Gates AddDevice workflow. | `id` (PK), `UNIQUE(model_name)`. |
| `interfaces` | Relational | Per-device interface index and descriptor. | `id` (PK), `device_id` (FK → devices). |
| `network_topology` | Relational | Graph adjacency list tracking source/target device physical connections. | `id` (PK), `source_device`, `target_device`. |
| `network_events` | Hypertable | Alarm lifecycle store. Stores trigger, severity, resolution, and timestamps. | Partitioned by `time`. `device_id` (FK → devices). `alarm_id` maps to catalog below. |
| `device_health_metrics` | Hypertable | TimescaleDB hypertable tracking node availability (ICMP) and core resource utilization (CPU/Mem). | Partitioned by `time`. Connected to `devices` via `device_id` (FK). |
| `interface_performance_metrics` | Hypertable | TimescaleDB hypertable tracking port speeds, states, and bandwidth deltas. | Partitioned by `time`. Connected to `interfaces` via `interface_id` (FK). |
| `snmp_device_health` | Hypertable | Telegraf SNMP sink for system-level health (CPU idle, memory). Used by alarm rules #3 and #6. | Partitioned by `time`. Written by Telegraf. |
| `snmp` | Raw Telemetry | Telegraf SNMP sink for standard MIB-II system polling. | Written purely by Telegraf. |
| `snmp_interface` | Raw Telemetry | Telegraf SNMP sink for interface metrics. Scanned by Go `discovery` engine. | Written purely by Telegraf. |
| `snmp_lldp_topology` | Raw Telemetry | Telegraf SNMP sink for `LLDP-MIB::lldpRemTable`. Scanned by Go `topology` engine. | Written purely by Telegraf. |

### Alarm ID Catalog

| Alarm ID | Condition | Severity | Strategy |
|---|---|---|---|
| 1 | Device down (ICMP failure) | CRITICAL | `DeviceDownStrategy` |
| 2 | Link down (ifOperStatus=DOWN, admin=UP) | WARNING | `LinkStateStrategy` |
| 3 | High CPU utilization (>85%, 2-min avg) | CRITICAL | `HighCPUUtilizationStrategy` |
| 4 | Admin/oper mismatch (admin=UP, oper=DOWN) | WARNING | `AdminOperMismatchStrategy` |
| 5 | High memory utilization (>80% warn, >95% crit) | WARNING/CRITICAL | `HighMemoryUtilizationStrategy` |
| 6 | SNMP communication failure (no poll data in 5 min) | CRITICAL | `SNMPCommunicationFailureStrategy` |

---

## 📦 3. Modular Go Package Inventory & State Tracking

| Package / Component | Responsibility | Current State |
|---|---|---|
| `db/` | Database connection pooling (`pgx`/`pgxpool`) + self-healing schema migration engine. | ✅ Stable |
| `topology/` | Layer 2 live graph sync worker. Scans `snmp_lldp_topology` → writes `network_topology`. | ✅ Stable |
| `discovery/` | Automated asset scanner. Provisions unknown devices from `snmp_interface` into `devices`. | ✅ Stable |
| `ping/` | High-speed concurrent ICMP sweep engine. Writes `device_health_metrics`. | ✅ Stable |
| `alert/` | Alarm evaluation engine: worker pool dispatches 6 strategies against all monitored devices every 30s. All strategies now support **auto-clear on recovery** with hysteresis. | ✅ Stable |
| `handlers/dashboard.go` | `GET /api/v1/dashboard` — overview, devices, events, metrics, topology. | ✅ Stable |
| `handlers/devices.go` | `POST/PUT/DELETE /api/devices` — device CRUD. | ✅ Stable |
| `handlers/inventory.go` | `GET /api/v1/devices` — paginated/filtered fleet inventory. `GET /api/v1/devices/{id}` — full device detail. `POST /api/v1/alarms/{id}/resolve` — manual alarm resolution. | ✅ New (2026-10-01) |
| `handlers/security.go` | Session auth, CSRF, CORS, RBAC middleware. | ✅ Stable |
| `main.go` | Orchestration: bootstraps DB, background workers, alarm engine (6 strategies), and REST API mux. | ✅ Stable |

---

## 🔄 4. The Live Topology Graph Data Lifecycle

The physical link discovery and visualization lifecycle operates completely automatically:

1. **Telegraf Queries**: Telegraf utilizes `inputs.snmp.table` to query `LLDP-MIB::lldpRemTable` across all monitored devices.
2. **Raw Storage**: Telegraf saves the resulting MIB payload natively into the `snmp_lldp_topology` table.
3. **Go Processing**: The Go `topology` engine sweeps `snmp_lldp_topology` every 30 seconds. It parses the `agent_host` and `target_device` and commits them as discrete edge pairs into the relational `network_topology` table.
4. **React Presentation**: The authenticated dashboard API returns the normalized nodes, live device status, and topology edges. The React topology view renders the graph and marks online, down, and unknown devices.

---

## 📊 5. Scope-to-Implementation Progress Tracker

Based on [`docs/product-future-scope.md`](docs/product-future-scope.md):

### Release A — Inventory and Monitoring Foundation

| Scope Item | Priority | Status | Implementation |
|---|---|---|---|
| Add-device form and credential-profile APIs | P0 | 🟡 Partial | `POST /api/devices` — basic add with `device_models` catalog gate. Credential profile table not yet separate. |
| Fleet inventory with pagination and filters | P0 | ✅ Done | `GET /api/v1/devices` — paginated (50/page), filterable by status, type, hostname/IP search. |
| Device overview tab | P0 | ✅ Done | `GET /api/v1/devices/{id}` — identity, reachability, CPU/mem, alarm count. |
| Device interfaces tab | P0 | ✅ Done | Returns per-interface oper/admin/speed/rx/tx/errors/optical in device detail response. |
| Device events/alarms tab | P0 | ✅ Done | Returns last 50 alarms (resolved + open) per device. Manual resolve via `POST /api/v1/alarms/{id}/resolve`. |
| Audit trail for device administration | P0 | 🔴 Not started | Audit log table and middleware not yet implemented. |
| Canonical device/interface telemetry normalization | P0 | 🟡 Partial | Telegraf → TimescaleDB raw. Go discovery normalizes devices. Interface join in alarm queries exists but not fully canonical. |
| Inventory-driven ICMP and SNMP polling | P0 | 🟡 Partial | ICMP sweeps are inventory-driven (`ping` pkg). SNMP still Telegraf-static config. |
| Stateful alarm schema with fingerprints, transitions, auto-recovery | P0 | 🟡 Partial | `resolved`/`resolved_at` lifecycle exists. All 6 strategies now auto-clear on recovery with hysteresis. Deterministic fingerprints (stable alarm code per device+resource) not yet enforced — duplicate prevention via read-before-write only. |
| Device-down alarm (P0 #1) | P0 | ✅ Done | `DeviceDownStrategy` — consecutive ICMP failures, telemetry-silence suppression, auto-clear. |
| Interface-down alarm (P0 #2) | P0 | ✅ Done | `LinkStateStrategy` — oper=DOWN with admin=UP guard, auto-clear on recovery. |
| High memory usage alarm (P0 #3) | P0 | ✅ Done | `HighMemoryUtilizationStrategy` — warning=80%, critical=95%, hysteresis, auto-clear. |
| SNMP communication failure alarm (P0 #4) | P0 | ✅ Done | `SNMPCommunicationFailureStrategy` — stale window=5min, checks `snmp_device_health` + `snmp_interface`, auto-clear. |
| Device reachability monitoring (P0 #5) | P0 | ✅ Done | `ping` package + `device_health_metrics` hypertable with ICMP status, RTT, packet loss. |
| High CPU alarm (#3 CPU path) | P0 | ✅ Done | `HighCPUUtilizationStrategy` — threshold=85%, hysteresis at 80%, auto-clear. |

### Release B — Operator Workflows

| Scope Item | Priority | Status | Notes |
|---|---|---|---|
| Dashboard builder and widget catalog | P1 | 🔴 Not started | Fixed overview layout only; no drag-and-drop builder yet. |
| Interface utilization (P1 #6) | P1 | 🟡 Partial | `rx_bytes_delta`/`tx_bytes_delta` captured. UI shows deltas; rate calculation from elapsed time not yet exposed as utilization %. |
| CRC/error monitoring (P1 #7) | P1 | 🟡 Partial | Error and discard columns captured in DB; graphing and alarm rule not yet implemented. |
| Optical/SFP monitoring (P1 #8) | P1 | 🟡 Partial | `optical_rx_dbm`/`optical_tx_dbm` columns exist and surface in device detail; alarm rule not yet written. |
| Alarm acknowledgement and suppression lifecycle | P1 | 🟡 Partial | Manual resolve implemented. Acknowledgement, assignment, suppression, and maintenance window not yet. |
| Notification policies and delivery | P1 | 🔴 Not started | No notification adapter yet (no email/webhook/SMS). |
| SNMP trap/syslog ingestion | P1 | 🔴 Not started | Roadmap Phase 2 item. |
| Syslog collection (P1 #10) | P1 | 🔴 Not started | Infrastructure not yet in place. |
| STP topology change alert (P1 #11) | P1 | 🔴 Not started | Requires SNMP trap receiver. |
| Loop detection (P1 #12) | P1 | 🔴 Not started | |
| Broadcast storm alert (P1 #13) | P1 | 🔴 Not started | |
| Fiber cut detection (P1 #17) | P1 | 🔴 Not started | Requires optical + topology correlation. |
| Interface speed/duplex mismatch (P1 #19) | P1 | 🟡 Partial | `AdminOperMismatchStrategy` covers admin/oper. Speed/duplex mismatch needs EtherLike-MIB OIDs. |

### Release C — Scale and Automation

| Scope Item | Priority | Status | Notes |
|---|---|---|---|
| Topology enhancements (force-directed, minimap, filters) | P1 | 🔴 Not started | Basic SVG topology exists; no layout modes or filters. |
| Discovery rules and dynamic groups | P2 | 🔴 Not started | |
| Reports and SLA views | P2 | 🔴 Not started | |
| Port security violation alert (#14) | P2 | 🔴 Not started | |
| DHCP snooping alert (#15) | P2 | 🔴 Not started | |
| ARP inspection alert (#16) | P2 | 🔴 Not started | |
| Unauthorized login alert (#18) | P2 | 🔴 Not started | |
| NTP sync alert (#9) | P2 | 🔴 Not started | `ntp_skew_seconds` column exists in `device_health_metrics`. |

---

## 🌐 6. REST API Surface

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/healthz` | None | Liveness probe |
| `POST` | `/api/session` | None | Exchange operator token for HttpOnly session + CSRF token |
| `GET` | `/api/session` | None | Validate existing session and refresh CSRF token |
| `DELETE` | `/api/session` | Session | Terminate session |
| `GET` | `/api/v1/dashboard` | Session | Full dashboard payload (overview, devices, events, metrics, topology) |
| `GET` | `/api/v1/devices` | Session | Paginated fleet inventory (`page`, `page_size`, `q`, `status`, `type`) |
| `GET` | `/api/v1/devices/{id}` | Session | Full device detail (identity, reachability, resource, interfaces, alarms) |
| `POST` | `/api/v1/alarms/{id}/resolve` | Session | Manually resolve an open alarm instance |
| `POST` | `/api/devices` | Session | Add a new managed device |
| `PUT` | `/api/devices/{id}` | Session | Edit device metadata |
| `DELETE` | `/api/devices/{id}` | Session | Delete device and associated topology edges |

---

## 🖥️ 7. React UI Views

| View | Nav Label | Status | Description |
|---|---|---|---|
| Operations | `Operations` | ✅ Done | Overview stats, latency sparkline, fleet posture donut, CPU/mem sparklines, mini device table |
| Network topology | `Network topology` | ✅ Done | SVG LLDP topology graph, topology summary, offline impact list |
| Infrastructure | `Infrastructure` | ✅ Done | Basic device table with search |
| Fleet inventory | `Fleet inventory` | ✅ New | Paginated/filtered inventory table; click row → Device Detail slide panel (Overview / Interfaces / Alarms tabs with manual alarm resolve) |
| Active alerts | `Active alerts` | ✅ Done | Event stream of unresolved network_events |

---

## 🐛 8. Known Constraints, Discovered Caveats & Potential Bugs

### The Identifier/Naming Mismatch Warning
If Telegraf writes a bare IP address to `agent_host` in the telemetry tables, but our Go topology package relies on writing raw string hostnames into `network_topology`, the Grafana Node Graph rendering engine will break. The Node Graph **requires absolute character string matching** between `Edges.source`/`Edges.target` and `Nodes.id`. We must ensure hostnames are cleanly resolved and identically mapped across all pipelines.

### The Overwrite Trap
Specifying a `command:` block in `docker-compose.yml` completely suppresses the native `CMD` string instructed inside `Dockerfile.snmpd`. Because Alpine Linux behaves uniquely with the net-snmp daemon, we must explicitly include the runtime flags `-I -smux,mteTrigger,mteTriggerConf` in our `docker-compose.yml` command arrays to keep the bare Alpine daemons alive without crashing natively on boot.

### Database Deadlocks
Concurrent writes executing continuously from the topology sync loop (`network_topology`) and the auto-discovery loops (`devices`) can cause locking contentions or race conditions at the database layer. We successfully mitigate this by aggressively enforcing `ON CONFLICT DO NOTHING` statements in our PostgreSQL `INSERT` logic and delegating deduplication constraints directly to the database engine.

### Alarm Deduplication (Scope Gap)
Current alarm deduplication uses a read-before-write pattern rather than a deterministic fingerprint enforced by a database UNIQUE constraint. This means concurrent alarm workers could theoretically race and insert duplicate active instances. The scope requires a stable alarm fingerprint (alarm_code + device_id + resource_id) enforced at the DB layer — not yet implemented.

### SNMP Failure False-Positives
`SNMPCommunicationFailureStrategy` joins `snmp_device_health` on `ip_address = agent_host::inet`. If a device's `agent_host` in Telegraf output is a hostname string rather than an IP, this join will produce zero rows and fire a false SNMP failure alarm. Ensure Telegraf `agent_host` is consistently an IP address for all managed devices.

### Alarm ID in Hypertable
`network_events` is a TimescaleDB hypertable partitioned on `time`. TimescaleDB does not support SERIAL PKs on chunk-partitioned tables in the same way as regular tables — the `id` column used by `ResolveAlarmHandler` requires using `time + device_id` as a composite lookup or migrating to a non-hypertable if individual row addressing becomes critical at scale.

---

## 🎯 9. Next Implementation Priorities (aligned to scope)

### Immediate (unblock P0 completion)
1. **Alarm fingerprint deduplication** — add `UNIQUE (alarm_id, device_id, resource_id)` partial index on `resolved=FALSE` rows to replace read-before-write anti-duplication.
2. **Inventory-driven SNMP polling** — replace static Telegraf targets with inventory-driven target generation (poll profile per device).
3. **Credential profile table** — separate `credential_profiles` table with envelope-encrypted secrets; reference by ID from `devices`.

### Short-term (Release B)
4. **Port utilization %** — derive bps and utilization % from `rx_bytes_delta`/`tx_bytes_delta` and `ifSpeed` in the device detail API.
5. **CRC/error alarm rule** — implement `InterfaceErrorRateStrategy` using `rx_errors_delta` and `tx_errors_delta` with configurable rate threshold.
6. **Notification adapter** — implement webhook delivery for CRITICAL alarms as the first notification channel.
7. **SNMP trap receiver** — Go UDP/TCP trap listener for STP, loop, and storm events.

### Medium-term (Release C)
8. **Topology enhancements** — force-directed layout, site-scoped graph queries, impact highlighting.
9. **Reports** — availability and SLA by device, site, and reporting period.
10. **SSO/RBAC roles** — viewer, operator, engineer, auditor, administrator role enforcement.
