#!/usr/bin/env bash
# =============================================================================
#  ZENLENET PingMesh - Ubuntu 24.04 一键部署脚本
#  (兼容 Ubuntu 22.04 / Debian 12 等 apt 系发行版)
#
#  部署主节点:
#    sudo ./deploy/install.sh
#
#  部署 Agent 节点(加入主节点, 自动全互联组网):
#    sudo ./deploy/install.sh --join http://<主节点IP>:8899 --token <接入令牌> --name 北京机房
#
#  其他:
#    sudo ./deploy/install.sh --port 8899          # 指定端口
#    sudo ./deploy/install.sh --dir /opt/pingmesh  # 指定安装目录
#    sudo ./deploy/install.sh --update             # 在线更新到最新版(保留服务配置与数据)
#    sudo ./deploy/install.sh --uninstall          # 卸载(保留数据目录)
#
#  也支持远程一键安装(无需提前克隆仓库):
#    curl -fsSL https://raw.githubusercontent.com/Sunsgne/smartping/master/deploy/install.sh | sudo bash
# =============================================================================
set -euo pipefail

REPO_URL="https://github.com/Sunsgne/smartping"
INSTALL_DIR=/opt/pingmesh
SERVICE=pingmesh
JOIN="" TOKEN="" NAME="" PORT="" UNINSTALL=0 UPDATE=0

info()  { echo -e "\033[32m[PingMesh]\033[0m $*"; }
warn()  { echo -e "\033[33m[PingMesh]\033[0m $*"; }
fatal() { echo -e "\033[31m[PingMesh]\033[0m $*"; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --join)      JOIN="$2";  shift 2 ;;
    --token)     TOKEN="$2"; shift 2 ;;
    --name)      NAME="$2";  shift 2 ;;
    --port)      PORT="$2";  shift 2 ;;
    --dir)       INSTALL_DIR="$2"; shift 2 ;;
    --update)    UPDATE=1; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    -h|--help)   grep '^#' "$0" | head -20; exit 0 ;;
    *) fatal "未知参数: $1 (使用 --help 查看用法)" ;;
  esac
done

[[ $EUID -eq 0 ]] || fatal "请使用 root 运行: sudo $0 $*"

HAS_SYSTEMD=0
[[ -d /run/systemd/system ]] && HAS_SYSTEMD=1

# ---------------------------------------------------------------- 卸载 ----
if [[ $UNINSTALL -eq 1 ]]; then
  info "停止并卸载 ${SERVICE} ..."
  if [[ $HAS_SYSTEMD -eq 1 ]]; then
    systemctl stop "$SERVICE" 2>/dev/null || true
    systemctl disable "$SERVICE" 2>/dev/null || true
    rm -f "/etc/systemd/system/${SERVICE}.service"
    systemctl daemon-reload
  else
    pkill -f "${INSTALL_DIR}/pingmesh" 2>/dev/null || true
  fi
  rm -f "${INSTALL_DIR}/pingmesh"
  info "已卸载二进制与服务。数据目录保留在 ${INSTALL_DIR}/{conf,db,logs},"
  info "如需彻底清除请手动执行: rm -rf ${INSTALL_DIR}"
  exit 0
fi

# ---------------------------------------------------------- 1. 系统检查 ----
. /etc/os-release 2>/dev/null || true
info "检测到系统: ${PRETTY_NAME:-unknown}"
if [[ "${ID:-}" != "ubuntu" && "${ID_LIKE:-}" != *debian* ]]; then
  warn "本脚本针对 Ubuntu 24.04 编写, 当前系统未经验证, 继续尝试..."
fi
command -v apt-get >/dev/null || fatal "未找到 apt-get, 请参考 readme 手动部署"

# ---------------------------------------------------------- 2. 安装依赖 ----
info "[1/5] 安装依赖 (git / golang / libcap2-bin) ..."
export DEBIAN_FRONTEND=noninteractive
NEED=()
command -v git >/dev/null     || NEED+=(git)
command -v setcap >/dev/null  || NEED+=(libcap2-bin)
command -v go >/dev/null      || NEED+=(golang-go)
NEED+=(ca-certificates)
apt-get update -qq
apt-get install -y -qq "${NEED[@]}" >/dev/null

GO_VER=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1)
if [[ $(echo "$GO_VER" | cut -d. -f1) -lt 1 || $(echo "$GO_VER" | cut -d. -f2) -lt 22 ]]; then
  fatal "需要 Go >= 1.22, 当前为 ${GO_VER}。Ubuntu 24.04 可直接 apt install golang-go; 其他系统请从 https://go.dev/dl 安装"
fi
info "Go 版本: $(go version | awk '{print $3}')"

# ------------------------------------------------------ 3. 获取源码并编译 ----
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-/dev/null}")" 2>/dev/null && pwd || echo /tmp)"
if [[ -f "${SCRIPT_DIR}/../go.mod" ]]; then
  SRC_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
  info "[2/5] 使用本地仓库: ${SRC_DIR}"
else
  SRC_DIR=/tmp/pingmesh-src
  info "[2/5] 克隆源码到 ${SRC_DIR} ..."
  rm -rf "$SRC_DIR"
  git clone -q --depth 1 "$REPO_URL" "$SRC_DIR"
