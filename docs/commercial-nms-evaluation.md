# Commercial NMS Evaluation Baseline

## Decision for planning

Use **ManageEngine OpManager Enterprise** as the leading commercial evaluation
candidate, with the **Network Configuration Manager (NCM)** and **NetFlow
Analyzer** add-ons. Evaluate **Firewall Analyzer** as an additional module only
if it is needed to meet PRL's security-event analysis requirement.

This is a product-selection baseline, not a final vendor award or compliance
claim. The switch OEM, exact model and firmware list, PRL's RHEL release, device
count, required license scope, and proof-of-concept results are not yet
available. Requirements 1 and 13 are satisfied only if the selected product
and support arrangement meet PRL's OEM/commercial-product conditions.

## Progress and timeline checkpoint — 2026-11-14

- Requirement coverage and the initial product-fit questions are documented in
  [`client-requirements-gap-plan.md`](client-requirements-gap-plan.md) and the
  [source transcription](prl-requirements-source.md). The received source cuts
  off in the last sentence of clause 13; recover that text before submission.
- The OpManager API read-only inventory slice is in progress on
  `feat/commercial-nms-adoption`. The working tree currently contains
  uncommitted changes. It is not considered release-ready or merged until
  live integration validation is complete. On 2026-11-14, `go test ./...`
  passed in Ubuntu with `CGO_ENABLED=0`, and the UI production build passed
  with Bun. OpManager client tests use a mocked API; no live lab credentials
  are configured.
- Next checkpoint: validate URL, authentication, pagination, timeout/error
  handling, TLS, and returned fields against an authorized lab account. Then
  review and merge the finished branch. Remind the user to test and merge it.
- Indicative schedule remains 2–4 weeks for the eligibility/scope gate,
  4–6 weeks for device/platform discovery, 4–12 weeks for commercial product
  selection/procurement, and 8–12 months from kickoff for the commercial path
  through multi-campus acceptance. These ranges overlap where noted in the
  delivery estimate and must be rebaselined after Gate 0 and discovery.
- New feature work belongs on a dedicated `feat/...` branch. Merge only after
  implementation and tests are complete; remind the user to test and merge
  each finished branch.

## Why OpManager is the leading candidate

ManageEngine's current product documentation describes:

- RHEL as a supported production operating system. PRL's exact RHEL release and
  OpManager version still need confirmation.
- A central/probe Enterprise deployment and vendor-published sizing guidance.
  Final VM sizing depends on the actual device/interface count, polling
  interval, retention, and enabled add-ons.
- Custom user roles and access scopes.
- Notification templates for email, SNMP traps, syslog, and running a program.
- NCM features for configuration backup, change tracking, templates, and
  firmware management on supported device models.
- NetFlow Analyzer support for flow-based traffic analysis, including NetFlow,
  sFlow, J-Flow, and IPFIX.
- Broad device-template coverage, with custom templates available for devices
  that are not natively classified.

These capabilities make OpManager a credible product to evaluate against the
specification. Add-on licensing and model-specific support must be included in
the commercial and technical evaluation.

## Requirement fit to prove

| PRL requirement | Candidate capability to demonstrate | Qualification still required |
|---|---|---|
| 1, 11a. Unified switch management and configuration | OpManager plus NCM | Prove every required configuration domain on the exact proposed switch/firmware matrix; verify the OEM/commercial eligibility evidence |
| 2. RHEL and VM resources | OpManager Linux deployment | Confirm the precise RHEL release, approved install method, sizing, and resource details with the vendor |
| 3, 10e, 10f. Audit and change history | Audit, configuration-change, and device-history features | Demonstrate retained, searchable records and reports that meet PRL's post-audit needs |
| 4. Policies and roles | Custom roles and access scopes | Demonstrate policy creation and authorized personnel activating/deactivating it |
| 5–7. Topology, links, traffic, and four-campus coverage | Site/business views, topology, probes, interface monitoring | Validate access-terminal data, highlighted devices/links, required link colors, MLLN reachability, and full-campus scale |
| 8. VLAN configuration and monitoring | NCM configuration functions | Demonstrate system-wide VLAN deployment and state monitoring on the exact devices |
| 9. Alarm management | Alarm rules and notification templates | Demonstrate real-time behavior and every required output: topology, SNMP trap, syslog, script, and customizable email |
| 10a, 10c, 10i. Performance, inventory, port analysis | Performance monitors, hardware inventory, interface views | Demonstrate all four required alarm levels, thresholds, requested attributes, and port-level details |
| 10b, 11b. QoS/bandwidth/security policies and events | NCM, supported workflows, and possibly Firewall Analyzer | Prove each requested action (rate-limit, connection throttle, L2/L3 QoS, packet tags, quarantine, predefined actions) and security-policy analysis on the target models |
| 10d. Saved-configuration details | NCM configuration archive | Demonstrate capture date/time, file size, firmware version, and required device/configuration metadata |
| 10g, 12a, 12b. Reports and views | OpManager reports and web/device/event views | Demonstrate customizable reports, inventory-planning outputs, flexible web views, and whole-setup event logs |
| 10h. Firmware downloads | NCM firmware functions | Confirm whether PRL means downloading images to the NMS or deploying/upgrading devices; demonstrate single and simultaneous multi-device handling |
| 12c. NTA | NetFlow Analyzer | Confirm switches export a supported flow format and demonstrate traffic distribution, protocol types, port statistics, and required reports |
| 13. Commercial/OEM product standing | Vendor, product, license, support, and lifecycle documents | Obtain official product/version, licensing, support, update, reference, and long-term maintenance evidence for the bid |

