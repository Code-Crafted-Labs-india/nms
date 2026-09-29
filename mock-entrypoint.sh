#!/bin/bash
# NMS Mock Device Entrypoint
# Configures snmpd with per-device UCD metric simulation and generates
# realistic inter-container traffic to populate interface counters.

DEVICE_TYPE="${DEVICE_TYPE:-switch}"
BASE_CPU_IDLE="${BASE_CPU_IDLE:-80}"
MEM_TOTAL_KB="${MEM_TOTAL_KB:-8388608}"
MEM_USED_PCT="${MEM_USED_PCT:-35}"
PEER_IPS="${PEER_IPS:-}"
PING_INTERVAL="${PING_INTERVAL:-0.1}"
PING_SIZE="${PING_SIZE:-1024}"

# ── 1. Configure snmpd ─────────────────────────────────────────────────────────
# Use pass_persist for the ucdavis subtree so CPU/memory OIDs return
# per-device simulated values instead of the shared Colima VM counters.
cat > /etc/snmp/snmpd.conf << 'EOF'
view all included .1
rocommunity public default -V all
pass_persist .1.3.6.1.4.1.99999 /usr/local/sbin/ucd-sim
EOF

# ── 2. Add virtual dummy interfaces to simulate device ports ──────────────────
# Requires NET_ADMIN capability (set in docker-compose.yml).
# These interfaces appear in snmpd's ifTable, giving each device multiple
# ports like a real switch or router.
IFACE_COUNT=4
case "$DEVICE_TYPE" in
  core)    IFACE_COUNT=8 ;;
  router)  IFACE_COUNT=6 ;;
  switch)  IFACE_COUNT=4 ;;
esac

for i in $(seq 1 "$IFACE_COUNT"); do
    ip link add "eth${i}" type dummy 2>/dev/null && \
    ip link set "eth${i}" up 2>/dev/null || true
done
echo "[mock-${DEVICE_TYPE}] added ${IFACE_COUNT} virtual interfaces"

# ── 3. Start snmpd with per-device env exported for pass_persist ──────────────
export BASE_CPU_IDLE MEM_TOTAL_KB MEM_USED_PCT
snmpd -f -Lo -c /etc/snmp/snmpd.conf 0.0.0.0:161 &
SNMPD_PID=$!
echo "[mock-${DEVICE_TYPE}] snmpd started (cpu_idle_base=${BASE_CPU_IDLE}%, mem=${MEM_TOTAL_KB}KB, used=${MEM_USED_PCT}%)"

# ── 4. Generate continuous inter-device traffic ────────────────────────────────
# Sleep briefly to allow the network namespace and routing to converge,
# then start a ping flood towards each peer IP.  Different device types
# use different packet sizes / rates to simulate realistic utilization:
#   core:   high rate (heavy backbone traffic)
#   router: medium rate (WAN uplink simulation)
#   switch: lower rate (access-layer traffic)
sleep 5

if [ -n "$PEER_IPS" ]; then
    for ip in $PEER_IPS; do
        (
            while true; do
                ping -q -s "$PING_SIZE" -i "$PING_INTERVAL" -c 300 "$ip" &>/dev/null
                # Short random pause between bursts to create traffic variation
                sleep $((RANDOM % 4 + 1))
            done
        ) &
    done
    echo "[mock-${DEVICE_TYPE}] traffic generation active → peers: ${PEER_IPS}"
fi

wait "$SNMPD_PID"
