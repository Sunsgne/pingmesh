#!/usr/bin/env bash
# 在主节点上执行: 检查并修正内网 Agent 时区与 systemd TZ
set -uo pipefail
ENV_FILE="${PINGMESH_ENV_FILE:-/opt/pingmesh-docker/.env}"
if [[ -f "$ENV_FILE" ]]; then
  set -a; # shellcheck disable=SC1091
  source "$ENV_FILE"; set +a
fi
PASSWORD="${PINGMESH_SSH_PASSWORD:-}"
if [[ -z "$PASSWORD" ]]; then echo "错误: 未设置 PINGMESH_SSH_PASSWORD" >&2; exit 1; fi

TZ_NAME='Asia/Shanghai'
FIX_SCRIPT='export DEBIAN_FRONTEND=noninteractive
apt-get install -y -qq tzdata >/dev/null 2>&1 || true
timedatectl set-timezone Asia/Shanghai 2>/dev/null || (ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo Asia/Shanghai > /etc/timezone)
if [ -f /etc/systemd/system/pingmesh.service ] && ! grep -q "TZ=Asia/Shanghai" /etc/systemd/system/pingmesh.service; then
  sed -i "/^\[Service\]/a Environment=TZ=Asia/Shanghai" /etc/systemd/system/pingmesh.service
  systemctl daemon-reload
fi
systemctl is-active pingmesh >/dev/null 2>&1 && systemctl restart pingmesh
printf "sys_tz=%s\n" "$(timedatectl show -p Timezone --value 2>/dev/null || cat /etc/timezone)"
curl -s --max-time 3 http://127.0.0.1:8899/healthz 2>/dev/null || echo health_fail'

AGENTS=(
  "10.100.1.1 PVG-XTL"
  "10.100.1.2 TYO-EQTY8"
  "10.100.1.4 CAN-XXG"
  "10.100.1.5 PVG-SJHL"
  "10.100.1.7 FRA-EQFRA5"
  "10.100.1.9 TYO-EQTY7"
  "10.100.1.10 LAX-CORESITE"
  "10.100.1.11 SIN-GS"
  "10.100.1.12 HKG-EQHK2"
  "10.100.1.13 HKG-EQHK3"
  "10.100.1.15 PEK-BJHK"
  "10.100.1.17 SZX-SZX"
  "10.100.1.18 TPE-EQTPE"
  "10.100.1.19 CAN-HXY"
  "10.100.1.20 PVG-GDS"
  "10.100.1.21 SEL-LG"
)

OK=0 FAIL=0
for entry in "${AGENTS[@]}"; do
  read -r ip name <<< "$entry"
  echo "=== ${name} (${ip}) ==="
  if out=$(sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "root@${ip}" "bash -s" <<< "$FIX_SCRIPT" 2>&1); then
    echo "$out"
    if echo "$out" | grep -q "sys_tz=${TZ_NAME}"; then OK=$((OK+1)); else FAIL=$((FAIL+1)); fi
  else
    echo "FAIL: $out"; FAIL=$((FAIL+1))
  fi
done
echo "完成: 成功 ${OK}, 失败 ${FAIL}"
