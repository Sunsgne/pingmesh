#!/usr/bin/env bash
# 提高 ICMP echo 回复配额, 避免全网状 15pps 互 ping 时内核限速造成「假丢包」。
# 不降低探测包数/频率, 只放开对端回包能力。
set -euo pipefail

SYSCTL_FILE=/etc/sysctl.d/99-pingmesh-icmp.conf
cat >"$SYSCTL_FILE" <<'EOF'
# ZENLENET PingMesh: allow higher ICMP echo reply rate under full-mesh probing
net.ipv4.icmp_msgs_per_sec = 10000
net.ipv4.icmp_msgs_burst = 500
EOF

sysctl -p "$SYSCTL_FILE" >/dev/null
echo "applied $SYSCTL_FILE"
sysctl net.ipv4.icmp_msgs_per_sec net.ipv4.icmp_msgs_burst
