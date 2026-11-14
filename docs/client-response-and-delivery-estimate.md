# PRL client response draft and delivery estimate

> **Internal note:** This is a draft for review, not a compliance declaration
> or a binding delivery commitment. Replace bracketed fields and get technical,
> procurement, security, and delivery-owner approval before sending.

The specification requires an OEM-native or established commercial NMS as the
primary platform and explicitly excludes a custom solution unless PRL approves
it. NMS is currently a custom-developed monitoring platform and does not meet
that product eligibility requirement or the complete feature set.

For planning, use **ManageEngine OpManager Enterprise** with the **NCM** and
**NetFlow Analyzer** add-ons as the leading commercial evaluation candidate.
Evaluate **Firewall Analyzer** if it is needed for PRL's security-event analysis
scope. Vendor documentation describes RHEL support, configuration management,
flow analysis, custom roles, and notification templates. This is not a final
vendor award or compliance claim. Validate the exact switch models and firmware,
all required policy actions, add-on licensing, and PRL's RHEL release in a
proof of concept. The qualification checklist is in
[commercial-nms-evaluation.md](commercial-nms-evaluation.md).

Keep the current custom NMS out of the primary-product role. Offer it only as
a supplementary integration if PRL approves that role in writing.

Do not represent roadmap work as delivered capability. Do not mark a line item
“Yes” unless there is a tested feature and supporting evidence. The
requirement-by-requirement findings are in
[`client-requirements-gap-plan.md`](client-requirements-gap-plan.md).

## Client-ready response draft

**Subject: Response to NMS technical requirements and proposed next steps**

Dear [Client Contact/PRL Evaluation Committee],

Thank you for sharing the Network Management Software requirements. We have
reviewed the requested capabilities, including multi-campus monitoring,
topology, performance and alarm management, role-based operation, auditing,
VLAN and security-policy management, configuration and firmware history, and
network traffic analysis.

### Product approach and compliance

We understand that the primary NMS must be either supported by the switch OEM
or be an established, commercially available product with proven deployments,
vendor support, licensing, documentation, and a maintained upgrade lifecycle.
Our current NMS software is a custom-developed monitoring platform. It
provides a foundation for inventory, basic telemetry, polling-based alarms,
device views, and LLDP topology, but it is not currently an OEM-supported or
commercially established NMS and does not currently provide the full switch
configuration, VLAN/policy deployment, firmware lifecycle, full person-level
person-level audit controls, notification, or NTA scope in the specification. A
basic metadata audit log for authentication and API activity is now present,
but it attributes users only to the shared bootstrap account and does not
provide before/after change diffs or protected external retention.

Accordingly, we will not represent NMS as fully compliant with the
OEM/commercial-product requirements or claim that planned features are already
available. Our recommended approach is to nominate a qualifying OEM or
commercial NMS as the primary platform. NMS may be considered for a
supplementary role only if PRL confirms that such a component is acceptable.
If PRL wishes to evaluate NMS as the primary solution, we request written
confirmation that PRL will consider an exception to the product eligibility
clauses before we submit a revised compliance position.

### Proposed planning timeline

These are planning ranges, not committed dates. The full scope is materially larger
than the current prototype. The primary-product eligibility decision in clauses
1 and 13 is the first gate. Schedule starts at kickoff and assumes PRL provides
the required device, network, and RHEL details promptly.

