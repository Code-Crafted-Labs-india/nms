-- Repeatable development/demo data for the NMS dashboards.
-- This script is intentionally NOT mounted into docker-entrypoint-initdb.d.
-- Run it manually against a development database only.

BEGIN;

INSERT INTO devices (hostname, ip_address, device_type, location, is_monitored)
VALUES
    ('mock-switch-01', '192.168.10.10', 'Switch', 'Virtual Rack 1', TRUE),
    ('mock-switch-02', '192.168.10.11', 'Switch', 'Virtual Rack 1', TRUE),
    ('mock-router-01', '192.168.10.19', 'Router', 'Virtual Rack 2', TRUE),
    ('mock-router-02', '192.168.10.20', 'Router', 'Virtual Rack 2', TRUE),
    ('mock-core-01',   '192.168.10.30', 'Core',   'Virtual Core',   TRUE)
ON CONFLICT (hostname) DO UPDATE SET
    ip_address = EXCLUDED.ip_address,
    device_type = EXCLUDED.device_type,
    location = EXCLUDED.location,
    is_monitored = EXCLUDED.is_monitored;

INSERT INTO interfaces (device_id, if_index, if_descr, if_type, speed)
SELECT d.id, v.if_index, v.if_descr, 6, v.speed
FROM devices d
CROSS JOIN (VALUES
    (1, 'GigabitEthernet0/1', 1000000000::BIGINT),
    (2, 'GigabitEthernet0/2', 1000000000::BIGINT),
    (49, 'TenGigabitEthernet0/49', 10000000000::BIGINT),
    (50, 'TenGigabitEthernet0/50', 10000000000::BIGINT)
) AS v(if_index, if_descr, speed)
WHERE d.hostname IN ('mock-switch-01', 'mock-switch-02', 'mock-router-01', 'mock-router-02', 'mock-core-01')
ON CONFLICT (device_id, if_index) DO UPDATE SET
    if_descr = EXCLUDED.if_descr,
    if_type = EXCLUDED.if_type,
    speed = EXCLUDED.speed;

-- Six hours of five-minute device telemetry. mock-router-02 is intentionally
-- down for the latest 45 minutes so outage panels and alert colors are visible.
WITH samples AS (
    SELECT
        d.id AS device_id,
        d.hostname,
        gs AS sample_time,
        EXTRACT(EPOCH FROM gs)::BIGINT AS tick
    FROM devices d
    CROSS JOIN generate_series(
        date_trunc('minute', NOW()) - INTERVAL '6 hours',
        date_trunc('minute', NOW()),
        INTERVAL '5 minutes'
    ) AS gs
    WHERE d.hostname LIKE 'mock-%'
), generated AS (
    SELECT
        sample_time AS time,
        device_id,
        CASE
            WHEN hostname = 'mock-router-02' AND sample_time >= date_trunc('minute', NOW()) - INTERVAL '45 minutes' THEN 0
            ELSE 1
        END AS icmp_status,
        CASE
            WHEN hostname = 'mock-router-02' AND sample_time >= date_trunc('minute', NOW()) - INTERVAL '45 minutes' THEN NULL
            ELSE ROUND((2.5 + device_id * 0.8 + ABS(SIN(tick / 420.0)) * 8)::NUMERIC, 2)::DOUBLE PRECISION
        END AS icmp_rtt_ms,
        CASE
            WHEN hostname = 'mock-router-02' AND sample_time >= date_trunc('minute', NOW()) - INTERVAL '45 minutes' THEN 100.0
            ELSE ROUND((ABS(SIN(tick / 900.0)) * 1.5)::NUMERIC, 2)::DOUBLE PRECISION
        END AS icmp_packet_loss,
        ROUND((28 + device_id * 3 + ABS(SIN(tick / 600.0)) * 35)::NUMERIC, 2)::DOUBLE PRECISION AS cpu_utilization,
        ROUND((35 + device_id * 2 + ABS(COS(tick / 720.0)) * 24)::NUMERIC, 2)::DOUBLE PRECISION AS memory_utilization,
        ROUND((SIN(tick / 1200.0) * 0.025)::NUMERIC, 4)::DOUBLE PRECISION AS ntp_skew_seconds
    FROM samples
)
INSERT INTO device_health_metrics (
    time, device_id, icmp_status, icmp_rtt_ms, icmp_packet_loss,
    cpu_utilization, memory_utilization, ntp_skew_seconds
)
SELECT
    g.time, g.device_id, g.icmp_status, g.icmp_rtt_ms,
    g.icmp_packet_loss, g.cpu_utilization, g.memory_utilization,
    g.ntp_skew_seconds
FROM generated g
WHERE NOT EXISTS (
    SELECT 1
    FROM device_health_metrics existing
    WHERE existing.device_id = g.device_id
      AND existing.time = g.time
);

