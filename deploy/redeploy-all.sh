#!/usr/bin/env bash
# PingMesh 全集群重新部署
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh
deploy_require_join

INSTALL_DIR='/opt/pingmesh-docker'
AGENT_DIR='/opt/pingmesh'

info()  { echo -e "\033[32m[deploy]\033[0m $*"; }
warn()  { echo -e "\033[33m[deploy]\033[0m $*"; }
err()   { echo -e "\033[31m[deploy]\033[0m $*"; }

ssh_run() {
  local host="$1" port="${2:-22}"; shift 2
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -p "$port" "root@${host}" "$@"
}

scp_to() {
  local host="$1" port="${2:-22}" src="$3" dst="$4"
  sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no -r -P "$port" "$src" "root@${host}:${dst}"
}

ssh_retry() {
  local host="$1" port="${2:-22}"; shift 2
  local i=0
  while (( i < 10 )); do
    if ssh_run "$host" "$port" "$@" 2>/dev/null; then return 0; fi
    ((i++)); sleep 15
  done
  return 1
}

install_docker() {
  local host="$1" port="${2:-22}"
  ssh_retry "$host" "$port" 'export DEBIAN_FRONTEND=noninteractive
    apt-get install -y -qq tzdata >/dev/null 2>&1 || true
    timedatectl set-timezone Asia/Shanghai 2>/dev/null || {
      ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
      echo Asia/Shanghai > /etc/timezone
    }
    command -v docker >/dev/null || (
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq && apt-get install -y -qq ca-certificates curl
    curl -fsSL https://get.docker.com | sh && systemctl enable --now docker
  )'
}

build_image() {
  local host="$1" port="${2:-22}"
  ssh_run "$host" "$port" "mkdir -p ${INSTALL_DIR}/src"
  scp_to "$host" "$port" "/workspace/Dockerfile" "${INSTALL_DIR}/src/Dockerfile"
  scp_to "$host" "$port" "/workspace/go.mod" "${INSTALL_DIR}/src/go.mod"
  scp_to "$host" "$port" "/workspace/go.sum" "${INSTALL_DIR}/src/go.sum"
  scp_to "$host" "$port" "/workspace/embed.go" "${INSTALL_DIR}/src/embed.go"
  scp_to "$host" "$port" "/workspace/src" "${INSTALL_DIR}/src/"
  scp_to "$host" "$port" "/workspace/html" "${INSTALL_DIR}/src/"
  scp_to "$host" "$port" "/workspace/conf" "${INSTALL_DIR}/src/"
  ssh_run "$host" "$port" "cd ${INSTALL_DIR}/src && docker build -t pingmesh:local ."
}

deploy_control() {
  local host="$1" port="$2" name="$3" addr="$4" role="$5"
  info "部署控制节点 ${name} [${role}] ${host} 内网${addr}"
  install_docker "$host" "$port"
  build_image "$host" "$port"
  ssh_run "$host" "$port" "mkdir -p ${INSTALL_DIR}/certs ${INSTALL_DIR}/data && rm -rf ${INSTALL_DIR}/data/*"
  scp_to "$host" "$port" "/workspace/deploy/docker/docker-compose.control.yml" "${INSTALL_DIR}/docker-compose.yml"
  scp_to "$host" "$port" "/workspace/deploy/docker/nginx.conf" "${INSTALL_DIR}/nginx.conf"
  ssh_run "$host" "$port" "openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
    -keyout ${INSTALL_DIR}/certs/server.key -out ${INSTALL_DIR}/certs/server.crt \
    -subj '/CN=${host}/O=PingMesh/C=CN'"
  ssh_run "$host" "$port" "cd ${INSTALL_DIR} && docker compose down 2>/dev/null; \
    export NODE_NAME='${name}' NODE_ADDR='${addr}' && docker compose up -d"
  if [[ "$role" == "primary" ]]; then
    sleep 8
    deploy_setup_admin_user "$host" "$port" "$INSTALL_DIR"
    info "  管理员账号已设置（见 deploy/.env 中 PINGMESH_ADMIN_*）"
  fi
  ssh_run "$host" "$port" "curl -sk https://127.0.0.1:443/healthz; echo; \
    openssl x509 -in ${INSTALL_DIR}/certs/server.crt -noout -dates"
}

