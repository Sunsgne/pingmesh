#!/usr/bin/env bash
# 预编译二进制 + systemd 部署 Agent (无需 Docker/Go 网络)
set -euo pipefail
SSH_PASS='Monitor@678!9981'
BIN=/tmp/pingmesh-bin
JOIN='http://43.229.152.51:8899'
TOKEN='Zenlenet@PingMesh2026'
MASTERS='43.229.152.51:8899,163.53.245.91:8899'
INSTALL_DIR=/opt/pingmesh

[[ -f "$BIN" ]] || { echo "missing $BIN"; exit 1; }

deploy_bin() {
  local host="$1" port="$2" name="$3" addr="$4"
  echo ">>> Binary deploy ${name} @ ${host}:${port}"
  if ! sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -p "$port" "root@${host}" "echo ok" 2>/dev/null; then
    echo "SKIP SSH: ${name}"; return 0
  fi
  sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no -P "$port" "$BIN" "root@${host}:/tmp/pingmesh-bin"
  sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -p "$port" "root@${host}" bash -s "$name" "$addr" <<'REMOTE' && echo "OK: '"$name"'" || echo "FAIL: '"$name"'"
set -euo pipefail
NAME="$1"; ADDR="$2"
INSTALL_DIR=/opt/pingmesh
apt-get install -y -qq libcap2-bin >/dev/null 2>&1 || true
mkdir -p "$INSTALL_DIR"
install -m 755 /tmp/pingmesh-bin "$INSTALL_DIR/pingmesh"
setcap cap_net_raw+ep "$INSTALL_DIR/pingmesh" || true
cat > "$INSTALL_DIR/pingmesh.env" <<ENV
PINGMESH_OPTS=-p 8899 -name ${NAME} -addr ${ADDR} -join http://43.229.152.51:8899 -token Zenlenet@PingMesh2026 -masters 43.229.152.51:8899,163.53.245.91:8899
ENV
chmod 600 "$INSTALL_DIR/pingmesh.env"
cat > /etc/systemd/system/pingmesh.service <<'UNIT'
[Unit]
Description=ZENLENET PingMesh Agent
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
WorkingDirectory=/opt/pingmesh
EnvironmentFile=-/opt/pingmesh/pingmesh.env
ExecStart=/opt/pingmesh/pingmesh $PINGMESH_OPTS
Restart=always
RestartSec=3
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW
NoNewPrivileges=true
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now pingmesh
sleep 5
curl -sf http://127.0.0.1:8899/healthz
REMOTE
}

PENDING=(
  "106.75.160.24:20002:can-xxg:106.75.160.24"
  "42.240.152.238:20002:can-hxy:42.240.152.238"
  "106.38.203.8:20002:pek:106.38.203.8"
  "61.172.165.219:20002:gds:61.172.165.219"
  "113.31.161.79:20002:sjhl:113.31.161.79"
  "109.244.32.190:20002:xtl:109.244.32.190"
  "59.36.211.118:20002:szx:59.36.211.118"
  "61.172.165.219:20002:tyo-7:61.172.165.219"
)
for e in "${PENDING[@]}"; do
  IFS=: read -r h p n a <<< "$e"
  deploy_bin "$h" "$p" "$n" "$a"
done
echo "=== BINARY DEPLOY DONE ==="
