package funcs

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// AlertLinkHealth 单条监测链路的健康快照(供主节点汇总告警)
type AlertLinkHealth struct {
	OK       bool    `json:"ok"`
	Muted    bool    `json:"muted"`
	Name     string  `json:"name"`
	AvgDelay float64 `json:"avgdelay"`
	Loss     float64 `json:"loss"`
	Jitter   float64 `json:"jitter"`
}

// AlertHealthSnapshot 某探测节点对外暴露的告警判定快照
type AlertHealthSnapshot struct {
	From     string                     `json:"from"`
	FromName string                     `json:"fromname"`
	Links    map[string]AlertLinkHealth `json:"links"`
}

// LocalAlertHealth 本节点根据本地 pinglog 生成健康快照(信息收集节点侧)
func LocalAlertHealth() AlertHealthSnapshot {
	out := AlertHealthSnapshot{
		From:     g.Cfg.Addr,
		FromName: g.Cfg.Name,
		Links:    map[string]AlertLinkHealth{},
	}
	for _, v := range g.SelfCfg.Topology {
		addr := v["Addr"]
		if addr == "" || addr == g.SelfCfg.Addr {
			continue
		}
		ok := CheckAlertStatus(v)
		h := AlertLinkHealth{
			OK:    ok,
			Muted: IsMuted(addr),
			Name:  v["Name"],
		}
		if n, has := g.Cfg.Network[addr]; has && n.Name != "" {
			h.Name = n.Name
		}
		if avg, loss, jitter, has := recentStat(addr, 15); has {
			h.AvgDelay, h.Loss, h.Jitter = avg, loss, jitter
		}
		out.Links[addr] = h
	}
	return out
}

func fetchAlertHealth(endpoint string, timeout time.Duration) (AlertHealthSnapshot, bool) {
	snap := AlertHealthSnapshot{Links: map[string]AlertLinkHealth{}}
	client := http.Client{Timeout: timeout}
	// 优先新接口; 旧 Agent 回退到 topology.json
	url := "http://" + endpoint + "/api/alerthealth.json"
	resp, err := client.Get(g.SignURL(url, g.Cfg.Password))
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode == 200 && json.Unmarshal(body, &snap) == nil {
			if snap.Links == nil {
				snap.Links = map[string]AlertLinkHealth{}
			}
			return snap, true
		}
	}
	url = "http://" + endpoint + "/api/topology.json"
	resp2, err := client.Get(g.SignURL(url, g.Cfg.Password))
	if err != nil {
		return snap, false
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp2.Body, 1<<20))
	if resp2.StatusCode != 200 {
		return snap, false
	}
	raw := map[string]string{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return snap, false
	}
	for addr, st := range raw {
		snap.Links[addr] = AlertLinkHealth{OK: st == "true", Name: addr}
	}
	return snap, true
}

func nodeHTTPEndpoint(addr string) string {
	return addr + ":" + strconv.Itoa(g.Cfg.Port)
}

func topologyRule(member g.NetworkMember, target string) map[string]string {
	for _, t := range member.Topology {
		if t["Addr"] == target {
			return t
		}
	}
	// 兜底: 用目标名构造最小规则, 便于通知文案
	name := target
	if n, ok := g.Cfg.Network[target]; ok && n.Name != "" {
		name = n.Name
	}
	return map[string]string{
		"Addr":        target,
		"Name":        name,
		"Thdavgdelay": "200",
		"Thdloss":     "30",
		"Thdchecksec": "900",
		"Thdoccnum":   "3",
	}
}

func masterIncidentKey(from, to string) string {
	return from + "|" + to
}

var (
	masterAlertMu     sync.Mutex
	masterAlertStatus = map[string]bool{} // key=from|to, true=正常
)

func masterGetStatus(key string) (bool, bool) {
	masterAlertMu.Lock()
	defer masterAlertMu.Unlock()
	v, ok := masterAlertStatus[key]
	return v, ok
}

func masterSetStatus(key string, healthy bool) {
	masterAlertMu.Lock()
	masterAlertStatus[key] = healthy
	masterAlertMu.Unlock()
}

