# Custom Enterprise Network Management System (NMS)

This repository contains the core pipeline configurations and custom backend applications for an enterprise-grade Network Management System (NMS). The system leverages **Telegraf** for metric data ingestion, **TimescaleDB** for unified relational inventory and high-velocity time-series processing, and a concurrent **Custom Go Middleware Service** to execute high-speed ICMP sweeps and handle advanced alert routing.

---

## 🏗️ High-Level Architecture (HLD)

The NMS is structured as a decoupled, multi-tier data pipeline designed to prevent blocking bottlenecks, guarantee processing throughput, and safely scale to thousands of network managed elements.

```text
   ┌────────────────────────────────────────────────────────┐
   │             Network Managed Elements Network           │
   │     (Core/Access Switches, Edge Routers, SFPs)        │
   └──────────────┬──────────────────────────┬──────────────┘
                  │                          │
          (SNMP Polling)               (ICMP Pings &
                │                       SNMP Traps)
                ▼                            ▼
   ┌──────────────────────────┐  ┌──────────────────────────┐
   │     Telegraf Engine      │  │   Custom Go Middleware   │
   │    (Collector Layer)     │  │  (High-Throughput Core)  │
   └────────────┬─────────────┘  └────────────┬─────────────┘
                │                             │
           (SQL Inserts)                 (SQL Inserts)
                └──────────────┬──────────────┘
                               ▼
   ┌────────────────────────────────────────────────────────┐
   │                      TimescaleDB                       │
   │  ┌────────────────────────┐  ┌──────────────────────┐  │
   │  │ Relational Directory   │  │ Time-Series Metrics  │  │
   │  │ (Inventory, Devices)   │  │ (Hypertables, Chunks)│  │
   │  └────────────────────────┘  └──────────────────────┘  │
   └───────────────────────────┬────────────────────────────┘
                               │
                         (SQL Queries)
                               ▼
   ┌────────────────────────────────────────────────────────┐
   │                 Grafana Visualization                  │
   │        (Live Dashboards & Threshold Alerting)          │
   └────────────────────────────────────────────────────────┘
```

### Component Breakdown
1. **Collector Ingestion Layer (Telegraf):** An optimized metric collector agent tasked with pulling standard tabular interface indicators every 30 seconds via SNMPv2c.
2. **Custom Concurrency Layer (Go Middleware):** A highly concurrent, stateless Go binary daemon that runs parallel goroutines to sweep device nodes for reachability (every 15 seconds) and handles backend database interaction layers.
3. **Unified Storage Layer (TimescaleDB):** A specialized PostgreSQL instance leveraging hypertables partitioned by time vectors to seamlessly ingest hundreds of thousands of entries per second while maintaining standard relational referential integrity.
4. **Visualization Layer (Grafana):** An analytics visualization platform that continuously polls TimescaleDB using native SQL queries to provide real-time NOC dashboards and fire threshold-based notifications.

---

## 🛠️ Low-Level Design (LLD)

### Go Middleware Package Architecture
The custom Go daemon uses a structured, concurrent architecture to isolate processing domains and prevent blocking conditions:



```text
nms-middleware/
├── main.go             # Application initialization, environment parsing, and system context orchestration
├── db/
│   └── database.go     # Thread-safe pgxpool implementation managing active connection states
└── ping/
    └── pinger.go       # Goroutine pool executing native OS ping execution and stripping CIDR network masks
```


### Core Execution Flow Mapping


```text
[StartSweeper] ──► Ticker (15s) ──► [fetchMonitoredDevices] ──► SQL: HOST(ip_address)
│
(Returns Pure IPs)
▼
[executeParallelSweep]
│
(Spawns Go Routines)
▼
[pingTarget]
│
┌────────────────────┴────────────────────┐
▼                                         ▼
Command Execution                        Context Timeout Watch
(exec.CommandContext)                         (context.WithTimeout)
│                                         │
└────────────────────┬────────────────────┘
▼
[savePingResult]
│
▼
SQL: INSERT INTO hypertable
```



---

## 🗄️ Relational & Time-Series Database Schema

The complete structural SQL build script utilized to model the inventory nodes and generate the corresponding TimescaleDB hypertables:

