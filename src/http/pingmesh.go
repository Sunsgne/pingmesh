package http

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// PingmeshCell 网格单元: 本节点 -> 目标 的聚合统计
type PingmeshCell struct {
	AvgDelay  float64 `json:"avgdelay"`
	MaxDelay  float64 `json:"maxdelay"`
	MinDelay  float64 `json:"mindelay"`
	Loss      float64 `json:"loss"`
	Jitter    float64 `json:"jitter"`
	Baseline  float64 `json:"baseline"` // 近24小时正常延迟基线, 用于相对涨幅着色
	LastCheck string  `json:"lastcheck"`
	Points    int     `json:"points"`
}

// PingmeshRow 本节点的 Pingmesh 行数据
type PingmeshRow struct {
	From     string                  `json:"from"`
	FromName string                  `json:"fromname"`
	Mins     int                     `json:"mins"`
	Targets  []string                `json:"targets"`
	Cells    map[string]PingmeshCell `json:"cells"`
}

func configPingmeshRoutes() {

	// Pingmesh行数据API: 返回本节点最近N分钟对各监测目标的聚合指标
	http.HandleFunc("/api/pingmesh.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		mins := 15
		if len(r.Form["mins"]) > 0 {
			if m, err := strconv.Atoi(r.Form["mins"][0]); err == nil && m > 0 && m <= 30*24*60 {
				mins = m
			}
		}
		// 支持自定义起止时间(格式 2006-01-02 15:04), 优先于 mins
		timeStartStr := time.Unix(time.Now().Unix()-int64(mins)*60, 0).Format("2006-01-02 15:04")
		timeEndStr := time.Now().Format("2006-01-02 15:04")
		if len(r.Form["start"]) > 0 && len(r.Form["end"]) > 0 {
			if s, err := time.Parse("2006-01-02 15:04", r.Form["start"][0]); err == nil {
				if e, err2 := time.Parse("2006-01-02 15:04", r.Form["end"][0]); err2 == nil && e.After(s) {
					// 自定义区间最长 31 天: 更长的聚合要几十秒, 超出界面超时也白算
					if e.Sub(s) > 31*24*time.Hour {
						s = e.Add(-31 * 24 * time.Hour)
					}
					timeStartStr = s.Format("2006-01-02 15:04")
					timeEndStr = e.Format("2006-01-02 15:04")
					mins = int(e.Sub(s).Minutes())
				}
			}
		}
		row := PingmeshRow{
			From:     g.Cfg.Addr,
			FromName: g.Cfg.Name,
			Mins:     mins,
			Targets:  []string{},
			Cells:    map[string]PingmeshCell{},
		}
		if g.SelfCfg.Ping != nil {
			row.Targets = g.SelfCfg.Ping
		}
		// 按目标列表查询(主键前缀), 聚簇表上没有单独的时间索引
		args := make([]interface{}, 0, len(row.Targets)+2)
		for _, t := range row.Targets {
			args = append(args, t)
		}
		args = append(args, timeStartStr, timeEndStr)
		in := strings.TrimSuffix(strings.Repeat("?,", len(row.Targets)), ",")
		// 全丢包的样本 avgdelay 记为 0, 不计入延迟均值, 否则中断期间会把延迟拉低成"更好"
		querySql := "select target, ifnull(avg(case when losspk < 100 then avgdelay end),0), ifnull(max(case when losspk < 100 then maxdelay end),0), ifnull(min(case when losspk < 100 then (case when mindelay < 0 then 0 else mindelay end) end),0), ifnull(avg(losspk),0), ifnull(avg(ifnull(jitter,0)),0), max(logtime), count(1) from pinglog where target in (" + in + ") and logtime >= ? and logtime <= ? group by target"
		// 6 小时以上的窗口读 5 分钟汇总表
		if mins >= 360 && g.RollupCovers(timeStartStr) {
			querySql = "select target, ifnull(sum(sum_avg)/nullif(sum(n_ok),0),0), ifnull(max(max_max),0), ifnull(min(min_min),0), ifnull(sum(sum_loss)/sum(n),0), ifnull(sum(sum_jit)/sum(n),0), max(bucket), sum(n) from pinglog_5m where target in (" + in + ") and bucket >= ? and bucket <= ? group by target"
		}
		if len(row.Targets) == 0 {
			querySql = "select '', 0, 0, 0, 0, 0, '', 0 where 0"
			args = nil
		}
		// 只读查询不加全局写锁(WAL 下读写并发安全), 避免长查询阻塞每 10 秒的入库与登录
		rows, err := g.Db.QueryContext(r.Context(), querySql, args...)
		if err != nil {
			seelog.Error("[func:/api/pingmesh.json] Query ", err)
		} else {
			for rows.Next() {
				var target string
				c := PingmeshCell{}
				if err := rows.Scan(&target, &c.AvgDelay, &c.MaxDelay, &c.MinDelay, &c.Loss, &c.Jitter, &c.LastCheck, &c.Points); err != nil {
					seelog.Error("[func:/api/pingmesh.json] Rows ", err)
					continue
				}
				row.Cells[target] = c
			}
			rows.Close()
		}
		// 近24小时基线(有效样本: 有延迟且非全丢包), 供前端按相对涨幅着色
		for target, base := range baselines24h() {
			if c, ok := row.Cells[target]; ok {
				c.Baseline = base
				row.Cells[target] = c
			}
		}
		// 无历史基线时用当前窗口均值兜底 → 稳态链路显示为绿色
		for t, c := range row.Cells {
			if c.Baseline <= 0 && c.AvgDelay > 0 && c.Loss < 100 {
				c.Baseline = c.AvgDelay
				row.Cells[t] = c
			}
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, row)
	})
}

// 近 24 小时基线变化很慢, 缓存 5 分钟: 每次打开矩阵都重算要 100ms+ 的全量聚合
var (
	baselineMu    sync.Mutex
	baselineCache map[string]float64
	baselineAt    time.Time
)

func baselines24h() map[string]float64 {
	baselineMu.Lock()
	defer baselineMu.Unlock()
	if baselineCache != nil && time.Since(baselineAt) < 5*time.Minute {
		return baselineCache
	}
	out := map[string]float64{}
	baseStart := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04")
	baseEnd := time.Now().Format("2006-01-02 15:04")
	targets := g.SelfCfg.Ping
	if len(targets) == 0 {
		return out
	}
	args := make([]interface{}, 0, len(targets)+2)
	for _, t := range targets {
		args = append(args, t)
	}
	args = append(args, baseStart, baseEnd)
	in := strings.TrimSuffix(strings.Repeat("?,", len(targets)), ",")
	q := "select target, avg(avgdelay) from pinglog where target in (" + in + ") and logtime >= ? and logtime <= ? and avgdelay > 0 and losspk < 100 group by target"
	if g.RollupCovers(baseStart) {
		q = "select target, sum(sum_avg)/sum(n_ok) from pinglog_5m where target in (" + in + ") and bucket >= ? and bucket <= ? and n_ok > 0 group by target"
	}
	rows, err := g.Db.Query(q, args...)
	if err != nil {
		seelog.Error("[func:/api/pingmesh.json] Baseline ", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var target string
		var base float64
		if rows.Scan(&target, &base) == nil {
			out[target] = base
		}
	}
	baselineCache, baselineAt = out, time.Now()
	return out
}
