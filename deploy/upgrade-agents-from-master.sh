#!/usr/bin/env bash
# 从主控制节点经内网 SSH 滚动升级 Agent 二进制(保留 conf/db/logs)
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIST="${AGENTS_LIST:-${SCRIPT_DIR}/agents.list}"
PASSWORD="${PINGMESH_SSH_PASSWORD:-${PASSWORD:-}}"
if [[ -z "$PASSWORD" && -f "${SCRIPT_DIR}/lib.sh" ]]; then
  # shellcheck disable=SC1091
  source "${SCRIPT_DIR}/lib.sh"
  PASSWORD="${PINGMESH_SSH_PASSWORD:-}"
fi
if [[ -z "$PASSWORD" ]]; then
  echo "错误: 未设置 PINGMESH_SSH_PASSWORD" >&2
  exit 1
fi

INSTALL_DIR='/opt/pingmesh'
BINARY="${BINARY:-/tmp/pingmesh-bin.gz}"

info()  { echo -e "\033[32m[upgrade-int]\033[0m $*"; }
err()   { echo -e "\033[31m[upgrade-int]\033[0m $*"; }

ssh_run() {
  local host="$1" port="${2:-22}"; shift 2
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -p "$port" "root@${host}" "$@"
}

upgrade_agent() {
  local host="$1" port="$2" name="$3"
  info "升级 ${name} -> ${host}:${port}"
  if ! ssh_run "$host" "$port" "echo ok" 2>/dev/null; then
    err "  SSH 连接失败, 跳过 ${name}"; return 1
  fi
  sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no -P "$port" "$BINARY" "root@${host}:/tmp/pingmesh-upgrade.gz" || {
    err "  scp 失败, 跳过 ${name}"; return 1
  }
  ssh_run "$host" "$port" "DIR=${INSTALL_DIR} bash -s" <<'REMOTE'
set -e
if [ ! -d "$DIR" ] || [ ! -f "$DIR/pingmesh" ]; then
  echo "NO_INSTALL"
  exit 2
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
sleep 5
curl -sf --max-time 8 http://127.0.0.1:8899/healthz | grep -q ok
REMOTE
  if [[ $? -eq 0 ]]; then
    info "  ${name} 成功"
  else
    err "  ${name} 失败"; return 1
  fi
}

AGENTS=()
while read -r _host port name addr _; do
  [[ -z "${name:-}" || "$name" =~ ^# ]] && continue
  # 从主节点走内网: SSH 目标用 addr(10.100.1.x), 不用公网 host
  AGENTS+=("$addr $port $name")
done < "${LIST}"

if ((${#AGENTS[@]} == 0)); then
  err "未找到 Agent 清单: ${LIST}"
  exit 1
fi

if [[ ! -f "$BINARY" ]]; then
  err "缺少二进制包: $BINARY"
  exit 1
fi

apt-get install -y -qq sshpass curl >/dev/null 2>&1 || true
OK=0 FAIL=0
for entry in "${AGENTS[@]}"; do
  read -r host port name <<< "$entry"
  if upgrade_agent "$host" "$port" "$name"; then OK=$((OK+1)); else FAIL=$((FAIL+1)); fi
  sleep 2
done
info "完成: 成功 ${OK}, 失败 ${FAIL}"
[[ $FAIL -eq 0 ]]
