# 📑 PROJECT_STATUS.md

> Last updated: 2026-10-12. Scope source: [`docs/product-future-scope.md`](docs/product-future-scope.md)
>
> Architecture documentation: [`docs/architecture.md`](docs/architecture.md) is
> the code-aligned description of current runtime behavior and is the place to
> start when learning the system or changing code. The tables below are a
> project status/roadmap snapshot; they are not a substitute for checking the
> implementation. In particular, “schema exists” does not mean a running
> writer populates a table or column.

---

## 1. System architecture overview

The current Compose demo has Telegraf poll statically configured targets and
write telemetry to TimescaleDB. A Go process reads inventory to run ICMP,
discovers IPs from recent SNMP interface telemetry, syncs LLDP graph edges,
evaluates six alarm strategies, and serves the REST API. The React application
calls that API through the Nginx container. Grafana is a separate engineering
aid. The Compose stack's mock devices and static Telegraf target list are demo
configuration; inventory CRUD does not configure Telegraf.

See [`docs/architecture.md`](docs/architecture.md) for service networks,
startup sequence, actual data writers/readers, table semantics, API/auth flow,
known implementation gaps, and a repository map for manual coding.

---

## 2. Relational database and table map

This high-level table is a quick orientation only. The canonical DDL is
[`migration.sql`](migration.sql); runtime schema repair is in [`db/db.go`](db/db.go).
The detailed writer/consumer caveats are in
[`docs/architecture.md`](docs/architecture.md#5-database-model-and-data-ownership).

| Table | Type | Purpose | Primary/Foreign Keys & Connections |
|---|---|---|---|
| `devices` | Relational | Inventory catalog of managed elements. | `id` (PK). Connected to `network_topology`, `device_health_metrics`, and `interfaces`. |
| `device_models` | Relational | Supported hardware SKU catalog. Gates AddDevice workflow. | `id` (PK), `UNIQUE(model_name)`. |
| `interfaces` | Relational | Intended canonical per-device interface index and descriptor. | `id` (PK), `device_id` (FK → devices). Current discovery does not populate this automatically. |
| `network_topology` | Relational | Graph adjacency list tracking source/target device physical connections. | `id` (PK), `source_device`, `target_device`. |
| `network_events` | Hypertable | Alarm lifecycle store. Stores trigger, severity, resolution, and timestamps. | Partitioned by `time`. `device_id` (FK → devices). `alarm_id` maps to catalog below. |
| `device_health_metrics` | Hypertable | TimescaleDB hypertable tracking node availability (ICMP) and core resource utilization (CPU/Mem). | Partitioned by `time`. Connected to `devices` via `device_id` (FK). |
| `interface_performance_metrics` | Hypertable | Raw IF-MIB samples plus columns reserved for normalized metrics. | Telegraf writes raw fields (`agent_host`, quoted `"ifIndex"`, counters); current Telegraf path does not fill `interface_id` or normalized deltas. |
| `snmp_device_health` | Hypertable | Telegraf SNMP sink for system-level health (CPU idle, memory). Used by alarm rules #3 and #6. | Partitioned by `time`. Written by Telegraf. |
| `snmp` | Raw Telemetry | Telegraf SNMP sink for standard MIB-II system polling. | Written purely by Telegraf. |
| `snmp_interface` | Raw Telemetry | Generic SNMP interface table used by Go discovery. | Telegraf writes it only if configured; current IF-MIB table is instead named `interface_performance_metrics`. |
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
| `db/` | PostgreSQL connection pool and selected idempotent startup schema repairs. | ✅ Implemented; not a versioned migration system |
| `topology/` | Reads recent LLDP rows and updates the current `network_topology` edge set. | ✅ Implemented; requires device LLDP support and matching |
| `discovery/` | Inserts generic inventory records for unknown `agent_host` values in `snmp_interface`. Current Telegraf config writes its IF-MIB table to `interface_performance_metrics`, so this scanner's input is not populated by the checked-in config. | ⚠️ Requires wiring |
| `ping/` | Concurrent inventory-driven ICMP sweeps; writes device-linked `device_health_metrics` rows. | ✅ Implemented; scale not characterized |
| `alert/` | Worker pool dispatches 6 strategies against monitored devices every 30s. Strategies have recovery paths; CPU/memory thresholds use a five-point clear margin. Some inputs are not written by current Telegraf config. | 🟡 Partial |
| `handlers/dashboard.go` | `GET /api/v1/dashboard` — overview, devices, events, metrics, topology. | ✅ Implemented |
| `handlers/devices.go` | `POST/PUT/DELETE /api/devices` — device CRUD. Updated to include location metadata and proper Cisco NCS 540 catalog validation. | ✅ Updated |
| `handlers/inventory.go` | `GET /api/v1/devices` — paginated/filtered fleet inventory. `GET /api/v1/devices/{id}` — full device detail. `POST /api/v1/alarms/{id}/resolve` — manual alarm resolution. | ✅ New (2026-10-01) |
| `handlers/security.go` | Shared bootstrap-token session auth, CSRF, same-origin checks, and security headers. Per-user RBAC is not implemented. | ✅ Implemented baseline |
| `main.go` | Orchestration: bootstraps DB, background workers, alarm engine (6 strategies), and REST API mux. | ✅ Implemented |

---

## 🔄 4. The Live Topology Graph Data Lifecycle

The physical link discovery and visualization lifecycle operates completely automatically:

1. **Telegraf Queries**: Telegraf uses `inputs.snmp.table` to poll the standard LLDP remote-systems columns for the configured SNMP target list. Devices must advertise LLDP and grant the configured SNMP principal LLDP-MIB read access.
2. **Raw Storage**: Telegraf saves the resulting MIB payload natively into the `snmp_lldp_topology` table.
3. **Go Processing**: The Go `topology` engine sweeps fresh `snmp_lldp_topology` rows every 30 seconds, resolves neighbor system names to inventory hostnames, falls back to advertised chassis IDs, and expires edges that are no longer observed.
4. **React Presentation**: The authenticated dashboard API returns the normalized nodes, live device status, and topology edges. The React topology view renders the graph and marks online, down, and unknown devices.

---

## 📊 5. Scope-to-Implementation Progress Tracker

Based on [`docs/product-future-scope.md`](docs/product-future-scope.md):

### Release A — Inventory and Monitoring Foundation

| Scope Item | Priority | Status | Implementation |
|---|---|---|---|
| Add-device form and credential-profile APIs | P0 | 🟡 Partial | Inventory now provides Add/Edit/Delete forms backed by authenticated CRUD endpoints and the supported-model catalog. Device fields include address, model, location, SNMP version/community, and monitoring state. Credential profile table and connectivity preflight are still pending. |
| Fleet inventory with pagination and filters | P0 | ✅ Done | `GET /api/v1/devices` — paginated (50/page), filterable by status, type, hostname/IP search. |
| Device overview tab | P0 | ✅ Done | `GET /api/v1/devices/{id}` — identity, reachability, CPU/mem, alarm count. |
| Device interfaces tab | P0 | ✅ Done | Returns per-interface oper/admin/speed/rx/tx/errors/optical in device detail response. |
| Device events/alarms tab | P0 | ✅ Done | Returns last 50 alarms (resolved + open) per device. Manual resolve via `POST /api/v1/alarms/{id}/resolve`. |
| Audit trail for device administration | P0 | 🟡 Partial | Metadata-only login/authentication and API activity events are stored in `audit_events`; authenticated operators can read paginated events at `GET /api/v1/audit`. Shared bootstrap authentication means events identify only `bootstrap-admin`, not a person. Field-level before/after diffs, SIEM forwarding, retention controls, and tamper protection remain pending. |
| Canonical device/interface telemetry normalization | P0 | 🟡 Partial | Telegraf → TimescaleDB raw. Go discovery normalizes devices. Interface join in alarm queries exists but not fully canonical. |
| Inventory-driven ICMP and SNMP polling | P0 | 🟡 Partial | ICMP sweeps are inventory-driven (`ping` pkg). SNMP still Telegraf-static config. |
| Stateful alarm schema with fingerprints, transitions, auto-recovery | P0 | 🟡 Partial | `resolved`/`resolved_at` lifecycle and recovery paths exist. Only CPU/memory use a threshold clear margin. Deterministic fingerprints are not enforced; deduplication uses read-before-write. Fresh-schema alarm row IDs also need reconciliation. |
| Device-down alarm (P0 #1) | P0 | ✅ Done | `DeviceDownStrategy` checks all linked ICMP samples in a fixed one-minute window; any success clears the event and an empty window does not alarm. |
| Interface-down alarm (P0 #2) | P0 | 🟡 Partial | `LinkStateStrategy` checks normalized rows joined through `interfaces`; checked-in Telegraf writes raw rows without this relationship. |
| High memory usage alarm (P0 #3) | P0 | ✅ Done | `HighMemoryUtilizationStrategy` — warning=80%, critical=95%, hysteresis, auto-clear. |
| SNMP communication failure alarm (P0 #4) | P0 | ✅ Done | `SNMPCommunicationFailureStrategy` — stale window=5min, checks `snmp_device_health` + `snmp_interface`, auto-clear. |
| Device reachability monitoring (P0 #5) | P0 | ✅ Done | `ping` package + `device_health_metrics` hypertable with ICMP status, RTT, packet loss. |
| High CPU alarm (#3 CPU path) | P0 | 🟡 Partial | Strategy threshold=85%, clears at 80%; it reads `device_health_metrics.cpu_utilization`, which current configured writers do not populate. |

### Release B — Operator Workflows

| Scope Item | Priority | Status | Notes |
|---|---|---|---|
| Dashboard builder and widget catalog | P1 | 🔴 Not started | Fixed overview layout only; no drag-and-drop builder yet. |
| Interface utilization (P1 #6) | P1 | 🟡 Partial | Telegraf now polls IF-MIB 64-bit counters and high-speed capacity where supported. Device detail calculates live receive/transmit rates from two recent samples, detects 32-bit wrap and counter resets, and shows utilization meters when capacity is known. Historical utilization graphs and vendor/device validation remain pending. |
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
| `GET` | `/api/v1/device-models` | Session | Supported device model catalog |
| `GET` | `/api/v1/devices` | Session | Paginated fleet inventory (`page`, `page_size`, `q`, `status`, `type`) |
| `GET` | `/api/v1/devices/{id}` | Session | Full device detail (identity, reachability, resource, interfaces, alarms) |
| `POST` | `/api/v1/alarms/{id}/resolve` | Session | Manually resolve an open alarm instance |
| `GET` | `/api/v1/audit` | Session | Paginated metadata-only authentication and API activity |
| `POST` | `/api/devices` | Session | Add a new managed device |
| `PUT` | `/api/devices/{id}` | Session | Edit device metadata |
| `DELETE` | `/api/devices/{id}` | Session | Delete device and associated topology edges |

---

## 🖥️ 7. React UI Views

| View | Nav Label | Status | Description |
|---|---|---|---|
| Operations | `Operations` | ✅ Done | Overview stats, latency sparkline, fleet posture donut, CPU/mem sparklines, mini device table |
| Network topology | `Network topology` | ✅ Done | SVG LLDP topology graph, topology summary, offline impact list |
| Infrastructure | `Infrastructure` | ✅ Updated | Code Crafted Labs branded fleet inventory with Add/Edit/Delete device forms and supported-model selection |
| Fleet inventory | `Fleet inventory` | ✅ New | Paginated/filtered inventory table; click row → Device Detail slide panel (Overview / Interfaces / Alarms tabs with manual alarm resolve) |
| Active alerts | `Active alerts` | ✅ Done | Event stream of unresolved network_events |
| Branding | N/A | ✅ Done | UI successfully integrated Code Crafted Labs logo and assets. |

---

## 🏗️ 8. Recent Progress Summary
* **Expanded Fleet (14 Devices)**: Scaled the mock environment by adding 9 new devices to `docker-compose.yml` (stress tests, WAN edges, firewalls, and distribution switches).
* **Hardware Identity Updates**: Refactored the `ucd-sim` and container definitions to report as **Cisco NCS 540 Series** (e.g., `NCS 540-24Z8Q2C-SYS`, `NCS 540X-16Z4G8Q2C-A`). The Go db model catalog was upgraded from the ATS placeholders.
* **Telemetry Adjustments**: Pushed all 14 mock IPs into `telegraf.conf` across ICMP, SNMP health, and IF-MIB polling tracks.
* **Device CRUD Endpoints**: Injected `location` handling into `DevicePayload` and updated INSERT/UPDATE queries in `devices.go` along with specific conflict error mappings (hostname vs. IP duplication).
* **UI Branding**: Imported the Code Crafted Labs custom logos (`ccl-logo.png` & `ccl-logo-dark.png`) into the frontend's public path.

---

## 8. Known implementation constraints

### The Identifier/Naming Mismatch Warning
Topology/API constraint: `network_topology` edges use resolved hostnames where possible; the React UI consumes those values as graph node identifiers. LLDP names that cannot be matched to inventory can produce inconsistent identifiers or be skipped. The current resolver and its limits are described in [`docs/architecture.md`](docs/architecture.md#4-telemetry-pipeline).

### Interface state alarms depend on normalized rows
`LinkStateStrategy` and `AdminOperMismatchStrategy` join
`interface_performance_metrics.interface_id` to `interfaces.id`. Telegraf's
current raw IF-MIB rows do not provide that foreign key, and discovery does not
populate `interfaces`. See [architecture.md](docs/architecture.md).

### CPU alarm input is not currently populated
The high-CPU strategy reads `device_health_metrics.cpu_utilization`, but the
checked-in Telegraf health track writes CPU fields to `snmp_device_health` and
the Go ping worker writes no CPU metric. The high-CPU alarm needs an explicit
writer/mapping before it can evaluate real telemetry.

### Alarm deduplication is not database-enforced
Strategies query for an open event before inserting another. The database has
no unique active-alarm fingerprint, so concurrent evaluations could create
duplicates.

### Alarm row identifier in a fresh database
`migration.sql` defines `network_events` without an `id` column, while device-detail and manual-resolve handlers select/update alarm rows by `id`. The `db.RunMigrations` repair spec currently adds `resolved_at`, but does not add `id`. Confirm/fix this schema/API mismatch before relying on alarm detail or manual resolution in a newly initialized database. See [`docs/architecture.md`](docs/architecture.md#5-database-model-and-data-ownership).

---

## 9. Future work

The full intended product scope and sequencing belong in
[`docs/product-future-scope.md`](docs/product-future-scope.md). This status file
records implementation state and known constraints; planned items there are
not delivered capabilities.
