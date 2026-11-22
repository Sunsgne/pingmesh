#!/usr/bin/env bash
# SSH 与对外端口加固(幂等, 在每个节点以 root 执行)。不关闭密码登录，不会把人锁在外面:
#  1) sshd: 每连接最多 3 次密码、20 秒登录超时、限制未认证并发、关 X11 转发
#  2) 22 端口: 内网与控制节点不限; 其他来源每分钟最多 6 个新连接(突发 12), 超出直接丢弃
#  3) 5201(iperf3 测速): 只对内网开放
#  4) 可选: 写入运维公钥(OPS_PUBKEY), 为以后改成只允许密钥登录做准备
set -euo pipefail
INTERNAL_CIDR="${INTERNAL_CIDR:-10.100.1.0/24}"
TRUSTED_V4="${TRUSTED_V4:-43.229.152.50 163.53.245.90}"

# ---------- 1) sshd ----------
install -d -m 755 /etc/ssh/sshd_config.d
DROPIN=/etc/ssh/sshd_config.d/00-pingmesh-harden.conf
cat > "${DROPIN}.new" <<'CONF'
# 由 PingMesh ssh-harden.sh 生成。Include 在 sshd_config 顶部，先读到的值生效。
MaxAuthTries 3
LoginGraceTime 20
MaxStartups 10:30:60
X11Forwarding no
ClientAliveInterval 300
ClientAliveCountMax 2
CONF
had_old=0
[[ -f "$DROPIN" ]] && { cp -a "$DROPIN" "${DROPIN}.bak"; had_old=1; }
mv -f "${DROPIN}.new" "$DROPIN"
chmod 644 "$DROPIN"
if ! sshd -t; then
  echo "sshd: config test failed, rolling back"
  if [[ $had_old -eq 1 ]]; then mv -f "${DROPIN}.bak" "$DROPIN"; else rm -f "$DROPIN"; fi
  exit 1
fi
rm -f "${DROPIN}.bak"
systemctl reload ssh 2>/dev/null || systemctl reload sshd
echo "sshd: $(sshd -T | grep -E '^(maxauthtries|logingracetime|maxstartups|x11forwarding) ' | tr '\n' ';')"

# ---------- 2/3) 防火墙 ----------
install -d -m 755 /usr/local/sbin
{
  echo '#!/usr/bin/env bash'
  echo 'iptables -N PMSSH 2>/dev/null || iptables -F PMSSH'
  echo 'iptables -A PMSSH -i lo -j ACCEPT'
  echo "iptables -A PMSSH -s ${INTERNAL_CIDR} -j ACCEPT"
  for ip in ${TRUSTED_V4}; do echo "iptables -A PMSSH -s ${ip} -j ACCEPT"; done
  echo 'iptables -A PMSSH -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT'
  echo 'iptables -A PMSSH -m conntrack --ctstate NEW -m hashlimit --hashlimit-name pmssh --hashlimit-mode srcip --hashlimit-above 6/minute --hashlimit-burst 12 --hashlimit-htable-size 65536 --hashlimit-htable-max 200000 --hashlimit-htable-expire 600000 -j DROP'
  echo 'iptables -A PMSSH -j ACCEPT'
  echo 'iptables -C INPUT -p tcp --dport 22 -j PMSSH 2>/dev/null || iptables -I INPUT -p tcp --dport 22 -j PMSSH'
  echo 'iptables -N PMIPERF 2>/dev/null || iptables -F PMIPERF'
  echo 'iptables -A PMIPERF -i lo -j ACCEPT'
  echo "iptables -A PMIPERF -s ${INTERNAL_CIDR} -j ACCEPT"
  echo 'iptables -A PMIPERF -j DROP'
  echo 'for p in tcp udp; do iptables -C INPUT -p $p --dport 5201 -j PMIPERF 2>/dev/null || iptables -I INPUT -p $p --dport 5201 -j PMIPERF; done'
  echo 'if command -v ip6tables >/dev/null 2>&1; then'
  echo '  ip6tables -N PMSSH 2>/dev/null || ip6tables -F PMSSH'
  echo '  ip6tables -A PMSSH -i lo -j ACCEPT'
  echo '  ip6tables -A PMSSH -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT'
  echo '  ip6tables -A PMSSH -m conntrack --ctstate NEW -m hashlimit --hashlimit-name pmssh6 --hashlimit-mode srcip --hashlimit-srcmask 64 --hashlimit-above 6/minute --hashlimit-burst 12 --hashlimit-htable-size 65536 -j DROP'
  echo '  ip6tables -A PMSSH -j ACCEPT'
  echo '  ip6tables -C INPUT -p tcp --dport 22 -j PMSSH 2>/dev/null || ip6tables -I INPUT -p tcp --dport 22 -j PMSSH'
  echo '  ip6tables -N PMIPERF 2>/dev/null || ip6tables -F PMIPERF'
  echo '  ip6tables -A PMIPERF -i lo -j ACCEPT'
  echo '  ip6tables -A PMIPERF -j DROP'
  echo '  for p in tcp udp; do ip6tables -C INPUT -p $p --dport 5201 -j PMIPERF 2>/dev/null || ip6tables -I INPUT -p $p --dport 5201 -j PMIPERF; done'
  echo 'fi'
} > /usr/local/sbin/pm-ssh-firewall.sh
chmod 755 /usr/local/sbin/pm-ssh-firewall.sh
cat > /etc/systemd/system/pm-ssh-firewall.service <<'UNIT'
[Unit]
Description=PingMesh: rate-limit SSH and keep iperf3 internal
After=network-pre.target docker.service
Wants=network-pre.target
[Service]
Type=oneshot
ExecStart=/usr/local/sbin/pm-ssh-firewall.sh
RemainAfterExit=yes
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable pm-ssh-firewall.service >/dev/null 2>&1
/usr/local/sbin/pm-ssh-firewall.sh
systemctl start pm-ssh-firewall.service >/dev/null 2>&1 || true
echo "firewall: ssh=$(iptables -S PMSSH | wc -l) rules, iperf=$(iptables -S PMIPERF | wc -l) rules"

# ---------- 4) 运维公钥 ----------
if [[ -n "${OPS_PUBKEY:-}" ]]; then
  install -d -m 700 /root/.ssh
  touch /root/.ssh/authorized_keys
  chmod 600 /root/.ssh/authorized_keys
  grep -qF "$OPS_PUBKEY" /root/.ssh/authorized_keys || echo "$OPS_PUBKEY" >> /root/.ssh/authorized_keys
  echo "key: $(grep -c . /root/.ssh/authorized_keys) authorized"
fi
