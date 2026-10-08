# Security Hardening — Future Scope

## Objective

Prepare the NMS stack for confidential production data and operation in an
isolated or air-gapped network. The application disables its identified
optional phone-home features, but the Compose bridge still permits egress from
services that must reach monitored devices. See [Data boundary](data-boundary.md)
for the current traffic model and the host controls required for production.

## Priority 1: Network exposure and access control

- Bind Grafana to `127.0.0.1:3000` or a dedicated management VLAN address
  instead of all host interfaces.
- Remove the public middleware `8080` binding unless it is operationally
  required. If it is required, add authentication and authorization.
- Replace the middleware's wildcard CORS policy with the exact Grafana origin.
- Enforce host or network firewall egress rules. Permit only the configured
  monitored-device addresses and protocols from Telegraf and the middleware;
  their device-facing Compose bridge is not an egress firewall. Also deny
  external egress from the UI bridge. The database and Grafana are on an
  internal Docker network.
- Put Grafana behind an HTTPS reverse proxy for any non-local access.

## Priority 2: Credentials and sessions

- Remove fallback admin and database passwords from Compose configuration.
- Load secrets from Docker secrets or root-readable files outside Git.
- Rotate all development credentials before production deployment.
- Set a persistent, protected Grafana `secret_key`.
- Enable secure cookies under HTTPS and evaluate strict SameSite cookies.
- Disable user sign-up and anonymous access explicitly.

## Priority 3: Outbound requests and telemetry

- App UI assets and API calls are same-origin; Grafana reporting, update checks,
  Gravatar, news feeds, public dashboards, plugin administration, suggested
  plugin preinstallation, and runtime plugin installation are disabled in the
  current Compose configuration.
- Verify the effective egress boundary with firewall logs or packet capture
  during a soak test after deploying the customer-specific firewall rules.

## Priority 4: Supply-chain and least privilege

- Pin Grafana, TimescaleDB, Telegraf, and plugin versions or image digests.
- Remove the unsigned-plugin allowlist if it is not required.
- Run containers with read-only filesystems and reduced Linux capabilities
  where compatible.
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
