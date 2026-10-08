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
    tar czf /root/pingmesh-pre-uninstall-$(date +%F-%H%M).tgz /opt/pingmesh-docker/data /opt/pingmesh/db /opt/pingmesh/conf 2>/dev/null || true
    rm -rf /opt/pingmesh /opt/pingmesh-docker
    pkill -f pingmesh 2>/dev/null || true
    echo cleaned
  ' && info "  ${name} 已拆除" || info "  ${name} 拆除不完整"
}

# 控制节点 + agents.list 里的全部 Agent(host port name)
NODES=("${MASTER_PUBLIC} 22 primary" "${BACKUP_PUBLIC} 22 backup")
while read -r h p n _; do NODES+=("$h $p $n"); done < <(grep -vE '^\s*(#|$)' "${SCRIPT_DIR}/agents.list")

for entry in "${NODES[@]}"; do
  read -r host port name <<< "$entry"
  uninstall_node "$host" "$port" "$name"
  sleep 3
done
info "全部拆除完成"
