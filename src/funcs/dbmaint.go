package funcs

import (
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// StartDbMaintenance 启动后台存储维护: 旧库在线迁移为聚簇表, 然后回填 5 分钟汇总表。
// 两步都分小批持写锁执行, 期间探测照常入库; 中途重启会安全地重新开始或断点续跑。
func StartDbMaintenance() {
	go func() {
		time.Sleep(30 * time.Second)
		for {
			done := compactPinglog()
			backfillRollup()
			if done {
				return
			}
			time.Sleep(time.Hour)
		}
	}()
}

func dbPath() string { return g.Root + "/db/database.db" }

func fileSize(p string) int64 {
	if st, err := os.Stat(p); err == nil {
		return st.Size()
	}
	return 0
}

func freeBytes(dir string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}

const pinglogCols = "logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk, jitter"

// compactPinglog 把旧堆表(rowid + 两个索引)迁移为 PRIMARY KEY(target, logtime) WITHOUT ROWID。
// 返回 true 表示已是聚簇表(无需再试); 磁盘不足等情况返回 false, 一小时后重试。
func compactPinglog() bool {
	if g.PinglogCompact() {
		return true
	}
	size := fileSize(dbPath()) + fileSize(dbPath()+"-wal")
	// 新表约为旧表一半, VACUUM 还需要一份新库大小的临时文件和 WAL
	if need := size*3/2 + 1<<30; freeBytes(g.Root+"/db") < need {
		seelog.Error("[func:compactPinglog] not enough free disk (need ", need>>20, " MB), retry later")
		return false
	}
	t0 := time.Now()
	seelog.Info("[func:compactPinglog] migrating pinglog to clustered table, db ", size>>20, " MB")
	g.DLock.Lock()
	g.Db.Exec("DROP TABLE IF EXISTS pinglog_c")
	_, err := g.Db.Exec(strings.Replace(g.CompactPinglogDDL, "%s", "pinglog_c", 1))
	var maxRowid int64
	g.Db.QueryRow("SELECT ifnull(max(rowid), 0) FROM pinglog").Scan(&maxRowid)
	g.DLock.Unlock()
	if err != nil {
		seelog.Error("[func:compactPinglog] create ", err)
		return false
	}
	copySQL := "INSERT OR IGNORE INTO pinglog_c (" + pinglogCols + ") SELECT logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk, ifnull(jitter, 0) FROM pinglog WHERE rowid > ? AND rowid <= ? AND target IS NOT NULL AND logtime IS NOT NULL"
	const chunk = 50000
	last := int64(0)
	for last < maxRowid {
		next := last + chunk
		g.DLock.Lock()
		_, err := g.Db.Exec(copySQL, last, next)
		g.DLock.Unlock()
		if err != nil {
			seelog.Error("[func:compactPinglog] copy ", err)
			return false
		}
		last = next
		time.Sleep(20 * time.Millisecond)
	}
	// 收尾: 补拷迁移期间新写入的行, 在同一事务内交换表名
	g.DLock.Lock()
	tx, err := g.Db.Begin()
	if err == nil {
		if _, err = tx.Exec(copySQL, last, int64(1)<<62); err == nil {
			if _, err = tx.Exec("ALTER TABLE pinglog RENAME TO pinglog_legacy"); err == nil {
				_, err = tx.Exec("ALTER TABLE pinglog_c RENAME TO pinglog")
			}
		}
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}
	if err == nil {
		g.SetPinglogCompact(true)
	}
	g.DLock.Unlock()
	if err != nil {
		seelog.Error("[func:compactPinglog] swap ", err)
		return false
	}
	seelog.Info("[func:compactPinglog] swapped in ", time.Since(t0).Round(time.Second), ", dropping legacy table")

	g.DLock.Lock()
	if _, err := g.Db.Exec("DROP TABLE pinglog_legacy"); err != nil {
		seelog.Error("[func:compactPinglog] drop legacy ", err)
	}
	g.DLock.Unlock()
	// 回收空间: VACUUM 期间探测入库会排队等写锁(不丢), 只做这一次。
	// 同时开启 incremental auto_vacuum, 之后保留期清理释放的页可以随时归还给磁盘。
	t1 := time.Now()
	g.DLock.Lock()
	g.Db.Exec("PRAGMA auto_vacuum = INCREMENTAL")
	_, err = g.Db.Exec("VACUUM")
	g.DLock.Unlock()
	g.Db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		seelog.Error("[func:compactPinglog] vacuum ", err)
	}
	seelog.Info("[func:compactPinglog] done, vacuum ", time.Since(t1).Round(time.Second), ", db now ", fileSize(dbPath())>>20, " MB")
	return true
}

