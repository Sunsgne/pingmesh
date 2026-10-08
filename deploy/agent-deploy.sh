#!/usr/bin/env bash
# PingMesh Agent 轻量部署: 上传预编译二进制 + systemd
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh
deploy_require_join

INSTALL_DIR='/opt/pingmesh'
BINARY='/tmp/pingmesh-bin.gz'

info()  { echo -e "\033[32m[agent]\033[0m $*"; }
err()   { echo -e "\033[31m[agent]\033[0m $*"; }

ssh_run() {
  local host="$1" port="${2:-22}"; shift 2
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -p "$port" "root@${host}" "$@"
}

deploy_agent() {
  local host="$1" port="$2" name="$3" addr="$4"
  info "部署 Agent ${name} -> ${host}:${port} (内网 ${addr})"
  if ! ssh_run "$host" "$port" "echo ok" 2>/dev/null; then
    err "  SSH 连接失败, 跳过 ${name}"
    return 1
  fi
  sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no -P "$port" "$BINARY" "root@${host}:/tmp/pingmesh-bin.gz" 2>/dev/null || {
    ssh_run "$host" "$port" "mkdir -p ${INSTALL_DIR}"
    sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no -P "$port" "$BINARY" "root@${host}:/tmp/pingmesh-bin.gz"
  }
  ssh_run "$host" "$port" "
    set -e
    apt-get install -y -qq libcap2-bin psmisc >/dev/null 2>&1 || true
    systemctl stop pingmesh 2>/dev/null || true
    pkill -f '${INSTALL_DIR}/pingmesh' 2>/dev/null || true
    rm -rf ${INSTALL_DIR}/conf ${INSTALL_DIR}/db
    gunzip -c /tmp/pingmesh-bin.gz > ${INSTALL_DIR}/pingmesh
    rm -f /tmp/pingmesh-bin.gz
    chmod 755 ${INSTALL_DIR}/pingmesh
    setcap cap_net_raw+ep ${INSTALL_DIR}/pingmesh 2>/dev/null || true
    cat > /etc/sysctl.d/99-pingmesh-icmp.conf <<'SYSCTL'
net.ipv4.icmp_msgs_per_sec = 10000
net.ipv4.icmp_msgs_burst = 500
SYSCTL
    sysctl -p /etc/sysctl.d/99-pingmesh-icmp.conf >/dev/null 2>&1 || true
    cat > ${INSTALL_DIR}/pingmesh.env <<ENV
PINGMESH_OPTS=-p 8899 -join http://${MASTER_INTERNAL}:8899 -token ${JOIN_TOKEN} -name ${name} -addr ${addr} -masters ${MASTER_INTERNAL}:8899,${BACKUP_INTERNAL}:8899
ENV
    chmod 600 ${INSTALL_DIR}/pingmesh.env
    cat > /etc/systemd/system/pingmesh.service <<UNIT
[Unit]
Description=ZENLENET PingMesh Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=TZ=Asia/Shanghai
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=-${INSTALL_DIR}/pingmesh.env
ExecStart=${INSTALL_DIR}/pingmesh -p 8899 -join http://${MASTER_INTERNAL}:8899 -token ${JOIN_TOKEN} -name ${name} -addr ${addr} -masters ${MASTER_INTERNAL}:8899,${BACKUP_INTERNAL}:8899
Restart=always
RestartSec=3
LimitNOFILE=65536
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true
PrivateTmp=true
ReadWritePaths=${INSTALL_DIR}

[Install]
WantedBy=multi-user.target
UNIT
    systemctl daemon-reload
    systemctl enable pingmesh >/dev/null
    systemctl restart pingmesh
    ok=0
    for w in 1 2 3 4 5 6 7 8 9 10; do
      sleep 3
      curl -s --max-time 5 http://127.0.0.1:8899/healthz 2>/dev/null | grep -q ok && ok=1 && break
    done
    [ \"\$ok\" = \"1\" ]
  " && info "  ${name} 部署成功" || { err "  ${name} 部署失败"; return 1; }
}

# 统一读取 agents.list(host port name addr), 不再在脚本里维护第二份清单
mapfile -t AGENTS < <(grep -vE '^\s*(#|$)' "${SCRIPT_DIR}/agents.list")

OK=0 FAIL=0
for entry in "${AGENTS[@]}"; do
  read -r host port name addr <<< "$entry"
  if deploy_agent "$host" "$port" "$name" "$addr"; then OK=$((OK+1)); else FAIL=$((FAIL+1)); fi
  sleep 10
done
info "Agent 部署完成: 成功 ${OK}, 失败 ${FAIL}"
