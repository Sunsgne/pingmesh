#!/usr/bin/env bash
# =============================================================================
# CAN-HXY 手动部署脚本
# 在 can-hxy 机器上以 root 执行（公网 42.240.152.238:20001 / 内网 10.100.1.19）
#
# 用法:
#   1) 先把 pingmesh 二进制放到本机（见下方「获取二进制」）
#   2) 上传本脚本到机器: scp -P 20001 deploy/can-hxy-manual-install.sh root@42.240.152.238:/root/
#   3) SSH 登录后执行: bash /root/can-hxy-manual-install.sh
#
# 或在 can-hxy 上一条命令（能从内网访问主节点时）:
#   curl -fsSL https://raw.githubusercontent.com/.../can-hxy-manual-install.sh | bash
# =============================================================================
set -euo pipefail

# ----- 节点参数（与集群一致，一般不用改）-----
NODE_NAME='CAN-HXY'
NODE_ADDR='10.100.1.19'
NODE_GROUP='ZENLENET'
MASTER_INTERNAL='10.100.1.8'
BACKUP_INTERNAL='10.100.1.3'
JOIN_TOKEN="${PINGMESH_JOIN_TOKEN:-}"
LISTEN_PORT='8899'
INSTALL_DIR='/opt/pingmesh'
TZ_NAME='Asia/Shanghai'

# 二进制来源（按实际情况选一种，脚本会依次尝试）
# 方式 A: 已手动 scp 到 /tmp/pingmesh-bin 或 /tmp/pingmesh-bin.gz
# 方式 B: 从内网主节点拉取（can-hxy 能 ping 通 10.100.1.8 时自动尝试）
BINARY_LOCAL=''   # 可填 /root/pingmesh-bin.gz；留空则自动探测

info()  { echo -e "\033[32m[can-hxy]\033[0m $*"; }
warn()  { echo -e "\033[33m[can-hxy]\033[0m $*"; }
fatal() { echo -e "\033[31m[can-hxy]\033[0m $*"; exit 1; }

[[ $(id -u) -eq 0 ]] || fatal "请使用 root 执行"
[[ -n "$JOIN_TOKEN" ]] || fatal "请先设置接入令牌: export PINGMESH_JOIN_TOKEN='你的主控接入令牌'"

info "===== 1/6 系统准备 ====="
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq libcap2-bin psmisc curl tzdata ca-certificates iproute2 >/dev/null
timedatectl set-timezone "${TZ_NAME}" 2>/dev/null || {
  ln -sf "/usr/share/zoneinfo/${TZ_NAME}" /etc/localtime
  echo "${TZ_NAME}" > /etc/timezone
}
info "时区: $(timedatectl show -p Timezone --value 2>/dev/null || cat /etc/timezone)"

info "===== 2/6 网络检查 ====="
if ! ip -4 addr show | grep -q "${NODE_ADDR}"; then
  warn "本机网卡未发现 ${NODE_ADDR}，请确认内网 IP 已配置（Agent 必须用内网地址注册）"
  ip -4 addr show | grep 'inet ' || true
else
  info "内网 IP ${NODE_ADDR} 已就绪"
fi
if ping -c 2 -W 3 "${MASTER_INTERNAL}" >/dev/null 2>&1; then
  info "主节点 ${MASTER_INTERNAL} 可达"
else
  warn "暂时 ping 不通主节点 ${MASTER_INTERNAL}，请检查 VPN/内网路由；仍会继续安装，启动后会自动重试加入"
fi

info "===== 3/6 获取 pingmesh 二进制 ====="
resolve_binary() {
  local src="$1" dst="${INSTALL_DIR}/pingmesh"
  mkdir -p "${INSTALL_DIR}"
  if [[ "$src" == *.gz ]]; then
    gunzip -c "$src" > "$dst"
  else
    cp -f "$src" "$dst"
  fi
  chmod 755 "$dst"
  setcap cap_net_raw+ep "$dst" 2>/dev/null || warn "setcap 失败，请确认以 root 运行且已安装 libcap2-bin"
}

FOUND=''
for cand in "${BINARY_LOCAL}" /tmp/pingmesh-bin.gz /tmp/pingmesh-bin /root/pingmesh-bin.gz /root/pingmesh-bin; do
  [[ -n "$cand" && -f "$cand" ]] && FOUND="$cand" && break
done

