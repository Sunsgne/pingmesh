#!/usr/bin/env bash
# =============================================================================
#  ZENLENET PingMesh - 一键部署脚本 (新手友好 / 自带交互向导)
#  适配 Ubuntu 24.04 / 22.04 · Debian 12 等 apt 系发行版
#
#  最简单用法(无参数运行会进入交互向导):
#    sudo ./deploy/install.sh
#
#  非交互(主节点):
#    sudo ./deploy/install.sh --port 8899 --name 北京主节点 --yes
#
#  非交互(Agent 节点, 加入主节点并自动全互联组网):
#    sudo ./deploy/install.sh --join http://<主节点IP>:8899 --token <接入令牌> --name 上海机房 --yes
#    (Agent 的 Web 页面默认关闭、账户密码随主节点; 加 --webui 可开启本节点页面)
#
#  容灾备选主节点(主挂自动接管, 逗号分隔, 可选):
#    sudo ./deploy/install.sh --join http://<主IP>:8899 --token xxx --name 节点B --masters 10.0.0.2:8899,10.0.0.3:8899
#
#  其他:
#    sudo ./deploy/install.sh --dir /opt/pingmesh   # 指定安装目录
#    sudo ./deploy/install.sh --update              # 在线更新到最新版(保留启动参数与数据)
#    sudo ./deploy/install.sh --uninstall           # 卸载(保留数据目录)
#
#  远程一键安装(无需提前克隆仓库):
#    curl -fsSL https://raw.githubusercontent.com/Sunsgne/smartping/master/deploy/install.sh | sudo bash
#
#  所有启动参数会写入 systemd 引用的环境文件 <安装目录>/pingmesh.env,
#  日后可直接编辑该文件后 `systemctl restart pingmesh` 生效, 无需重跑脚本。
# =============================================================================
set -euo pipefail

REPO_URL="https://github.com/Sunsgne/smartping"
INSTALL_DIR=/opt/pingmesh
SERVICE=pingmesh
REQUIRED_GO_MINOR=21   # >=1.21 即可: 会按 go.mod 自动拉取所需 Go 工具链
JOIN="" TOKEN="" NAME="" PORT="" ADDR="" MASTERS=""
UNINSTALL=0 UPDATE=0 ASSUME_YES=0 ROLE_GIVEN=0 WEBUI=0

info()  { echo -e "\033[32m[PingMesh]\033[0m $*"; }
warn()  { echo -e "\033[33m[PingMesh]\033[0m $*"; }
fatal() { echo -e "\033[31m[PingMesh]\033[0m $*"; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --join)      JOIN="$2";  ROLE_GIVEN=1; shift 2 ;;
    --token)     TOKEN="$2"; shift 2 ;;
    --name)      NAME="$2";  ROLE_GIVEN=1; shift 2 ;;
    --port)      PORT="$2";  ROLE_GIVEN=1; shift 2 ;;
    --addr)      ADDR="$2";  shift 2 ;;
    --masters)   MASTERS="$2"; shift 2 ;;
    --webui)     WEBUI=1; shift ;;
    --dir)       INSTALL_DIR="$2"; shift 2 ;;
    --update)    UPDATE=1; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    -y|--yes)    ASSUME_YES=1; shift ;;
    -h|--help)   grep '^#' "$0" | sed 's/^#//' | head -40; exit 0 ;;
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
  rm -f "${INSTALL_DIR}/pingmesh" "${INSTALL_DIR}/pingmesh.env"
  info "已卸载二进制与服务。数据目录保留在 ${INSTALL_DIR}/{conf,db,logs},"
  info "如需彻底清除请手动执行: rm -rf ${INSTALL_DIR}"
  exit 0
fi

