#!/usr/bin/env bash
# PingMesh 节点磁盘防护: 扩容 LVM 根分区 + journal/logrotate + 小时巡检
# 可在任意节点本地执行: bash disk-harden.sh
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

info() { echo -e "\033[32m[disk-harden]\033[0m $*"; }

info "host=$(hostname) before: $(df -h / | tail -1)"

# ---------- 1) 扩容 LVM 根分区到磁盘剩余空间 ----------
ROOT_DEV=$(findmnt -n -o SOURCE / || true)
if ! command -v growpart >/dev/null 2>&1; then
  apt-get update -qq >/dev/null 2>&1 || true
  apt-get install -y -qq cloud-guest-utils lvm2 >/dev/null 2>&1 || true
fi
if [[ "$ROOT_DEV" == /dev/mapper/ubuntu--vg-ubuntu--lv ]] && [[ -b /dev/sda3 ]]; then
  growpart /dev/sda 3 >/dev/null 2>&1 || true
  partprobe /dev/sda 2>/dev/null || true
  sleep 1
  pvresize /dev/sda3 >/dev/null 2>&1 || true
  lvextend -l +100%FREE /dev/ubuntu-vg/ubuntu-lv >/dev/null 2>&1 || true
  resize2fs /dev/ubuntu-vg/ubuntu-lv >/dev/null 2>&1 || true
fi

# ---------- 2) journald 上限 ----------
mkdir -p /etc/systemd/journald.conf.d
cat >/etc/systemd/journald.conf.d/size.conf <<EOF
[Journal]
SystemMaxUse=200M
RuntimeMaxUse=50M
SystemMaxFileSize=50M
EOF
systemctl restart systemd-journald 2>/dev/null || true
journalctl --vacuum-size=200M >/dev/null 2>&1 || true

# ---------- 3) pingmesh 日志轮转 ----------
if [[ -d /opt/pingmesh/logs ]]; then
  cat >/etc/logrotate.d/pingmesh <<'EOF'
/opt/pingmesh/logs/*.log {
    daily
    rotate 7
    size 50M
    missingok
    notifempty
    compress
    delaycompress
    copytruncate
}
EOF
fi

# ---------- 4) 使用率过高时紧急清理 ----------
USE=$(df -P / | awk 'NR==2{gsub(/%/,"",$5); print $5}')
if [[ "${USE:-0}" -ge 80 ]]; then
  info "disk ${USE}% - emergency clean"
  apt-get clean >/dev/null 2>&1 || true
  find /var/log -type f \( -name '*.gz' -o -name '*.1' -o -name '*.old' \) -delete 2>/dev/null || true
  find /opt/pingmesh/logs -type f \( -name '*.log.[0-9]*' -o -name '*.gz' \) -delete 2>/dev/null || true
fi

# ---------- 5) 小时巡检: 超 85% 告警并清理; 有 LVM 空闲则自动扩容 ----------
cat >/usr/local/sbin/pm-disk-watch.sh <<'EOF'
#!/usr/bin/env bash
USE=$(df -P / | awk 'NR==2{gsub(/%/,"",$5); print $5}')
AVAIL=$(df -h / | awk 'NR==2{print $4}')
if [ "${USE:-0}" -ge 85 ]; then
  logger -t pingmesh-disk "CRITICAL root disk ${USE}% used, avail=${AVAIL}"
  journalctl --vacuum-size=100M >/dev/null 2>&1 || true
  apt-get clean >/dev/null 2>&1 || true
  find /opt/pingmesh/logs -type f \( -name '*.log.[0-9]*' -o -name '*.gz' \) -delete 2>/dev/null || true
  find /var/log -type f \( -name '*.gz' -o -name '*.1' \) -delete 2>/dev/null || true
  USE2=$(df -P / | awk 'NR==2{gsub(/%/,"",$5); print $5}')
  logger -t pingmesh-disk "after clean root disk ${USE2}%"
fi
# 若 VG 仍有空闲空间则自动扩根分区
if command -v vgs >/dev/null 2>&1 && [[ -b /dev/sda3 ]]; then
  FREE=$(vgs --noheadings -o vg_free --units g 2>/dev/null | awk '{print $1}' | tr -d 'gG<')
  python3 - <<PY 2>/dev/null || true
try:
    f=float("${FREE:-0}".replace(",", ".") or 0)
except Exception:
    f=0
if f >= 1.0:
    import subprocess
    subprocess.call(["bash","-lc","growpart /dev/sda 3 || true; pvresize /dev/sda3 || true; lvextend -l +100%FREE /dev/ubuntu-vg/ubuntu-lv || true; resize2fs /dev/ubuntu-vg/ubuntu-lv || true"])
PY
fi
EOF
chmod +x /usr/local/sbin/pm-disk-watch.sh
cat >/etc/cron.d/pm-disk-watch <<'EOF'
# PingMesh disk safeguard - every hour
0 * * * * root /usr/local/sbin/pm-disk-watch.sh
EOF
chmod 644 /etc/cron.d/pm-disk-watch

info "after: $(df -h / | tail -1)"