if [[ -z "$FOUND" ]] && ping -c 1 -W 3 "${MASTER_INTERNAL}" >/dev/null 2>&1; then
  info "尝试从主节点 ${MASTER_INTERNAL} 拉取二进制..."
  if [[ -n "${PINGMESH_SSH_PASSWORD:-}" ]] && command -v sshpass >/dev/null 2>&1; then
    if sshpass -p "${PINGMESH_SSH_PASSWORD}" scp -o StrictHostKeyChecking=no \
        "root@${MASTER_INTERNAL}:/tmp/pingmesh-bin.gz" /tmp/pingmesh-bin.gz 2>/dev/null; then
      FOUND=/tmp/pingmesh-bin.gz
    fi
  else
    warn "未设置 PINGMESH_SSH_PASSWORD 或未安装 sshpass，跳过从主节点自动拉取二进制"
  fi
fi

[[ -n "$FOUND" ]] || fatal "$(cat <<'EOF'
未找到 pingmesh 二进制。请先获取后重试:

【推荐】在你的电脑上，从主节点下载再传到 can-hxy:
  scp root@<主节点公网IP>:/tmp/pingmesh-bin.gz /tmp/
  scp -P 20001 /tmp/pingmesh-bin.gz root@42.240.152.238:/tmp/

或从主节点 Docker 导出:
  ssh root@<主节点公网IP> 'docker cp pingmesh:/app/pingmesh /tmp/pingmesh-bin && gzip -c /tmp/pingmesh-bin > /tmp/pingmesh-bin.gz'

然后重新运行本脚本。
EOF
)"
info "使用二进制: ${FOUND}"
resolve_binary "${FOUND}"

info "===== 4/6 停止旧服务 ====="
systemctl stop pingmesh 2>/dev/null || true
pkill -f "${INSTALL_DIR}/pingmesh" 2>/dev/null || true
docker rm -f pingmesh-agent pingmesh 2>/dev/null || true
sleep 1

info "===== 5/6 安装 systemd 服务 ====="
cat > /etc/systemd/system/pingmesh.service <<UNIT
[Unit]
Description=ZENLENET PingMesh Agent (${NODE_NAME})
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/pingmesh \\
  -p ${LISTEN_PORT} \\
  -join http://${MASTER_INTERNAL}:${LISTEN_PORT} \\
  -token ${JOIN_TOKEN} \\
  -name ${NODE_NAME} \\
  -addr ${NODE_ADDR} \\
  -masters ${MASTER_INTERNAL}:${LISTEN_PORT},${BACKUP_INTERNAL}:${LISTEN_PORT}
Restart=always
RestartSec=3
LimitNOFILE=65536
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable pingmesh
systemctl restart pingmesh

info "===== 6/6 健康检查 ====="
ok=0
for i in $(seq 1 15); do
  sleep 3
  if curl -s --max-time 5 "http://127.0.0.1:${LISTEN_PORT}/healthz" 2>/dev/null | grep -q ok; then
    ok=1
    break
  fi
  echo "  等待启动... (${i}/15)"
done

if [[ "$ok" -ne 1 ]]; then
  warn "本地 healthz 未通过，查看日志:"
  journalctl -u pingmesh -n 40 --no-pager || true
  fatal "Agent 未正常启动，请检查内网连通性与主节点令牌"
fi

HEALTH=$(curl -s --max-time 5 "http://127.0.0.1:${LISTEN_PORT}/healthz")
info "本地 healthz: ${HEALTH}"
systemctl status pingmesh --no-pager -l | head -15

cat <<'POST'

================================================================================
Agent 本机安装完成。还需在主控 Web 添加节点（否则不会出现在拓扑/Pingmesh）:

1. 打开主控 Web（HTTPS 443）并用管理员账号登录
2. 系统配置 → 节点管理 → 「+ 添加节点」
3. 填写:
     名称:   CAN-HXY
     地址:   10.100.1.19
     分组:   ZENLENET
     类型:   探测节点（会主动 PING 其他节点）
4. 监测关系: 勾选「全互联」或手动选择要监测的节点
5. 保存后等待 1~2 分钟，在「集群状态」确认 CAN-HXY 在线

【批量导入 JSON】系统配置 → 节点管理 → 批量导入，粘贴 deploy/can-hxy-node.json

【验证】在主节点执行:
  curl -s "http://10.100.1.8:8899/api/proxy.json?g=http://10.100.1.19:8899/healthz"

【常用命令】
  systemctl status pingmesh
  journalctl -u pingmesh -f
  curl http://127.0.0.1:8899/healthz
================================================================================
POST