fi

info "[3/5] 编译 (CGO_ENABLED=0, 纯静态二进制) ..."
cd "$SRC_DIR"
CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/pingmesh-bin ./src

# ---------------------------------------------------------- 4. 安装部署 ----
info "[4/5] 安装到 ${INSTALL_DIR} ..."
mkdir -p "$INSTALL_DIR"
# 升级场景: 先停旧进程再覆盖
if [[ $HAS_SYSTEMD -eq 1 ]]; then systemctl stop "$SERVICE" 2>/dev/null || true
else pkill -f "${INSTALL_DIR}/pingmesh" 2>/dev/null || true; fi
install -m 755 /tmp/pingmesh-bin "${INSTALL_DIR}/pingmesh"
rm -f /tmp/pingmesh-bin
setcap cap_net_raw+ep "${INSTALL_DIR}/pingmesh" || warn "setcap 失败, 将依赖 root 权限运行"

EXTRA_ARGS=""
[[ -n "$PORT" ]] && EXTRA_ARGS+=" -p $PORT"
if [[ -n "$JOIN" ]]; then
  [[ -n "$TOKEN" && -n "$NAME" ]] || fatal "--join 需要同时提供 --token 与 --name"
  # 每次启动自动注册并同步主节点配置(幂等, 自愈)
  EXTRA_ARGS+=" -join $JOIN -token $TOKEN -name $NAME"
fi

# ---------------------------------------------------------- 5. 启动服务 ----
if [[ $UPDATE -eq 1 ]]; then
  info "[5/5] 更新模式: 保留现有服务配置, 仅替换二进制并重启 ..."
  if [[ $HAS_SYSTEMD -eq 1 && -f /etc/systemd/system/${SERVICE}.service ]]; then
    systemctl restart "$SERVICE"
    sleep 1
    systemctl is-active --quiet "$SERVICE" && info "更新完成, 服务已重启 (版本: $(${INSTALL_DIR}/pingmesh -v))" \
      || fatal "服务重启失败, 请查看: journalctl -u ${SERVICE} -n 50"
  else
    cd "$INSTALL_DIR"
    nohup "${INSTALL_DIR}/pingmesh" >/dev/null 2>&1 &
    sleep 1
    pgrep -f "${INSTALL_DIR}/pingmesh" >/dev/null && info "更新完成, 进程已重启" || fatal "重启失败"
  fi
  exit 0
fi

info "[5/5] 配置并启动服务 ..."
if [[ $HAS_SYSTEMD -eq 1 ]]; then
  cat > "/etc/systemd/system/${SERVICE}.service" <<UNIT
[Unit]
Description=ZENLENET PingMesh - network quality monitor
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/pingmesh${EXTRA_ARGS}
Restart=always
RestartSec=5
AmbientCapabilities=CAP_NET_RAW
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
UNIT
  systemctl daemon-reload
  systemctl enable "$SERVICE" >/dev/null
  systemctl restart "$SERVICE"
  sleep 1
  systemctl is-active --quiet "$SERVICE" && info "服务已启动 (systemctl status ${SERVICE})" \
    || fatal "服务启动失败, 请查看: journalctl -u ${SERVICE} -n 50"
else
  warn "未检测到 systemd, 以后台进程方式启动"
  cd "$INSTALL_DIR"
  nohup "${INSTALL_DIR}/pingmesh"${EXTRA_ARGS} >/dev/null 2>&1 &
  sleep 1
  pgrep -f "${INSTALL_DIR}/pingmesh" >/dev/null && info "进程已启动 (pid: $(pgrep -f "${INSTALL_DIR}/pingmesh" | head -1))" \
    || fatal "启动失败, 请手动执行: cd ${INSTALL_DIR} && ./pingmesh"
fi

# 防火墙放行
HTTP_PORT="${PORT:-8899}"
if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
  ufw allow "${HTTP_PORT}/tcp" >/dev/null && info "ufw 已放行 ${HTTP_PORT}/tcp"
fi

IP=$(hostname -I 2>/dev/null | awk '{print $1}')
echo
info "=============================================="
info " 部署完成!"
info " 访问地址:  http://${IP:-<本机IP>}:${HTTP_PORT}"
info " 默认账号:  admin / admin123  (登录后请立即修改)"
[[ -n "$JOIN" ]] && info " 本节点为 Agent, 已配置自动加入 ${JOIN}"
if [[ $HAS_SYSTEMD -eq 1 ]]; then
  info " 常用命令:  systemctl status|restart|stop ${SERVICE}"
else
  info " 重启方式:  pkill -f ${INSTALL_DIR}/pingmesh && cd ${INSTALL_DIR} && nohup ./pingmesh &"
fi
if [[ -f "${BASH_SOURCE[0]:-}" ]]; then
  info " 卸载:      sudo ${BASH_SOURCE[0]} --uninstall"
else
  info " 卸载:      curl -fsSL ${REPO_URL}/raw/master/deploy/install.sh | sudo bash -s -- --uninstall"
fi
info "=============================================="
