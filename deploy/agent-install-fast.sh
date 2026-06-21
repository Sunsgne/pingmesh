#!/usr/bin/env bash
# Agent 部署: 优先加载预构建镜像, 避免各节点重复 docker build
set -euo pipefail
NAME="$1"
ADDR="$2"
JOIN_URL="http://43.229.152.51:8899"
TOKEN="Zenlenet@PingMesh2026"
MASTERS="43.229.152.51:8899,163.53.245.91:8899"
INSTALL_DIR=/opt/pingmesh

install_docker_simple() {
  if command -v docker >/dev/null 2>&1; then return; fi
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
}

install_docker_simple
mkdir -p ${INSTALL_DIR}/data/{conf,db,logs}

if [[ -f /tmp/pingmesh-image.tar ]]; then
  docker load -i /tmp/pingmesh-image.tar
elif ! docker image inspect pingmesh:latest >/dev/null 2>&1; then
  echo "ERROR: no pingmesh image" >&2; exit 1
fi

docker rm -f pingmesh 2>/dev/null || true
docker run -d --name pingmesh --restart unless-stopped \
  --network host --cap-add NET_RAW \
  -v ${INSTALL_DIR}/data/conf:/app/conf \
  -v ${INSTALL_DIR}/data/db:/app/db \
  -v ${INSTALL_DIR}/data/logs:/app/logs \
  pingmesh:latest -p 8899 -name "${NAME}" -addr "${ADDR}" \
  -join ${JOIN_URL} -token ${TOKEN} -masters ${MASTERS}
sleep 6
curl -sf http://127.0.0.1:8899/healthz && echo " ${NAME} OK" || { docker logs pingmesh 2>&1 | tail -8; exit 1; }
