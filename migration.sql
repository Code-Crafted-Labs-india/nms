-- migration.sql
-- Run this against nms_db to align TimescaleDB schemas with the Go middleware constraints.

-- 1. Align snmp_lldp_topology to support the topology engine expectations
-- Telegraf's outputs.postgresql plugin might not generate these columns fast enough or correctly 
-- if the fields are registered as 'tags' instead of 'fields', causing our Go SQL queries to panic.
ALTER TABLE snmp_lldp_topology 
ADD COLUMN IF NOT EXISTS target_device TEXT,
ADD COLUMN IF NOT EXISTS target_port TEXT;

-- Note on 'is_monitered' bug:
-- The error 'ERROR: column "is_monitered" does not exist' was actually a typo in the Go codebase 
-- (specifically in alert/evaluator.go). The database correctly expects 'is_monitored' (with an 'o').
-- The Go code has been patched directly to query 'is_monitored' rather than renaming the database column.