# ------------------------------------------------------- 交互式部署向导 ----
# 无角色参数 + 终端可交互 + 非 --yes/--update 时进入向导, 新手零参数可用。
maybe_wizard() {
  [[ $ROLE_GIVEN -eq 0 && $ASSUME_YES -eq 0 && $UPDATE -eq 0 ]] || return 0
  [[ -e /dev/tty ]] || return 0
  local role master_url
  echo
  info "================  ZENLENET PingMesh 部署向导  ================"
  info "直接回车即使用 [方括号] 中的默认值"
  echo
  printf "  本机角色  1) 主节点(默认)   2) Agent 节点: " > /dev/tty
  read -r role < /dev/tty || true
  printf "  HTTP 端口 [8899]: " > /dev/tty
  read -r PORT < /dev/tty || true; PORT="${PORT:-8899}"
  printf "  节点名称 [$(hostname)]: " > /dev/tty
  read -r NAME < /dev/tty || true; NAME="${NAME:-$(hostname)}"
  if [[ "${role:-1}" == "2" ]]; then
    while [[ -z "${master_url:-}" ]]; do
      printf "  主节点地址 (如 http://1.2.3.4:8899): " > /dev/tty
      read -r master_url < /dev/tty || true
    done
    JOIN="$master_url"
    while [[ -z "$TOKEN" ]]; do
      printf "  接入令牌 (主节点「系统配置→节点接入」查看): " > /dev/tty
      read -r TOKEN < /dev/tty || true
    done
    printf "  本机对外IP (NAT/多网卡时填写, 否则留空自动识别): " > /dev/tty
    read -r ADDR < /dev/tty || true
    printf "  开启本节点 Web 页面? (Agent 默认关闭, 统一在主节点管理) [y/N]: " > /dev/tty
    local wui; read -r wui < /dev/tty || true
    [[ "${wui:-N}" =~ ^[Yy] ]] && WEBUI=1
  fi
  printf "  容灾备选主节点 host:port(逗号分隔, 可留空自动): " > /dev/tty
  read -r MASTERS < /dev/tty || true
  echo
  info "将以如下配置部署:"
  if [[ -n "$JOIN" ]]; then
    info "  角色=Agent 端口=${PORT} 名称=${NAME} 主节点=${JOIN}"
  else
    info "  角色=主节点 端口=${PORT} 名称=${NAME}"
  fi
  [[ -n "$MASTERS" ]] && info "  容灾备选=${MASTERS}"
  printf "  确认开始? [Y/n]: " > /dev/tty
  local ok; read -r ok < /dev/tty || true
  [[ "${ok:-Y}" =~ ^[Nn] ]] && fatal "已取消"
}
maybe_wizard

# ---------------------------------------------------------- 1. 系统检查 ----
. /etc/os-release 2>/dev/null || true
info "检测到系统: ${PRETTY_NAME:-unknown}"
if [[ "${ID:-}" != "ubuntu" && "${ID_LIKE:-}" != *debian* ]]; then
  warn "本脚本针对 Ubuntu/Debian 编写, 当前系统未经验证, 继续尝试..."
fi
command -v apt-get >/dev/null || fatal "未找到 apt-get, 请参考 readme 手动部署"

# ---------------------------------------------------------- 2. 安装依赖 ----
info "[1/5] 安装依赖 (git / golang / libcap2-bin / curl) ..."
export DEBIAN_FRONTEND=noninteractive
NEED=()
command -v git >/dev/null     || NEED+=(git)
command -v setcap >/dev/null  || NEED+=(libcap2-bin)
command -v curl >/dev/null    || NEED+=(curl)
command -v fuser >/dev/null   || NEED+=(psmisc)
NEED+=(ca-certificates)
apt-get update -qq
apt-get install -y -qq "${NEED[@]}" >/dev/null

