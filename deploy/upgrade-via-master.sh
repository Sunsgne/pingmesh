#!/usr/bin/env bash
# 本地编译后, 上传至主节点并从内网批量升级 Agent
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh

PRIMARY="${PINGMESH_MASTER_PUBLIC:-43.229.152.50}"
REMOTE_DIR=/tmp/pm-upgrade
BUILD=/tmp/pingmesh-upgrade

info()  { echo -e "\033[32m[upgrade-via-master]\033[0m $*"; }

info "编译二进制..."
cd /workspace && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BUILD" ./src
gzip -c "$BUILD" > "${BUILD}.gz"

info "上传至主节点 ${PRIMARY}..."
sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no "root@${PRIMARY}" "mkdir -p ${REMOTE_DIR}"
sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no \
  "${BUILD}.gz" \
  "${SCRIPT_DIR}/upgrade-agents-from-master.sh" \
  "${SCRIPT_DIR}/agents.list" \
  "root@${PRIMARY}:${REMOTE_DIR}/"

info "主节点经内网升级 Agent..."
sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no "root@${PRIMARY}" \
  "apt-get install -y -qq sshpass curl >/dev/null 2>&1 || true
   chmod +x ${REMOTE_DIR}/upgrade-agents-from-master.sh
   PINGMESH_SSH_PASSWORD='${PASSWORD}' BINARY=${REMOTE_DIR}/pingmesh-upgrade.gz AGENTS_LIST=${REMOTE_DIR}/agents.list \
     bash ${REMOTE_DIR}/upgrade-agents-from-master.sh"
