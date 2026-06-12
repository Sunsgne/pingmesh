package http

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// 告警运维接口: 屏蔽(暂停)告警 / 确认告警 / ASN 查询。
// 屏蔽与确认数据存储在产生告警的源节点上, 页面通过 proxy 路由到对应节点,
// 因此鉴权采用集群互信(AuthData)。

func configOpsRoutes() {

	// 屏蔽管理: action=list|add|del
	http.HandleFunc("/api/mute.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		now := time.Now().Format("2006-01-02 15:04:05")
		switch r.FormValue("action") {
		case "add":
			target := r.FormValue("target")
			reason := r.FormValue("reason")
			by := r.FormValue("by")
			minutes, _ := strconv.Atoi(r.FormValue("minutes"))
			if target == "" || minutes <= 0 {
				renderErr(w, "参数错误: 需要 target 与 minutes(>0)")
				return
			}
			until := time.Now().Add(time.Duration(minutes) * time.Minute).Format("2006-01-02 15:04:05")
			g.DLock.Lock()
			_, err := g.Db.Exec("REPLACE INTO alertmute(target, reason, muteduntil, createdby, created_at) values(?,?,?,?,?)",
				target, reason, until, by, now)
			g.DLock.Unlock()
			if err != nil {
				renderErr(w, err.Error())
				return
			}
			seelog.Info("[func:/api/mute.json] mute ", target, " until ", until, " by ", by, " reason: ", reason)
			renderOk(w, map[string]interface{}{"muteduntil": until})
		case "del":
			target := r.FormValue("target")
			g.DLock.Lock()
			g.Db.Exec("DELETE FROM alertmute WHERE target = ?", target)
			g.DLock.Unlock()
			seelog.Info("[func:/api/mute.json] unmute ", target)
			renderOk(w, nil)
		default: // list
			type muteRow struct {
				Target     string `json:"target"`
				Targetname string `json:"targetname"`
				Reason     string `json:"reason"`
				Muteduntil string `json:"muteduntil"`
				Createdby  string `json:"createdby"`
			}
			out := []muteRow{}
			g.DLock.Lock()
			rows, err := g.Db.Query("SELECT target, ifnull(reason,''), muteduntil, ifnull(createdby,'') FROM alertmute WHERE muteduntil > ?", now)
			g.DLock.Unlock()
			if err == nil {
				for rows.Next() {
					m := muteRow{}
					if rows.Scan(&m.Target, &m.Reason, &m.Muteduntil, &m.Createdby) == nil {
						if n, ok := g.Cfg.Network[m.Target]; ok {
							m.Targetname = n.Name
						} else {
							m.Targetname = m.Target
						}
						out = append(out, m)
					}
				}
				rows.Close()
			}
			renderOk(w, map[string]interface{}{"mutes": out, "from": g.Cfg.Addr, "fromname": g.Cfg.Name})
		}
	})

	// 告警确认(已知晓+原因)
	http.HandleFunc("/api/alertack.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
		reason := r.FormValue("reason")
		by := r.FormValue("by")
		if id <= 0 {
			renderErr(w, "参数错误: 需要告警 id")
			return
		}
		g.DLock.Lock()
		_, err := g.Db.Exec("UPDATE alertlog SET ack=1, ackreason=?, ackby=?, acktime=? WHERE rowid=?",
			reason, by, time.Now().Format("2006-01-02 15:04:05"), id)
		g.DLock.Unlock()
		if err != nil {
			renderErr(w, err.Error())
			return
		}
		seelog.Info("[func:/api/alertack.json] ack #", id, " by ", by, " reason: ", reason)
		renderOk(w, nil)
	})

	// 告警判定诊断: 解释某条链路当前为什么告警/不告警
	http.HandleFunc("/api/alertdiag.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		target := r.FormValue("target")
		var rule map[string]string
		for _, t := range g.SelfCfg.Topology {
			if t["Addr"] == target {
				rule = t
				break
			}
		}
		if rule == nil {
			renderErr(w, "本节点没有对该目标的监测规则")
			return
		}
		sec, _ := strconv.Atoi(rule["Thdchecksec"])
		occ, _ := strconv.Atoi(rule["Thdoccnum"])
		since := time.Unix(time.Now().Unix()-int64(sec), 0).Format("2006-01-02 15:04")
		var total, badDelay, badLoss, badJitter, bad int
		g.DLock.Lock()
		row := g.Db.QueryRow(`select count(1),
			sum(case when cast(avgdelay as double) >= cast(? as double) then 1 else 0 end),
			sum(case when cast(losspk as double) >= cast(? as double) then 1 else 0 end),
			sum(case when ? != '' and cast(ifnull(jitter,0) as double) >= cast(? as double) then 1 else 0 end),
			sum(case when cast(avgdelay as double) >= cast(? as double)
				or cast(losspk as double) >= cast(? as double)
				or (? != '' and cast(ifnull(jitter,0) as double) >= cast(? as double)) then 1 else 0 end)
			from pinglog where logtime > ? and target = ?`,
			rule["Thdavgdelay"], rule["Thdloss"], rule["Thdjitter"], rule["Thdjitter"],
			rule["Thdavgdelay"], rule["Thdloss"], rule["Thdjitter"], rule["Thdjitter"],
			since, target)
		row.Scan(&total, &badDelay, &badLoss, &badJitter, &bad)
		g.DLock.Unlock()
		muted := false
		var muteUntil, muteReason string
		g.DLock.Lock()
		g.Db.QueryRow("SELECT muteduntil, ifnull(reason,'') FROM alertmute WHERE target = ? AND muteduntil > ?",
			target, time.Now().Format("2006-01-02 15:04:05")).Scan(&muteUntil, &muteReason)
		g.DLock.Unlock()
		if muteUntil != "" {
			muted = true
		}
		capacity := sec / 60
		verdict := "normal"
		if bad >= occ && occ > 0 {
			verdict = "alerting"
		}
		hint := ""
		if capacity < occ {
			hint = "窗口太小: " + strconv.Itoa(sec) + "秒最多累计" + strconv.Itoa(capacity) + "个异常分钟, 永远达不到触发次数" + strconv.Itoa(occ) + ", 请加大窗口或减小次数"
		} else if total == 0 {
			hint = "窗口内没有任何探测数据(节点刚启动或探测未运行)"
		}
		renderOk(w, map[string]interface{}{
			"from": g.Cfg.Addr, "fromname": g.Cfg.Name,
			"target": target, "rule": rule,
			"window_sec": sec, "capacity": capacity, "occ": occ,
			"total": total, "bad": bad,
			"bad_delay": badDelay, "bad_loss": badLoss, "bad_jitter": badJitter,
			"muted": muted, "mute_until": muteUntil, "mute_reason": muteReason,
			"verdict": verdict, "hint": hint,
		})
	})

	// ASN 查询: 基于 RIPE NCC(权威 RIR)的 RIPEstat 接口, 24h 内存缓存
	http.HandleFunc("/api/asn.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		host := r.FormValue("ip")
		if host == "" {
			renderErr(w, "参数错误: 需要 ip(或域名)")
			return
		}
		// 域名先解析
		ip := host
		if !ValidIP4(host) {
			ipaddr, err := net.ResolveIPAddr("ip4", host)
			if err != nil {
				renderErr(w, "域名解析失败: "+err.Error())
				return
			}
			ip = ipaddr.String()
		}
		if info, ok := asnLookup(ip); ok {
			renderOk(w, map[string]interface{}{
				"ip": ip, "asn": info.Asn, "holder": info.Holder, "prefix": info.Prefix,
			})
			return
		}
		renderErr(w, "未查询到 ASN 信息(可能为内网地址或查询超时)")
	})
}

