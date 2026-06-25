#!/usr/bin/env bash
# 将集群全部节点系统时区设为 Asia/Shanghai
# 优先使用同目录 finish-timezone.py（不依赖 sshpass）
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if command -v python3 >/dev/null 2>&1 && [[ -f "${SCRIPT_DIR}/finish-timezone.py" ]]; then
  exec python3 "${SCRIPT_DIR}/finish-timezone.py" "$@"
fi
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh

TZ_NAME='Asia/Shanghai'
warn()  { echo -e "\033[33m[tz]\033[0m $*"; }
err()   { echo -e "\033[31m[tz]\033[0m $*"; }

ssh_run() {
  local host="$1" port="${2:-22}"; shift 2
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -p "$port" "root@${host}" "$@"
}

set_timezone_remote() {
  local host="$1" port="$2" name="$3"
  info "设置时区 ${name} (${host}:${port})"
  if ! ssh_run "$host" "$port" "echo ok" 2>/dev/null; then
    err "  SSH 连接失败, 跳过 ${name}"
    return 1
  fi
  ssh_run "$host" "$port" "bash -s" <<REMOTE
export DEBIAN_FRONTEND=noninteractive
apt-get install -y -qq tzdata >/dev/null 2>&1 || true
if command -v timedatectl >/dev/null 2>&1; then
  timedatectl set-timezone ${TZ_NAME}
else
  ln -sf /usr/share/zoneinfo/${TZ_NAME} /etc/localtime
  echo '${TZ_NAME}' > /etc/timezone
  dpkg-reconfigure -f noninteractive tzdata >/dev/null 2>&1 || true
fi
# Docker 控制节点容器继承镜像 TZ; 显式重启使进程读到新环境
if [ -d /opt/pingmesh-docker ] && command -v docker >/dev/null 2>&1; then
  cd /opt/pingmesh-docker && docker compose restart pingmesh pingmesh-agent 2>/dev/null || \
    docker compose restart pingmesh 2>/dev/null || true
fi
# systemd Agent 随主机时区, 重启后日志时间一致
if systemctl is-active pingmesh >/dev/null 2>&1; then
  systemctl restart pingmesh
fi
echo "TZ=\$(timedatectl show -p Timezone --value 2>/dev/null || cat /etc/timezone 2>/dev/null)"
date '+%Y-%m-%d %H:%M:%S %Z'
REMOTE
  if [[ $? -eq 0 ]]; then
    info "  ${name} 完成"
    return 0
  fi
  err "  ${name} 失败"
  return 1
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

command -v sshpass >/dev/null || apt-get install -y -qq sshpass >/dev/null 2>&1 || true

OK=0 FAIL=0
for entry in "${NODES[@]}"; do
  read -r host port name <<< "$entry"
  if set_timezone_remote "$host" "$port" "$name"; then
    OK=$((OK + 1))
  else
    FAIL=$((FAIL + 1))
  fi
  sleep 2
done

# 主节点内网可达、公网列表未单独列出的 Agent
INTERNAL_FROM_MASTER=(
  "10.100.1.2 22 TYO-EQTY8"
  "10.100.1.9 22 TYO-EQTY7"
  "10.100.1.20 22 PVG-GDS"
)
info "经主节点内网补设时区..."
for entry in "${INTERNAL_FROM_MASTER[@]}"; do
  read -r ip port name <<< "$entry"
  if ssh_run "43.229.152.50" 22 "sshpass -p '${PASSWORD}' ssh -o StrictHostKeyChecking=no -p ${port} root@${ip} 'timedatectl set-timezone ${TZ_NAME} 2>/dev/null || (ln -sf /usr/share/zoneinfo/${TZ_NAME} /etc/localtime && echo ${TZ_NAME} > /etc/timezone); timedatectl show -p Timezone --value 2>/dev/null || cat /etc/timezone; date'" 2>/dev/null; then
    info "  ${name} (${ip}) 完成"
    OK=$((OK + 1))
  else
    warn "  ${name} (${ip}) 跳过(可能已与公网入口同一台)"
  fi
done

info "完成: 成功 ${OK}, 失败 ${FAIL}"
