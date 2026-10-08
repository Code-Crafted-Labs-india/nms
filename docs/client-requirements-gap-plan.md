# PRL client requirements: compliance analysis and delivery plan

## Executive assessment

The client specification describes a supported, enterprise-class network
management product that can both monitor and configure a multi-campus switch
estate. The current repository is a custom NetPulse monitoring prototype/demo
with a React UI, Go API, Telegraf, TimescaleDB, basic inventory, alarms, and
LLDP topology. It is not currently a general-purpose switch configuration
manager or a commercially supported NMS product.

**Recommendation:** Do not submit NetPulse as compliant with this specification
or mark the requirements “Yes” as-is. Requirements 1 and 13 explicitly rule
out a self-developed/custom NMS as the primary platform unless PRL approves an
exception. Seek that written approval before investing in feature development,
or select a supported OEM/enterprise NMS as the primary platform and position
NetPulse only as an approved supplemental component. If PRL approves NetPulse,
the other gaps still represent a substantial product and deployment program.

This assessment is based on the provided client text and the code/docs in this
repository, especially `PROJECT_STATUS.md`, `docs/product-future-scope.md`,
`docs/real-world-deployment.md`, and `docs/ui-security-baseline.md`. “Partial”
means some adjacent capability exists, not that the full requirement is met.

## Status key

- **No** — no implemented capability meeting the requirement was found.
- **Partial** — a limited or related capability exists; material acceptance
  criteria remain unmet.
- **Unverified** — documentation or code does not establish the required
  platform, scale, certification, or vendor support.
- **Blocking eligibility** — the stated product/ownership criterion conflicts
  with this custom-built project unless the client grants an exception.

## Requirement-by-requirement gap matrix

