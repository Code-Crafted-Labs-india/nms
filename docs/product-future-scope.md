# Product Future Scope

## Purpose

This document converts the supplied NMS reference screens into product
requirements for NetPulse. The references are used only to identify useful
workflows and data; their visual design and implementation are not copied.

The current release provides authenticated operations, inventory status,
basic event display, ICMP reachability, performance trends, and LLDP topology.
It also contains initial device-down, interface-state, admin/oper mismatch, and
high-CPU evaluation code. These rules are not yet a complete production alarm
system: live SNMP data still needs normalization, alarms do not automatically
clear, and notification delivery is not implemented. The following scope turns
that foundation into a complete operator-facing network management product.

## Priority overview

| Priority | Capability | Intended outcome |
| --- | --- | --- |
| P0 | Device onboarding and credential profiles | Operators can securely add and validate managed devices. |
| P0 | Device inventory and detail pages | Operators can move from fleet state to full device diagnostics. |
| P0 | Telemetry normalization and alarm foundation | Every collected signal is attributable, current, and evaluated through a reliable alarm lifecycle. |
| P1 | Configurable dashboards | Each role can arrange and save the operational information it needs. |
| P1 | Interface engineering | Operators can identify failing, congested, or error-prone interfaces. |
| P1 | Scalable topology | Large networks remain understandable and show status and traffic impact. |
| P1 | Alarm operations and event ingestion | Events from polling, traps, and syslog become actionable incidents with ownership and delivery history. |
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

### Current baseline and mandatory foundation

Before adding more alarm rules, complete the shared telemetry and state model:

- Normalize Telegraf fields such as `ifIndex`, `ifOperStatus`, counters, and
  device identity into canonical device/interface records before evaluation.
- Replace hard-coded polling targets with inventory-driven polling profiles so
  onboarding a monitored device activates ICMP and SNMP collection.
- Record every poll attempt with protocol, start/end time, success, failure
  category, duration, collector identity, and last-success time.
- Identify an alarm by a stable alarm code, device, resource type, resource ID,
  and deterministic fingerprint. Enforce one active instance per fingerprint
  in the database to prevent worker races from producing duplicates.
- Support configurable warning and critical thresholds, sustained-duration and
  consecutive-sample rules, hysteresis, automatic recovery, stale-data state,
  and flapping control.
- Keep raw observations separate from alarm instances and immutable alarm
  transition history.
- Allow policy inheritance from global, site, device-model, device, and
  interface scopes with an operator-visible effective policy.
- Distinguish a failed target from a failed collector. Missing telemetry must
  not automatically be reported as a device outage.
- Make SNMPv3 `authPriv` the production default and select vendor/model polling
  profiles through capability discovery.

### Required alarm and monitoring catalog

| # | Capability | Priority | Collection and evaluation requirement |
| --- | --- | --- | --- |
| 1 | Device down alarm | P0 | Evaluate consecutive ICMP failures over a configurable window, correlate with SNMP reachability and parent-device state, suppress duplicates, and automatically clear after stable recovery. |
| 2 | Interface down alarm | P0 | Evaluate canonical interface state. Alarm only when operational state is down and policy expects the port to be up; suppress administratively down and maintenance-state ports. |
| 3 | High memory usage alarm | P0 | Calculate memory utilization from supported standard or vendor OIDs and apply warning/critical thresholds, sustained duration, hysteresis, and recovery. |
| 4 | SNMP communication failure | P0 | Alarm when a monitored device has no successful SNMP poll within policy while distinguishing timeout, authentication, authorization, malformed response, and collector failure. |
| 5 | Device reachability monitoring | P0 | Retain ICMP status, response time, and loss; add configurable probe count, interval, timeout, IPv4/IPv6 support, source collector, and stale-data indication. |
| 6 | Port utilization graphs | P1 | Use 64-bit interface counters and elapsed sample time to derive bits per second and utilization percentage from effective interface speed, with reset/wrap protection. Expose this through the operator UI as well as engineering dashboards. |
| 7 | CRC/error monitoring | P1 | Collect IF-MIB errors/discards plus EtherLike-MIB or vendor CRC/alignment counters. Graph totals and rates and alarm on configurable sustained rates or sudden deltas. |
| 8 | Optical/SFP transmit-receive monitoring | P1 | Use vendor/model capability profiles to collect DOM receive/transmit power, temperature, bias current, module state, and device-reported thresholds. Alarm on degradation and threshold crossing. |
| 9 | NTP synchronization alert (optional) | P2 | Collect source, stratum, reachability, synchronization state, and offset. Alarm on loss of synchronization or configurable clock skew and clear after stable recovery. |
| 10 | Syslog collection and alerting (optional catalog, required infrastructure) | P1 | Provide a secured, rate-limited syslog receiver, raw-event retention, structured parsing, vendor rule packs, deduplication, and event-to-alarm mapping. TLS is preferred; controlled TCP/UDP support may be enabled by deployment policy. |
| 11 | STP topology change alert | P1 | Ingest standard/vendor STP traps, syslog, and topology-change counters. Include instance/VLAN, root/bridge identity, port, reason, and change rate. |
| 12 | Loop detection alert | P1 | Ingest vendor loop-detection events and correlate repeated STP changes, MAC movement, and traffic amplification where direct device events are unavailable. |
| 13 | Broadcast storm alert | P1 | Collect broadcast/multicast packet counters and storm-control events; evaluate packets per second and percentage-of-traffic policies per interface. |
| 14 | Port security violation alert | P2 | Parse vendor traps/syslog and retain interface, VLAN, offending MAC, configured action, and resulting port state. |
| 15 | DHCP snooping alert (optional) | P2 | Parse rogue-server, invalid-packet, binding-limit, and rate-limit events with VLAN, interface, MAC/IP, and reason. |
| 16 | ARP inspection alert | P2 | Parse Dynamic ARP Inspection violations with VLAN, interface, source MAC/IP, binding result, and device-provided reason. |
| 17 | Fiber cut detection | P1 | Correlate interface transition, sudden optical receive-power loss, local/peer port state, and topology impact. Do not classify an ordinary link-down event as a fiber cut without supporting evidence. |
| 18 | Unauthorized login attempt alert | P2 | Audit failed UI/API authentication and ingest device AAA, TACACS+, RADIUS, SSH, and console failures from syslog/traps. Apply rate thresholds and forward security events to the approved SIEM. |
| 19 | Interface speed/duplex mismatch alert | P1 | Collect configured and negotiated speed/duplex through EtherLike-MIB or vendor OIDs, compare peer data where available, and correlate with interface errors. Keep this separate from admin/oper mismatch. |

