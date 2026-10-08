package g

import (
	"database/sql"
	"strings"
	"sync/atomic"
	"time"
)

// 探测数据存储
// ------------------------------------------------------------------
//   - pinglog: 原始 10 秒样本。新库直接建为 PRIMARY KEY(target, logtime) WITHOUT ROWID
//     (按目标聚簇, 体积约为旧堆表+双索引的一半); 旧库由后台任务在线迁移。
//   - pinglog_5m: 每目标每 5 分钟一行的汇总(样本数/和/极值), 与原始样本同事务写入。
//     步长 ≥ 5 分钟的图表直接读它, 一年跨度只需扫描约 10 万行而不是 300 万行。
//   - pm_meta: 迁移/回填进度(断点续跑, 保证不会重复累加)。

const RollupBucketSec = 300

// CompactPinglogDDL 聚簇 pinglog 表结构, %s 为表名
const CompactPinglogDDL = `CREATE TABLE IF NOT EXISTS %s (
	logtime  VARCHAR (19) NOT NULL,
	target   VARCHAR (64) NOT NULL,
	maxdelay FLOAT,
	mindelay FLOAT,
	avgdelay FLOAT,
	sendpk   INT,
	revcpk   INT,
	losspk   FLOAT,
	jitter   FLOAT DEFAULT 0,
	PRIMARY KEY (target, logtime)
) WITHOUT ROWID`

const rollupDDL = `CREATE TABLE IF NOT EXISTS pinglog_5m (
	target   VARCHAR (64) NOT NULL,
	bucket   VARCHAR (19) NOT NULL,
	n        INT  NOT NULL,
	n_ok     INT  NOT NULL,
	sum_avg  REAL NOT NULL,
	max_max  REAL,
	min_min  REAL,
	sum_loss REAL NOT NULL,
	sum_jit  REAL NOT NULL,
	PRIMARY KEY (target, bucket)
) WITHOUT ROWID`

// RollupUpsertSQL 累加一个或多个样本到 5 分钟桶。
// n_ok/sum_avg/max_max/min_min 只统计非全丢包样本(全丢包时延迟记为 0, 不代表真实延迟)。
const RollupUpsertSQL = ` ON CONFLICT (target, bucket) DO UPDATE SET
	n = n + excluded.n,
	n_ok = n_ok + excluded.n_ok,
	sum_avg = sum_avg + excluded.sum_avg,
	max_max = CASE WHEN excluded.max_max IS NULL THEN max_max WHEN max_max IS NULL THEN excluded.max_max ELSE max(max_max, excluded.max_max) END,
	min_min = CASE WHEN excluded.min_min IS NULL THEN min_min WHEN min_min IS NULL THEN excluded.min_min ELSE min(min_min, excluded.min_min) END,
	sum_loss = sum_loss + excluded.sum_loss,
	sum_jit = sum_jit + excluded.sum_jit`

// RollupRowSQL 单个样本写入汇总表
const RollupRowSQL = `INSERT INTO pinglog_5m (target, bucket, n, n_ok, sum_avg, max_max, min_min, sum_loss, sum_jit)
	VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?)` + RollupUpsertSQL

// RollupSelectSQL 由原始样本按 5 分钟分组生成汇总行(回填用), 需追加 WHERE 条件
const RollupSelectSQL = `INSERT INTO pinglog_5m (target, bucket, n, n_ok, sum_avg, max_max, min_min, sum_loss, sum_jit)
	SELECT target,
		substr(logtime, 1, 15) || CASE WHEN cast(substr(logtime, 16, 1) AS INT) < 5 THEN '0' ELSE '5' END || ':00',
		count(1),
		sum(CASE WHEN losspk < 100 THEN 1 ELSE 0 END),
		ifnull(sum(CASE WHEN losspk < 100 THEN avgdelay END), 0),
		max(CASE WHEN losspk < 100 THEN maxdelay END),
		min(CASE WHEN losspk < 100 THEN (CASE WHEN mindelay < 0 THEN 0 ELSE mindelay END) END),
		ifnull(sum(losspk), 0),
		ifnull(sum(ifnull(jitter, 0)), 0)
	FROM pinglog`