| Client ref. | Current status | Evidence / gap | Changes or action needed |
|---|---|---|---|
| **1. OEM/established NMS; configure all switch parameters** | **Blocking eligibility; No** | NetPulse is described by the repo as a custom backend and UI. Current device operations are inventory CRUD and monitoring; no general switch configuration, configuration backup/restore, or full parameter management workflow is implemented. | First obtain written PRL approval for a custom solution, or propose a supported OEM/commercial NMS as primary. If approved, scope vendor-specific configuration adapters, safe change plans, authorization, pre/post validation, rollback, and configuration backup. “All configurable parameters” needs a switch model/firmware matrix and an agreed supported command/API scope. |
| **2. RHEL compatibility and virtual hardware details** | **Unverified** | The documented deployment uses Docker Compose. No RHEL release certification, Podman compatibility statement, resource sizing benchmark, or validated PRL virtual-machine bill of materials is present. | Agree the exact RHEL version and container runtime with PRL. Build and qualify on that target. Benchmark representative device/interface counts and retention; then provide CPU, RAM, disk/IOPS, storage growth, NICs, and HA/backup requirements. Do not invent sizing before workload data exists. |
| **3. Unified monitoring and post-audit system** | **Partial** | Dashboard, inventory, telemetry, and alarms are centralized. Metadata-only login/authentication and API activity are now written to PostgreSQL and readable in the Audit Events UI and `GET /api/v1/audit`. The shared bootstrap login identifies all successful operators as `bootstrap-admin`; there are no field-level before/after records, retention controls, SIEM forwarding, or tamper protection. | Add named-user identity and roles, field-level change records for administrative operations, SIEM export, retention/access controls, and tamper-evident or protected storage. Define a PRL audit event/retention policy and verify coverage against each operation. |
| **4. Preconfigured policies, designated personnel, roles/privileges** | **No** | Authentication is a shared bootstrap-token/session model. No individual accounts, role mapping, policy lifecycle, approval, or role-based privilege enforcement exists. | Implement OIDC/SAML and MFA, user/group synchronization, roles (for example admin, network engineer, operator, auditor, viewer), granular authorization, policy templates, activation/deactivation approval, effective-policy view, and audit history. Define who can create, approve, activate, and roll back policies. |
| **5. Topology with traffic, performance, access terminals** | **Partial** | Basic LLDP topology is rendered and device health/performance is shown elsewhere. No evidence of traffic overlays, terminal/client discovery, end-to-end campus topology validation, or topology-based navigation for all requested data. | Normalize device/site/link identities; add campus/site hierarchy, edge status, per-link utilization and performance overlays, access-terminal/MAC/ARP/DHCP correlation where available, filters/search/drilldown, and stale-data indication. Agree what “access terminals” means and which device protocols/data sources expose them. |
| **6. Highlight important devices/links; rates, bandwidth, link colors** | **Partial** | Basic topology and interface counters exist, but important-device/link annotations, configurable link colors, and reliable in/out bit-rate and capacity utilization are not complete. The status doc says utilization percentage is not exposed. | Add operator-managed criticality/pinning and link annotations; calculate bps and utilization from 64-bit counters, elapsed sample time, and interface speed with wrap/reset handling; provide configurable state/load color bands and legends. Validate vendor counter behavior. |
| **7. All switches across four campuses / MLLN** | **Partial; scale unverified** | The system can monitor configured IP targets and show LLDP links, but Telegraf targets are static and currently demo addresses. No multi-campus scale or service-provider MLLN validation is documented. | Confirm routed reachability and firewall/ACLs from the NMS to every campus management subnet over the MLLN. Add campus/site inventory and inventory-driven polling, discovery controls, topology boundaries, per-campus views, and scale/latency tests at the full device/interface count. Coordinate MLLN routing, MTU, latency, and availability with the provider. |
| **8. System-wide VLAN configuration and monitoring** | **No** | No VLAN configuration workflow, VLAN inventory/telemetry model, or system-wide VLAN deployment capability was found. | Add VLAN/port membership collection and normalized inventory first. For configuration, implement vendor-compatible transactional workflows, diff/preview, staged rollout, approval, prechecks, rollback, and per-device result reporting. Identify whether PRL expects monitoring only, configuration only, or both. |
| **9. Real-time alarm and notifications (topology, trap, syslog, script, email)** | **Partial** | Several polling-based alarm strategies and an active-alert view exist. Email/webhook delivery, SNMP trap reception, syslog ingestion, custom-script actions, and customizable email templates are not implemented. | Add event ingestion with authentication/rate limits, incident lifecycle (acknowledge/assign/suppress/resolve), severity mapping, deduplication, notification policies, delivery retries/status, templates, and safe action integrations. Ensure topology reflects active alarms and events can be correlated across devices. |
| **10a. Task and long-term performance; four threshold levels** | **Partial** | ICMP/CPU/memory/interface telemetry and several fixed alarm strategies exist. Task/scheduled collection and configurable Critical/Major/Minor/Warning thresholds are not evidenced; current rules use fixed thresholds and the severity model is narrower. | Add configurable polling/diagnostic jobs, retention and historical query controls, per-scope threshold policies, four-level severity mapping, sustained-duration/hysteresis, maintenance windows, stale-data handling, and operator-visible effective thresholds. |
| **10b. Bandwidth, connection, QoS, tags, quarantine, actions policies** | **No** | No policy authoring or switch-control/actuation implementation was found. | Treat as a separate network automation/security control plane. Confirm supported switches and whether capabilities are exposed via NETCONF, RESTCONF, vendor API, or CLI. Implement least-privilege credentials, policy simulation, approvals, staged deployment, rollback, and guardrails. Do not claim support based solely on telemetry. |
| **10c. Asset tags, serial, CPU type, firmware, memory** | **Partial** | Inventory contains hostname/address/type/model/location and some CPU/memory metrics. Asset tag, serial number, CPU model, and firmware inventory are not established. | Extend the inventory schema/API and poll supported ENTITY-MIB/vendor MIBs or approved APIs. Track source and timestamp per attribute, and expose unsupported/unknown fields clearly. |
| **10d. Configuration saved time, size, firmware details** | **No** | No configuration file backup/versioning or configuration metadata view was found. | Add encrypted configuration archive with capture time, size, checksum, device/firmware metadata, access control, retention, and restore/download workflow. Decide whether credentials/secrets are redacted before storage. |
| **10e. Device attribute history and change reporting** | **No** | Current inventory stores current state; no attribute change history/audit is documented. | Add time-stamped snapshots/change events with old/new values, source, actor (if user-initiated), diff view, filters, and reports. Distinguish observed device changes from NMS edits. |
| **10f. Configuration-change and firmware history** | **No** | No config change workflow, firmware deployment, or firmware history exists. | Add change records linked to approvals, config diffs, execution results, firmware version before/after, and rollback result. Build change windows and maintenance protections. Firmware actions require a model/image compatibility and signed image validation process. |
| **10g. In-depth inventory-planning reports** | **No** | No report builder or scheduled inventory/capacity reports were found. | Define required reports and filters with PRL; add CSV/PDF/export, scheduled delivery, access control, historical inventory, and capacity trends. Audit exports. |
| **10h. Single/multiple device firmware downloads** | **No** | Firmware repository, compatibility validation, image distribution, and upgrade orchestration are absent. | Confirm whether “downloading” means retrieving firmware images or upgrading devices. Add approved image catalog, checksums/signatures, compatibility gates, bandwidth/concurrency controls, staged rollout, progress, failure recovery, and maintenance-window approvals. This is high-risk device automation and should be separately accepted. |
| **10i. Port-level analysis** | **Partial** | Interface status, counters, errors, and some detail are present. Derived utilization graphs, error-rate analysis, VLAN/peer/duplex detail and complete per-port workflows remain incomplete. | Finish canonical interface model and per-port time-series; derive utilization and counter rates; add CRC/errors/discards/flaps, duplex/speed, peer/VLAN, filters, comparisons, thresholds, and export. |
| **11a. Unified management of switches** | **Partial** | Central monitoring and inventory exist, but only limited supported-model catalog behavior is implemented; no uniform switch configuration plane exists. | Establish vendor/model/firmware capability profiles. Unify inventory, credentials, telemetry, config, change control, and outcomes behind a capability-based adapter model. Maintain explicit unsupported-feature reporting. |
| **11b. Security policy configuration and event analysis** | **No** | No switch security-policy deployment or security-event analytics are implemented. | Define policy scope (ACL, port security, 802.1X, DHCP snooping, DAI, etc.), event sources, roles, workflow, and audit requirements. Implement only against vendor-supported interfaces with approval/rollback. Integrate traps/syslog and SIEM. |
| **12a. Customizable reports** | **No** | Fixed dashboards are present; report authoring/scheduling is not. | Build a controlled report catalog and configurable filters/columns/time ranges, exports, schedules, access controls, and audit. Avoid arbitrary client-supplied SQL. |
| **12b. Web, device, and system-wide event views** | **Partial** | A web UI, device detail, and active alarm/event displays exist. Full-history searchable event logs, saved views, and role-aware customization are incomplete. | Add searchable/filterable event history, pagination/retention, device and site drill-through, saved views, user preferences, and export/audit controls. |
| **12c. NTA / traffic distribution and protocol analysis** | **No** | Interface SNMP counters are not NTA. No NetFlow/IPFIX/sFlow/packet-flow receiver and traffic classification pipeline was found. | Confirm switch export support and licensing. Add flow collectors, templates/sampling handling, retention, top-talkers/protocol/application views, campus/site aggregation, and scale testing. Define privacy and data-retention rules. |
| **13. OEM/standard software, references, support, license, lifecycle** | **Blocking eligibility; No** | This repository is a locally developed custom product. It has no OEM product identity, commercial license, official vendor support contract, deployment references, or lifecycle commitments stated in the repo. | Resolve procurement eligibility before technical bid: use an OEM/established supported NMS, or obtain explicit PRL written approval for NetPulse and document owner, version, license, support/SLA, update cadence, references, vulnerability response, and lifecycle. A feature roadmap cannot satisfy this product-provenance requirement. |

