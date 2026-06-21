#!/usr/bin/env bash
# 从本地仓库批量部署 PingMesh 到多台服务器
set -euo pipefail

SSH_PASS='Monitor@678!9981'
CLUSTER_TOKEN='Zenlenet@PingMesh2026'
PRIMARY_IP='43.229.152.51'
BACKUP_IP='163.53.245.91'
JOIN_URL="http://${PRIMARY_IP}:8899"
MASTERS="${PRIMARY_IP}:8899,${BACKUP_IP}:8899"
ADMIN_USER='xiaoqiang'
ADMIN_PASS='njupt@NJ-5353'
INSTALL_DIR=/opt/pingmesh
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

info()  { echo -e "\033[36m[remote]\033[0m $*"; }
warn()  { echo -e "\033[33m[remote]\033[0m $*"; }
fatal() { echo -e "\033[31m[remote]\033[0m $*"; exit 1; }

command -v sshpass >/dev/null || fatal "需要 sshpass"

ssh_run() {
  local host="$1" port="${2:-22}"; shift 2
  sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -p "$port" "root@${host}" "$@"
}

scp_to() {
  local host="$1" port="${2:-22}"; shift 2
  sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no -P "$port" -r "$@" "root@${host}:"
}

sync_repo() {
  local host="$1" port="${2:-22}"
  info "同步代码到 ${host}:${port} ..."
  ssh_run "$host" "$port" "rm -rf /tmp/pingmesh-src && mkdir -p /tmp/pingmesh-src"
  tar -C "$REPO_ROOT" --exclude='.git' --exclude='data' --exclude='data-agent' -czf /tmp/pingmesh-src.tgz .
  scp_to "$host" "$port" /tmp/pingmesh-src.tgz /tmp/
  ssh_run "$host" "$port" "tar -xzf /tmp/pingmesh-src.tgz -C /tmp/pingmesh-src && rm -f /tmp/pingmesh-src.tgz"
}

deploy_control() {
  local host="$1" name="$2" addr="$3" role="$4" port="${5:-22}" extra_args="${6:-}"
  info "==== 部署控制节点 ${name} (${host}) role=${role} ===="
  sync_repo "$host" "$port"
  ssh_run "$host" "$port" bash -s <<REMOTE
set -euo pipefail
bash /tmp/pingmesh-src/deploy/docker-setup.sh ${role}
mkdir -p ${INSTALL_DIR}
rsync -a /tmp/pingmesh-src/ ${INSTALL_DIR}/src/
cd ${INSTALL_DIR}/src
docker build -t pingmesh:latest \
  --build-arg GIT_COMMIT=deploy \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) .
docker rm -f pingmesh 2>/dev/null || true
docker run -d --name pingmesh --restart unless-stopped \
  --network host --cap-add NET_RAW \
  -v ${INSTALL_DIR}/data/conf:/app/conf \
  -v ${INSTALL_DIR}/data/db:/app/db \
  -v ${INSTALL_DIR}/data/logs:/app/logs \
  pingmesh:latest -p 8899 -name ${name} -addr ${addr} ${extra_args}
sleep 3
curl -sf http://127.0.0.1:8899/healthz || (docker logs pingmesh 2>&1 | tail -20; exit 1)
REMOTE
  info "控制节点 ${name} 部署完成"
}

setup_admin_user() {
  local host="$1" port="${2:-22}"
  info "配置默认用户 ${ADMIN_USER} @ ${host} ..."
  local hash
  hash=$(python3 -c "import bcrypt; print(bcrypt.hashpw(b'${ADMIN_PASS}', bcrypt.gensalt()).decode())")
  ssh_run "$host" "$port" bash -s <<REMOTE
set -euo pipefail
DB=${INSTALL_DIR}/data/db/database.db
for i in \$(seq 1 30); do
  [[ -f "\$DB" ]] && break
  sleep 2
done
[[ -f "\$DB" ]] || { echo "database not ready"; exit 1; }
apt-get install -y -qq sqlite3 >/dev/null 2>&1 || true
sqlite3 "\$DB" "DELETE FROM users WHERE username='admin';"
sqlite3 "\$DB" "DELETE FROM users WHERE username='${ADMIN_USER}';"
sqlite3 "\$DB" "INSERT INTO users(username,password,role,created_at) VALUES('${ADMIN_USER}','${hash}','admin',datetime('now'));"
# 设置集群接入令牌
CONF=${INSTALL_DIR}/data/conf/config.json
if [[ -f "\$CONF" ]]; then
  python3 - <<'PY'
import json
p="${CONF}"
with open(p) as f: c=json.load(f)
c["Password"]="${CLUSTER_TOKEN}"
with open(p,"w") as f: json.dump(c,f,indent=2,ensure_ascii=False)
PY
  docker restart pingmesh
  sleep 2
fi
REMOTE
}

