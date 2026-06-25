#!/usr/bin/env bash
# PingMesh 全集群拆除脚本
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh

info()  { echo -e "\033[32m[uninstall]\033[0m $*"; }

ssh_run() {
  local host="$1" port="${2:-22}"; shift 2
  local i=0
  while (( i < 8 )); do
    if sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -p "$port" "root@${host}" "$@" 2>/dev/null; then
      return 0
    fi
    ((i++)); sleep 15
  done
  return 1
}

uninstall_node() {
  local host="$1" port="$2" name="$3"
  info "拆除 ${name} (${host}:${port})"
  ssh_run "$host" "$port" '
    systemctl stop pingmesh 2>/dev/null || true
    systemctl disable pingmesh 2>/dev/null || true
    rm -f /etc/systemd/system/pingmesh.service
    systemctl daemon-reload 2>/dev/null || true
    pkill -f /opt/pingmesh/pingmesh 2>/dev/null || true
  ' || { info "  SSH 失败, 跳过"; return 1; }
  ssh_run "$host" "$port" '
    if [ -d /opt/pingmesh-docker ]; then
      cd /opt/pingmesh-docker && docker compose down 2>/dev/null || true
    fi
    docker rm -f pingmesh pingmesh-agent pingmesh-nginx 2>/dev/null || true
    rm -rf /opt/pingmesh /opt/pingmesh-docker
    pkill -f pingmesh 2>/dev/null || true
    echo cleaned
  ' && info "  ${name} 已拆除" || info "  ${name} 拆除不完整"
}

NODES=(
  "43.229.152.50 22 sin1-sg2"
  "163.53.245.90 22 hkg1"
  "106.75.160.24 20001 can-xxg"
  "42.240.152.238 20001 can-hxy"
  "217.217.29.250 22 fra"
  "104.251.226.39 20001 hkg2"
  "163.53.245.136 20001 hkg3"
  "149.119.41.156 22 lax"
  "106.38.203.8 20001 pek"
  "61.172.165.219 20001 gds"
  "113.31.161.79 20001 sjhl"
  "109.244.32.190 20001 xtl"
  "149.51.125.226 20001 sin2-gs"
  "59.36.211.118 20001 szx"
  "192.169.120.12 22 tpe"
  "43.230.52.242 22 tyo-8"
  "61.172.165.219 20001 tyo-7"
)

for entry in "${NODES[@]}"; do
  read -r host port name <<< "$entry"
  uninstall_node "$host" "$port" "$name"
  sleep 3
done
info "全部拆除完成"