## Recommended delivery plan

### Gate 0 — Procurement and scope decision (do first)

1. Ask PRL whether a custom-built NetPulse platform is acceptable under clauses
   1 and 13, and obtain the answer in writing.
2. If no exception is granted, stop treating NetPulse as the primary NMS and
   evaluate the switch OEM's NMS or an established supported commercial product.
3. Identify the exact switch OEM, models, firmware versions, license/support
   requirements, RHEL release, virtual environment, campus subnets, MLLN
   topology, device/interface counts, and acceptance criteria.
4. Clarify ambiguous phrases: “all configurable parameters,” “access
   terminals,” “throttle new network connections,” and firmware “downloading.”
5. Split monitoring requirements from configuration/automation requirements.
   Confirm whether each is mandatory for initial acceptance or can be phased.

**Exit:** procurement path, product eligibility, supported device matrix, and
written requirements/acceptance tests are agreed. Do not commit to a compliance
matrix until this gate passes.

### Gate 1 — Platform qualification and architecture

- Prove installation and upgrade on PRL's RHEL image and approved runtime.
- Produce a security architecture, network-flow/firewall matrix, HA/backup
  approach, and hardware sizing from representative benchmarks.
- Define identity, roles, audit retention/SIEM, secret handling, TLS, database
  roles, image supply chain, and security-assessment needs.
