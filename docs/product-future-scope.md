# Product Future Scope

## Purpose

This document converts the supplied NMS reference screens into product
requirements for NetPulse. The references are used only to identify useful
workflows and data; their visual design and implementation are not copied.

The current release provides authenticated operations, inventory status,
alerts, basic performance trends, and LLDP topology. The following scope turns
that foundation into a complete operator-facing network management product.

## Priority overview

| Priority | Capability | Intended outcome |
| --- | --- | --- |
| P0 | Device onboarding and credential profiles | Operators can securely add and validate managed devices. |
| P0 | Device inventory and detail pages | Operators can move from fleet state to full device diagnostics. |
| P1 | Configurable dashboards | Each role can arrange and save the operational information it needs. |
| P1 | Interface engineering | Operators can identify failing, congested, or error-prone interfaces. |
| P1 | Scalable topology | Large networks remain understandable and show status and traffic impact. |
| P1 | Alarm operations | Events become actionable incidents with ownership and lifecycle. |
| P2 | Discovery, grouping, reports, and automation | The system can manage large estates with less manual work. |

## 1. Device onboarding and secure credentials (P0)

### Add-device workflow

Create a guided form available from the Inventory page with:

- Hostname or display name
- IPv4 or IPv6 address, CIDR validation, and duplicate detection
- Device category, vendor, model, site, rack, and tags
- Monitoring state and polling profile
- ICMP, SNMP, LLDP/CDP, and optional SSH capability selection
- Existing credential-profile selection or creation of a new profile
- Test-credentials action before saving
- Connectivity preflight showing DNS, ICMP, SNMP, and required-port results
- Explicit confirmation of discovered identity before enrollment

### Credential profiles

- SNMPv3 `authPriv` must be the production default.
- Secrets must be envelope-encrypted and referenced by ID; plaintext community
  strings or passwords must never be returned to the browser or logs.
- Restrict profile use by site, device group, and operator role.
- Track creator, last rotation, last validation, and affected-device count.
- Provide test, rotate, disable, and audit workflows.
- Retain SNMPv2c only as a clearly marked migration option.

### Bulk onboarding

- CSV import with preview, validation errors, and dry-run mode
- CIDR discovery jobs with rate and concurrency controls
- Scheduled discovery rules and exclusion ranges
- Approval queue for unmanaged discoveries
- Bulk assignment of site, group, credential profile, and polling policy

### Acceptance criteria

- Invalid, duplicate, disallowed, or unreachable targets cannot be silently
  enrolled.
- A failed credential test provides a safe diagnostic without revealing a
  secret.
- Every create, edit, delete, credential test, and bulk action is audited.

## 2. Device inventory and detailed device data (P0)

### Fleet inventory

Build a server-paginated, filterable, sortable inventory with saved views and
bulk actions. Recommended columns:

- Status and active severity
- Hostname, management IP, and aliases
- Category, vendor, model, serial number, and OS/firmware version
- Site, region, rack, group, and tags
- Availability, response time, packet loss, and open-alarm count
- Interface totals: up, down, administratively down, erroring, and saturated
- CPU, memory, temperature, fan, power-supply, and storage summaries
- Discovery source, polling profile, credential profile name, and last poll
- Uptime, first discovered, last configuration change, and last seen

Filters should cover status, severity, type, vendor, site, group, tags,
firmware, monitoring state, and last-seen age. Export must respect the same
RBAC and active filters.

### Device detail page

Each device should have a stable URL and the following tabs:

1. **Overview:** identity, reachability, uptime, availability SLA, resource
   cards, recent changes, and current alarms.
2. **Interfaces:** name/index, description, admin/oper state, speed, duplex,
   VLAN, MAC, peer, utilization, errors, discards, and optical levels.
3. **Performance:** selectable time range for latency, loss, CPU, memory,
   temperature, traffic, and availability.
4. **Topology:** upstream/downstream neighbors and impact path centered on the
   selected device.
5. **Hardware:** chassis, modules, serials, fans, power supplies, sensors, and
   transceivers where supported.
6. **Events and alarms:** searchable history, acknowledgement, assignment,
   notes, and resolution timeline.
7. **Configuration:** monitoring policy, maintenance window, tags, credentials
   reference, and audited administrative actions.

All metrics must display units, source, sample timestamp, stale-data state, and
the effective polling interval.

## 3. Configurable dashboards (P1)

### Dashboard builder

- Add, remove, resize, drag, and reorder widgets on a responsive grid
- Create, rename, clone, import, export, and archive dashboards
- Personal dashboards plus administrator-published team dashboards
- Role-based default dashboards for NOC, network engineering, management, and
  site operations
- Time range, refresh interval, site, group, vendor, tag, and severity filters
- Cross-widget filtering and drill-through into device or interface details
- Save layout revisions and restore a previous revision

### Initial widget catalog