The optional designation controls whether a rule pack is enabled for a
deployment. Shared syslog/event-ingestion infrastructure is still required for
the non-optional Layer 2 and security alarms that depend on device events.

### Event ingestion and vendor support

- Receive SNMP traps/informs and syslog through independently scalable,
  authenticated collectors with backpressure and bounded queues.
- Preserve the raw source event for investigation while mapping it to a
  vendor-neutral schema containing device, resource, event code, severity,
  message, observed time, collector time, and vendor attributes.
- Maintain tested vendor/model rule packs for supported hardware. Obtain the
  supported device list, MIB files, sample SNMP walks, trap definitions, and
  representative syslog messages before accepting requirements 8 and 11-19.
- Quarantine unknown event formats for operator review rather than silently
  discarding them or creating an unbounded number of alarms.
- Apply rate limits, duplicate suppression, and retention policies separately
  to raw events, normalized events, alarm instances, and notification history.

### Alarm lifecycle and operator workflow

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

Notification adapters must be explicitly configured and may include email,
webhook, approved chat services, SMS, or ticketing systems. Store each delivery
attempt, response, retry, escalation, and terminal failure. Alarm creation must
not depend on a notification provider being available.

### Alarm acceptance criteria

- Every rule has documented trigger, sustain, clear, severity, suppression,
  and stale-data behavior and identifies the telemetry or event source used.
- Every active alarm identifies its device and, where applicable, interface,
  SFP, VLAN, MAC address, user, or other affected resource.
- Repeated evaluation and concurrent workers cannot create duplicate active
  instances for the same alarm fingerprint.
- A recovered condition is automatically resolved only after its configured
  recovery window; brief oscillation is recorded as flapping rather than a
  stream of independent incidents.
- Planned maintenance and upstream dependency failures suppress downstream
  notifications without deleting observations or transition history.
- Unit and integration tests cover trigger, no-trigger, duplicate suppression,
  recovery, stale telemetry, counter reset/wrap, malformed events, and database
  or notification failures.
- Acceptance uses representative production hardware or recorded device data,
  not only generated sample rows.

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

### Release A — Inventory and monitoring foundation

- Add-device form and credential-profile APIs
- Fleet inventory with pagination and filters
- Device overview and interface tabs
- Audit trail for device administration
- Canonical device/interface telemetry normalization
- Inventory-driven ICMP and SNMP polling with collector-health records
- Stateful alarm schema with fingerprints, transitions, automatic recovery,
  hysteresis, stale-data handling, and tests
- Production-ready device-down, interface-down, high-memory, SNMP-failure, and
  reachability monitoring

### Release B — Operator workflows

- Dashboard builder and widget catalog
- Advanced interface engineering covering utilization, CRC/errors, optical
  monitoring, fiber-cut correlation, and speed/duplex mismatch
- Alarm acknowledgement, assignment, suppression, maintenance, and lifecycle
- Notification policies, escalation, retry, and delivery history
- SNMP trap/inform and secured syslog ingestion with initial vendor rule packs
- Saved filters and exports

### Release C — Scale and automation

- Large-estate topology and impact analysis
- STP topology change, loop, broadcast storm, port security, DHCP snooping, ARP
  inspection, unauthorized-login, and optional NTP rule packs
- Discovery rules, dynamic groups, and maintenance windows
- Reports, SLA views, correlation, dependency suppression, and SIEM forwarding
- Performance tests at agreed device/interface/event volumes

## Definition of done

Each capability requires API authorization tests, UI accessibility checks,
audit verification, database migration/rollback coverage, bounded-load tests,
operator documentation, and acceptance against representative production
devices. A screenshot match alone is not acceptance; the data must be current,
traceable, secure, and operationally actionable.