A requirement remains **unverified** until its proof-of-concept result and
supporting vendor evidence are recorded. Product brochures alone are not
acceptance evidence.

## Build and integration direction

Treat the commercial NMS as the primary product. Do not continue building
duplicate monitoring, configuration, alarm, reporting, or NTA features in this
repository as though they were the primary NMS. Keep the current custom app as
a prototype or approved supplementary integration only. Any supplementary
component must not weaken the product eligibility in clauses 1 and 13.

If PRL approves a supplementary role, integrate through the product's supported
API and keep secrets out of source control. Keep vendor-specific behavior
behind a small adapter boundary, record the product/version and integration
contract, and avoid direct database writes into the vendor product.

## Proof-of-concept exit criteria

1. PRL supplies the exact switch make/model/firmware inventory and RHEL target.
2. Vendor confirms supported product version, device models, add-ons, licenses,
   and support coverage in writing.
3. A lab demonstrates representative monitoring, configuration, VLAN, alarm,
   report, firmware, policy, and NTA workflows against target hardware or
   vendor-approved test devices.
4. PRL confirms the four-campus/MLLN path, device/interface capacity, retention,
   and acceptance thresholds.
5. The project team records pass/fail evidence for every clause before
   producing a final compliance response and deployment bill of materials.

## Vendor documentation

- [OpManager hardware/software requirements](https://www.manageengine.com/network-monitoring/help/hardware-and-software-requirements.html)
- [OpManager user roles and access](https://www.manageengine.com/network-monitoring/help/user-roles-types.html)
- [OpManager notification templates](https://www.manageengine.com/network-monitoring/help/notification-templates.html)
- [OpManager NCM capabilities and supported devices](https://www.manageengine.com/network-monitoring/nccm-faqs.html)
- [OpManager traffic analysis / NetFlow add-on](https://www.manageengine.com/network-monitoring/network-traffic-analysis.html)
- [OpManager device discovery and supported templates](https://www.manageengine.com/network-monitoring/help/what-you-should-monitor.html)
- [OpManager and Firewall Analyzer integration](https://www.manageengine.com/products/firewall/help/firewall-misc/integrating-with-opmanager.html)

## Read-only integration slice

The repository now contains an optional read-only integration for OpManager's
documented `listDevices` API. It requests one page at a time and presents the
data in a separate **OpManager inventory** view; it does not create or update
records in the prototype inventory.

To enable it for a lab instance:

1. Create an OpManager account with only the required device-read access and
   enable REST API access for that account.
2. Generate an API key and supply the instance URL and key through the local,
   Git-ignored `.env` file:
   `OPMANAGER_BASE_URL=https://<host>:8060` and
   `OPMANAGER_API_KEY=<key>`.
3. Rebuild/restart `nms-middleware` and `nms-ui`, sign in, and open
   **OpManager inventory**. The read-only endpoint is
   `GET /api/v1/integrations/opmanager/devices?page=1&page_size=50`.

The API key is sent in the documented `apikey` request header, never in the
URL or browser. HTTPS is required except for loopback development. The handler
is session-authenticated, paginated, bounded by a timeout/response-size limit,
and returns no-store responses. Use a valid TLS certificate trusted by the
middleware runtime; do not disable certificate verification. For production,
inject the key through the approved secrets manager rather than committing it
or placing it in a tracked file.

This slice verifies read-only connectivity only. It does not yet synchronize
inventory, alarms, performance history, configuration, reports, NTA, or policy
actions. Those remain gated on the OpManager edition/add-on licenses and the
PRL device/firmware proof of concept.

Vendor API reference: [OpManager listDevices API](https://www.manageengine.com/network-monitoring/help/restapi/devices.html).