- Fleet totals and availability
- Status/severity heatmap
- Infrastructure snapshot grouped by device type
- Problematic-device ranking
- CPU and memory top-N with minimum, maximum, and average values
- Interface errors and discards ranking
- Utilization and packet-loss trends
- Recent and active alarms
- Availability/SLA summary
- Topology or site map
- Data freshness and collector health

### Storage model

Store dashboard definition, widget type, validated query parameters, grid
position, filters, owner, visibility, version, and timestamps. Do not store or
execute arbitrary SQL supplied by a browser. Widgets must use allow-listed API
queries and server-side validation.

## 4. Interface engineering (P1)

- Interface inventory with device, port, description, peer, state, speed,
  duplex, MTU, VLAN, and last transition
- In/out bits per second and utilization based on 64-bit counters
- Errors, CRC, discards, drops, flaps, and congestion trends
- Optical receive/transmit power, bias current, temperature, and threshold
  state where DOM data is available
- Admin-up/oper-down, speed/duplex mismatch, high errors, high utilization,
  optical degradation, and fiber-cut indicators
- Top-N tables, multi-interface comparison, selectable time windows, and CSV
  export
- Counter-reset and wrap handling so derived rates never show false spikes

## 5. Scalable topology and traffic view (P1)

Extend the current topology chart with:

- Force-directed and hierarchical layouts with saved coordinates
- Site, region, group, VLAN, layer, and device-type filters
- Collapse/expand by site or device group
- Zoom, pan, minimap, search, focus, and full-screen modes
- Link aggregation and parallel-link handling
- Node state: online, warning, critical, unmanaged, maintenance, or stale
- Link state and traffic-load color bands
- Interface names, capacity, current utilization, errors, and peer information
- Click-through to device and interface detail pages
- Impact highlighting for neighbors and downstream devices when a node or link
  fails
- Historical topology snapshots and change comparison

For large networks, layout and aggregation must be performed incrementally;
the API must support site-scoped or viewport-scoped graph queries instead of
returning the entire estate to the browser.

## 6. Alarm and incident operations (P1)

- Severity levels: clear, information, warning, critical, and unknown
- Active, acknowledged, assigned, suppressed, resolved, and closed states
- Assignee, notes, timestamps, maintenance context, and resolution reason
- Deduplication, correlation, flapping detection, dependency suppression, and
  parent/child incidents
- Filters by time, device, interface, severity, site, type, and owner
- Device and interface timelines showing state transitions
- Notification and escalation policies with delivery history
- Optional ticket integration after explicit administrative configuration
- Immutable audit records for every operator action

## 7. Discovery, grouping, and operations (P2)

- Scheduled Layer 2, Layer 3, interface, and virtualization discovery
- Managed/unmanaged classification and remediation queue
- Static and dynamic groups based on site, tags, vendor, type, status, or query
- Maintenance windows and planned-change suppression
- Polling templates by vendor/model and capability detection
- Configuration backup, diff, and compliance checks as a later controlled
  module

## 8. Reports and service objectives (P2)

- Availability and SLA by device, site, group, and reporting period
- Capacity and utilization trends with configurable percentiles
- Interface quality, error, discard, and optical reports
- Firmware, hardware lifecycle, inventory, and stale-device reports
- Alarm volume, mean time to acknowledge, and mean time to resolve
- Scheduled PDF/CSV delivery only through approved channels
- Report generation in background jobs with retention and access controls

## 9. Platform and security requirements

- Organizational SSO, phishing-resistant MFA, RBAC, and scoped API tokens
- Separate roles for viewer, operator, engineer, auditor, and administrator
- Tenant/site boundaries enforced by the API and database, not only the UI
- Parameterized database access and allow-listed dashboard queries
- Pagination and bounded time ranges for every high-volume endpoint
- Secrets manager integration, rotation, and redaction
- TLS for browsers and mutually authenticated service/database connections
- Audit and security events forwarded to an immutable SIEM destination
- Rate limits, request size limits, safe exports, and spreadsheet-injection
  prevention
- Dependency, container, SBOM, signature, backup, restore, and disaster-recovery
  controls defined in the security baseline

## Proposed delivery sequence

### Release A — Inventory foundation

- Add-device form and credential-profile APIs
- Fleet inventory with pagination and filters
- Device overview and interface tabs
- Audit trail for device administration

### Release B — Operator workflows

- Dashboard builder and widget catalog
- Advanced interface engineering
- Alarm acknowledgement, assignment, and lifecycle
- Saved filters and exports

### Release C — Scale and automation

- Large-estate topology and impact analysis
- Discovery rules, dynamic groups, and maintenance windows
- Reports, SLA views, and controlled notifications
- Performance tests at agreed device/interface/event volumes

## Definition of done

Each capability requires API authorization tests, UI accessibility checks,
audit verification, database migration/rollback coverage, bounded-load tests,
operator documentation, and acceptance against representative production
devices. A screenshot match alone is not acceptance; the data must be current,
traceable, secure, and operationally actionable.
