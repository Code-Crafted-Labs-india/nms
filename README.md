# Network Management System (NMS)

This repository contains a single-site NMS prototype: a Go API and background
workers, a Telegraf collector, a TimescaleDB/PostgreSQL database, and a React
operator interface. The checked-in Compose configuration and Telegraf targets
are built around a simulated network. The implementation and production gaps
are documented explicitly; this repository does not claim scale or production
readiness without a deployment-specific pilot.

## Architecture and implementation guide

[`docs/architecture.md`](docs/architecture.md) is the code-aligned guide to
startup, service/network boundaries, polling and data flow, database tables,
alarm rules, API/authentication, UI requests, package responsibilities, known
gaps, and the files to change when implementing a feature. It describes current
behavior, not a target design.

The shorter operational workflow is in [`docs/running.md`](docs/running.md).

## 🚀 Deployment & Operational Verification

For complete setup, development, verification, shutdown, and troubleshooting
instructions, see [`docs/running.md`](docs/running.md).

For a real-device pilot and production-readiness checklist, see the
[`real-world deployment guide`](docs/real-world-deployment.md). The checked-in
Compose and Telegraf configurations are for the simulated demo environment and
must be adapted and security-reviewed before polling real equipment.

The PRL client-specification gap analysis and staged delivery plan are in
[`docs/client-requirements-gap-plan.md`](docs/client-requirements-gap-plan.md).
The client-facing response draft, indicative timeline, assumptions, and
internal delivery checklist are in
[`docs/client-response-and-delivery-estimate.md`](docs/client-response-and-delivery-estimate.md).

The prioritized operator-product roadmap derived from the supplied NMS
references is documented in
[`docs/product-future-scope.md`](docs/product-future-scope.md).

### Secure React command center

The operator UI is a static React/TypeScript application built with Bun, Vite,
and Tailwind CSS. It is available at `http://127.0.0.1:3000` by default;
Grafana remains engineering-only at `http://127.0.0.1:3001`.

Create local secrets before starting the stack:

```bash
cp .env.example .env
# Replace every placeholder. Generate the operator token with:
openssl rand -base64 48
docker compose up --build -d
```

Enter the `NMS_BOOTSTRAP_TOKEN` value stored in the Git-ignored `.env` at the
command-center login. Retrieval and rotation instructions are in
[`docs/running.md`](docs/running.md). For any networked deployment, terminate
approved TLS in front of the UI and complete the controls in
`docs/ui-security-baseline.md`; the loopback HTTP binding is development only.

### Complete Environment Initialization

Spin up the coordinated core pipeline infrastructure stack using Docker Compose:

```bash
# Bring up the stack without deleting the database volume or historical metrics
docker compose up --build -d
```

The Grafana image is built locally with the NMS logo in place of Grafana's
sidebar, sign-in, and browser icon assets. To use a different logo, replace
`grafana/branding/nms-logo.svg`, regenerate the two PNG files in that folder,
and rebuild the `grafana` service.

The TimescaleDB container applies `migration.sql` automatically when a new,
empty database volume is initialized. Grafana dashboards are stored as JSON
under `grafana/provisioning/dashboards/json`, so they are recreated from source
even if the Grafana data volume is replaced.

### Load dashboard sample data

For development or demonstrations, load the repeatable sample dataset after
the database is healthy:

```bash
docker compose exec -T timescaledb \
  psql -U postgres -d nms_db -v ON_ERROR_STOP=1 -f /dev/stdin \
  < db/sample_data.sql
```

The sample seed is not applied automatically and must not be run against a
production database. Production isolation and security work is tracked in
`docs/security-hardening-future-scope.md`.

The running mock devices expose SNMP and ICMP for the demo configuration.
Telegraf polls the addresses in `telegraf.conf`; this validates the demo
pipeline, not vendor compatibility or real-network readiness. Follow
[`docs/real-world-deployment.md`](docs/real-world-deployment.md) for a real
device pilot.



### Streaming the Real-time Event Pipeline Logs

To verify that the Go application loop and the database integration layer are writing transactions correctly, monitor the container stream output:

```bash
docker compose logs -f nms-middleware
```



### Manual Metric Inspection via Interactive Shell

Log straight into the relational engine to assert that raw metrics are correctly writing down to disk:

```bash
# Connect directly to the underlying database engine console instance
docker exec -it nms-timescaledb psql -U postgres -d nms_db
```

```sql
-- Query the time-series hypertable to assert metric ingestion
SELECT time, device_id, icmp_status, icmp_rtt_ms 
FROM device_health_metrics 
ORDER BY time DESC 
LIMIT 4;
```