-- Six hours of interface counters, errors, state, and optical readings.
WITH samples AS (
    SELECT
        i.id AS interface_id,
        i.if_index,
        d.hostname,
        gs AS sample_time,
        EXTRACT(EPOCH FROM gs)::BIGINT AS tick
    FROM interfaces i
    JOIN devices d ON d.id = i.device_id
    CROSS JOIN generate_series(
        date_trunc('minute', NOW()) - INTERVAL '6 hours',
        date_trunc('minute', NOW()),
        INTERVAL '5 minutes'
    ) AS gs
    WHERE d.hostname LIKE 'mock-%'
), generated AS (
    SELECT
        sample_time AS time,
        interface_id,
        CASE
            WHEN hostname = 'mock-switch-02' AND if_index = 2
             AND sample_time >= date_trunc('minute', NOW()) - INTERVAL '30 minutes' THEN 2
            ELSE 1
        END AS if_oper_status,
        1 AS if_admin_status,
        (2000000 + interface_id * 120000 + ABS(SIN(tick / 300.0)) * 8000000)::BIGINT AS rx_bytes_delta,
        (1500000 + interface_id * 90000 + ABS(COS(tick / 360.0)) * 6500000)::BIGINT AS tx_bytes_delta,
        CASE WHEN MOD(tick / 300 + interface_id, 17) = 0 THEN 4 ELSE 0 END::BIGINT AS rx_errors_delta,
        CASE WHEN MOD(tick / 300 + interface_id, 23) = 0 THEN 2 ELSE 0 END::BIGINT AS tx_errors_delta,
        CASE WHEN MOD(tick / 300 + interface_id, 29) = 0 THEN 3 ELSE 0 END::BIGINT AS rx_discards_delta,
        0::BIGINT AS tx_discards_delta,
        ROUND((-4.8 - ABS(SIN(tick / 800.0)) * 1.4 - if_index * 0.01)::NUMERIC, 2)::DOUBLE PRECISION AS optical_rx_dbm,
        ROUND((-2.1 - ABS(COS(tick / 850.0)) * 0.8 - if_index * 0.005)::NUMERIC, 2)::DOUBLE PRECISION AS optical_tx_dbm
    FROM samples
)
INSERT INTO interface_performance_metrics (
    time, interface_id, if_oper_status, if_admin_status,
    rx_bytes_delta, tx_bytes_delta, rx_errors_delta, tx_errors_delta,
    rx_discards_delta, tx_discards_delta, optical_rx_dbm, optical_tx_dbm
)
SELECT
    g.time, g.interface_id, g.if_oper_status, g.if_admin_status,
    g.rx_bytes_delta, g.tx_bytes_delta, g.rx_errors_delta,
    g.tx_errors_delta, g.rx_discards_delta, g.tx_discards_delta,
    g.optical_rx_dbm, g.optical_tx_dbm
FROM generated g
WHERE NOT EXISTS (
    SELECT 1
    FROM interface_performance_metrics existing
    WHERE existing.interface_id = g.interface_id
      AND existing.time = g.time
);

INSERT INTO network_topology (
    source_device, source_port, target_device, target_port, updated_at
)
VALUES
    ('mock-core-01',   'TenGigabitEthernet0/49', 'mock-router-01', 'TenGigabitEthernet0/49', NOW()),
    ('mock-core-01',   'TenGigabitEthernet0/50', 'mock-router-02', 'TenGigabitEthernet0/49', NOW()),
    ('mock-router-01', 'GigabitEthernet0/1',     'mock-switch-01', 'GigabitEthernet0/1',     NOW()),
    ('mock-router-02', 'GigabitEthernet0/1',     'mock-switch-02', 'GigabitEthernet0/1',     NOW()),
    ('mock-switch-01', 'GigabitEthernet0/2',     'mock-switch-02', 'GigabitEthernet0/2',     NOW())
ON CONFLICT (source_device, source_port, target_device, target_port)
DO UPDATE SET updated_at = EXCLUDED.updated_at;

INSERT INTO network_events (
    time, device_id, alarm_id, event_type, severity, message, resolved, resolved_at
)
SELECT
    NOW() - INTERVAL '40 minutes', d.id, 1, 'device_down', 'CRITICAL',
    'Sample: mock-router-02 has failed ICMP checks for the current test window.',
    FALSE, NULL
FROM devices d
WHERE d.hostname = 'mock-router-02'
  AND NOT EXISTS (
      SELECT 1 FROM network_events e
      WHERE e.device_id = d.id
        AND e.event_type = 'device_down'
        AND e.message LIKE 'Sample:%'
        AND e.resolved = FALSE
  );

INSERT INTO network_events (
    time, device_id, alarm_id, event_type, severity, message, resolved, resolved_at
)
SELECT
    NOW() - INTERVAL '25 minutes', d.id, 2, 'link_down', 'WARNING',
    'Sample: GigabitEthernet0/2 is operationally down while testing.',
    FALSE, NULL
FROM devices d
WHERE d.hostname = 'mock-switch-02'
  AND NOT EXISTS (
      SELECT 1 FROM network_events e
      WHERE e.device_id = d.id
        AND e.event_type = 'link_down'
        AND e.message LIKE 'Sample:%'
        AND e.resolved = FALSE
  );

INSERT INTO network_events (
    time, device_id, alarm_id, event_type, severity, message, resolved, resolved_at
)
SELECT
    NOW() - INTERVAL '3 hours', d.id, 3, 'high_cpu', 'CRITICAL',
    'Sample: CPU threshold exceeded and automatically recovered.',
    TRUE, NOW() - INTERVAL '2 hours 45 minutes'
FROM devices d
WHERE d.hostname = 'mock-core-01'
  AND NOT EXISTS (
      SELECT 1 FROM network_events e
      WHERE e.device_id = d.id
        AND e.event_type = 'high_cpu'
        AND e.message = 'Sample: CPU threshold exceeded and automatically recovered.'
  );

COMMIT;

