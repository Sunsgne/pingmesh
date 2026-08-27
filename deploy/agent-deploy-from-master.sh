#!/usr/bin/env bash
# 从主控制节点通过内网 SSH 批量部署 Agent (关闭 Web, 接入主节点)
set -o pipefail

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
    err "  SSH 连接失败, 跳过 ${name}"; return 1
  fi
  sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no -P "$port" "$BINARY" "root@${host}:/tmp/pingmesh-bin.gz" || {
    err "  scp 失败, 跳过 ${name}"; return 1
  }
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -p "$port" "root@${host}" \
    "NAME=${name} ADDR=${addr} MASTER=${MASTER_INTERNAL} BACKUP=${BACKUP_INTERNAL} TOKEN=${JOIN_TOKEN} DIR=${INSTALL_DIR} bash -s" <<'REMOTE'
apt-get install -y -qq libcap2-bin psmisc curl tzdata >/dev/null 2>&1 || true
timedatectl set-timezone Asia/Shanghai 2>/dev/null || {
  ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
  echo 'Asia/Shanghai' > /etc/timezone
}
systemctl stop pingmesh 2>/dev/null || true
pkill -f "${DIR}/pingmesh" 2>/dev/null || true
sleep 1
rm -rf "${DIR}"
mkdir -p "${DIR}"
gunzip -c /tmp/pingmesh-bin.gz > "${DIR}/pingmesh"
rm -f /tmp/pingmesh-bin.gz
chmod 755 "${DIR}/pingmesh"
setcap cap_net_raw+ep "${DIR}/pingmesh" 2>/dev/null || true
cat > /etc/sysctl.d/99-pingmesh-icmp.conf <<'SYSCTL'
net.ipv4.icmp_msgs_per_sec = 10000
net.ipv4.icmp_msgs_burst = 500
SYSCTL
sysctl -p /etc/sysctl.d/99-pingmesh-icmp.conf >/dev/null 2>&1 || true
cat > /etc/systemd/system/pingmesh.service <<UNIT
[Unit]
Description=ZENLENET PingMesh Agent
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
WorkingDirectory=${DIR}
ExecStart=${DIR}/pingmesh -p 8899 -join http://${MASTER}:8899 -token ${TOKEN} -name ${NAME} -addr ${ADDR} -masters ${MASTER}:8899,${BACKUP}:8899
Restart=always
RestartSec=3
LimitNOFILE=65536
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable pingmesh 2>/dev/null || true
systemctl restart pingmesh
sleep 25
curl -s --max-time 5 http://127.0.0.1:8899/healthz 2>/dev/null | grep -q ok
REMOTE
  if [[ $? -eq 0 ]]; then
    info "  ${name} 成功"
  else
    err "  ${name} 失败"; return 1
  fi
}

AGENTS=()
while read -r host port name addr _; do
  [[ -z "${host:-}" || "$host" =~ ^# ]] && continue
  AGENTS+=("$host $port $name $addr")
done < "${SCRIPT_DIR}/agents.list"

if ((${#AGENTS[@]} == 0)); then
  err "未找到 Agent 清单: ${SCRIPT_DIR}/agents.list"
  exit 1
fi

apt-get install -y -qq sshpass >/dev/null 2>&1 || true
OK=0 FAIL=0
for entry in "${AGENTS[@]}"; do
  read -r host port name addr <<< "$entry"
  if deploy_agent "$host" "$port" "$name" "$addr"; then OK=$((OK+1)); else FAIL=$((FAIL+1)); fi
  sleep 3
done
info "完成: 成功 ${OK}, 失败 ${FAIL}"
