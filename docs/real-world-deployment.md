# Real-world deployment guide

This guide describes how to prepare NetPulse for a controlled deployment that
polls real network equipment. It is a deployment and pilot guide, not a claim
that this repository is production-certified. The current Compose stack is a
development/demo environment: it includes simulated devices, static SNMPv2c
targets and credentials, and a middleware database connection with TLS
disabled. Do not expose that stack to a production network unchanged.

For organizational security requirements, use this guide together with the
[command-center security baseline](ui-security-baseline.md),
[security hardening scope](security-hardening-future-scope.md),
[data boundary](data-boundary.md), and [running guide](running.md).

## 1. Decide the deployment boundary

Before installing anything, record:

- Network ranges and device management addresses approved for monitoring.
- Device vendors, models, operating-system versions, and supported SNMP MIBs.
- Expected device and interface counts, polling intervals, and retention period.
- The NMS host's management address, operating system, Docker version, storage,
  backup target, and recovery owner.
- Operator identity provider, MFA, role mapping, TLS gateway, log/SIEM target,
  maintenance window, and escalation contacts.
- Applicable security or regulatory authorization requirements.

Start with a small pilot segment (for example, 5–20 devices) and a dedicated
management host or VM. Size CPU, RAM, and storage from measured pilot ingestion
and retention; the repository does not provide a validated production sizing
model.

## 2. Prepare the network and devices

Use a dedicated management VLAN or equivalent routed management plane. Permit
only the NMS collector to reach approved device management addresses. Example
flows to review with the network/security team:

| Source | Destination | Protocol | Purpose |
|---|---|---|---|
| Telegraf collector | Approved device management IPs | UDP 161 | SNMP polling |
| Go middleware | Approved device management IPs | ICMP | Reachability checks |
| Network devices | Approved NMS receiver | UDP 162, if trap receiver is later deployed | Traps; current app does not ingest traps |
| Operator workstation | Approved HTTPS gateway | TCP 443 | Command center |
| NMS services | Internal DNS/NTP, if required | Organization-approved | Name resolution and time sync |

Restrict source and destination addresses at the host and network firewalls.
The Docker bridge in this repository is not an egress firewall. Do not allow
the collector or middleware unrestricted access to the corporate network or
Internet. Keep the database and Grafana off user/device-facing networks. If
the environment is air-gapped, import reviewed images and dependencies through
the organization's approved process.

On each pilot device, configure a dedicated read-only SNMPv3 `authPriv` user,
permit it only from the collector's source address, and expose only the MIB
views needed for approved metrics and LLDP. Enable LLDP where topology
collection is desired. Confirm that ICMP is permitted if reachability checks
are required. Never use the mock `public` community on real devices.

## 3. Build a production deployment configuration

Create and review a production-specific Compose/configuration set before
starting the stack. The supplied `docker-compose.yml` includes mock device
containers on `192.168.10.0/24`, binds UI/Grafana to the host, and uses
development-oriented image tags. The supplied `telegraf.conf` has mock IPs,
SNMPv2c `public`, and simulator-specific CPU/memory OIDs. It must be replaced
with customer-specific targets, credentials, and vendor-appropriate OIDs.

At minimum, the production configuration must:

1. Omit every `mock-*` service and the demonstration seed data.
2. Bind the command center only to loopback behind the approved HTTPS/identity
   gateway, or use an explicitly reviewed management interface and firewall.
   Do not publish the middleware API or database ports to the host.
3. Replace static mock Telegraf targets with only approved device addresses.
   Configure SNMPv3 security level, username, authentication and privacy
   protocols, and secrets using the Telegraf version's supported secret
   mechanism. Keep secrets out of Git and logs.
4. Replace simulator-specific CPU and memory OIDs with tested vendor MIBs.
   Validate interface counter widths and counter rollover behavior. Configure
   LLDP polling only where the device supports it and the SNMP view permits it.
5. Replace all `latest` image references with reviewed, pinned versions or
   digests; record the approved image provenance.
6. Store secrets in the organization's secrets manager or protected files with
   restrictive ownership and permissions. Use distinct high-entropy values;
   rotate all bootstrap/development credentials.
7. Configure TLS for database connections and least-privilege database roles.
   The checked-in middleware DSN currently sets `sslmode=disable`, and Telegraf
   connects as the PostgreSQL superuser. These require deployment changes.
8. Keep logs, database volumes, backups, and Grafana data on protected storage
   with documented retention, capacity monitoring, and restore procedures.

The inventory form accepts device SNMP metadata, but current polling does not
automatically consume inventory credentials: Telegraf's target list is static.
Until inventory-driven polling and credential profiles are implemented,
changes to managed devices must be coordinated between inventory and the
Telegraf configuration, then reviewed and deployed as configuration changes.

## 4. Apply identity and security controls

