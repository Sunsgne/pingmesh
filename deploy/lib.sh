#!/usr/bin/env bash
# 部署脚本公共库：从 deploy/.env 加载敏感配置
_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${_LIB_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${_LIB_DIR}/.env"
  set +a
fi

deploy_require() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "错误: 未设置环境变量 ${name}" >&2
    echo "请执行: cp deploy/env.example deploy/.env  并填写后重试" >&2
    exit 1
  fi
}

# 统一变量名（脚本内使用）
PASSWORD="${PINGMESH_SSH_PASSWORD:-}"
JOIN_TOKEN="${PINGMESH_JOIN_TOKEN:-}"
ADMIN_USER="${PINGMESH_ADMIN_USER:-}"
ADMIN_PASSWORD="${PINGMESH_ADMIN_PASSWORD:-}"
MASTER_INTERNAL="${PINGMESH_MASTER_INTERNAL:-10.100.1.8}"
BACKUP_INTERNAL="${PINGMESH_BACKUP_INTERNAL:-10.100.1.3}"
MASTER_PUBLIC="${PINGMESH_MASTER_PUBLIC:-43.229.152.50}"
BACKUP_PUBLIC="${PINGMESH_BACKUP_PUBLIC:-163.53.245.90}"

deploy_require_ssh() { deploy_require PINGMESH_SSH_PASSWORD; }
deploy_require_join() { deploy_require PINGMESH_JOIN_TOKEN; }

# 远程创建/更新管理员账号（bcrypt）
deploy_setup_admin_user() {
  local host="$1" port="${2:-22}" install_dir="$3"
  deploy_require PINGMESH_ADMIN_USER
  deploy_require PINGMESH_ADMIN_PASSWORD
  deploy_require_ssh
  local user_b64 pass_b64
  user_b64=$(printf '%s' "$ADMIN_USER" | base64 -w0 2>/dev/null || printf '%s' "$ADMIN_USER" | base64)
  pass_b64=$(printf '%s' "$ADMIN_PASSWORD" | base64 -w0 2>/dev/null || printf '%s' "$ADMIN_PASSWORD" | base64)
  sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -p "$port" "root@${host}" \
    "apt-get install -y -qq python3 python3-pip >/dev/null 2>&1 || true
    pip3 install bcrypt -q 2>/dev/null || true
    DB=${install_dir}/data/db/database.db
    for i in \$(seq 1 30); do [ -f \"\$DB\" ] && break; sleep 2; done
    python3 -c \"
import sqlite3, bcrypt, base64
user = base64.b64decode('${user_b64}').decode()
pwd = base64.b64decode('${pass_b64}').decode()
h = bcrypt.hashpw(pwd.encode(), bcrypt.gensalt()).decode()
c = sqlite3.connect('${install_dir}/data/db/database.db')
c.execute('DELETE FROM users WHERE username=?', ('admin',))
c.execute('INSERT OR REPLACE INTO users(username,password,role,created_at) VALUES(?,?,?,datetime(\\\"now\\\"))', (user, h, 'admin'))
c.execute('UPDATE users_meta SET rev=rev+1, mtime=datetime(\\\"now\\\") WHERE id=1')
c.commit()
\""
}