```sql
-- Enable TimescaleDB Extension
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- 1. RELATIONAL STRUCTURES: Core Inventory Management
CREATE TABLE devices (
    id SERIAL PRIMARY KEY,
    hostname VARCHAR(255) NOT NULL UNIQUE,
    ip_address INET NOT NULL UNIQUE,
    snmp_version VARCHAR(10) DEFAULT 'v2c',
    snmp_community VARCHAR(100) DEFAULT 'public',
    device_type VARCHAR(50) CHECK (device_type IN ('Switch', 'Router', 'Firewall')),
    location VARCHAR(255),
    is_monitored BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE interfaces (
    id SERIAL PRIMARY KEY,
    device_id INT REFERENCES devices(id) ON DELETE CASCADE,
    if_index INT NOT NULL,
    if_descr VARCHAR(255) NOT NULL,
    if_type INT,
    speed BIGINT,
    UNIQUE(device_id, if_index)
);

-- 2. TIME-SERIES STRUCTURES: Metric Accumulation Hypertables
CREATE TABLE device_health_metrics (
    time TIMESTAMPTZ NOT NULL,
    device_id INT NOT NULL,
    icmp_status INT DEFAULT 1,             -- 1 = Up, 0 = Down
    icmp_rtt_ms DOUBLE PRECISION,
    icmp_packet_loss DOUBLE PRECISION,
    cpu_utilization DOUBLE PRECISION,       -- Captured via SNMP Polling
    memory_utilization DOUBLE PRECISION,    -- Captured via SNMP Polling
    ntp_skew_seconds DOUBLE PRECISION
);

CREATE TABLE interface_performance_metrics (
    time TIMESTAMPTZ NOT NULL,
    interface_id INT NOT NULL,
    if_oper_status INT NOT NULL,           -- 1 = Up, 2 = Down, 3 = Testing
    if_admin_status INT NOT NULL,
    rx_bytes_delta BIGINT,
    tx_bytes_delta BIGINT,
    rx_errors_delta INT,
    tx_errors_delta INT
);

CREATE TABLE network_events (
    time TIMESTAMPTZ NOT NULL,
    device_id INT REFERENCES devices(id),
    alarm_id INT NOT NULL,
    severity VARCHAR(20) CHECK (severity IN ('INFO', 'WARNING', 'CRITICAL')),
    message TEXT,
    resolved BOOLEAN DEFAULT FALSE
);

-- 3. TIMESCALEDB HYPERTABLE TRANSLATION
SELECT create_hypertable('device_health_metrics', 'time', migration_check => false);
SELECT create_hypertable('interface_performance_metrics', 'time', migration_check => false);
SELECT create_hypertable('network_events', 'time', migration_check => false);

-- 4. PERFORMANCE TUNING INDEXES
CREATE INDEX idx_device_metrics_lookup ON device_health_metrics (device_id, time DESC);
CREATE INDEX idx_interface_metrics_lookup ON interface_performance_metrics (interface_id, time DESC);

-- 5. SEED DATA (Virtual Network Verification Elements)
INSERT INTO devices (hostname, ip_address, device_type, location) VALUES
('mock-switch-01', '192.168.10.10', 'Switch', 'Virtual-Rack-1'),
('mock-router-02', '192.168.10.20', 'Router', 'Virtual-Rack-2')
ON CONFLICT DO NOTHING;
```



---

## 📈 Covered System Alarms Blueprint

The system components actively capture raw metrics that formulate the functional signature for **5 key network alarms**:

1. **Alarm 1 (Device Down):** Triggered when Go's sweeper sets `icmp_status = 0` and `icmp_packet_loss = 100.0`.
2. **Alarm 2 (Interface Status Link Down):** Tracked by Telegraf SNMP polling inside `snmp_interface` when `ifOperStatus = 2`.
3. **Alarm 3 (High CPU / Memory Exhaustion):** Extracted via Telegraf collecting OID `1.3.6.1.4.1.2021.11.11.0` and `1.3.6.1.4.1.2021.4.11.0`.
4. **Alarm 4 (State Mismatch):** Generated via SQL checking for admin/oper misalignment (`ifAdminStatus = 1` AND `ifOperStatus = 2`).
5. **Alarm 5 (Device Performance Degradation):** Checked dynamically via the continuous time-series logging of `icmp_rtt_ms`.

---

## 🚀 Deployment & Operational Verification

### Complete Environment Initialization

Spin up the coordinated core pipeline infrastructure stack using Docker Compose:

```bash
# Erase old volumes and bring up all containers in a clean detached state
docker compose down -v
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

The running mock devices also expose standard SNMP system, UCD resource, and
IF-MIB interface data. Telegraf polls all five mock devices exactly as it would
poll production network equipment; this is the preferred end-to-end test path.



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