Before operators use the system over a network:

- Terminate TLS 1.3 at an approved gateway, enable HSTS there, and use the
  secure cookie behavior described in the UI security baseline.
- Replace the bootstrap-token login with the organization's OIDC/SAML,
  phishing-resistant MFA, account lifecycle, and centrally managed roles.
  The current bootstrap token is not organizational SSO or per-user RBAC.
- Enforce access so only authorized operator workstations can reach the UI.
- Forward authentication and administrative audit events to the SIEM. The
  current application records metadata-only login/authentication and API
  activity events in PostgreSQL and exposes them through an authenticated,
  paginated endpoint. It still uses one shared bootstrap identity, and it does
  not yet provide person-level attribution, before/after field diffs, SIEM
  forwarding, retention controls, or tamper protection. These are required
  before treating the log as a production audit system.
- Use a read-only Grafana database account and keep Grafana private to the
  operations team if it is used.
- Pin and scan images/dependencies, produce an SBOM, and document update and
  vulnerability response ownership.
- Complete threat modeling, configuration review, and an independent security
  assessment appropriate to the environment before production authorization.

These are deployment requirements/gaps, not settings that the UI alone
provides. Do not treat the current local bootstrap authentication as an
enterprise identity integration.

## 5. Install and configure the pilot

1. Provision a supported host and install the organization's approved Docker
   Engine and Compose versions.
2. Check out a reviewed release/commit. Review the Compose file, database
   migration, Telegraf configuration, container images, and firewall policy.
3. Create secrets in the approved secret store; do not commit `.env` or secrets
   into this repository. The local `.env.example` is a development template.
4. Prepare the production Compose and Telegraf files described above, then run
   `docker compose -f <production-compose-file> config --quiet` to validate
   their syntax. Do not use the demo compose file unchanged for real devices.
5. Apply host/network firewall policy before bringing up services.
6. Start database, middleware, Telegraf, and UI in the approved order. Use the
   organization's deployment process; the standard local command is documented
   in [running.md](running.md), but it starts the demo stack as checked in.
7. Confirm database schema/migrations and service health. Do not load
   `db/sample_data.sql` in a real deployment.
8. Enroll one supported pilot device at a time. Add its address to the reviewed
   Telegraf target configuration as well as the inventory. Confirm SNMP and
   ICMP reachability from the actual container/host network namespaces.
9. Check Telegraf logs and database records for each metric table. Confirm
   device identity, interface indexes, units, timestamps, counter deltas, and
   topology adjacency against the device CLI or an independent SNMP query.
10. Trigger controlled, approved test conditions (for example, a lab interface
    shutdown or temporary test target) and verify alert creation, recovery,
    timestamps, and operator interpretation. Do not disrupt production links
    just to test alerts.

## 6. Pilot acceptance checklist

Expand beyond the pilot only when all applicable checks pass:

- Only approved sources can access the command center; HTTPS and identity
  controls work as intended.
- Firewall logs confirm only approved SNMP/ICMP targets and required service
  flows; unexpected DNS/HTTP/HTTPS egress is investigated.
- No mock services, sample data, default credentials, or SNMPv2c `public`
  settings remain in the deployed configuration.
- System health, interfaces, and LLDP records match the source devices and
  vendor MIB documentation; unsupported values are understood.
- Poll duration and database ingest keep up with the configured interval at
  representative device/interface scale.
- Alarm thresholds, debounce/hysteresis, stale-data behavior, and recovery are
  reviewed with operators. Notifications are not available until a delivery
  adapter is implemented and configured.
- Disk growth, database retention, backups, restore, upgrade, and rollback have
  been exercised and have named owners.
- Authentication/admin audit forwarding, monitoring of the NMS itself, and
  incident procedures are operational.
- A soak period completes with no unexplained gaps, resource exhaustion, or
  unexpected network egress. Set the duration with the change authority; the
  hardening scope calls for a 24-hour egress soak.

## 7. Known product limits to account for

The repository status and roadmap identify several capabilities that affect
real-world operations: inventory-driven SNMP polling and credential profiles,
connectivity preflight, full audit controls (person-level attribution,
before/after changes, SIEM forwarding, retention and tamper protection),
identity-provider login, notification delivery, SNMP trap/syslog ingestion,
and production scale validation are incomplete or not implemented. Do not
assume the presence of these features based on the UI or architecture diagrams.
Decide whether each is required for the deployment and implement/validate it
before relying on it.

## 8. Operations after rollout

Document who owns device onboarding, Telegraf config review, credential
rotation, alarm tuning, database capacity, backups, restore, image updates, and
security incidents. Review service health and ingestion gaps daily during the
pilot, then set a routine appropriate to the operational policy. Track every
configuration change, retain approved versions, and test upgrades and rollback
in a non-production environment before deploying them to the monitored network.
