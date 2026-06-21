#!/usr/bin/env bash
# PingMesh Docker 部署辅助: 安装 Docker、生成 10 年自签证书、配置 nginx 443 反代
set -euo pipefail

INSTALL_DIR=/opt/pingmesh
SSL_DIR=/etc/nginx/ssl/pingmesh
NGINX_CONF=/etc/nginx/sites-available/pingmesh
ROLE="${1:-}"   # master | backup | agent
CERT_IP="${2:-}"

info()  { echo -e "\033[32m[deploy]\033[0m $*"; }
fatal() { echo -e "\033[31m[deploy]\033[0m $*"; exit 1; }

[[ $EUID -eq 0 ]] || fatal "请使用 root 运行"

# ---- 安装 Docker ----
install_docker() {
  if command -v docker >/dev/null 2>&1; then
    info "Docker 已安装: $(docker --version)"
    return
  fi
  info "安装 Docker ..."
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl gnupg
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
  systemctl enable --now docker
  info "Docker 安装完成"
}

# ---- 控制节点: nginx + 10 年自签 HTTPS ----
setup_nginx_ssl() {
  local cert_ip="${1:-$(hostname -I | awk '{print $1}')}"
  apt-get install -y -qq nginx openssl
  mkdir -p "$SSL_DIR"
  local cnf
  cnf=$(mktemp)
  cat > "$cnf" <<EOF
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no
[req_distinguished_name]
CN = pingmesh
[v3_req]
subjectAltName = @alt_names
[alt_names]
IP.1 = ${cert_ip}
DNS.1 = pingmesh
EOF
  info "生成 10 年自签 HTTPS 证书 (SAN IP=${cert_ip}) ..."
  openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
    -keyout "$SSL_DIR/pingmesh.key" \
    -out "$SSL_DIR/pingmesh.crt" \
    -config "$cnf" -extensions v3_req 2>/dev/null
  rm -f "$cnf"
  chmod 600 "$SSL_DIR/pingmesh.key"
  cat > "$NGINX_CONF" <<'NGX'
server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name _;

    ssl_certificate     /etc/nginx/ssl/pingmesh/pingmesh.crt;
    ssl_certificate_key /etc/nginx/ssl/pingmesh/pingmesh.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    client_max_body_size 20m;

    location / {
        proxy_pass http://127.0.0.1:8899;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 120s;
    }
}
NGX
  ln -sf "$NGINX_CONF" /etc/nginx/sites-enabled/pingmesh
  rm -f /etc/nginx/sites-enabled/default
  nginx -t
  systemctl enable --now nginx
  systemctl reload nginx
  info "nginx 443 HTTPS 反代已就绪"
}

prepare_dirs() {
  mkdir -p "$INSTALL_DIR"/{data/conf,data/db,data/logs,src}
}

install_docker
prepare_dirs

case "$ROLE" in
  master|backup)
    setup_nginx_ssl "${CERT_IP:-$(hostname -I | awk '{print $1}')}"
  ;;
  agent)
    info "Agent 模式: 跳过 nginx/HTTPS"
  ;;
  *)
    fatal "用法: $0 master|backup|agent"
  ;;
esac

info "基础环境准备完成 (role=$ROLE)"
