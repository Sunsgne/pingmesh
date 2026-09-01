#!/usr/bin/env bash
set -o pipefail
PASS="${PASSWORD:?}"
SCRIPT=/tmp/check-probe-cadence.py
LIST=/tmp/agents.list
apt-get install -y -qq python3 sqlite3 sshpass >/dev/null 2>&1 || true

echo "######## MASTER SIN-EQSG2 (10.100.1.8) ########"
mkdir -p /tmp/pm-m/conf /tmp/pm-m/db
cp /opt/pingmesh-docker/data/conf/config.json /tmp/pm-m/conf/config.json
sqlite3 /opt/pingmesh-docker/data/db/database.db ".backup /tmp/pm-m/db/database.db"
sed 's|/opt/pingmesh|/tmp/pm-m|g' "$SCRIPT" > /tmp/check-m.py
python3 /tmp/check-m.py | grep -E '^(NODE|SUMMARY|BAD)' 

while read -r _h _p name addr _; do
  [[ -z "$name" || "$_h" =~ ^# ]] && continue
  echo "######## $name ($addr) ########"
  if ! sshpass -p "$PASS" scp -o StrictHostKeyChecking=no "$SCRIPT" "root@${addr}:/tmp/" </dev/null 2>/dev/null; then
    echo "SUMMARY|FAIL|scp"
    continue
  fi
  OUT=$(sshpass -p "$PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 "root@${addr}" 'python3 /tmp/check-probe-cadence.py' </dev/null 2>&1) || true
  echo "$OUT" | grep -E '^(NODE|SUMMARY|BAD)' | tail -5
done < "$LIST"
