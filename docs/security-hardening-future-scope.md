# Security Hardening — Future Scope

## Objective

This is a backlog of additional hardening for a customer deployment. It is not
a description of the current controls. The current Compose stack requires
secrets from `.env`, binds the UI and Grafana to loopback by default, does not
publish the middleware API port, and applies read-only filesystems and dropped
capabilities to the middleware and UI. The database connection uses
`sslmode=disable` on the private Compose network, image tags are not all pinned,
and network/firewall policy is external to Compose. See
[Data boundary](data-boundary.md) and [architecture.md](architecture.md) for
current behavior.

## Priority 1: Network exposure and identity

- Keep the UI/Grafana host binds on loopback unless access is deliberately
  provided through a protected management interface. `NMS_BIND_ADDRESS` changes
  both published addresses.
- Put the UI behind approved TLS and an identity gateway before allowing remote
  operator access; the application currently has one shared bootstrap identity.
- Keep middleware port 8080 internal. Compose exposes it to attached containers
  but does not publish it on the host.
- Enforce customer firewall egress rules. The device-facing Compose bridge is
  not an egress firewall; permit only required monitoring traffic to approved
  device addresses and deny unrelated external egress.
- Use TLS for database traffic if the database connection leaves the private
  local Compose network; the current DSN explicitly disables TLS.

## Priority 2: Credentials and session management

- Move DB, bootstrap, SNMP, and Grafana secrets from a local `.env` workflow to
  the deployment's approved secret manager or Docker secrets.
- Rotate all development credentials before a customer pilot. Use per-device
  SNMPv3 credentials where supported; current Telegraf configuration uses a
  shared SNMPv2c community.
- Replace shared bootstrap-token identity with named users, role-based
  authorization, and an approved identity provider if required.
- Configure a persistent, protected Grafana `secret_key` and verify secure
  cookies under the actual TLS proxy.
- Continue to disable Grafana sign-up and anonymous access; verify the effective
  settings in the deployed version.

## Priority 3: Outbound requests and telemetry

- App UI assets and API calls are same-origin; Grafana reporting, update checks,
  Gravatar, news feeds, public dashboards, plugin administration, suggested
  plugin preinstallation, and runtime plugin installation are disabled in the
  current Compose configuration.
- Verify the effective egress boundary with firewall logs or packet capture
  during a soak test after deploying customer-specific firewall rules.

## Priority 4: Supply chain and least privilege

- Pin Grafana, TimescaleDB, Telegraf, and base image versions or digests.
- Remove the unsigned-plugin allowlist if it is not required.
- Extend and verify read-only filesystems and reduced Linux capabilities for
  services beyond the UI and middleware where compatible.
- Add image vulnerability and software-bill-of-materials scanning to CI.
- Use a read-only PostgreSQL account for Grafana instead of the database owner.
- Restrict each Grafana organization and user to only the required data.

## Acceptance criteria

- No unexpected outbound DNS, HTTP, or HTTPS traffic during a 24-hour test.
- Only approved management hosts can reach Grafana or the middleware API.
- Grafana cannot modify schema or application data through its datasource.
- No default or repository-stored production credentials remain.
- Images and plugins are reproducible from pinned, reviewed artifacts.
- Backup and restore tests cover Grafana metadata, TimescaleDB, and provisioned
  dashboard source files.