# Go 工具链: 优先系统 Go(>=1.21 即可, 其余由 go.mod 自动拉取), 否则官方兜底
install_official_go() {
  local arch
  case "$(uname -m)" in
    x86_64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    armv7l|armv6l) arch=armv6l ;;
    *) arch=amd64 ;;
  esac
  local ver
  ver=$(curl -fsSL "https://go.dev/VERSION?m=text" 2>/dev/null | head -1 || true)
  [[ -n "$ver" ]] || ver=go1.25.4
  info "从 go.dev 安装官方 Go 工具链 ${ver} (${arch}) ..."
  local tgz="/tmp/${ver}.linux-${arch}.tar.gz"
  curl -fsSL "https://go.dev/dl/${ver}.linux-${arch}.tar.gz" -o "$tgz" \
    || fatal "下载 Go 失败, 请检查网络或手动安装: https://go.dev/dl"
  rm -rf /usr/local/go && tar -C /usr/local -xzf "$tgz" && rm -f "$tgz"
  export PATH=/usr/local/go/bin:$PATH
}
go_minor_ok() {
  command -v go >/dev/null 2>&1 || return 1
  local v maj min
  v=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1)
  maj=${v%%.*}; min=${v##*.}
  [[ ${maj:-0} -gt 1 || ( ${maj:-0} -eq 1 && ${min:-0} -ge $REQUIRED_GO_MINOR ) ]]
}
if ! go_minor_ok; then
  apt-get install -y -qq golang-go >/dev/null 2>&1 || true
fi
if ! go_minor_ok; then
  install_official_go