// pinglogTargets 列出 pinglog 中出现过的全部目标(松散索引扫描, 每个目标一次 B 树查找)
func pinglogTargets() []string {
	rows, err := g.Db.Query(`WITH RECURSIVE t(x) AS (
		SELECT (SELECT min(target) FROM pinglog)
		UNION ALL
		SELECT (SELECT min(target) FROM pinglog WHERE target > t.x) FROM t WHERE t.x IS NOT NULL
	) SELECT x FROM t WHERE x IS NOT NULL`)
	if err != nil {
		seelog.Error("[func:pinglogTargets] ", err)
		return nil
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			out = append(out, t)
		}
	}
	return out
}

func inList(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// backfillRollup 由历史原始样本生成 5 分钟汇总。按 6 小时分段, 每段在写锁内由原始样本整桶重算并覆盖
// (而不是累加): 无论实时入库已经往这些桶里加过多少, 重算结果都只取决于原始样本, 重跑也不会重复计数,
// 与系统时钟/时区是否一致无关。进度标记与数据同事务提交, 重启后从标记处继续。
func backfillRollup() {
	if g.RollupCovers("") {
		return
	}
	targets := pinglogTargets()
	markDone := func() {
		g.DLock.Lock()
		g.MetaSet(g.Db, "rollup_backfill_done", "1")
		g.DLock.Unlock()
		g.SetRollupReady()
	}
	if len(targets) == 0 {
		markDone()
		return
	}
	targetArgs := make([]interface{}, len(targets))
	for i, t := range targets {
		targetArgs[i] = t
	}
	in := inList(len(targets))
	var minT, maxT string
	g.Db.QueryRow("SELECT ifnull(min(logtime), ''), ifnull(max(logtime), '') FROM pinglog WHERE target IN ("+in+")", targetArgs...).Scan(&minT, &maxT)
	if from := g.MetaGet(g.Db, "rollup_backfill_to"); from != "" && from > minT {
		minT = from
	}
	start, err1 := g.ParseProbeTime(minT)
	end, err2 := g.ParseProbeTime(maxT)
	if err1 != nil || err2 != nil {
		markDone()
		return
	}
	start = start.Truncate(6 * time.Hour)
	t0 := time.Now()
	seelog.Info("[func:backfillRollup] rebuilding 5-minute rollup ", start.Format("2006-01-02 15:04"), " .. ", maxT)
	sqlText := strings.Replace(g.RollupSelectSQL, "INSERT INTO", "INSERT OR REPLACE INTO", 1) +
		" WHERE target IN (" + in + ") AND logtime >= ? AND logtime < ? GROUP BY 1, 2"
	for cur := start; !cur.After(end); {
		next := cur.Add(6 * time.Hour)
		lo, hi := cur.Format("2006-01-02 15:04:05"), next.Format("2006-01-02 15:04:05")
		args := append(append([]interface{}{}, targetArgs...), lo, hi)
		g.DLock.Lock()
		tx, err := g.Db.Begin()
		if err == nil {
			if _, err = tx.Exec(sqlText, args...); err == nil {
				err = g.MetaSet(tx, "rollup_backfill_to", hi)
			}
			if err != nil {
				tx.Rollback()
			} else {
				err = tx.Commit()
			}
		}
		g.DLock.Unlock()
		if err != nil {
			seelog.Error("[func:backfillRollup] ", err)
			return
		}
		cur = next
		time.Sleep(10 * time.Millisecond)
	}
	markDone()
	seelog.Info("[func:backfillRollup] done in ", time.Since(t0).Round(time.Second))
}
