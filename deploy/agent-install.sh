#!/usr/bin/env bash
# 单节点 Agent 部署 (在目标机上执行)
set -euo pipefail
NAME="$1"
ADDR="$2"
JOIN_URL="http://43.229.152.51:8899"
TOKEN="Zenlenet@PingMesh2026"
MASTERS="43.229.152.51:8899,163.53.245.91:8899"
INSTALL_DIR=/opt/pingmesh

bash /tmp/pingmesh-src/deploy/docker-setup.sh agent
mkdir -p ${INSTALL_DIR}
rsync -a /tmp/pingmesh-src/ ${INSTALL_DIR}/src/
cd ${INSTALL_DIR}/src
if ! docker image inspect pingmesh:latest >/dev/null 2>&1; then
  docker build -t pingmesh:latest --build-arg GIT_COMMIT=deploy --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) .
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
curl -sf http://127.0.0.1:8899/healthz && echo " ${NAME} OK" || { docker logs pingmesh 2>&1 | tail -5; exit 1; }