// RollupBucket 样本时间所属 5 分钟桶(与 RollupSelectSQL 的字符串截断一致)
func RollupBucket(t time.Time) string {
	return t.Truncate(RollupBucketSec * time.Second).Format("2006-01-02 15:04:05")
}

var (
	pinglogCompact int32
	rollupReady    int32
	rollupLiveFrom atomic.Value // string: 汇总表开始实时写入的时刻, 之前的数据靠回填
)

// PinglogCompact 当前 pinglog 是否已是聚簇(WITHOUT ROWID)表
func PinglogCompact() bool { return atomic.LoadInt32(&pinglogCompact) == 1 }

func SetPinglogCompact(v bool) {
	if v {
		atomic.StoreInt32(&pinglogCompact, 1)
	} else {
		atomic.StoreInt32(&pinglogCompact, 0)
	}
}

// RollupCovers 汇总表是否完整覆盖 [start, ...): 回填完成, 或起点晚于实时写入开始时刻
func RollupCovers(start string) bool {
	if atomic.LoadInt32(&rollupReady) == 1 {
		return true
	}
	lf, _ := rollupLiveFrom.Load().(string)
	return lf != "" && start >= lf
}

func SetRollupReady() { atomic.StoreInt32(&rollupReady, 1) }

func RollupLiveFrom() string {
	lf, _ := rollupLiveFrom.Load().(string)
	return lf
}

// MetaGet / MetaSet 读写 pm_meta(可在事务中使用)
type execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func MetaGet(q execer, k string) string {
	var v string
	q.QueryRow("SELECT v FROM pm_meta WHERE k = ?", k).Scan(&v)
	return v
}

func MetaSet(q execer, k, v string) error {
	_, err := q.Exec("INSERT INTO pm_meta (k, v) VALUES (?, ?) ON CONFLICT (k) DO UPDATE SET v = excluded.v", k, v)
	return err
}

func tableSQL(name string) string {
	var s string
	Db.QueryRow("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&s)
	return s
}

func IsCompactTable(name string) bool {
	return strings.Contains(strings.ToUpper(tableSQL(name)), "WITHOUT ROWID")
}

// initProbeStorage 在 InitDbSchema 中调用(已持 DLock)
func initProbeStorage() error {
	if _, err := Db.Exec(`CREATE TABLE IF NOT EXISTS pm_meta (k VARCHAR (64) PRIMARY KEY, v TEXT)`); err != nil {
		return err
	}
	if tableSQL("pinglog") == "" {
		if _, err := Db.Exec(strings.Replace(CompactPinglogDDL, "%s", "pinglog", 1)); err != nil {
			return err
		}
	}
	SetPinglogCompact(IsCompactTable("pinglog"))
	if !PinglogCompact() {
		// 旧堆表在迁移完成前继续使用原索引
		Db.Exec(`CREATE INDEX IF NOT EXISTS idx_pinglog_target_time ON pinglog (target, logtime)`)
		Db.Exec(`CREATE INDEX IF NOT EXISTS idx_pinglog_time ON pinglog (logtime)`)
		Db.Exec(`ALTER TABLE pinglog ADD COLUMN jitter FLOAT DEFAULT 0`)
	}
	if _, err := Db.Exec(rollupDDL); err != nil {
		return err
	}
	// 汇总表首次出现时记下实时写入起点: 此前的原始样本由后台回填, 此后的由入库同步累加
	lf := MetaGet(Db, "rollup_live_from")
	if lf == "" {
		lf = ProbeLogTime(time.Now())
		if err := MetaSet(Db, "rollup_live_from", lf); err != nil {
			return err
		}
	}
	rollupLiveFrom.Store(lf)
	if MetaGet(Db, "rollup_backfill_done") == "1" {
		SetRollupReady()
	}
	return nil
}
