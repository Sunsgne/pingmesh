#!/usr/bin/env bash
# 将 / 扩展到磁盘剩余全部空间 (LVM: sda3 + ubuntu-vg/ubuntu-lv)
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

ROOT_DEV=$(findmnt -n -o SOURCE /)
echo "ROOT before: $(df -h / | tail -1)"

# growpart
if ! command -v growpart >/dev/null 2>&1; then
  apt-get update -qq && apt-get install -y -qq cloud-guest-utils lvm2 >/dev/null 2>&1 || true
fi

# 标准布局: /dev/mapper/ubuntu--vg-ubuntu--lv <- sda3
if [[ "$ROOT_DEV" == /dev/mapper/ubuntu--vg-ubuntu--lv ]] && [[ -b /dev/sda3 ]]; then
  growpart /dev/sda 3 || true
  partprobe /dev/sda 2>/dev/null || true
  sleep 2
  pvresize /dev/sda3
  lvextend -l +100%FREE /dev/ubuntu-vg/ubuntu-lv
  resize2fs /dev/ubuntu-vg/ubuntu-lv
else
  echo "Unknown layout ROOT=$ROOT_DEV, trying generic resize..."
  # 尝试 xfs_growfs 或 resize2fs on root directly
  if findmnt -n -o FSTYPE / | grep -q xfs; then
    xfs_growfs /
  elif [[ -n "$ROOT_DEV" ]]; then
    resize2fs "$ROOT_DEV" 2>/dev/null || true
  fi
fi

echo "ROOT after: $(df -h / | tail -1)"