| Stage | Indicative duration | Main outputs |
|---|---:|---|
| Product eligibility and scope gate | 2–4 weeks | Written decision to use an eligible OEM/commercial NMS, or PRL approval to evaluate the custom NMS; confirm every clause and acceptance evidence |
| Device, campus, and platform discovery | 4–6 weeks, partly parallel with the gate | Switch model/firmware and interface inventory; MLLN reachability; RHEL release and VM resources; management access methods; required reports, alarm channels, NTA source, and firmware-download interpretation |
| Product selection and procurement (commercial path) | 4–12 weeks; may overlap discovery | Confirmed product/version, licensing, vendor support, compatible device matrix, and implementation plan |
| RHEL deployment and device lab qualification (commercial path) | 6–10 weeks after product access | RHEL installation, integration, representative device checks, and accepted deployment design |
| Campus rollout and acceptance (commercial path) | 8–12 weeks after lab qualification | Main-campus pilot, staged rollout to the remaining campuses over the MLLN, training, and clause-by-clause evidence |
| Custom NMS foundations (only after written PRL exception) | 3–5 months | RHEL deployment, unified monitoring, inventory, roles/privileges, audit, and multi-campus data model |
| Custom NMS monitoring and operations | 4–6 months, partly overlapping foundations | Topology views, performance and inventory attributes, port analysis, configured alarm levels and channels, customizable reports, and required event views |
| Custom NMS switch control and advanced data | 6–9 months, partly overlapping monitoring | System-wide VLAN, switch configuration/policy controls, configuration and firmware history/details, simultaneous firmware downloads, security-event analysis, and NTA |
| Custom NMS integration and production acceptance | 3–5 months after core functions | Model/firmware qualification, load and MLLN validation, operator acceptance, and evidence for every clause |

For a **supported OEM/commercial product**, plan on **8–12 months from kickoff**
to multi-campus acceptance, including product selection and procurement. After
the product and licenses are available, implementation is approximately
**5–8 months**.

For the **custom NMS**, plan on **18–24 months from kickoff** for the complete
specified scope, assuming a dedicated **6–8 person delivery team** and timely
PRL inputs. A limited pilot may be possible around **months 9–12**, but that is
not full compliance or production acceptance. The custom route remains
conditional on PRL's written exception to clauses 1 and 13; the timeline cannot
resolve that eligibility issue.

The ranges exclude delays for procurement appeals, missing device access,
unsupported firmware, hardware changes, or additional certification. Rebaseline
the schedule after the eligibility gate and device discovery. Do not promise
a final completion date before those inputs and the delivery team are confirmed.

### Delivery assumptions and dependencies

The timelines above assume:

1. PRL confirms whether the selected product can meet the OEM/commercial
   eligibility requirement and provides a procurement decision without an
   extended approval delay.
2. PRL supplies the supported RHEL release, virtual environment constraints,
   required access method, and infrastructure provisioning on schedule.
3. PRL provides a complete device list with campus, management IP/subnet,
   manufacturer, exact model, firmware, interface count, and support status.
4. The MLLN provider and PRL network team provide routing, ACL/firewall,
   latency, MTU, redundancy, and troubleshooting support for management-plane
   access from the NMS to all campuses.
5. Required device management protocols and credentials, plus the MIB/API/CLI
   access needed for the agreed models, are available for lab and pilot devices.
   Configuration actions have authorized write access.
6. PRL defines the required operator roles/privileges, audit record scope,
   alarm notification channels and email content, and event-log expectations.
7. PRL defines measurable acceptance criteria, including device and interface
   counts, polling intervals, data retention, alarm latency, topology coverage,
   report examples, availability, recovery, and performance thresholds.
8. The client-approved product/vendor provides required licenses, product
   documentation, technical support, patches, and implementation assistance.

### Inputs requested from PRL

To finalize product selection, sizing, schedule, and the compliance matrix,
please provide or confirm:

- Switch OEMs, exact models, firmware versions, device and interface counts,
  planned growth, and whether any non-switch device types are in scope.
- Main and campus network diagrams, management subnets, MLLN service details,
  routing/ACL policy, bandwidth/latency expectations, and redundant paths.
- RHEL version, hypervisor/virtualization standards, approved container or
  application runtime, VM provisioning process, storage and backup standards.
- Operator roles/privileges, audit record scope, alarm notification channels,
  email customization needs, and event-log expectations.
- Exact switch configuration domains; how PRL interprets firmware “downloading”;
  and which listed policy actions must be included in initial acceptance.
- Expected NTA sources (NetFlow, IPFIX, sFlow, or other), flow retention,
  sampling expectations, and privacy constraints.
- Required notification channels, maintenance windows, change approval and
  rollback process, service hours, response times, and support expectations.
- PRL's interpretation of “access terminals,” “all configurable parameters,”
  “throttle new network connections,” and firmware “downloading.”
