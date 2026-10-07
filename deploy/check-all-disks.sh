#!/usr/bin/env bash
# 从主节点经内网巡检全集群根磁盘/LVM/防护脚本状态
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh

LIST="${AGENTS_LIST:-${SCRIPT_DIR}/agents.list}"
PRIMARY="${PINGMESH_MASTER_PUBLIC:-43.229.152.50}"

info() { echo -e "\033[32m[disk-check]\033[0m $*"; }
warn() { echo -e "\033[33m[disk-check]\033[0m $*"; }
err()  { echo -e "\033[31m[disk-check]\033[0m $*"; }

# 上传清单到主节点并执行
sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no \
  "$LIST" "${SCRIPT_DIR}/disk-harden.sh" "${SCRIPT_DIR}/expand-root-disk.sh" \
  "root@${PRIMARY}:/tmp/" >/dev/null

sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no "root@${PRIMARY}" \
  "PINGMESH_SSH_PASSWORD='${PASSWORD}' bash -s" <<'REMOTE'
set +e
PASSWORD="${PINGMESH_SSH_PASSWORD}"
printf "%-14s %-14s %-32s %-8s %-8s\n" NAME ADDR ROOT CRON WARN
warns=0
check() {
  local name="$1" addr="$2"
  local out root cron use
  if [ "$addr" = "10.100.1.8" ]; then
    out=$(df -hP / | tail -1; test -f /etc/cron.d/pm-disk-watch && echo CRON_OK || echo CRON_NO)
  else
    out=$(sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 root@$addr \
      'df -hP / | tail -1; test -f /etc/cron.d/pm-disk-watch && echo CRON_OK || echo CRON_NO' 2>/dev/null)
  fi
  if [ -z "$out" ]; then
    printf "%-14s %-14s %-32s %-8s %-8s\n" "$name" "$addr" "SSH_FAIL" - FAIL
    warns=$((warns+1)); return
  fi
  root=$(echo "$out" | head -1 | awk '{print $2" used="$3" free="$4" "$5}')
  cron=$(echo "$out" | grep -E 'CRON_' | head -1)
  use=$(echo "$out" | head -1 | awk '{gsub(/%/,"",$5); print $5}')
  flag=OK
  if [ "${use:-0}" -ge 70 ]; then flag=HIGH; warns=$((warns+1)); fi
  if [ "$cron" != "CRON_OK" ]; then flag=NO_CRON; warns=$((warns+1)); fi
  printf "%-14s %-14s %-32s %-8s %-8s\n" "$name" "$addr" "$root" "$cron" "$flag"
}
check SIN-EQSG2 10.100.1.8
check HKG-backup 10.100.1.3
while read -r _host _port name addr _; do
  [[ -z "${name:-}" || "${_host:-}" =~ ^# ]] && continue
  check "$name" "$addr"
done < /tmp/agents.list
echo "WARN_COUNT=$warns"
exit $warns
REMOTE
