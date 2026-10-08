package funcs

import (
	"strconv"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// ClearArchive 清理超过保留天数的历史数据。
// 分批删除(每批 5000 行, 批间让出写锁): 一次性 DELETE 数百万行会长时间占住写锁、阻塞探测入库,
// 并把 WAL 撑到数 GB(曾导致节点磁盘写满)。按北京时间(localtime)计算截止日, 与 logtime 一致。
func ClearArchive() {
	days := g.Cfg.Base["Archive"]
	if days <= 0 {
		return
	}
	seelog.Info("[func:ClearArchive] starting, keep ", days, " days")
	cut := "date('now','localtime','start of day','-" + strconv.Itoa(days) + " day')"
	total := int64(0)
	for _, tbl := range []string{"pinglog", "alertlog", "mappinglog"} {
		for {
			g.DLock.Lock()
			res, err := g.Db.Exec("DELETE FROM " + tbl + " WHERE rowid IN (SELECT rowid FROM " + tbl + " WHERE logtime < " + cut + " LIMIT 5000)")
			g.DLock.Unlock()
			if err != nil {
				seelog.Error("[func:ClearArchive] ", tbl, " ", err)
				break
			}
			n, _ := res.RowsAffected()
			total += n
			if n == 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	// 回收 WAL: 删除产生的 WAL 截断回 0, 不再长期占盘
	if _, err := g.Db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		seelog.Error("[func:ClearArchive] checkpoint ", err)
	}
	seelog.Info("[func:ClearArchive] finished, deleted ", total, " rows")
}