- Confirmation whether NMS may be considered only as a supplementary
  component, or whether PRL will consider an exception to the primary-product
  eligibility requirement.

### Commercial and operational items to finalize

The proposal should separately identify product license/subscription costs,
OEM support and renewal, implementation services, integrations, VM/storage
resources, security assessment, training, travel if needed, taxes, and any
customization. It should state the support hours, severity definitions,
response targets, escalation path, patch cadence, end-of-life handling, backup
ownership, and acceptance/warranty terms. These values cannot be responsibly
filled in until the product route and deployment scope are agreed.

We propose a requirements and architecture workshop as the next step. Following
that workshop and receipt of the requested inputs, we can provide a baselined
compliance matrix, validated hardware bill of materials, implementation
schedule, responsibility matrix, and commercial proposal.

Regards,

[Name]  
[Title / Organization]  
[Contact details]

## Internal work required before sending or committing

### Decisions and approvals

- Confirm whether to pursue a supported third-party NMS or seek a written PRL
  exception for NMS. This decision determines whether engineering work is
  a deployment/integration effort or a major product build.
- Have procurement/legal verify product eligibility, license terms, OEM
  authorization, support coverage in the region, and any tender requirements.
- Have engineering/security review each “current status” statement and attach
  evidence; do not sign a “Yes” compliance response for untested capabilities.
- Assign a named delivery owner who can commit to schedule, scope, price, and
  acceptance language.

### Minimum discovery team

- Network architect familiar with campus switching, routing, VLANs, QoS, and
  MLLN/provider handoffs.
- Systems/platform engineer for RHEL, virtualization, resource sizing, and
  container/runtime constraints.
- Security specialist for operator access to configuration controls, device
  credential handling, and review of audit/event behavior.
- NMS/vendor solution architect and OEM support contact for exact model and
  firmware compatibility.
- Application/integration engineer for product integration, notification
  channels, NTA data sources, and any approved supplemental NMS role.
- Operations representative(s) from PRL to define alarms, workflows, reports,
  maintenance windows, training, and service acceptance.
- Procurement/commercial owner for licensing, support, and contract terms.

For a NMS product-development path, also plan dedicated backend, frontend,
QA/automation, and device-integration engineering capacity. A small pilot team
cannot in parallel deliver a mature configuration manager, firmware system,
multi-vendor adapter set, NTA platform, role/audit features, and production
support model without prioritization or schedule growth.

### Technical workstreams if NMS is approved

1. Product eligibility, ownership, licensing, support, security lifecycle.
2. RHEL installation qualification, deployment approach, and VM resource sizing.
3. Role and privilege enforcement for policy activation, and the audit records
   needed for post-audit review.
4. Vendor/model compatibility, device onboarding, inventory-driven monitoring,
   and telemetry normalization.
5. Multi-campus switch inventory and monitoring over the MLLN; topology,
   traffic/performance views, interface rates, and access-terminal visibility.
6. Configurable performance alarm levels and the required alarm display,
   SNMP trap, syslog, custom-script, and email notifications, including email
   content customization.
7. Switch configuration and system-wide VLAN management; configuration details
   and change history; firmware details, history, and simultaneous downloads;
   bandwidth, connection-rate, QoS, packet-tagging, quarantine, and action policies.
8. Device-attribute history and change reporting, inventory-planning reports,
   customizable reports, required web/device/event views, port-level analysis,
   and NTA.
9. Device lab and acceptance checks across the agreed model/firmware matrix,
   performance testing at the agreed scale, and evidence for each clause.

## Timeline confidence and change control

The 8–12 month commercial-product range is medium confidence after a qualifying
product has been identified; procurement and device compatibility are the
largest unknowns. The 18–24 month custom-NMS range is low confidence because it
depends on PRL granting an exception, scope decisions, device-specific APIs,
and the availability of a dedicated 6–8 person team. Re-estimate after
eligibility and discovery using actual device counts, interfaces, polling,
retention, notification channels, and accepted criteria. Any change to supported
models, configuration domains, firmware-download interpretation, NTA volume,
or compliance obligations should trigger a schedule and cost review.