// MasterAlertSweep 仅代理主节点执行: 汇总各探测节点健康快照, 统一判定并发送告警。
// 其他节点只负责探测入库 + 暴露 /api/alerthealth.json, 不再自行发邮件/Webhook。
func MasterAlertSweep() {
	if !g.IsActingMaster() {
		return
	}
	seelog.Info("[func:MasterAlertSweep] starting cluster alert sweep")
	remindMin := g.Cfg.Base["Remindmin"]

	for addr, member := range g.Cfg.Network {
		if !member.Pingmesh {
			continue
		}
		var snap AlertHealthSnapshot
		ok := false
		if addr == g.Cfg.Addr {
			snap = LocalAlertHealth()
			ok = true
		} else {
			snap, ok = fetchAlertHealth(nodeHTTPEndpoint(addr), 5*time.Second)
		}
		if !ok {
			seelog.Info("[func:MasterAlertSweep] skip unreachable probe ", member.Name, " (", addr, ")")
			continue
		}
		if snap.FromName == "" {
			snap.FromName = member.Name
		}
		if snap.From == "" {
			snap.From = addr
		}

		// 以配置中的 Topology 为准遍历, 快照缺失的链路视为不可判定(跳过, 避免误报)
		for _, rule := range member.Topology {
			target := rule["Addr"]
			if target == "" || target == addr {
				continue
			}
			h, has := snap.Links[target]
			if !has {
				continue
			}
			key := masterIncidentKey(addr, target)
			old, haskey := masterGetStatus(key)
			// true=正常, false=异常 (与旧 StartAlert 语义一致)
			healthy := h.OK
			st := incidentOf(key)
			l := g.AlertLog{
				Fromname:   snap.FromName,
				Fromip:     addr,
				Targetname: h.Name,
				Targetip:   target,
				Logtime:    time.Now().Format("2006-01-02 15:04"),
			}
			if l.Targetname == "" {
				l.Targetname = rule["Name"]
			}
			r := topologyRule(member, target)
			// 合并配置规则字段(阈值文案)
			for k, v := range rule {
				if v != "" {
					r[k] = v
				}
			}

			if healthy {
				masterSetStatus(key, true)
				if haskey && !old {
					seelog.Info("[func:MasterAlertSweep] recovered ", key)
					if !h.Muted {
						extras := [][2]string{}
						if !st.BadSince.IsZero() {
							extras = append(extras, [2]string{"故障持续", fmtDur(time.Since(st.BadSince))})
						}
						if h.AvgDelay > 0 || h.Loss > 0 {
							extras = append(extras, [2]string{"近15分钟实测",
								"平均延迟 " + strconv.FormatFloat(h.AvgDelay, 'f', 1, 64) + " ms / 丢包 " +
									strconv.FormatFloat(h.Loss, 'f', 0, 64) + "% / 抖动 " +
									strconv.FormatFloat(h.Jitter, 'f', 1, 64) + " ms"})
						}
						go NotifyAll(l, r, "recovery", extras)
					}
					st.BadSince = time.Time{}
					st.MutedSkip = false
					st.AckedUntil = time.Time{}
				}
				continue
			}

			// 异常
			if !haskey || old {
				seelog.Info("[func:MasterAlertSweep] alert ", key)
				masterSetStatus(key, false)
				st.BadSince = time.Now()
				st.AckedUntil = time.Time{}
				go AlertStorage(l)
				if h.Muted {
					st.MutedSkip = true
					seelog.Info("[func:MasterAlertSweep] ", key, " muted, notification skipped")
				} else {
					st.LastNotify = time.Now()
					extras := [][2]string{}
					if h.AvgDelay > 0 || h.Loss > 0 {
						extras = append(extras, [2]string{"近15分钟实测",
							"平均延迟 " + strconv.FormatFloat(h.AvgDelay, 'f', 1, 64) + " ms / 丢包 " +
								strconv.FormatFloat(h.Loss, 'f', 0, 64) + "% / 抖动 " +
								strconv.FormatFloat(h.Jitter, 'f', 1, 64) + " ms"})
					}
					go NotifyAll(l, r, "alert", extras)
				}
				continue
			}

			dur := [2]string{"故障持续", fmtDur(time.Since(st.BadSince))}
			if h.Muted {
				st.MutedSkip = true
			} else if st.MutedSkip {
				seelog.Info("[func:MasterAlertSweep] ", key, " mute expired but still down")
				st.MutedSkip = false
				st.LastNotify = time.Now()
				go NotifyAll(l, r, "mute_expired", [][2]string{dur})
			} else if remindMin > 0 && time.Now().After(st.AckedUntil) && !st.LastNotify.IsZero() &&
				time.Since(st.LastNotify) >= time.Duration(remindMin)*time.Minute {
				seelog.Info("[func:MasterAlertSweep] ", key, " reminder")
				st.LastNotify = time.Now()
				go NotifyAll(l, r, "reminder", [][2]string{dur})
			}
		}
	}
	seelog.Info("[func:MasterAlertSweep] finish")
}
