#!/usr/bin/env bash
# 从主节点经内网检查各 Agent 真实探测频率(读本地 pinglog, 非配置表面)
set -o pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh

PRIMARY="${PINGMESH_MASTER_PUBLIC:-43.229.152.50}"
REMOTE_DIR=/tmp/pm-probe-check
CHECK="${SCRIPT_DIR}/check-probe-cadence.py"
LIST="${SCRIPT_DIR}/agents.list"
WINDOW=30

info()  { echo -e "\033[36m[probe-check]\033[0m $*"; }
warn()  { echo -e "\033[33m[probe-check]\033[0m $*"; }
bad()   { echo -e "\033[31m[probe-check]\033[0m $*"; }

info "上传检查脚本到主节点..."
sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no "root@${PRIMARY}" "mkdir -p ${REMOTE_DIR}"
sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no "$CHECK" "root@${PRIMARY}:${REMOTE_DIR}/check-probe-cadence.py"

check_master() {
  info "检查主节点 (Docker)..."
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no "root@${PRIMARY}" \
    "docker cp ${REMOTE_DIR}/check-probe-cadence.py pingmesh:/tmp/check-probe-cadence.py
     docker exec pingmesh python3 /tmp/check-probe-cadence.py" 2>&1 | sed 's/^/  /'
}

check_agent() {
  local addr="$1" name="$2"
  info "检查 ${name} (${addr})..."
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no "root@${PRIMARY}" \
    "sshpass -p '${PASSWORD}' scp -o StrictHostKeyChecking=no ${REMOTE_DIR}/check-probe-cadence.py root@${addr}:/tmp/check-probe-cadence.py
     sshpass -p '${PASSWORD}' ssh -o StrictHostKeyChecking=no -o ConnectTimeout=12 root@${addr} 'python3 /tmp/check-probe-cadence.py'" 2>&1 | sed 's/^/  /'
}

OK=0 FAIL=0 SKIP=0
MASTER_OUT=$(check_master) || true
echo "$MASTER_OUT"
if echo "$MASTER_OUT" | grep -q 'SUMMARY|OK'; then OK=$((OK+1)); else FAIL=$((FAIL+1)); fi

while read -r _host port name addr _; do
  [[ -z "${name:-}" || "${_host:-}" =~ ^# ]] && continue
  OUT=$(check_agent "$addr" "$name") || true
  echo "$OUT"
  if echo "$OUT" | grep -q 'SUMMARY|OK'; then
    OK=$((OK+1))
  elif echo "$OUT" | grep -qE 'FAIL|NO_CFG|NO_DB'; then
    FAIL=$((FAIL+1))
  else
    SKIP=$((SKIP+1))
  fi
done < "$LIST"

echo ""
info "汇总: 正常 ${OK}, 异常 ${FAIL}, 跳过 ${SKIP}"
[[ $FAIL -eq 0 ]]