fi
go_minor_ok || fatal "无法获得 Go >= 1.${REQUIRED_GO_MINOR}, 请手动安装: https://go.dev/dl"
export GOTOOLCHAIN=auto   # 按 go.mod 自动下载所需 Go 版本
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
GIT_COMMIT=$(git -C "$SRC_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
CGO_ENABLED=0 go build \
  -ldflags="-s -w -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
  -o /tmp/pingmesh-bin ./src

# ---------------------------------------------------------- 4. 安装部署 ----
ENV_FILE="${INSTALL_DIR}/pingmesh.env"

# 彻底停掉所有旧进程: systemd 单元(含旧版服务名 smartping) + nohup 游离进程。
# 否则旧进程会继续拿着"已被替换/删除"的旧二进制运行, 表现为页面新、接口旧(404)。
stop_old_processes() {
  if [[ $HAS_SYSTEMD -eq 1 ]]; then
    systemctl stop "$SERVICE" 2>/dev/null || true
    systemctl stop smartping 2>/dev/null || true
  fi
  pkill -f "${INSTALL_DIR}/pingmesh" 2>/dev/null || true
  pkill -f "${INSTALL_DIR}/bin/smartping" 2>/dev/null || true
  # 兜底: 相对路径启动的游离进程(如 cd 安装目录后 nohup ./pingmesh)按服务端口定位后清理
  local port pids p
  port=$(detect_port)
  pids="$(ss -ltnp 2>/dev/null | grep ":${port} " | grep -oP 'pid=\K[0-9]+' || true) "
  pids+="$(netstat -ltnp 2>/dev/null | grep ":${port} " | awk '{print $NF}' | cut -d/ -f1 | grep -E '^[0-9]+$' || true) "
  pids+="$(fuser "${port}/tcp" 2>/dev/null | tr -s ' \t' '\n' | grep -E '^[0-9]+$' || true)"
  for p in $pids; do
    [[ "$p" == "$$" ]] && continue
    kill -9 "$p" 2>/dev/null || true
  done
  sleep 1
}

# 实际服务端口: 命令行 > 环境文件 > 已有配置 > 默认 8899
detect_port() {
  local p="$PORT"
  [[ -z "$p" && -f "$ENV_FILE" ]] && p=$(grep -oP '(?<=-p )[0-9]+' "$ENV_FILE" 2>/dev/null | head -1 || true)
  [[ -z "$p" && -f "${INSTALL_DIR}/conf/config.json" ]] && p=$(grep -oP '"Port"\s*:\s*\K[0-9]+' "${INSTALL_DIR}/conf/config.json" 2>/dev/null | head -1 || true)
  echo "${p:-8899}"
}

# 升级后自校验: 端口上实际服务的版本必须等于刚安装的版本,
# 否则一定存在旧进程占用端口, 给出占用进程与处理命令。
verify_running() {
  local port want got
  port=$(detect_port)
  want=$("${INSTALL_DIR}/pingmesh" -v 2>/dev/null | awk '{print $3}')
  sleep 1
  got=$(curl -s --max-time 4 "http://127.0.0.1:${port}/healthz" 2>/dev/null | grep -oP '"version":"\K[^"]+' || true)
  if [[ -n "$got" && "$got" == "$want" ]]; then
    info "运行版本校验通过: v${got} 正在端口 ${port} 上服务"
    return 0
  fi
  warn "=============================================================="
  if [[ -z "$got" ]]; then
    warn "健康检查异常: http://127.0.0.1:${port}/healthz 无有效响应"
    warn "端口 ${port} 上可能仍是旧版本进程(旧版本没有 /healthz 接口)"
  else
    warn "端口 ${port} 上实际运行的是 v${got}, 与刚安装的 v${want} 不一致!"
  fi
  warn "当前监听该端口的进程:"
  ss -ltnp 2>/dev/null | grep ":${port} " \
    || netstat -ltnp 2>/dev/null | grep ":${port} " \
    || fuser -v "${port}/tcp" 2>&1 | grep -v '^$' \
    || warn "  (未能列出, 可执行: sudo ss -ltnp | grep ${port})"
  warn "请手动结束旧进程后重启服务:"
  warn "  sudo pkill -f ${INSTALL_DIR}/pingmesh; sudo systemctl restart ${SERVICE}"
  warn "  然后验证: curl http://127.0.0.1:${port}/healthz"
  warn "=============================================================="
  return 1
}

info "[4/5] 安装到 ${INSTALL_DIR} ..."
mkdir -p "$INSTALL_DIR"
stop_old_processes
install -m 755 /tmp/pingmesh-bin "${INSTALL_DIR}/pingmesh"
rm -f /tmp/pingmesh-bin
setcap cap_net_raw+ep "${INSTALL_DIR}/pingmesh" || warn "setcap 失败, 将依赖 root 权限运行"

# 组装启动参数(写入 systemd 引用的环境文件)
build_opts() {
  local opts=""
  [[ -n "$PORT" ]]    && opts+=" -p $PORT"
  [[ -n "$ADDR" ]]    && opts+=" -addr $ADDR"
  [[ -n "$NAME" ]]    && opts+=" -name $NAME"
  if [[ -n "$JOIN" ]]; then
    [[ -n "$TOKEN" ]] || fatal "--join 需要同时提供 --token"
    opts+=" -join $JOIN -token $TOKEN"
  fi
  [[ -n "$MASTERS" ]] && opts+=" -masters $MASTERS"
  [[ $WEBUI -eq 1 ]] && opts+=" -webui"
  echo "${opts# }"
}

# 更新模式: 仅当带新角色参数时才重写环境文件, 否则沿用旧参数
if [[ $UPDATE -eq 1 && $ROLE_GIVEN -eq 0 && -z "$MASTERS" ]]; then
  info "[5/5] 更新模式: 保留现有启动参数与服务配置, 仅替换二进制并重启 ..."
  if [[ $HAS_SYSTEMD -eq 1 && -f /etc/systemd/system/${SERVICE}.service ]]; then
    systemctl restart "$SERVICE"; sleep 1
    systemctl is-active --quiet "$SERVICE" && info "服务已重启 (版本: $(${INSTALL_DIR}/pingmesh -v))" \
      || fatal "服务重启失败: journalctl -u ${SERVICE} -n 50"
  else
    cd "$INSTALL_DIR"; nohup "${INSTALL_DIR}/pingmesh" >/dev/null 2>&1 &
    sleep 1; pgrep -f "${INSTALL_DIR}/pingmesh" >/dev/null || fatal "重启失败"
  fi
  verify_running && info "更新完成" || true
  exit 0
fi

OPTS="$(build_opts)"
info "写入启动参数到 ${ENV_FILE}"
umask 077
cat > "$ENV_FILE" <<ENV
# ZENLENET PingMesh 启动参数 (本文件由安装脚本生成)
# 修改后执行: systemctl restart ${SERVICE}  即可生效。
# 说明: -p 端口  -name 节点名  -addr 本机对外IP  -join 主节点  -token 接入令牌
#       -masters 容灾备选(逗号分隔)  -webui 开启Agent节点的Web页面(默认关闭, 统一在主节点管理)
# 注意: 节点名称请勿包含空格(systemd 按空格拆分参数); 改名可登录后在「节点管理」中进行。
PINGMESH_OPTS=${OPTS}
ENV
chmod 600 "$ENV_FILE"

# ---------------------------------------------------------- 5. 启动服务 ----
info "[5/5] 配置并启动服务 ..."
if [[ $HAS_SYSTEMD -eq 1 ]]; then
  cat > "/etc/systemd/system/${SERVICE}.service" <<UNIT
[Unit]
Description=ZENLENET PingMesh - network quality monitor & DR cluster
Documentation=${REPO_URL}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=-${ENV_FILE}
ExecStart=${INSTALL_DIR}/pingmesh \$PINGMESH_OPTS
Restart=always
RestartSec=3
LimitNOFILE=65536
# ---- 安全加固 ----
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true
PrivateTmp=true
ProtectControlGroups=true
ProtectKernelTunables=true
RestrictSUIDSGID=true
ReadWritePaths=${INSTALL_DIR}

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
  # shellcheck disable=SC2086
  nohup "${INSTALL_DIR}/pingmesh" ${OPTS} >/dev/null 2>&1 &
  sleep 1
  pgrep -f "${INSTALL_DIR}/pingmesh" >/dev/null && info "进程已启动 (pid: $(pgrep -f "${INSTALL_DIR}/pingmesh" | head -1))" \
    || fatal "启动失败, 请手动执行: cd ${INSTALL_DIR} && ./pingmesh ${OPTS}"
fi
verify_running || true

# 防火墙放行
HTTP_PORT="${PORT:-8899}"
if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
  ufw allow "${HTTP_PORT}/tcp" >/dev/null && info "ufw 已放行 ${HTTP_PORT}/tcp"
fi

IP=$(hostname -I 2>/dev/null | awk '{print $1}')
echo
info "=============================================="
info " 部署完成!  (版本: $(${INSTALL_DIR}/pingmesh -v 2>/dev/null || echo '-'))"
if [[ -n "$JOIN" && $WEBUI -eq 0 ]]; then
  info " 本节点为 Agent: Web 页面已默认关闭, 登录账户随主节点同步"
  info " 管理入口:  ${JOIN}  (需开启本节点页面: 在 ${ENV_FILE} 的参数中追加 -webui 后重启)"
else
  info " 访问地址:  http://${IP:-<本机IP>}:${HTTP_PORT}"
  info " 默认账号:  admin / admin123  (登录后请立即修改)"
fi
info " 启动参数:  ${ENV_FILE}  (编辑后 systemctl restart ${SERVICE})"
[[ -n "$JOIN" ]] && info " 本节点为 Agent, 已配置自动加入 ${JOIN}"
[[ -n "$MASTERS" ]] && info " 容灾备选主节点: ${MASTERS}"
if [[ $HAS_SYSTEMD -eq 1 ]]; then
  info " 常用命令:  systemctl status|restart|stop ${SERVICE}"
  info " 健康检查:  curl http://127.0.0.1:${HTTP_PORT}/healthz"
else
  info " 重启方式:  pkill -f ${INSTALL_DIR}/pingmesh && cd ${INSTALL_DIR} && nohup ./pingmesh &"
fi
if [[ -f "${BASH_SOURCE[0]:-}" ]]; then
  info " 卸载:      sudo ${BASH_SOURCE[0]} --uninstall"
else
  info " 卸载:      curl -fsSL ${REPO_URL}/raw/master/deploy/install.sh | sudo bash -s -- --uninstall"
fi
info "=============================================="
