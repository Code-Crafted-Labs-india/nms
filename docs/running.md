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

## Start the complete stack

From the repository root:

```bash
docker compose up --build -d
docker compose ps
```

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
