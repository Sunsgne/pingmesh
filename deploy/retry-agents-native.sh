#!/usr/bin/env bash
# 非 Docker 方式部署 Agent (install.sh), 适用于 Docker 安装/拉取失败的节点
set -euo pipefail
SSH_PASS='Monitor@678!9981'
JOIN='http://43.229.152.51:8899'
TOKEN='Zenlenet@PingMesh2026'
MASTERS='43.229.152.51:8899,163.53.245.91:8899'
TAR=/tmp/pingmesh-src.tgz
tar -C /workspace --exclude=.git --exclude=data -czf "$TAR" .

deploy_native() {
  local host="$1" port="$2" name="$3" addr="$4"
  echo ">>> Native ${name} @ ${host}:${port}"
  sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no -P "$port" "$TAR" "root@${host}:/tmp/pingmesh-src.tgz"
  sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -p "$port" "root@${host}" bash -s "$name" "$addr" <<'REMOTE' || { echo "FAIL: '"$name"'"; return 0; }
set -euo pipefail
NAME="$1"; ADDR="$2"
rm -rf /tmp/pingmesh-src && mkdir -p /tmp/pingmesh-src
tar -xzf /tmp/pingmesh-src.tgz -C /tmp/pingmesh-src
cd /tmp/pingmesh-src
bash deploy/install.sh --join http://43.229.152.51:8899 --token Zenlenet@PingMesh2026 \
  --name "$NAME" --addr "$ADDR" --masters 43.229.152.51:8899,163.53.245.91:8899 --yes
sleep 3
curl -sf http://127.0.0.1:8899/healthz
REMOTE
  echo "OK: $name"
}

deploy_native 106.75.160.24 20002 can-xxg 106.75.160.24 || true
deploy_native 42.240.152.238 20002 can-hxy 42.240.152.238 || true
deploy_native 106.38.203.8 20002 pek 106.38.203.8 || true
deploy_native 61.172.165.219 20002 gds 61.172.165.219 || true
deploy_native 113.31.161.79 20002 sjhl 113.31.161.79 || true
deploy_native 109.244.32.190 20002 xtl 109.244.32.190 || true
deploy_native 59.36.211.118 20002 szx 59.36.211.118 || true
deploy_native 61.172.165.219 20002 tyo-7 61.172.165.219 || true
echo "=== NATIVE RETRY DONE ==="