deploy_agent() {
  local host="$1" port="$2" name="$3" addr="$4"
  info "---- Agent ${name} (${addr}) @ ${host}:${port} ----"
  sync_repo "$host" "$port"
  ssh_run "$host" "$port" bash -s <<REMOTE
set -euo pipefail
bash /tmp/pingmesh-src/deploy/docker-setup.sh agent
mkdir -p ${INSTALL_DIR}
rsync -a /tmp/pingmesh-src/ ${INSTALL_DIR}/src/
cd ${INSTALL_DIR}/src
docker build -t pingmesh:latest \
  --build-arg GIT_COMMIT=deploy \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) . 2>/dev/null || docker build -t pingmesh:latest .
docker rm -f pingmesh 2>/dev/null || true
docker run -d --name pingmesh --restart unless-stopped \
  --network host --cap-add NET_RAW \
  -v ${INSTALL_DIR}/data/conf:/app/conf \
  -v ${INSTALL_DIR}/data/db:/app/db \
  -v ${INSTALL_DIR}/data/logs:/app/logs \
  pingmesh:latest -p 8899 -name ${name} -addr ${addr} \
  -join ${JOIN_URL} -token ${CLUSTER_TOKEN} -masters ${MASTERS}
sleep 4
curl -sf http://127.0.0.1:8899/healthz >/dev/null && echo OK || docker logs pingmesh 2>&1 | tail -10
REMOTE
}

# ---- 主控制节点 ----
deploy_control "$PRIMARY_IP" "sin1-sg2" "$PRIMARY_IP" "master" 22 "-masters ${BACKUP_IP}:8899"

setup_admin_user "$PRIMARY_IP" 22

# ---- 备控制节点 (加入主集群, 开启 Web 作为控制节点) ----
deploy_control "$BACKUP_IP" "hkg2" "$BACKUP_IP" "backup" 22 \
  "-join ${JOIN_URL} -token ${CLUSTER_TOKEN} -masters ${MASTERS} -webui"

# ---- Agent 节点 ----
# 格式: host port name public_addr
AGENTS=(
  "106.75.160.24 20002 can-xxg 106.75.160.24"
  "42.240.152.238 20002 can-hxy 42.240.152.238"
  "217.217.29.251 22 fra 217.217.29.251"
  "104.251.226.39 20002 hkg3 104.251.226.39"
  "163.53.245.136 20002 hkg4 163.53.245.136"
  "149.119.41.157 22 lax 149.119.41.157"
  "106.38.203.8 20002 pek 106.38.203.8"
  "61.172.165.219 20002 gds 61.172.165.219"
  "113.31.161.79 20002 sjhl 113.31.161.79"
  "109.244.32.190 20002 xtl 109.244.32.190"
  "149.51.125.226 20002 sin2-gs 149.51.125.226"
  "59.36.211.118 20002 szx 59.36.211.118"
  "192.169.120.13 22 tpe 192.169.120.13"
  "43.230.52.243 22 tyo-8 43.230.52.243"
  "61.172.165.219 20002 tyo-7 61.172.165.219"
)

for entry in "${AGENTS[@]}"; do
  read -r h p n a <<< "$entry"
  deploy_agent "$h" "$p" "$n" "$a" || warn "Agent ${n} 部署失败, 继续下一个"
done

info "============================================"
info "全部部署流程执行完毕"
info "主控 HTTPS: https://${PRIMARY_IP}/"
info "备控 HTTPS: https://${BACKUP_IP}/"
info "登录账号: ${ADMIN_USER} / ${ADMIN_PASS}"
info "集群令牌: ${CLUSTER_TOKEN}"
info "============================================"
