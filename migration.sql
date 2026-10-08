-- Canonical, idempotent NMS schema recovery migration.
-- Usage: psql -U postgres -d nms_db -v ON_ERROR_STOP=1 -f migration.sql

CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

CREATE TABLE IF NOT EXISTS device_models (
    id SERIAL PRIMARY KEY,
    series TEXT NOT NULL,
    model_name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS devices (
    id SERIAL PRIMARY KEY,
    hostname VARCHAR(255) NOT NULL UNIQUE,
    ip_address INET NOT NULL UNIQUE,
    snmp_version VARCHAR(10) NOT NULL DEFAULT 'v2c',
    snmp_community VARCHAR(100) NOT NULL DEFAULT 'public',
    device_type VARCHAR(100) NOT NULL DEFAULT 'unknown_discovered',
    location VARCHAR(255),
    is_monitored BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS interfaces (
    id SERIAL PRIMARY KEY,
    device_id INT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    if_index INT NOT NULL,
    if_descr VARCHAR(255) NOT NULL,
    if_type INT,
    speed BIGINT,
    UNIQUE (device_id, if_index)
);

CREATE TABLE IF NOT EXISTS device_health_metrics (
    time TIMESTAMPTZ NOT NULL,
    device_id INT REFERENCES devices(id) ON DELETE CASCADE,
    icmp_status INT,
    icmp_rtt_ms DOUBLE PRECISION,
    icmp_packet_loss DOUBLE PRECISION,
    cpu_utilization DOUBLE PRECISION,
    memory_utilization DOUBLE PRECISION,
    ntp_skew_seconds DOUBLE PRECISION,
    host TEXT,
    url TEXT,
    result_code INT,
    packets_transmitted INT,
    packets_received INT,
    percent_packet_loss DOUBLE PRECISION,
    average_response_ms DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS interface_performance_metrics (
    time TIMESTAMPTZ NOT NULL,
    interface_id INT REFERENCES interfaces(id) ON DELETE CASCADE,
    if_oper_status INT,
    if_admin_status INT,
    rx_bytes_delta BIGINT,
    tx_bytes_delta BIGINT,
    rx_errors_delta BIGINT,
    tx_errors_delta BIGINT,
    rx_discards_delta BIGINT,
    tx_discards_delta BIGINT,
    optical_rx_dbm DOUBLE PRECISION,
    optical_tx_dbm DOUBLE PRECISION,
    agent_host TEXT,
    host TEXT,
    "sysName" TEXT,
    "ifIndex" BIGINT,
    "ifDescr" TEXT,
    "ifSpeed" BIGINT,
    "ifOperStatus" BIGINT,
    "ifAdminStatus" BIGINT,
    "ifInOctets" BIGINT,
    "ifOutOctets" BIGINT,
    "ifInErrors" BIGINT,
    "ifOutErrors" BIGINT,
    "ifInDiscards" BIGINT,
    "ifOutDiscards" BIGINT,
    target_device TEXT,
    target_port TEXT
);

ALTER TABLE interface_performance_metrics
    ADD COLUMN IF NOT EXISTS "ifIndex" BIGINT,
    ADD COLUMN IF NOT EXISTS "ifDescr" TEXT,
    ADD COLUMN IF NOT EXISTS "ifSpeed" BIGINT,
    ADD COLUMN IF NOT EXISTS "ifInOctets" BIGINT,
    ADD COLUMN IF NOT EXISTS "ifOutOctets" BIGINT,
    ADD COLUMN IF NOT EXISTS "ifInErrors" BIGINT,
    ADD COLUMN IF NOT EXISTS "ifOutErrors" BIGINT,
    ADD COLUMN IF NOT EXISTS "ifInDiscards" BIGINT,
    ADD COLUMN IF NOT EXISTS "ifOutDiscards" BIGINT;

CREATE TABLE IF NOT EXISTS snmp_device_health (
    time TIMESTAMPTZ NOT NULL,
    agent_host TEXT,
    host TEXT,
    "sysName" TEXT,
    cpu_idle DOUBLE PRECISION,
    cpu_load_1 DOUBLE PRECISION,
    mem_total_kb BIGINT,
    mem_available_kb BIGINT
);

ALTER TABLE snmp_device_health
    ADD COLUMN IF NOT EXISTS cpu_idle DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS cpu_load_1 DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS mem_total_kb BIGINT,
    ADD COLUMN IF NOT EXISTS mem_available_kb BIGINT;

CREATE TABLE IF NOT EXISTS network_events (
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    device_id INT REFERENCES devices(id) ON DELETE CASCADE,
    alarm_id INT NOT NULL,
    -- alarm_id catalog:
    --   1=device_down  2=link_down  3=high_cpu  4=admin_oper_mismatch
    --   5=high_memory  6=snmp_failure
    event_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('INFO', 'WARNING', 'CRITICAL')),
    message TEXT,
    resolved BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_at TIMESTAMPTZ
);

-- Application access and administration activity. Store request metadata only;
-- never persist request bodies, credentials, query strings, or client addresses.
CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure')),
    method TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS network_topology (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source_device TEXT NOT NULL,
    source_port TEXT,
    target_device TEXT NOT NULL,
    target_port TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_device, source_port, target_device, target_port)
);

-- Raw Telegraf tables are pre-created so discovery/topology workers can query
-- safely before the first successful SNMP poll. Telegraf may add more columns.
CREATE TABLE IF NOT EXISTS snmp (
    time TIMESTAMPTZ NOT NULL,
    agent_host TEXT,
    host TEXT,
    "sysName" TEXT
);

CREATE TABLE IF NOT EXISTS snmp_interface (
    time TIMESTAMPTZ NOT NULL,
    agent_host TEXT,
    host TEXT,
    "sysName" TEXT,
    "ifIndex" BIGINT,
    "ifDescr" TEXT,
    "ifOperStatus" BIGINT,
    "ifAdminStatus" BIGINT
);

CREATE TABLE IF NOT EXISTS snmp_lldp_topology (
    time TIMESTAMPTZ NOT NULL,
    agent_host TEXT,
    host TEXT,
    "index" TEXT,
    lldp_local_port_num INTEGER,
    lldp_rem_chassis_id TEXT,
    lldp_rem_sys_name TEXT,
    lldp_rem_port_id TEXT,
    target_device TEXT,
    target_port TEXT
);

SELECT create_hypertable('device_health_metrics', 'time', if_not_exists => TRUE);
SELECT create_hypertable('interface_performance_metrics', 'time', if_not_exists => TRUE);
SELECT create_hypertable('snmp_device_health', 'time', if_not_exists => TRUE);
SELECT create_hypertable('network_events', 'time', if_not_exists => TRUE);
SELECT create_hypertable('snmp', 'time', if_not_exists => TRUE);
SELECT create_hypertable('snmp_interface', 'time', if_not_exists => TRUE);
SELECT create_hypertable('snmp_lldp_topology', 'time', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_device_health_device_time
    ON device_health_metrics (device_id, time DESC);
CREATE INDEX IF NOT EXISTS idx_interface_perf_interface_time
    ON interface_performance_metrics (interface_id, time DESC);
CREATE INDEX IF NOT EXISTS idx_snmp_health_agent_time
    ON snmp_device_health (agent_host, time DESC);
CREATE INDEX IF NOT EXISTS idx_network_events_open
    ON network_events (resolved, severity, time DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_occurred_at
    ON audit_events (occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_topology_updated
    ON network_topology (updated_at DESC);

INSERT INTO devices (hostname, ip_address, device_type, location) VALUES
    ('mock-switch-01', '192.168.10.10', 'Switch', 'Virtual Rack 1'),
    ('mock-switch-02', '192.168.10.11', 'Switch', 'Virtual Rack 1'),
    ('mock-router-01', '192.168.10.19', 'Router', 'Virtual Rack 2'),
    ('mock-router-02', '192.168.10.20', 'Router', 'Virtual Rack 2'),
    ('mock-core-01',   '192.168.10.30', 'Core',   'Virtual Core')
ON CONFLICT DO NOTHING;
