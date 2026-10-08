# Running NetPulse

## Prerequisites

- Docker Engine with Docker Compose v2
- Bun 1.3 or newer only when developing the UI outside Docker
- Go 1.25 or newer only when developing the middleware outside Docker

The normal deployment path uses Docker and does not require Bun or Go on the
host.

## First-time secure setup

Copy the environment template:

```bash
cp .env.example .env
```

Generate three different random values:

```bash
openssl rand -base64 48
openssl rand -base64 48
openssl rand -base64 48
```

Put them in `.env` as `NMS_BOOTSTRAP_TOKEN`, `DB_PASSWORD`, and
`GRAFANA_ADMIN_PASSWORD`. Do not reuse values. The `.env` file is ignored by
Git and must not be committed.

The default bind address is `127.0.0.1`. Keep it that way for local operation.
For network access, place the application behind an approved TLS and identity
gateway before changing `NMS_BIND_ADDRESS`.

Review [the data boundary and outbound traffic](data-boundary.md) before
deployment. NMS stores operational data locally and disables its identified
optional phone-home features, while still sending SNMP and ICMP to configured
devices. Enforce device-only egress with customer firewall rules.

## Start the complete stack

From the repository root:

```bash
docker compose up --build -d
docker compose ps
```

After changing UI or API source, rebuild and replace the running containers with:

```bash
docker compose up -d --build --force-recreate nms-ui nms-middleware
```

`docker compose build` alone only creates updated images; it does not replace
containers already serving the old images. This command preserves the database
volume and historical metrics.

Wait until TimescaleDB is healthy and the remaining services show as running.
The initial image build can take several minutes.

Open:

- Command center: `http://127.0.0.1:3000`
- Engineering Grafana: `http://127.0.0.1:3001`

Sign in to the command center using the value of `NMS_BOOTSTRAP_TOKEN`. The
credential is exchanged for an eight-hour HttpOnly session and is not retained
by the React application.

To display the login credential locally from the repository root:

```bash
sed -n 's/^NMS_BOOTSTRAP_TOKEN=//p' .env
```

The live value belongs only in the Git-ignored `.env` or an approved secrets
manager. It must never be copied into tracked documentation, source code,
screenshots, tickets, or chat messages.

To rotate it, generate a replacement and update `NMS_BOOTSTRAP_TOKEN` in
`.env`, then recreate the middleware container:

```bash
openssl rand -hex 32
docker compose up -d --force-recreate nms-middleware
```

All existing command-center sessions are invalidated when the middleware is
recreated.

## Verify ingestion

Follow the collector and middleware logs:

```bash
docker compose logs --tail=100 telegraf nms-middleware
docker compose logs -f telegraf nms-middleware
```

Verify that the database is receiving records:

```bash
docker compose exec timescaledb psql -U postgres -d nms_db -c \
  "SELECT COUNT(*) AS health_samples FROM device_health_metrics;"

docker compose exec timescaledb psql -U postgres -d nms_db -c \
  "SELECT COUNT(*) AS interface_samples FROM interface_performance_metrics;"
```

The command center refreshes every 15 seconds. Allow at least one polling cycle
after all containers have started.

The topology collector walks the standard LLDP-MIB remote systems table for the
SNMP targets configured in `telegraf.conf`. Devices must have LLDP enabled and
the configured SNMP credentials must be allowed to read LLDP-MIB. The bundled
mock `snmpd` agents do not implement LLDP, so an empty topology in the mock lab
is expected; the application does not invent neighbor links. Newly enrolled
devices must also be added to Telegraf's SNMP target configuration until
inventory-driven polling profiles are implemented.

The Fleet inventory page supports adding, editing, and deleting devices. New
devices must use a model in the middleware's supported hardware catalog.
SNMPv2c community values are write-only in the UI: an existing value is never
returned to the browser, and leaving the edit field blank preserves it. The
credential-profile workflow and connectivity preflight described in
`product-future-scope.md` are not implemented yet.

## Optional demonstration data

The mock devices generate live ICMP and SNMP telemetry. To additionally load
the repeatable demonstration dataset:

```bash
docker compose exec -T timescaledb \
  psql -U postgres -d nms_db -v ON_ERROR_STOP=1 -f /dev/stdin \
  < db/sample_data.sql
```

Do not load sample data in a production database.

## UI development with Bun

Keep the Docker database and middleware running. The middleware is intentionally
not published to the host in the hardened Compose configuration, so expose it
temporarily using a development-only override:

```yaml
# compose.dev.yaml
services:
  nms-middleware:
    ports:
      - "127.0.0.1:8080:8080"
```

Start the backend with the override:

```bash
docker compose -f docker-compose.yml -f compose.dev.yaml up -d timescaledb nms-middleware telegraf
```

Then start Vite:

```bash
cd ui
bun install --frozen-lockfile
bun run dev
```

Open `http://127.0.0.1:5173`. Vite proxies `/api` to
`http://127.0.0.1:8080`. Delete the temporary override when it is no longer
needed; never expose port 8080 on an untrusted interface.

## Build and test

```bash
go test ./...
go vet ./...

cd ui
bun run lint
bun run build
bun audit --production
```

Validate the resolved Compose file before deployment:

```bash
docker compose config --quiet
```

## Stop and restart

Stop containers while retaining database and Grafana volumes:

```bash
docker compose down
```

Start them again:

```bash
docker compose up -d
```

Do not add `-v` unless the intention is to permanently remove stored database
and Grafana data.

## Troubleshooting an empty dashboard

1. Confirm every container is running with `docker compose ps`.
2. Check `telegraf` logs for SNMP or PostgreSQL errors.
3. Check `nms-middleware` logs for migration, database, or ICMP errors.
4. Run the two database count queries above.
5. Verify the selected devices have `is_monitored = TRUE`.
6. Verify the browser is using port 3000, not the engineering Grafana port.
7. Restart only the affected service after correcting its configuration:

```bash
docker compose restart telegraf
docker compose restart nms-middleware
```

If the database volume was created using a different `DB_PASSWORD`, changing
the `.env` value alone will not change PostgreSQL's existing password. Restore
the original secret or rotate the database role password deliberately; do not
delete the volume merely to bypass the mismatch.

## Production boundary

The included loopback deployment is a hardened development baseline, not a
military or government authorization to operate. Before production, complete
the TLS, organizational SSO/MFA, SNMPv3, database mTLS, image pinning, audit,
testing, and accreditation work listed in
[`ui-security-baseline.md`](ui-security-baseline.md).
