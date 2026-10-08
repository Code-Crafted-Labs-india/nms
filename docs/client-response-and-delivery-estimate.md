# PRL client response draft and delivery estimate

> **Internal note:** This is a draft for review, not a compliance declaration
> or a binding delivery commitment. Replace bracketed fields and get technical,
> procurement, security, and delivery-owner approval before sending.

## Recommended position

The specification requires an OEM-native or established commercial NMS as the
primary platform and explicitly excludes a custom solution unless PRL approves
it. NetPulse is currently a custom-developed monitoring platform and does not
meet that product eligibility requirement or the complete feature set. The
safest response is to propose an OEM/commercial NMS as the primary solution.
Offer NetPulse only as a supplemental or custom component if PRL approves that
role in writing.

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
Our current NetPulse software is a custom-developed monitoring platform. It
provides a foundation for inventory, basic telemetry, polling-based alarms,
device views, and LLDP topology, but it is not currently an OEM-supported or
commercially established NMS and does not currently provide the full switch
configuration, VLAN/policy deployment, firmware lifecycle, audit, notification,
or NTA scope in the specification.

Accordingly, we will not represent NetPulse as fully compliant with the
OEM/commercial-product requirements or claim that planned features are already
available. Our recommended approach is to nominate a qualifying OEM or
commercial NMS as the primary platform. NetPulse may be considered for a
supplementary role only if PRL confirms that such a component is acceptable.
If PRL wishes to evaluate NetPulse as the primary solution, we request written
confirmation that PRL will consider an exception to the product eligibility
clauses before we submit a revised compliance position.

### Proposed planning timeline

The following is an **indicative planning range**, dependent on PRL's answers,
procurement route, device inventory, access approvals, and acceptance criteria.
It is not a fixed delivery commitment. Dates should be baselined after the
discovery and design activities below.

| Stage | Indicative duration | Main outputs |
|---|---:|---|
| Requirements clarification and product/procurement decision | 2–3 weeks | Written interpretation of mandatory clauses; OEM/commercial product shortlist or decision on custom-solution exception; confirmed scope and acceptance criteria |
| Site, device, security, and capacity discovery | 2–4 weeks | Campus and MLLN reachability design; device/model/firmware matrix; RHEL/runtime and VM sizing inputs; identity, firewall, audit, retention, and integration requirements |
| Product selection, licensing, and high-level design | 3–8 weeks, partly parallel | Vendor/product confirmation, license/support proposal, architecture, deployment bill of materials, implementation and migration plan |
| Lab installation and interoperability validation | 3–6 weeks after product and access are available | RHEL installation, device protocol/MIB validation, credentials, alert tests, reports, backup/restore and operations runbooks |
| Main-campus pilot | 4–6 weeks | Limited device onboarding, telemetry and topology validation, operator training, workload/retention measurements, issue closure |
| Additional-campus rollout and production acceptance | 4–8 weeks | Staged rollout over approved MLLN paths, site acceptance, security evidence, handover and support activation |

On these assumptions, a supported OEM/commercial-product deployment is
approximately **4–7 months from kickoff to multi-campus production acceptance**.
Procurement, license approvals, access/security reviews, device remediation,
or OEM lead times may extend this range. Some discovery, procurement, and
architecture work can run in parallel where PRL permits.

If PRL instead approves NetPulse as the primary platform, it should be treated
as a product-development program rather than a deployment project. A
preliminary planning range is **9–15 months or more** for a minimum production
scope, with a dedicated cross-functional team and phased acceptance. This
range is high uncertainty and excludes any additional certification or
accreditation period. Full parity with every item—including multi-vendor
configuration automation, safe firmware lifecycle, and NTA—may require more
time and should be estimated only after a discovery and architecture phase.

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
5. Read-only SNMPv3 credentials and required MIB/API/CLI access are approved
   and available for lab and pilot devices. Configuration and firmware
   functions require separately authorized, least-privilege write access.
6. PRL identifies its identity provider, MFA and role model, SIEM, email relay,
   syslog/trap policy, audit retention, backup target, and security assessment
   process.
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
- Identity, MFA, operator roles, SIEM, email relay, log retention, and required
  security/accreditation standards.
- Required switch configuration domains and whether policy automation,
  firmware staging/upgrades, and quarantine actions are mandatory at initial
  acceptance.
- Expected NTA sources (NetFlow, IPFIX, sFlow, or other), flow retention,
  sampling expectations, and privacy constraints.
- Required notification channels, maintenance windows, change approval and
  rollback process, service hours, response times, and support expectations.
- PRL's interpretation of “access terminals,” “all configurable parameters,”
  “throttle new network connections,” and firmware “downloading.”
- Confirmation whether NetPulse may be considered only as a supplementary
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
  exception for NetPulse. This decision determines whether engineering work is
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
- Systems/platform engineer for RHEL, virtualization, storage, backup, HA, and
  container/runtime constraints.
- Security architect for identity/MFA, SNMPv3 secrets, firewall policy, SIEM,
  audit, threat review, and security approval.
- NMS/vendor solution architect and OEM support contact for exact model and
  firmware compatibility.
- Application/integration engineer for data migration, API/SSO/SIEM/email
  integrations, and any NetPulse supplemental role.
- Operations representative(s) from PRL to define alarms, workflows, reports,
  maintenance windows, training, and service acceptance.
- Procurement/commercial owner for licensing, support, and contract terms.

For a NetPulse product-development path, also plan dedicated backend, frontend,
QA/automation, and device-integration engineering capacity. A small pilot team
cannot in parallel deliver a mature configuration manager, firmware system,
multi-vendor adapter set, NTA platform, enterprise identity/audit, and
production support model without prioritization or schedule growth.

### Technical workstreams if NetPulse is approved

1. Product eligibility, ownership, licensing, support, security lifecycle.
2. RHEL install/upgrade qualification, deployment automation, HA, backup,
   restore, monitoring of the NMS, and resource sizing.
3. Identity-provider integration, role-based authorization, approvals, and
   immutable audit/SIEM forwarding.
4. Vendor/model capability profiles, SNMPv3 credential vaulting, onboarding,
   inventory-driven polling, and telemetry normalization.
5. Multi-campus discovery/site hierarchy, MLLN resiliency, scalable topology,
   interface rates, VLAN and endpoint visibility.
6. Alarm lifecycle, four-level severity, traps/syslog, email/webhook/script
   integrations, notification retries, and event correlation.
7. Change/configuration management, policy templates, diff, approvals, staged
   rollout, rollback, config archive, and firmware lifecycle.
8. Reports, inventory history, port analysis, flow-based NTA, capacity,
   retention, exports, and operator workflows.
9. Device lab, regression testing across model/firmware matrix, performance
   tests, penetration/security review, disaster recovery, and acceptance
   evidence.

## Timeline confidence and change control

The 4–7 month commercial-product range is medium confidence only after a
qualifying product has been identified; procurement and device compatibility
are the largest unknowns. The 9–15+ month NetPulse range is low confidence
because it depends on scope decisions and device-specific API behavior. Either
range must be re-estimated after discovery using actual device counts,
interfaces, retention, integrations, security gates, and accepted feature
priorities. Any change to supported models, write/firmware scope, NTA volume,
HA, or compliance regime should trigger schedule and cost review.
