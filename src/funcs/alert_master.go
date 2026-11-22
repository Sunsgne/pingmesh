package funcs

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// AlertLinkHealth 单条监测链路的健康快照(供主节点汇总告警)
type AlertLinkHealth struct {
	OK         bool    `json:"ok"`
	Muted      bool    `json:"muted"`
	Name       string  `json:"name"`
	AvgDelay   float64 `json:"avgdelay"`
	Loss       float64 `json:"loss"`
	Jitter     float64 `json:"jitter"`
	PeakDelay  float64 `json:"peakdelay,omitempty"`
	PeakLoss   float64 `json:"peakloss,omitempty"`
	PeakJitter float64 `json:"peakjitter,omitempty"`
	DelayHits  int     `json:"delayhits,omitempty"`
	LossHits   int     `json:"losshits,omitempty"`
	JitterHits int     `json:"jitterhits,omitempty"`
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
		b := windowBreach(addr, v)
		h.AvgDelay, h.Loss, h.Jitter = b.MeanDelay, b.MeanLoss, b.MeanJitter
		h.PeakDelay, h.PeakLoss, h.PeakJitter = b.DelayMax, b.LossMax, b.JitterMax
		h.DelayHits, h.LossHits, h.JitterHits = b.DelayN, b.LossN, b.JitterN
		if b.Samples == 0 {
			if avg, loss, jitter, has := recentStat(addr, 15); has {
				h.AvgDelay, h.Loss, h.Jitter = avg, loss, jitter
			}
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
	if err != nil {
		// 网络不通时直接判不可达; 再试 topology.json 只会把一次巡检的耗时翻倍
		return snap, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 200 && json.Unmarshal(body, &snap) == nil {
		if snap.Links == nil {
			snap.Links = map[string]AlertLinkHealth{}
		}
		return snap, true
	}
	url = "http://" + endpoint + "/api/topology.json"
	resp2, err := client.Get(g.SignURL(url, g.Cfg.Password))
	if err != nil {
		return snap, false
	}
	defer resp2.Body.Close()
	body, _ = io.ReadAll(io.LimitReader(resp2.Body, 1<<20))
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

// masterTransition 判断是否要发告警或恢复。
// 进程刚启动时这条链路还没见过，只记基线，不把仍在故障中的链路再发一遍。
const recoveryHold = 2 * time.Minute

// recoveryHoldElapsed 恢复通知要等链路连续正常满 hold。since 为零表示还没开始计时。
func recoveryHoldElapsed(since, now time.Time, hold time.Duration) bool {
	return !since.IsZero() && now.Sub(since) >= hold
}

func masterTransition(seen, wasHealthy, nowHealthy bool) (alert, recovery bool) {
	if !seen {
		return false, false
	}
	if nowHealthy && !wasHealthy {
		return false, true
	}
	if !nowHealthy && wasHealthy {
		return true, false
	}
	return false, false
}

// MasterAlertSweep 仅代理主节点执行: 汇总各探测节点健康快照, 统一判定并发送告警。
// 其他节点只负责探测入库 + 暴露 /api/alerthealth.json, 不再自行发邮件/Webhook。
// 巡检每 10 秒一次; 上一轮未结束就跳过, 避免并发判定导致同一告警重复发送
var sweepRunning int32

type sweepResult struct {
	snap AlertHealthSnapshot
	ok   bool
}

func MasterAlertSweep() {
	if !g.IsActingMaster() {
		return
	}
	if !atomic.CompareAndSwapInt32(&sweepRunning, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&sweepRunning, 0)
	seelog.Debug("[func:MasterAlertSweep] starting cluster alert sweep")
	remindMin := g.Cfg.Base["Remindmin"]

	// 并行拉取各节点快照(串行时每个不可达节点要等满超时, 两三个节点断开一轮就超过 10 秒)
	results := map[string]sweepResult{}
	var resMu sync.Mutex
	var wg sync.WaitGroup
	for addr, member := range g.Cfg.Network {
		if !member.Pingmesh {
			continue
		}
		if addr == g.Cfg.Addr {
			results[addr] = sweepResult{LocalAlertHealth(), true}
			continue
		}
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			snap, ok := fetchAlertHealth(nodeHTTPEndpoint(addr), 4*time.Second)
			resMu.Lock()
			results[addr] = sweepResult{snap, ok}
			resMu.Unlock()
		}(addr)
	}
	wg.Wait()

	loadOpenEvents()
	now := time.Now()
	delta := sweepDelta{
		present: map[string]bool{}, reachable: map[string]bool{},
		now: now, remind: time.Duration(remindMin) * time.Minute,
	}
	for addr, member := range g.Cfg.Network {
		if !member.Pingmesh {
			continue
		}
		for _, rule := range member.Topology {
			if t := rule["Addr"]; t != "" && t != addr {
				delta.present[masterIncidentKey(addr, t)] = true
			}
		}
	}
	for addr, member := range g.Cfg.Network {
		if !member.Pingmesh {
			continue
		}
		res := results[addr]
		snap, ok := res.snap, res.ok
		if !ok {
			seelog.Info("[func:MasterAlertSweep] skip unreachable probe ", member.Name, " (", addr, ")")
			continue
		}
		delta.reachable[addr] = true
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
			healthy := h.OK
			st := incidentOf(key)
			l := g.AlertLog{
				Fromname:   snap.FromName,
				Fromip:     addr,
				Targetname: h.Name,
				Targetip:   target,
				Logtime:    now.Format("2006-01-02 15:04"),
			}
			if l.Targetname == "" {
				l.Targetname = rule["Name"]
			}
			r := topologyRule(member, target)
			for k, v := range rule {
				if v != "" {
					r[k] = v
				}
			}
			cellDetail := ""
			if !h.OK {
				_, cellDetail, _ = classifyBreach(addr, target, r, breachFromHealth(h))
			}
			delta.cells = append(delta.cells, meshCell{
				FromIP: addr, FromName: snap.FromName, ToIP: target, ToName: l.Targetname,
				OK: h.OK, Delay: h.AvgDelay, Loss: h.Loss, Detail: cellDetail,
			})

			alertNow, recoveryNow := masterTransition(haskey, old, healthy)
			if healthy {
				if recoveryNow {
					// 连续正常满 2 分钟才算恢复，期间仍按故障计，不会再开一个新事件。
					if st.HealthySince.IsZero() {
						st.HealthySince = now
					}
					if !recoveryHoldElapsed(st.HealthySince, now, recoveryHold) {
						continue
					}
					seelog.Info("[func:MasterAlertSweep] recovered ", key)
					delta.recovered = append(delta.recovered, key)
					st.BadSince = time.Time{}
					st.MutedSkip = false
					st.AckedUntil = time.Time{}
				}
				st.HealthySince = time.Time{}
				masterSetStatus(key, true)
				continue
			}

			// 异常。第一次见到只记状态，避免重启后把存量故障再通知一遍；这类链路恢复时也不发恢复。
			st.HealthySince = time.Time{}
			if !haskey {
				masterSetStatus(key, false)
				st.BadSince = now
				seelog.Info("[func:MasterAlertSweep] baseline ", key, " down, skip notify")
				continue
			}
			startEvent := false
			if alertNow {
				masterSetStatus(key, false)
				st.BadSince = now
				st.AckedUntil = time.Time{}
				if h.Muted {
					st.MutedSkip = true
					seelog.Info("[func:MasterAlertSweep] ", key, " muted, notification skipped")
				} else {
					startEvent = true
				}
			} else if h.Muted {
				st.MutedSkip = true
			} else if st.MutedSkip && eventOf(key) == "" {
				// 屏蔽结束仍故障：这时才第一次通知，按新事件算，之后也会有对应的恢复
				st.MutedSkip = false
				startEvent = true
				seelog.Info("[func:MasterAlertSweep] ", key, " mute expired but still down")
			}
			if startEvent {
				seelog.Info("[func:MasterAlertSweep] alert ", key)
				l.AvgDelay, l.Loss, l.Jitter = h.AvgDelay, h.Loss, h.Jitter
				l.AlertType, l.Reason, l.Tag = classifyBreach(addr, target, r, breachFromHealth(h))
				delta.newBad = append(delta.newBad, pendingNotify{log: l, rule: r, kind: "alert"})
			}
		}
	}
	settleEvents(delta)
	seelog.Debug("[func:MasterAlertSweep] finish")
}
