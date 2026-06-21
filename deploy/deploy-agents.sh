#!/usr/bin/env bash
set -euo pipefail
SSH_PASS='Monitor@678!9981'
REPO=/workspace
TAR=/tmp/pingmesh-src.tgz
tar -C "$REPO" --exclude=.git --exclude=data -czf "$TAR" .

deploy_one() {
  local host="$1" port="$2" name="$3" addr="$4"
  local log="/tmp/deploy-${name}.log"
  {
    echo "=== ${name} @ ${host}:${port} ==="
    if ! sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=15 -p "$port" "root@${host}" "echo SSH_OK" 2>/dev/null; then
      echo "FAIL: SSH unreachable"
      return 1
    fi
    sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no -P "$port" "$TAR" "root@${host}:/tmp/pingmesh-src.tgz"
    sshpass -p "$SSH_PASS" scp -o StrictHostKeyChecking=no -P "$port" "$REPO/deploy/agent-install.sh" "root@${host}:/tmp/agent-install.sh"
    sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no -p "$port" "root@${host}" bash -s "$name" "$addr" <<'REMOTE'
set -euo pipefail
rm -rf /tmp/pingmesh-src && mkdir -p /tmp/pingmesh-src
tar -xzf /tmp/pingmesh-src.tgz -C /tmp/pingmesh-src
chmod +x /tmp/agent-install.sh
bash /tmp/agent-install.sh "$1" "$2"
REMOTE
    echo "SUCCESS: ${name}"
  } >"$log" 2>&1
  cat "$log"
}

export -f deploy_one
export SSH_PASS TAR REPO

AGENTS=(
  "106.75.160.24:20002:can-xxg:106.75.160.24"
  "42.240.152.238:20002:can-hxy:42.240.152.238"
  "217.217.29.251:22:fra:217.217.29.251"
  "104.251.226.39:20002:hkg3:104.251.226.39"
  "163.53.245.136:20002:hkg4:163.53.245.136"
  "149.119.41.157:22:lax:149.119.41.157"
  "106.38.203.8:20002:pek:106.38.203.8"
  "61.172.165.219:20002:gds:61.172.165.219"
  "113.31.161.79:20002:sjhl:113.31.161.79"
  "109.244.32.190:20002:xtl:109.244.32.190"
  "149.51.125.226:20002:sin2-gs:149.51.125.226"
  "59.36.211.118:20002:szx:59.36.211.118"
  "192.169.120.13:22:tpe:192.169.120.13"
  "43.230.52.243:22:tyo-8:43.230.52.243"
  "61.172.165.219:20002:tyo-7:61.172.165.219"
)

for a in "${AGENTS[@]}"; do
  IFS=: read -r h p n addr <<< "$a"
  deploy_one "$h" "$p" "$n" "$addr" &
  # limit parallel jobs
  while [[ $(jobs -r | wc -l) -ge 4 ]]; do sleep 2; done
done
wait
echo "=== ALL AGENT DEPLOYS FINISHED ==="
