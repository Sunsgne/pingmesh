#!/usr/bin/env bash
# ZENLENET PingMesh 一键安装脚本 (Linux + systemd)
#
# 主节点:        ./deploy/install.sh
# 加入主节点:    ./deploy/install.sh --join http://10.0.0.1:8899 --token smartping --name 节点B
# 指定端口:      ./deploy/install.sh --port 8899
set -euo pipefail

INSTALL_DIR=/opt/pingmesh
JOIN="" TOKEN="" NAME="" PORT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --join)  JOIN="$2";  shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    --name)  NAME="$2";  shift 2 ;;
    --port)  PORT="$2";  shift 2 ;;
    --dir)   INSTALL_DIR="$2"; shift 2 ;;
    *) echo "未知参数: $1"; exit 1 ;;
  esac
done

if [[ $EUID -ne 0 ]]; then
  echo "请使用 root 运行 (sudo $0 ...)"; exit 1
fi

cd "$(dirname "$0")/.."

# 1. 准备二进制: 仓库已有构建产物则直接使用, 否则用 go 编译
BIN=""
if [[ -f bin/pingmesh ]]; then
  BIN=bin/pingmesh
elif command -v go >/dev/null 2>&1; then
  echo "[1/4] 编译 pingmesh ..."
  go build -ldflags="-s -w" -o bin/pingmesh ./src
  BIN=bin/pingmesh
else
  echo "未找到 bin/pingmesh 且系统无 go 编译器, 请先构建: go build -o bin/pingmesh ./src"
  exit 1
fi

# 2. 安装
echo "[2/4] 安装到 ${INSTALL_DIR} ..."
mkdir -p "$INSTALL_DIR"
install -m 755 "$BIN" "$INSTALL_DIR/pingmesh"
setcap cap_net_raw+ep "$INSTALL_DIR/pingmesh" 2>/dev/null || true

# 3. systemd 服务
echo "[3/4] 配置 systemd 服务 ..."
EXTRA_ARGS=""
[[ -n "$PORT" ]]  && EXTRA_ARGS+=" -p $PORT"
if [[ -n "$JOIN" ]]; then
  [[ -z "$TOKEN" || -z "$NAME" ]] && { echo "--join 需要同时提供 --token 与 --name"; exit 1; }
  # 每次启动自动注册并同步主节点配置(幂等, 自愈)
  EXTRA_ARGS+=" -join $JOIN -token $TOKEN -name $NAME"
fi
sed "s|ExecStart=.*|ExecStart=${INSTALL_DIR}/pingmesh${EXTRA_ARGS}|; s|WorkingDirectory=.*|WorkingDirectory=${INSTALL_DIR}|" \
  deploy/pingmesh.service > /etc/systemd/system/pingmesh.service
systemctl daemon-reload
systemctl enable pingmesh >/dev/null

# 4. 启动
echo "[4/4] 启动服务 ..."
systemctl restart pingmesh
sleep 1
systemctl --no-pager -l status pingmesh | head -8 || true

echo
echo "安装完成! 访问 http://<本机IP>:${PORT:-8899} (默认账号 admin / admin123)"
[[ -n "$JOIN" ]] && echo "本节点已配置为 Agent, 将自动加入主节点 ${JOIN} 并同步配置。"
