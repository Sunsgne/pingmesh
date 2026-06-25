#!/usr/bin/env python3
"""批量将集群节点时区设为 Asia/Shanghai（不依赖 sshpass）。"""
import os
import sys
import time
from pathlib import Path

try:
    import paramiko
except ImportError:
    import subprocess
    subprocess.check_call([sys.executable, '-m', 'pip', 'install', '-q', 'paramiko'])
    import paramiko

DEPLOY_DIR = Path(__file__).resolve().parent
ENV_FILE = DEPLOY_DIR / '.env'


def load_env_file():
    if not ENV_FILE.exists():
        return
    for line in ENV_FILE.read_text(encoding='utf-8').splitlines():
        line = line.strip()
        if not line or line.startswith('#') or '=' not in line:
            continue
        key, _, val = line.partition('=')
        key, val = key.strip(), val.strip().strip('"').strip("'")
        os.environ.setdefault(key, val)


load_env_file()
PASSWORD = os.environ.get('PINGMESH_SSH_PASSWORD', '')
if not PASSWORD:
    print('错误: 未设置 PINGMESH_SSH_PASSWORD，请配置 deploy/.env', file=sys.stderr)
    sys.exit(1)

MASTER = (os.environ.get('PINGMESH_MASTER_PUBLIC', '43.229.152.50'), 22)
TZ = 'Asia/Shanghai'
REMOTE = f'''export DEBIAN_FRONTEND=noninteractive
apt-get install -y -qq tzdata >/dev/null 2>&1 || true
timedatectl set-timezone {TZ} 2>/dev/null || (ln -sf /usr/share/zoneinfo/{TZ} /etc/localtime && echo {TZ} > /etc/timezone)
if [ -d /opt/pingmesh-docker ] && command -v docker >/dev/null 2>&1; then
  cd /opt/pingmesh-docker && docker compose up -d 2>/dev/null || true
fi
if systemctl is-active pingmesh >/dev/null 2>&1; then systemctl restart pingmesh; fi
timedatectl show -p Timezone --value 2>/dev/null || cat /etc/timezone
date '+%F %T %Z'
'''

NODES = [
    ('43.229.152.50', 22, 'sin1-sg2'),
    ('163.53.245.90', 22, 'hkg1'),
    ('106.75.160.24', 20001, 'can-xxg'),
    ('42.240.152.238', 20001, 'can-hxy'),
    ('217.217.29.250', 22, 'fra'),
    ('104.251.226.39', 20001, 'hkg2'),
    ('163.53.245.136', 20001, 'hkg3'),
    ('149.119.41.156', 22, 'lax'),
    ('106.38.203.8', 20001, 'pek'),
    ('61.172.165.219', 20001, 'gds'),
    ('113.31.161.79', 20001, 'sjhl'),
    ('109.244.32.190', 20001, 'xtl'),
    ('149.51.125.226', 20001, 'sin2-gs'),
    ('59.36.211.118', 20001, 'szx'),
    ('192.169.120.12', 22, 'tpe'),
    ('43.230.52.242', 22, 'tyo-8'),
    ('61.172.165.219', 20001, 'tyo-7'),
]
INTERNAL = [
    ('10.100.1.2', 'TYO-EQTY8'),
    ('10.100.1.9', 'TYO-EQTY7'),
    ('10.100.1.20', 'PVG-GDS'),
    ('10.100.1.19', 'can-hxy'),
]


def ssh_run(host, port, cmd, retries=3):
    last = None
    for _ in range(retries):
        try:
            c = paramiko.SSHClient()
            c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
            c.connect(host, port=port, username='root', password=PASSWORD,
                      timeout=30, banner_timeout=30, allow_agent=False, look_for_keys=False)
            _, out, err = c.exec_command(cmd, timeout=120)
            body = out.read().decode().strip()
            code = out.channel.recv_exit_status()
            c.close()
            return code, body, err.read().decode().strip()
        except Exception as e:
            last = e
            time.sleep(5)
    return -1, '', str(last)


def via_master(ip, name):
    cmd = (
        f"sshpass -p '{PASSWORD}' ssh -o StrictHostKeyChecking=no -o ConnectTimeout=12 root@{ip} "
        f"\"timedatectl set-timezone {TZ}; timedatectl show -p Timezone --value; date '+%F %T %Z'\" 2>&1"
    )
    return ssh_run(MASTER[0], MASTER[1], cmd)


def main():
    ok = fail = 0
    print('=== 公网/直连节点 ===')
    for host, port, name in NODES:
        code, out, err = ssh_run(host, port, REMOTE)
        if code == 0 and TZ in out:
            print(f'[OK] {name} ({host}:{port})\n     {out.splitlines()[-1]}')
            ok += 1
        else:
            print(f'[FAIL] {name} ({host}:{port}) {err or out}')
            fail += 1
        time.sleep(1)

    print('=== 主节点内网补设 ===')
    for ip, name in INTERNAL:
        code, out, err = via_master(ip, name)
        if code == 0 and TZ in out:
            print(f'[OK] {name} ({ip})\n     {out.splitlines()[-1]}')
            ok += 1
        else:
            print(f'[FAIL] {name} ({ip}) {err or out}')
            fail += 1

    print(f'\n完成: 成功 {ok}, 失败 {fail}')
    return 0 if fail == 0 else 1


if __name__ == '__main__':
    sys.exit(main())
