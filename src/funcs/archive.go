package funcs

import (
	"strconv"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// ClearArchive 清理超过保留天数的历史数据。
// 分批删除并在批间让出写锁: 一次性 DELETE 数百万行会长时间占住写锁、阻塞探测入库,
// 并把 WAL 撑到数 GB(曾导致节点磁盘写满)。截止日按北京时间计算, 与 logtime 一致。
func ClearArchive() {
	days := g.Cfg.Base["Archive"]
	if days <= 0 {
		return
	}
	cut := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	seelog.Info("[func:ClearArchive] starting, keep ", days, " days (delete before ", cut, ")")
	total := int64(0)
	exec := func(q string, args ...interface{}) (int64, bool) {
		g.DLock.Lock()
		res, err := g.Db.Exec(q, args...)
		g.DLock.Unlock()
		if err != nil {
			seelog.Error("[func:ClearArchive] ", err)
			return 0, false
		}
		n, _ := res.RowsAffected()
		total += n
		return n, true
	}

	if g.PinglogCompact() {
		// 聚簇表按 (target, logtime) 存放: 逐目标、逐天删除, 每批约 8640 行且都在相邻页上
		for _, t := range pinglogTargets() {
			for {
				var oldest string
				g.Db.QueryRow("SELECT ifnull(min(logtime), '') FROM pinglog WHERE target = ?", t).Scan(&oldest)
				if oldest == "" || oldest >= cut {
					break
				}
				ot, err := g.ParseProbeTime(oldest)
				if err != nil {
					break
				}
				hi := ot.AddDate(0, 0, 1).Format("2006-01-02")
				if hi > cut {
					hi = cut
				}
				if _, ok := exec("DELETE FROM pinglog WHERE target = ? AND logtime < ?", t, hi); !ok {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			exec("DELETE FROM pinglog_5m WHERE target = ? AND bucket < ?", t, cut)
		}
	} else {
		for {
			n, ok := exec("DELETE FROM pinglog WHERE rowid IN (SELECT rowid FROM pinglog WHERE logtime < ? LIMIT 5000)", cut)
			if !ok || n == 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		exec("DELETE FROM pinglog_5m WHERE bucket < ?", cut)
	}
	for _, tbl := range []string{"alertlog", "mappinglog"} {
		for {
			n, ok := exec("DELETE FROM "+tbl+" WHERE rowid IN (SELECT rowid FROM "+tbl+" WHERE logtime < ? LIMIT 5000)", cut)
			if !ok || n == 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	// 回收空间: WAL 截断回 0; 开启了 incremental auto_vacuum 的库把空闲页归还磁盘
	g.Db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if total > 0 {
		g.DLock.Lock()
		g.Db.Exec("PRAGMA incremental_vacuum")
		g.DLock.Unlock()
	}
	seelog.Info("[func:ClearArchive] finished, deleted ", strconv.FormatInt(total, 10), " rows")
}
