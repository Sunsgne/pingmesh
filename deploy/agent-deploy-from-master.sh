#!/usr/bin/env bash
# 从主控制节点通过内网 SSH 批量部署 Agent (在主节点上执行)
# 用法: 在主节点 sin1-sg2 上运行: bash agent-deploy-from-master.sh
set -uo pipefail

PASSWORD='Monitor@678!9981'
MASTER_INTERNAL='10.100.1.8'
BACKUP_INTERNAL='10.100.1.2'
JOIN_TOKEN='smartping'
INSTALL_DIR='/opt/pingmesh'
BINARY='/opt/pingmesh-docker/src/pingmesh-bin.gz'

info()  { echo -e "\033[32m[agent]\033[0m $*"; }
err()   { echo -e "\033[31m[agent]\033[0m $*"; }

ssh_run() {
  local host="$1" port="${2:-22}"; shift 2
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=15 -p "$port" "root@${host}" "$@"
}

deploy_agent() {
  local host="$1" port="$2" name="$3" addr="$4"
  info "部署 Agent ${name} -> ${host}:${port} (内网 ${addr})"
  if ! ssh_run "$host" "$port" "echo ok" 2>/dev/null; then
    err "  SSH 连接失败, 跳过 ${name}"
    return 1
  fi
  sshpass -p "$PASSWORD" scp -o StrictHostKeyChecking=no -P "$port" "$BINARY" "root@${host}:/tmp/pingmesh-bin.gz"
  ssh_run "$host" "$port" "
    set -e
    apt-get install -y -qq libcap2-bin psmisc curl >/dev/null 2>&1 || true
    mkdir -p ${INSTALL_DIR}
    systemctl stop pingmesh 2>/dev/null || true
    pkill -f '${INSTALL_DIR}/pingmesh' 2>/dev/null || true
    rm -rf ${INSTALL_DIR}/conf ${INSTALL_DIR}/db
    gunzip -c /tmp/pingmesh-bin.gz > ${INSTALL_DIR}/pingmesh
    rm -f /tmp/pingmesh-bin.gz
    chmod 755 ${INSTALL_DIR}/pingmesh
    setcap cap_net_raw+ep ${INSTALL_DIR}/pingmesh 2>/dev/null || true
    cat > ${INSTALL_DIR}/pingmesh.env <<ENV
PINGMESH_OPTS=-p 8899 -join http://${MASTER_INTERNAL}:8899 -token ${JOIN_TOKEN} -name ${name} -addr ${addr} -masters ${MASTER_INTERNAL}:8899,${BACKUP_INTERNAL}:8899
ENV
    chmod 600 ${INSTALL_DIR}/pingmesh.env
    cat > /etc/systemd/system/pingmesh.service <<'UNIT'
[Unit]
Description=ZENLENET PingMesh Agent
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
WorkingDirectory=/opt/pingmesh
EnvironmentFile=-/opt/pingmesh/pingmesh.env
ExecStart=/opt/pingmesh/pingmesh \$PINGMESH_OPTS
Restart=always
RestartSec=3
LimitNOFILE=65536
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/opt/pingmesh
[Install]
WantedBy=multi-user.target
UNIT
    systemctl daemon-reload
    systemctl enable pingmesh >/dev/null
    systemctl restart pingmesh
    ok=0
    for w in 1 2 3 4 5 6 7 8 9 10; do
      sleep 3
      curl -s --max-time 5 http://127.0.0.1:8899/healthz 2>/dev/null | grep -q ok && ok=1 && break
    done
    [ \"\$ok\" = \"1\" ]
  " && info "  ${name} 成功" || { err "  ${name} 失败"; return 1; }
}

# 准备二进制包
if [ ! -f "$BINARY" ]; then
  docker run --rm pingmesh:local cat /app/pingmesh | gzip > /tmp/pingmesh-bin.gz
  BINARY=/tmp/pingmesh-bin.gz
fi

AGENTS=(
  "106.75.160.24 20001 can-xxg 10.100.1.4"
  "42.240.152.238 20001 can-hxy 10.100.1.19"
  "217.217.29.250 22 fra 10.100.1.7"
  "104.251.226.39 20001 hkg2 10.100.1.12"
  "163.53.245.90 22 hkg1 10.100.1.3"
  "163.53.245.136 20001 hkg3 10.100.1.13"
  "149.119.41.156 22 lax 10.100.1.10"
  "106.38.203.8 20001 pek 10.100.1.15"
  "61.172.165.219 20001 gds 10.100.1.20"
  "113.31.161.79 20001 sjhl 10.100.1.5"
  "109.244.32.190 20001 xtl 10.100.1.1"
  "149.51.125.226 20001 sin2-gs 10.100.1.11"
  "59.36.211.118 20001 szx 10.100.1.17"
  "192.169.120.12 22 tpe 10.100.1.18"
  "61.172.165.219 20001 tyo-7 10.100.1.9"
)

apt-get install -y -qq sshpass >/dev/null 2>&1 || true
OK=0 FAIL=0
for entry in "${AGENTS[@]}"; do
  read -r host port name addr <<< "$entry"
  if deploy_agent "$host" "$port" "$name" "$addr"; then OK=$((OK+1)); else FAIL=$((FAIL+1)); fi
  sleep 5
done
info "完成: 成功 ${OK}, 失败 ${FAIL}"