/* ---------- ASN 缓存与查询 ---------- */

type asnInfo struct {
	Asn    int64
	Holder string
	Prefix string
	at     time.Time
}

var (
	asnCache   = map[string]asnInfo{}
	asnCacheMu sync.Mutex
)

func asnLookup(ip string) (asnInfo, bool) {
	asnCacheMu.Lock()
	if c, ok := asnCache[ip]; ok && time.Since(c.at) < 24*time.Hour {
		asnCacheMu.Unlock()
		return c, c.Asn != 0
	}
	asnCacheMu.Unlock()

	client := http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get("https://stat.ripe.net/data/prefix-overview/data.json?resource=" + ip)
	if err != nil {
		seelog.Error("[func:asnLookup] ", err)
		return asnInfo{}, false
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var out struct {
		Data struct {
			Resource string `json:"resource"`
			Asns     []struct {
				Asn    int64  `json:"asn"`
				Holder string `json:"holder"`
			} `json:"asns"`
		} `json:"data"`
	}
	info := asnInfo{at: time.Now()}
	if json.Unmarshal(body, &out) == nil && len(out.Data.Asns) > 0 {
		info.Asn = out.Data.Asns[0].Asn
		info.Holder = out.Data.Asns[0].Holder
		info.Prefix = out.Data.Resource
	}
	asnCacheMu.Lock()
	asnCache[ip] = info
	asnCacheMu.Unlock()
	return info, info.Asn != 0
}
