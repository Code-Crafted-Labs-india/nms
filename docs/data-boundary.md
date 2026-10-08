# Data boundary and outbound traffic

NetPulse is designed to run inside the operator's environment. Inventory,
topology, metrics, alarms, and credentials are stored in the local deployment
and are not sent to a NetPulse-hosted service by the application.

## Application traffic

- The command-center UI loads bundled assets and calls the API on the same
  origin. Its Content Security Policy only permits same-origin connections.
- Grafana reads local TimescaleDB data. Its usage reporting, version and plugin
  update checks, Gravatar requests, news feed, public dashboards, plugin UI,
  suggested plugin preinstallation, and startup plugin download are disabled.
- The UI is attached to a dedicated bridge shared with the middleware and
  separate from the device-facing network. The UI itself has no external
  service integrations; its browser Content Security Policy only allows
  same-origin requests. TimescaleDB and Grafana share an internal Docker
  network with no route to external networks.
- NMS intentionally sends monitoring traffic to configured devices: Telegraf
  sends SNMP requests and the middleware sends ICMP probes. Replies return to
  the collector. Configure only customer-approved device addresses and
  credentials.
- Container image pulls and builds may contact configured container registries
  and package sources. They are deployment-time supply-chain traffic, not
  application telemetry. An air-gapped deployment must preload reviewed images
  and build dependencies.

## Deployment boundary

The device-facing Compose bridge is not an egress-deny policy. Telegraf and the
middleware need routes to monitored devices, so this repository cannot infer
which destinations are approved in each customer architecture. Apply host or
network firewall rules that allow only the configured monitoring targets and
protocols. The database and Grafana are isolated from external networks by
their internal Docker network. The UI is separated from the device-facing
bridge, but its host-published bridge can route outward by default; use host
firewall rules to allow its middleware API traffic and deny other egress. Keep
published UI and Grafana ports bound to loopback unless access is intentionally
mediated by the customer's TLS and identity gateway.

Database volumes, backups, application/container logs, and host-level network
logs remain under the customer's control. Protect and retain them according to
the customer's policy. Nginx access logging is disabled, and API request logs
omit source addresses; operational logs can still include device identifiers
or addresses when needed to diagnose monitoring failures.

These controls remove the identified application phone-home behavior, but
source configuration alone cannot guarantee that a deployed host has no data
egress. Enforce and verify the boundary with the customer's firewall and
deployment monitoring.
