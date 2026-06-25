#!/usr/bin/env bash
# PingMesh 集群批量部署脚本
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"
deploy_require_ssh

INSTALL_DIR='/opt/pingmesh-docker'

info()  { echo -e "\033[32m[deploy]\033[0m $*"; }
warn()  { echo -e "\033[33m[deploy]\033[0m $*"; }
fatal() { echo -e "\033[31m[deploy]\033[0m $*"; exit 1; }

ssh_run() {
  local host="$1" port="${2:-22}"; shift 2
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -p "$port" "root@${host}" "$@"
}

scp_to() {
  local host="$1" port="${2:-22}" src="$3" dst="$4"
  sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no -P "$port" -r "$src" "root@${host}:${dst}"
}

install_docker_remote() {
  local host="$1" port="${2:-22}"
  ssh_run "$host" "$port" 'export DEBIAN_FRONTEND=noninteractive
    apt-get install -y -qq tzdata >/dev/null 2>&1 || true
    timedatectl set-timezone Asia/Shanghai 2>/dev/null || {
      ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
      echo Asia/Shanghai > /etc/timezone
    }
    command -v docker >/dev/null || (
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq ca-certificates curl
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
  )'
}

build_image_remote() {
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

gen_cert_remote() {
  local host="$1" port="${2:-22}" cn="$3"
  ssh_run "$host" "$port" "mkdir -p ${INSTALL_DIR}/certs && openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
    -keyout ${INSTALL_DIR}/certs/server.key \
    -out ${INSTALL_DIR}/certs/server.crt \
    -subj '/CN=${cn}/O=PingMesh/C=CN' 2>/dev/null"
}

setup_default_user() {
  local host="$1" port="${2:-22}"
  deploy_setup_admin_user "$host" "$port" "$INSTALL_DIR"
}

deploy_control() {
  local host="$1" port="${2:-22}" name="$3" addr="$4" role="$5"
  info "部署控制节点 ${name} (${host}:${port}, ${addr}) [${role}]"
  install_docker_remote "$host" "$port"
  build_image_remote "$host" "$port"
  ssh_run "$host" "$port" "mkdir -p ${INSTALL_DIR}"
  scp_to "$host" "$port" "/workspace/deploy/docker/docker-compose.control.yml" "${INSTALL_DIR}/docker-compose.yml"
  scp_to "$host" "$port" "/workspace/deploy/docker/nginx.conf" "${INSTALL_DIR}/nginx.conf"
  gen_cert_remote "$host" "$port" "$host"
  ssh_run "$host" "$port" "cd ${INSTALL_DIR} && \
    export NODE_NAME='${name}' NODE_ADDR='${addr}' && \
    docker compose down 2>/dev/null || true && \
    docker compose up -d"
  if [[ "$role" == "primary" ]]; then
    setup_default_user "$host" "$port"
  fi
}

deploy_agent() {
  local host="$1" port="${2:-22}" name="$3" addr="$4" token="$5"
  info "部署 Agent ${name} (${host}:${port}, 内网 ${addr})"
  install_docker_remote "$host" "$port"
  ssh_run "$host" "$port" "docker image inspect pingmesh:local >/dev/null 2>&1" || build_image_remote "$host" "$port"
  ssh_run "$host" "$port" "mkdir -p ${INSTALL_DIR}"
  scp_to "$host" "$port" "/workspace/deploy/docker/docker-compose.agent.yml" "${INSTALL_DIR}/docker-compose.yml"
  ssh_run "$host" "$port" "cd ${INSTALL_DIR} && \
    export NODE_NAME='${name}' NODE_ADDR='${addr}' JOIN_TOKEN='${token}' \
      MASTER_URL='http://${MASTER_INTERNAL}:8899' \
      MASTERS='${MASTER_INTERNAL}:8899,${BACKUP_INTERNAL}:8899' && \
    docker compose down 2>/dev/null || true && \
    docker compose up -d"
}

get_token() {
  if [[ -n "$JOIN_TOKEN" ]]; then
    echo "$JOIN_TOKEN"
    return
  fi
  local t
  t=$(ssh_run "$MASTER_PUBLIC" 22 "sqlite3 ${INSTALL_DIR}/data/db/database.db \"select value from config where key='Password'\" 2>/dev/null \
    || grep -oP '\"Password\"\\s*:\\s*\"\\K[^\"]+' ${INSTALL_DIR}/data/conf/config.json 2>/dev/null" | tr -d '\r')
  if [[ -z "$t" ]]; then
    deploy_require_join
  fi
  echo "$t"
}

# ---- 主流程 ----
case "${1:-all}" in
  primary)
    deploy_control "$MASTER_PUBLIC" 22 "sin1-sg2" "$MASTER_INTERNAL" "primary"
  ;;
  backup)
    deploy_control "$BACKUP_PUBLIC" 22 "hkg1" "$BACKUP_INTERNAL" "backup"
  ;;
  agents)
  deploy_require_join
  TOKEN=$(get_token)
  info "接入令牌: ${TOKEN}"
  deploy_agent "106.75.160.24" 20001 "can-xxg" "10.100.1.4" "$TOKEN"
  deploy_agent "42.240.152.238" 20001 "can-hxy" "10.100.1.19" "$TOKEN"
  deploy_agent "217.217.29.250" 22 "fra" "10.100.1.7" "$TOKEN"
  deploy_agent "104.251.226.39" 20001 "hkg2" "10.100.1.12" "$TOKEN"
  deploy_agent "163.53.245.136" 20001 "hkg3" "10.100.1.13" "$TOKEN"
  deploy_agent "149.119.41.156" 22 "lax" "10.100.1.10" "$TOKEN"
  deploy_agent "106.38.203.8" 20001 "pek" "10.100.1.15" "$TOKEN"
  deploy_agent "61.172.165.219" 20001 "gds" "10.100.1.20" "$TOKEN"
  deploy_agent "113.31.161.79" 20001 "sjhl" "10.100.1.5" "$TOKEN"
  deploy_agent "109.244.32.190" 20001 "xtl" "10.100.1.1" "$TOKEN"
  deploy_agent "149.51.125.226" 20001 "sin2-gs" "10.100.1.11" "$TOKEN"
  deploy_agent "59.36.211.118" 20001 "szx" "10.100.1.17" "$TOKEN"
  deploy_agent "192.169.120.12" 22 "tpe" "10.100.1.18" "$TOKEN"
  deploy_agent "43.230.52.242" 22 "tyo-8" "10.100.1.2" "$TOKEN"
  deploy_agent "61.172.165.219" 20001 "tyo-7" "10.100.1.9" "$TOKEN"
  ;;
  all)
    deploy_control "$MASTER_PUBLIC" 22 "sin1-sg2" "$MASTER_INTERNAL" "primary"
    deploy_control "$BACKUP_PUBLIC" 22 "hkg1" "$BACKUP_INTERNAL" "backup"
    deploy_require_join
    TOKEN=$(get_token)
    info "接入令牌: ${TOKEN}"
    deploy_agent "106.75.160.24" 20001 "can-xxg" "10.100.1.4" "$TOKEN"
    deploy_agent "42.240.152.238" 20001 "can-hxy" "10.100.1.19" "$TOKEN"
    deploy_agent "217.217.29.250" 22 "fra" "10.100.1.7" "$TOKEN"
    deploy_agent "104.251.226.39" 20001 "hkg2" "10.100.1.12" "$TOKEN"
    deploy_agent "163.53.245.136" 20001 "hkg3" "10.100.1.13" "$TOKEN"
    deploy_agent "149.119.41.156" 22 "lax" "10.100.1.10" "$TOKEN"
    deploy_agent "106.38.203.8" 20001 "pek" "10.100.1.15" "$TOKEN"
    deploy_agent "61.172.165.219" 20001 "gds" "10.100.1.20" "$TOKEN"
    deploy_agent "113.31.161.79" 20001 "sjhl" "10.100.1.5" "$TOKEN"
    deploy_agent "109.244.32.190" 20001 "xtl" "10.100.1.1" "$TOKEN"
    deploy_agent "149.51.125.226" 20001 "sin2-gs" "10.100.1.11" "$TOKEN"
    deploy_agent "59.36.211.118" 20001 "szx" "10.100.1.17" "$TOKEN"
    deploy_agent "192.169.120.12" 22 "tpe" "10.100.1.18" "$TOKEN"
    deploy_agent "43.230.52.242" 22 "tyo-8" "10.100.1.2" "$TOKEN"
    deploy_agent "61.172.165.219" 20001 "tyo-7" "10.100.1.9" "$TOKEN"
  ;;
  *)
    echo "用法: $0 [primary|backup|agents|all]"
    exit 1
  ;;
esac

info "部署完成"
