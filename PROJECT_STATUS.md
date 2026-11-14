# 📑 PROJECT_STATUS.md

> Last updated: 2026-11-14. Scope source: [`docs/product-future-scope.md`](docs/product-future-scope.md)
>
> Architecture documentation: [`docs/architecture.md`](docs/architecture.md) is
> the code-aligned description of current runtime behavior and is the place to
> start when learning the system or changing code. The tables below are a
> project status/roadmap snapshot; they are not a substitute for checking the
> implementation. In particular, “schema exists” does not mean a running
> writer populates a table or column.

---

## 1. System architecture overview

The current Compose demo has middleware generate Telegraf targets from monitored
inventory and write telemetry to TimescaleDB. A persistent Test/Production
mode scopes target generation, ICMP, alarms, and UI inventory to mock-prefixed
or non-mock devices. A Go process reads inventory to run ICMP, discovers IPs
from recent SNMP interface telemetry, syncs LLDP graph edges,
evaluates six alarm strategies, and serves the REST API. The React application
calls that API through the Nginx container. Grafana is a separate engineering
aid. The Compose stack's mock devices and seeded inventory are demo
configuration. Inventory CRUD now updates generated Telegraf targets for
SNMPv1/v2c; SNMPv3, protected credentials, subnet discovery, and config
validation/rollback remain unimplemented.

The Switch Configuration view stores draft VLAN and port membership plans and
now identifies version-qualified ATS, ATS-3900-24P6X AOS 3.3.1.1, and Routing
Switch CLI V3.3.1 profiles from recent SNMP `sysDescr` data. The plan API
repeats detection before saving and rejects stale, unknown, and unsupported
versions. This is still planning only: the NMS does
not connect to switches, push configuration, back up running configuration, or
roll back a change. Other Type 3 models and firmware still need qualification.

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

## 5. Active requirement scope

The active scope is limited to clauses 1–13 of the supplied PRL specification.
The canonical clause-by-clause compliance status and remaining work are in
[docs/client-requirements-gap-plan.md](docs/client-requirements-gap-plan.md).
No former product wishlist or roadmap item outside those clauses is an active
requirement or delivery commitment. The feature and API sections below describe
existing implementation only.

### Current progress checkpoint (2026-11-14)

- The supplied requirements cover clauses 1–9, 10a–10i, 11a–11b, 12a–12c,
  and 13. The gap matrix analyzes every item. A source transcription and
  completeness note are in [`docs/prl-requirements-source.md`](docs/prl-requirements-source.md).
- The supplied source ends mid-word in clause 13. Obtain the complete original
  before finalizing a compliance response.
- `feat/commercial-nms-adoption` contains an in-progress, uncommitted,
  read-only OpManager inventory integration. Go tests passed in Ubuntu with
  `CGO_ENABLED=0`, and the UI production build passed with Bun. The OpManager
  client is covered by mocked HTTP tests; live lab validation remains pending,
  so the branch is not merged.
- Keep new capabilities on dedicated `feat/...` branches. Merge only finished
  branches after checks pass; remind the operator to test and merge each one.

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

## 9. Scope boundary

Only the pasted PRL specification clauses 1–13 define active requirements.
Items outside those clauses are out of scope unless PRL approves them in
writing. See [docs/product-future-scope.md](docs/product-future-scope.md) for
the current scope statement and [docs/client-requirements-gap-plan.md](docs/client-requirements-gap-plan.md)
for the compliance matrix.
