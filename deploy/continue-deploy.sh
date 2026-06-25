#!/usr/bin/env bash
# 自动重试: SSH 恢复后从主节点完成 Agent 部署 + git push
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh

PRIMARY="${PINGMESH_MASTER_PUBLIC:-43.229.152.50}"
LOG=/tmp/pingmesh-continue.log
MAX_SSH=60
MAX_PUSH=20

log() { echo "[$(date '+%H:%M:%S')] $*" | tee -a "$LOG"; }

try_ssh() {
  timeout 20 sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=15 \
    "root@${PRIMARY}" "$@" 2>/dev/null
}

wait_ssh() {
  local i=0
  while (( i < MAX_SSH )); do
    if try_ssh 'echo ok' | grep -q ok; then
      log "SSH 已恢复"
      return 0
    fi
    ((i++))
    log "等待 SSH... ($i/$MAX_SSH)"
    sleep 30
  done
  return 1
}

deploy_from_master() {
  log "开始从主节点部署 Agent..."
  gzip -c /tmp/pingmesh-bin > /tmp/pingmesh-bin.gz 2>/dev/null || \
    gzip -c /workspace/src/pingmesh 2>/dev/null || \
    (cd /workspace && CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/pingmesh-bin ./src && gzip -c /tmp/pingmesh-bin > /tmp/pingmesh-bin.gz)

  for i in 1 2 3; do
    if sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no \
      /workspace/deploy/agent-deploy-from-master.sh /tmp/pingmesh-bin.gz \
      "root@${PRIMARY}:/opt/pingmesh-docker/" 2>/dev/null; then
      break
    fi
    sleep 15
  done

  try_ssh 'apt-get install -y -qq sshpass >/dev/null 2>&1
    mv -f /opt/pingmesh-docker/pingmesh-bin.gz /tmp/pingmesh-bin.gz
    chmod +x /opt/pingmesh-docker/agent-deploy-from-master.sh
    sed -i "s|BINARY=.*|BINARY=/tmp/pingmesh-bin.gz|" /opt/pingmesh-docker/agent-deploy-from-master.sh
    nohup bash /opt/pingmesh-docker/agent-deploy-from-master.sh > /tmp/agent-from-master.log 2>&1 &
    echo AGENT_DEPLOY_STARTED'

  log "Agent 部署已在主节点后台启动, 日志: /tmp/agent-from-master.log"
}

verify_cluster() {
  log "验证控制节点..."
  try_ssh '
    echo "=== 主节点 ==="
    docker ps --format "{{.Names}}: {{.Status}}"
    curl -sk https://127.0.0.1:443/healthz
    echo
    sqlite3 /opt/pingmesh-docker/data/db/database.db "select username,role from users;" 2>/dev/null
    openssl x509 -in /opt/pingmesh-docker/certs/server.crt -noout -dates 2>/dev/null
    echo "=== Agent 部署进度 ==="
    tail -20 /tmp/agent-from-master.log 2>/dev/null || echo "(暂无日志)"
  '
}

try_push() {
  local i=0
  while (( i < MAX_PUSH )); do
    if git -C /workspace push -u origin cursor/pingmesh-cluster-deploy-8a99 2>/dev/null; then
      log "Git push 成功"
      return 0
    fi
    ((i++))
    log "等待 git push... ($i/$MAX_PUSH)"
    sleep 20
  done
  return 1
}

# ---- 主流程 ----
log "========== PingMesh 继续部署 =========="

if wait_ssh; then
  verify_cluster
  deploy_from_master
  sleep 120
  verify_cluster
else
  log "SSH 在超时内未恢复, 请手动在主节点执行:"
  log "  bash /opt/pingmesh-docker/agent-deploy-from-master.sh"
fi

try_push || log "Git push 未成功, 请稍后手动: git push -u origin cursor/pingmesh-cluster-deploy-8a99"

log "========== 结束 =========="
