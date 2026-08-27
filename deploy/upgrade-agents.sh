#!/usr/bin/env bash
# 滚动升级 Agent 二进制(保留 conf/db/logs, 不删数据)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh

info()  { echo -e "\033[32m[upgrade]\033[0m $*"; }
err()   { echo -e "\033[31m[upgrade]\033[0m $*"; }

BUILD=/tmp/pingmesh-upgrade
info "编译二进制..."
cd /workspace && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BUILD" ./src
gzip -c "$BUILD" > "${BUILD}.gz"

ssh_run() {
  local host="$1" port="${2:-22}"; shift 2
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=15 -p "$port" "root@${host}" "$@"
}

upgrade_one() {
  local host="$1" port="$2" name="$3"
  info "升级 ${name} (${host}:${port})"
  if ! ssh_run "$host" "$port" "echo ok" >/dev/null 2>&1; then
    err "  SSH 失败, 跳过"; return 1
  fi
  sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no -P "$port" "${BUILD}.gz" "root@${host}:/tmp/pingmesh-upgrade.gz" || {
    err "  scp 失败"; return 1
  }
  ssh_run "$host" "$port" 'set -e
    DIR=/opt/pingmesh
    if [ ! -d "$DIR" ]; then
      # docker agent
      if [ -d /opt/pingmesh-docker ]; then
        cd /opt/pingmesh-docker
        gunzip -c /tmp/pingmesh-upgrade.gz > /tmp/pingmesh-new
        chmod +x /tmp/pingmesh-new
        # 仅提示: docker 部署请走 cluster-deploy agents 重建镜像
        echo DOCKER_AGENT
        exit 0
      fi
      echo NO_INSTALL; exit 2
    fi
    systemctl stop pingmesh 2>/dev/null || true
    pkill -f "$DIR/pingmesh" 2>/dev/null || true
    sleep 1
    gunzip -c /tmp/pingmesh-upgrade.gz > "$DIR/pingmesh.new"
    chmod 755 "$DIR/pingmesh.new"
    setcap cap_net_raw+ep "$DIR/pingmesh.new" 2>/dev/null || true
    mv -f "$DIR/pingmesh.new" "$DIR/pingmesh"
    rm -f /tmp/pingmesh-upgrade.gz
    systemctl start pingmesh
    sleep 3
    curl -sf --max-time 5 http://127.0.0.1:8899/healthz | grep -q ok
  ' && info "  ${name} OK" || { err "  ${name} 失败"; return 1; }
}

AGENTS=(
  "106.75.160.24 20001 CAN-XXG"
  "217.217.29.250 22 FRA-EQFRA5"
  "129.227.133.75 22 HKG-EQHK2"
  "163.53.245.136 20001 HKG-EQHK3"
  "149.119.41.156 22 LAX-CORESITE"
  "106.38.203.8 20001 PEK-BJHK"
  "61.172.165.219 20001 PVG-GDS"
  "113.31.161.79 20001 PVG-SJHL"
  "109.244.32.190 20001 PVG-XTL"
  "149.51.125.226 20001 SIN-GS"
  "59.36.211.118 20001 SZX-SZX"
  "192.169.120.12 22 TPE-EQTPE"
  "43.230.52.242 22 TYO-EQTY8"
)

OK=0 FAIL=0
for e in "${AGENTS[@]}"; do
  read -r host port name <<< "$e"
  if upgrade_one "$host" "$port" "$name"; then OK=$((OK+1)); else FAIL=$((FAIL+1)); fi
done
# tyo-7 shares host with gds in some listings; try internal via master jump if needed
info "完成: 成功 ${OK}, 失败 ${FAIL}"