- Validate management reachability across all campuses and MLLN paths.
- Define vendor capability profiles and test SNMPv3/MIB coverage against each
  model/firmware combination.

**Exit:** PRL accepts deployment architecture, resource bill of materials,
security controls, and the support/upgrade model.

### Gate 2 — Monitoring foundation and campus pilot

- Replace static demo targets with controlled device onboarding and polling
  profiles/credential profiles.
- Normalize device identity, site/campus, interfaces, VLANs, counters, alarms,
  and poll health.
- Add per-user identity/RBAC and audit trail before operational use.
- Complete reliable interface rates, topology annotations/traffic, severity
  thresholds, event history, and notification integrations needed for pilot.
- Pilot representative switches from each campus and model; compare telemetry
  against device CLI and validate failure/recovery cases.
- Run scale, storage-retention, failover/restore, security, and egress-soak
  exercises.

**Exit:** signed pilot results show coverage, data correctness, alert behavior,
performance, security, restore, and operator workflows meet agreed thresholds.

### Gate 3 — Configuration management and advanced functions

If NetPulse remains in scope after explicit approval, implement switch
configuration management, VLAN/QoS/security policy workflows, configuration
backup/history/diff, approvals and rollback, and firmware lifecycle as separate
capability releases. Add traps/syslog, event correlation, reports, and flow
analytics (NTA) according to agreed priority. Each device-changing operation
needs vendor/firmware qualification and lab acceptance before campus rollout.

**Exit:** every high-risk action has an approved test plan, authorization model,
audit trail, rollback, and device-matrix evidence; PRL accepts each capability.

### Gate 4 — Production acceptance and operations

- Complete security assessment and remediation; approve licensing/support and
  ownership; establish patch/vulnerability and incident processes.
- Demonstrate role separation, audit retrieval, backup/restore, upgrade and
  rollback, failover, alarm notification, and campus-wide inventory reports.
- Document runbooks, support contacts, maintenance process, retention, SLAs,
  training, and operational handover.
- Submit an evidence-based compliance matrix. Mark “Yes” only where a tested
  feature and supporting artifact satisfy the client's acceptance criteria;
  otherwise mark “No” or “Partial” with a dated remediation/exception.

## Bid/response guidance

- **Do not state “Yes”** for clauses 1 or 13 for NetPulse without written PRL
  exception and the required support/licensing evidence.
- Clauses **2, 3–12 are not generally compliant** based on current repository
  evidence. Use “Partial,” “No,” or “Unverified” per the matrix, rather than
  relying on architecture diagrams or future roadmap entries.
- Separate **current implementation**, **planned change**, and **third-party
  dependency** in the response. For example, email notifications, SNMP traps,
  syslog, and NTA require features/services not in this repo today.
- Hardware sizing, RHEL certification, device compatibility, support life, and
  multi-campus scale are evidence tasks; do not make unsupported claims.

## Evidence to collect from PRL

1. Switch OEM/model/firmware inventory and management protocol capabilities.
2. Campus topology, management VLAN/subnets, MLLN routing/latency/ACL details.
3. Total and growth device/interface counts; expected poll interval and data
   retention.
4. RHEL release, hypervisor, approved container runtime, storage class, HA and
   backup constraints.
5. Identity provider, role definitions, SIEM, email relay, syslog/trap policy,
   and audit retention.
6. Required configuration domains, approval/rollback rules, firmware
   sourcing/validation, and change windows.
7. Formal acceptance thresholds and procurement decision for custom software.