prepare_binary() {
  if [[ ! -f /tmp/pingmesh-bin ]]; then
    info "本地编译 pingmesh 二进制..."
    cd /workspace && CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/pingmesh-bin ./src
  fi
  gzip -c /tmp/pingmesh-bin > /tmp/pingmesh-bin.gz
}

deploy_agents_from_master() {
  info "上传 Agent 部署脚本到主节点..."
  prepare_binary
  base64 /tmp/pingmesh-bin.gz | ssh_run "$MASTER_PUBLIC" 22 "base64 -d > /tmp/pingmesh-bin.gz"
  base64 /workspace/deploy/agent-deploy-from-master.sh | ssh_run "$MASTER_PUBLIC" 22 \
    "base64 -d > ${INSTALL_DIR}/agent-deploy.sh && chmod +x ${INSTALL_DIR}/agent-deploy.sh"
  base64 /workspace/deploy/lib.sh | ssh_run "$MASTER_PUBLIC" 22 \
    "base64 -d > ${INSTALL_DIR}/lib.sh"
  if [[ -f "${SCRIPT_DIR}/.env" ]]; then
    scp_to "$MASTER_PUBLIC" 22 "${SCRIPT_DIR}/.env" "${INSTALL_DIR}/.env"
  else
    warn "未找到 deploy/.env，请先在主节点设置 PINGMESH_SSH_PASSWORD / PINGMESH_JOIN_TOKEN"
  fi
  ssh_run "$MASTER_PUBLIC" 22 "apt-get install -y -qq sshpass >/dev/null 2>&1 || true
    chmod 600 ${INSTALL_DIR}/.env 2>/dev/null || true
    nohup bash -c 'set -a; [ -f ${INSTALL_DIR}/.env ] && . ${INSTALL_DIR}/.env; set +a; bash ${INSTALL_DIR}/agent-deploy.sh' > /tmp/agent-deploy.log 2>&1 &
    echo AGENT_DEPLOY_STARTED"
}

verify() {
  info "========== 验证 =========="
  ssh_run "$MASTER_PUBLIC" 22 "
    echo '--- 主节点 ---'
    docker ps --format '{{.Names}}: {{.Status}}'
    curl -sk https://127.0.0.1:443/healthz; echo
    sqlite3 ${INSTALL_DIR}/data/db/database.db 'select username,role from users;' 2>/dev/null
    echo '--- Agent 进度 ---'
    tail -20 /tmp/agent-deploy.log 2>/dev/null
  " || warn "主节点验证 SSH 失败"
  ssh_run "$BACKUP_PUBLIC" 22 "
    echo '--- 备节点 ---'
    docker ps --format '{{.Names}}: {{.Status}}'
    curl -sk https://127.0.0.1:443/healthz; echo
  " || warn "备节点验证 SSH 失败"
}

case "${1:-all}" in
  uninstall) bash /workspace/deploy/uninstall-all.sh ;;
  primary)   deploy_control "$MASTER_PUBLIC" 22 "sin1-sg2" "$MASTER_INTERNAL" "primary" ;;
  backup)    deploy_control "$BACKUP_PUBLIC" 22 "hkg1" "$BACKUP_INTERNAL" "backup" ;;
  agents)    deploy_agents_from_master ;;
  verify)    verify ;;
  all)
    bash /workspace/deploy/uninstall-all.sh
    deploy_control "$MASTER_PUBLIC" 22 "sin1-sg2" "$MASTER_INTERNAL" "primary"
    deploy_control "$BACKUP_PUBLIC" 22 "hkg1" "$BACKUP_INTERNAL" "backup"
    deploy_agents_from_master
    sleep 30
    verify
    ;;
  *) echo "用法: $0 [uninstall|primary|backup|agents|verify|all]"; exit 1 ;;
esac
info "完成"
