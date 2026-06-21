#!/usr/bin/env bash
set -euo pipefail
SSH_PASS='Monitor@678!9981'
IMAGE=/tmp/pingmesh-image.tar
SCRIPT=/workspace/deploy/agent-install-fast.sh

retry() {
  local host="$1" port="$2" name="$3" addr="$4"
  echo ">>> Retry ${name} @ ${host}:${port}"
  if ! sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -p "$port" "root@${host}" "echo ok" 2>/dev/null; then
    echo "SKIP: SSH failed ${name}"; return 0
  fi
  sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no -P "$port" "$IMAGE" "$SCRIPT" "root@${host}:/tmp/"
  if sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -p "$port" "root@${host}" "chmod +x /tmp/agent-install-fast.sh && bash /tmp/agent-install-fast.sh '${name}' '${addr}'"; then
    echo "OK: ${name}"
  else
    echo "FAIL: ${name}"
  fi
}

retry 106.75.160.24 20002 can-xxg 106.75.160.24
retry 42.240.152.238 20002 can-hxy 42.240.152.238
retry 106.38.203.8 20002 pek 106.38.203.8
retry 61.172.165.219 20002 gds 61.172.165.219
retry 113.31.161.79 20002 sjhl 113.31.161.79
retry 109.244.32.190 20002 xtl 109.244.32.190
retry 59.36.211.118 20002 szx 59.36.211.118
retry 61.172.165.219 20002 tyo-7 61.172.165.219
echo "=== RETRY DONE ==="
