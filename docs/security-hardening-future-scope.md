# Security Hardening — Future Scope

## Objective

Prepare the NMS stack for confidential production data and operation in an
isolated or air-gapped network. The current deployment is suitable for local
development and demonstrations, but it is not yet an isolation boundary.

## Priority 1: Network exposure and access control

- Bind Grafana to `127.0.0.1:3000` or a dedicated management VLAN address
  instead of all host interfaces.
- Remove the public middleware `8080` binding unless it is operationally
  required. If it is required, add authentication and authorization.
- Replace the middleware's wildcard CORS policy with the exact Grafana origin.
- Deny container egress by default and allow only explicitly approved internal
  destinations such as TimescaleDB, SMTP, and alert receivers.
- Put Grafana behind an HTTPS reverse proxy for any non-local access.

## Priority 2: Credentials and sessions

- Remove fallback admin and database passwords from Compose configuration.
- Load secrets from Docker secrets or root-readable files outside Git.
- Rotate all development credentials before production deployment.
- Set a persistent, protected Grafana `secret_key`.
- Enable secure cookies under HTTPS and evaluate strict SameSite cookies.
- Disable user sign-up and anonymous access explicitly.

## Priority 3: Outbound requests and telemetry

- Disable anonymous usage reporting.
- Disable Grafana and plugin update checks.
- Disable Gravatar, news feeds, public dashboards, and external snapshots.
- Disable plugin administration from the web UI.
- Preinstall required plugins in the image and remove plugin downloads from the
  runtime entrypoint.
- Verify isolation with firewall logs or packet capture during a soak test.

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

