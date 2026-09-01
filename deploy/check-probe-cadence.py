#!/usr/bin/env python3
"""从节点本地 pinglog 实测探测频率是否与配置一致。"""
import json
import os
import sqlite3
import sys
from datetime import datetime, timedelta

CFG = "/opt/pingmesh/conf/config.json"
DB = "/opt/pingmesh/db/database.db"
WINDOW_MIN = 3
PROBE_STEP = 10


def expected_sendpk(interval, per_min_count):
    per_cycle = per_min_count * PROBE_STEP // 60
    max_fit = (PROBE_STEP * 1000 - 500) // interval
    return min(per_cycle, max_fit)


def parse_cfg():
    if not os.path.exists(CFG):
        return None, "无配置文件"
    with open(CFG, encoding="utf-8") as f:
        c = json.load(f)
    addr = c.get("Addr", "")
    selfn = c.get("Network", {}).get(addr, {})
    base = c.get("Base", {})
    interval = int(base.get("Pinginterval", 0))
    per_min = int(base.get("Pingcount", 0))
    count = expected_sendpk(interval, per_min)
    timeout = int(base.get("Pingtimeout", 0))
    targets = list(selfn.get("Ping") or [])
    topo = {t.get("Addr"): t for t in (selfn.get("Topology") or []) if t.get("Addr")}
    return {
        "name": c.get("Name", addr),
        "addr": addr,
        "interval": interval,
        "per_min": per_min,
        "count": count,
        "timeout": timeout,
        "targets": targets,
        "topo": topo,
    }, None


def expected_for(target, cfg):
    interval, per_min = cfg["interval"], cfg["per_min"]
    ov = cfg["topo"].get(target, {})
    if ov.get("Pinterval"):
        interval = int(ov["Pinterval"])
    if ov.get("Pcount"):
        per_min = int(ov["Pcount"])
    count = expected_sendpk(interval, per_min)
    cycle_s = (interval * count + cfg["timeout"] + 200) / 1000.0
    return interval, count, per_min, cycle_s


def analyze_target(cur, target, cfg, since):
    interval, count, per_min, cycle_s = expected_for(target, cfg)
    rows = cur.execute(
        "SELECT logtime, sendpk, revcpk, losspk FROM pinglog "
        "WHERE target=? AND logtime>=? ORDER BY logtime",
        (target, since),
    ).fetchall()
    if not rows:
        return {
            "target": target,
            "ok": False,
            "reason": "窗口内无数据",
            "expected_interval_ms": interval,
            "expected_count": count,
            "expected_per_min": per_min,
            "expected_cycle_s": round(cycle_s, 1),
        }
    sendpks = {int(r[1]) for r in rows if r[1] is not None}
    gaps = []
    prev = None
    for r in rows:
        lt = r[0]
        if prev:
            d = (
                datetime.strptime(lt, "%Y-%m-%d %H:%M:%S")
                - datetime.strptime(prev, "%Y-%m-%d %H:%M:%S")
            ).total_seconds()
            if abs(d - PROBE_STEP) > 0.1:
                gaps.append((prev, lt, int(d)))
        prev = lt
    count_ok = sendpks == {count}
    cadence_ok = len(gaps) == 0 and len(rows) >= (WINDOW_MIN * 60 // PROBE_STEP) - 2
    return {
        "target": target,
        "ok": count_ok and cadence_ok,
        "points": len(rows),
        "expected_interval_ms": interval,
        "expected_count": count,
        "expected_per_min": per_min,
        "expected_cycle_s": round(cycle_s, 1),
        "sendpk_seen": sorted(sendpks),
        "count_match": count_ok,
        "minute_cadence_ok": cadence_ok,
        "gaps": gaps[:5],
        "gap_count": len(gaps),
    }


def main():
    cfg, err = parse_cfg()
    if err:
        print(f"FAIL|{err}")
        sys.exit(2)
    if not os.path.exists(DB):
        print("FAIL|无数据库")
        sys.exit(2)

    since = (datetime.now() - timedelta(minutes=WINDOW_MIN)).strftime("%Y-%m-%d %H:%M")
    con = sqlite3.connect(DB)
    cur = con.cursor()

    cycle_s = (cfg["interval"] * cfg["count"] + cfg["timeout"] + 200) / 1000.0
    print(
        f"NODE|{cfg['name']}|{cfg['addr']}|"
        f"global={cfg['interval']}ms×{cfg['per_min']}包/分钟|cycle10s={cfg['count']}包|cycle≈{cycle_s:.1f}s|targets={len(cfg['targets'])}"
    )

    if not cfg["targets"]:
        print("WARN|无探测目标")
        con.close()
        return

    bad = []
    for t in cfg["targets"]:
        r = analyze_target(cur, t, cfg, since)
        status = "OK" if r["ok"] else "BAD"
        ov = cfg["topo"].get(t, {})
        ovr = ""
        if ov.get("Pinterval") or ov.get("Pcount"):
            ovr = f"|override={r['expected_interval_ms']}ms×{r['expected_count']}"
        print(
            f"{status}|{t}|pts={r.get('points',0)}|"
            f"sendpk={r.get('sendpk_seen','-')}|expect={r['expected_count']}/10s({r['expected_per_min']}/min)"
            f"{ovr}|gaps={r.get('gap_count',0)}"
        )
        if not r.get("ok"):
            bad.append(r)
    con.close()

    if bad:
        print(f"SUMMARY|FAIL|{len(bad)}/{len(cfg['targets'])} targets abnormal")
        sys.exit(1)
    print(f"SUMMARY|OK|{len(cfg['targets'])} targets, {WINDOW_MIN}min window")


if __name__ == "__main__":
    main()
