package http

import (
	"net/http"
	"strconv"
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
		querySql := "select target, avg(avgdelay), max(maxdelay), min(case when mindelay < 0 then 0 else mindelay end), avg(losspk), max(logtime), count(1) from pinglog where logtime >= ? and logtime <= ? group by target"
		g.DLock.Lock()
		rows, err := g.Db.Query(querySql, timeStartStr, timeEndStr)
		g.DLock.Unlock()
		if err != nil {
			seelog.Error("[func:/api/pingmesh.json] Query ", err)
		} else {
			for rows.Next() {
				var target string
				c := PingmeshCell{}
				if err := rows.Scan(&target, &c.AvgDelay, &c.MaxDelay, &c.MinDelay, &c.Loss, &c.LastCheck, &c.Points); err != nil {
					seelog.Error("[func:/api/pingmesh.json] Rows ", err)
					continue
				}
				row.Cells[target] = c
			}
			rows.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, row)
	})
}
