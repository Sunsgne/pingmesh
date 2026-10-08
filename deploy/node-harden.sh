#!/usr/bin/env bash
# 节点加固(幂等, 在每个节点以 root 执行):
#  1) 8899 只对本机与节点内网开放, 公网访问一律走 nginx(443) + Cloudflare
#  2) Agent: 接入令牌从 systemd unit 命令行移到 0600 的 EnvironmentFile
set -euo pipefail
INTERNAL_CIDR="${INTERNAL_CIDR:-10.100.1.0/24}"
PORT="${PM_PORT:-8899}"

# ---------- 1) 防火墙: 独立链 PM8899, 开机由 systemd 重新加载 ----------
install -d -m 755 /usr/local/sbin
cat > /usr/local/sbin/pm-firewall.sh <<FW
#!/usr/bin/env bash
iptables -N PM8899 2>/dev/null || iptables -F PM8899
iptables -A PM8899 -i lo -j ACCEPT
iptables -A PM8899 -s ${INTERNAL_CIDR} -j ACCEPT
iptables -A PM8899 -j DROP
iptables -C INPUT -p tcp --dport ${PORT} -j PM8899 2>/dev/null || iptables -I INPUT -p tcp --dport ${PORT} -j PM8899
if command -v ip6tables >/dev/null 2>&1; then
  ip6tables -N PM8899 2>/dev/null || ip6tables -F PM8899
  ip6tables -A PM8899 -i lo -j ACCEPT
  ip6tables -A PM8899 -j DROP
  ip6tables -C INPUT -p tcp --dport ${PORT} -j PM8899 2>/dev/null || ip6tables -I INPUT -p tcp --dport ${PORT} -j PM8899
fi
FW
chmod 755 /usr/local/sbin/pm-firewall.sh
cat > /etc/systemd/system/pm-firewall.service <<'UNIT'
[Unit]
Description=PingMesh: restrict node port to internal network
After=network-pre.target docker.service
Wants=network-pre.target
[Service]
Type=oneshot
ExecStart=/usr/local/sbin/pm-firewall.sh
RemainAfterExit=yes
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now pm-firewall.service >/dev/null 2>&1 || /usr/local/sbin/pm-firewall.sh
echo "firewall: $(iptables -S PM8899 | tr '\n' ';')"

# ---------- 2) Agent 令牌移出命令行 ----------
UNITF=/etc/systemd/system/pingmesh.service
if [[ -f "$UNITF" ]] && grep -q -- ' -token ' "$UNITF"; then
  TOKEN=$(grep -oP '(?<= -token )\S+' "$UNITF")
  install -d -m 700 /etc/pingmesh
  umask 077
  printf 'PINGMESH_JOIN_TOKEN=%s\n' "$TOKEN" > /etc/pingmesh/pingmesh.env
  chmod 600 /etc/pingmesh/pingmesh.env
  cp -a "$UNITF" "$UNITF.bak"
  sed -i -E 's/ -token \S+//' "$UNITF"
  grep -q '^EnvironmentFile=' "$UNITF" || sed -i '/^\[Service\]/a EnvironmentFile=/etc/pingmesh/pingmesh.env' "$UNITF"
  chmod 644 "$UNITF"
  systemctl daemon-reload
  systemctl restart pingmesh
  ok=0
  for i in $(seq 1 20); do
    sleep 2
    pid=$(systemctl show -p MainPID --value pingmesh)
    if systemctl is-active -q pingmesh && ss -lntp | grep ":${PORT} " | grep -q "pid=${pid}," \
      && curl -sf --max-time 3 "http://127.0.0.1:${PORT}/healthz" | grep -q '"status":"ok"'; then ok=1; break; fi
  done
  if [[ $ok -ne 1 ]]; then
    echo "token migration: health check failed, rolling back"
    mv -f "$UNITF.bak" "$UNITF"; systemctl daemon-reload; systemctl restart pingmesh
    exit 1
  fi
  rm -f "$UNITF.bak"
  echo "token: moved to /etc/pingmesh/pingmesh.env"
else
  echo "token: nothing to migrate"
fi
