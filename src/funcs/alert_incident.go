package funcs

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// alertEvent 一次告警通知对应的事件。告警和恢复按事件一一对应：
// 发过告警的事件，等它包含的链路全部恢复后只发一条恢复；没发过告警的链路恢复时不发通知。
type alertEvent struct {
	ID         string            `json:"id"`
	Opened     time.Time         `json:"opened"`
	Headline   string            `json:"headline"`
	Members    map[string]string `json:"members"` // key=from|to, value=链路名
	Resolved   map[string]bool   `json:"resolved"`
	LastNotify time.Time         `json:"lastnotify"`
	AckedUntil time.Time         `json:"ackeduntil"`
}

var (
	eventMu     sync.Mutex
	openEvents  = map[string]*alertEvent{}
	eventOfLink = map[string]string{}
	eventsReady bool
)

const eventDDL = `CREATE TABLE IF NOT EXISTS alert_event (
	id      VARCHAR (32) PRIMARY KEY,
	opened  VARCHAR (19),
	closed  VARCHAR (19) DEFAULT '',
	state   TEXT
)`

// loadOpenEvents 主节点重启后接上未恢复的事件，避免这些事件只有告警没有恢复。
func loadOpenEvents() {
	eventMu.Lock()
	defer eventMu.Unlock()
	if eventsReady || g.Db == nil {
		return
	}
	eventsReady = true
	if _, err := g.Db.Exec(eventDDL); err != nil {
		seelog.Error("[func:loadOpenEvents] ", err)
		return
	}
	rows, err := g.Db.Query(`SELECT state FROM alert_event WHERE closed = ''`)
	if err != nil {
		seelog.Error("[func:loadOpenEvents] ", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if rows.Scan(&s) != nil {
			continue
		}
		ev := &alertEvent{}
		if json.Unmarshal([]byte(s), ev) != nil || ev.ID == "" {
			continue
		}
		if ev.Resolved == nil {
			ev.Resolved = map[string]bool{}
		}
		openEvents[ev.ID] = ev
		for k := range ev.Members {
			if ev.Resolved[k] {
				continue
			}
			eventOfLink[k] = ev.ID
			masterSetStatus(k, false)
			if st := incidentOf(k); st.BadSince.IsZero() {
				st.BadSince = ev.Opened
			}
		}
		seelog.Info("[func:loadOpenEvents] resumed event ", ev.ID, " ", ev.Headline)
	}
}

func saveEvent(ev *alertEvent, closed bool) {
	if g.Db == nil {
		return
	}
	b, _ := json.Marshal(ev)
	c := ""
	if closed {
		c = time.Now().Format("2006-01-02 15:04:05")
	}
	if _, err := g.Db.Exec(`INSERT INTO alert_event(id, opened, closed, state) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET closed = excluded.closed, state = excluded.state`,
		ev.ID, ev.Opened.Format("2006-01-02 15:04:05"), c, string(b)); err != nil {
		seelog.Error("[func:saveEvent] ", err)
	}
}

func newEventID(now time.Time) string {
	id := now.Format("0102-150405")
	if _, dup := openEvents[id]; !dup {
		return id
	}
	for i := 2; ; i++ {
		alt := id + "-" + strconv.Itoa(i)
		if _, dup := openEvents[alt]; !dup {
			return alt
		}
	}
}

// eventOf 返回链路当前所属的未恢复事件。
func eventOf(key string) string {
	eventMu.Lock()
	defer eventMu.Unlock()
	return eventOfLink[key]
}

// ackEventsForTarget 确认告警后，含这个目标的事件 24 小时内不再提醒。
func ackEventsForTarget(target string) {
	eventMu.Lock()
	defer eventMu.Unlock()
	for _, ev := range openEvents {
		for k := range ev.Members {
			if strings.HasSuffix(k, "|"+target) {
				ev.AckedUntil = time.Now().Add(24 * time.Hour)
				saveEvent(ev, false)
				break
			}
		}
	}
}

// sweepDelta 一轮巡检里状态真正变化的链路。
type sweepDelta struct {
	newBad    []pendingNotify // 本轮新故障且未屏蔽，要开新事件
	recovered []string        // 本轮确认恢复的链路
	present   map[string]bool // 配置里仍存在的链路；探测源暂时不可达的链路也算在内，不会被当成已恢复
	reachable map[string]bool // 本轮能拿到快照的探测源
	cells     []meshCell      // 整张矩阵，用于定位
	now       time.Time
	remind    time.Duration
}

// settleEvents 先结算恢复，再开新事件，最后发提醒。每个事件只发一次告警、一次恢复。
func settleEvents(d sweepDelta) {
	eventMu.Lock()
	var toSend []alertMsg
	for _, k := range d.recovered {
		if id := eventOfLink[k]; id != "" {
			if ev := openEvents[id]; ev != nil {
				ev.Resolved[k] = true
			}
			delete(eventOfLink, k)
		}
	}
	for id, ev := range openEvents {
		for k := range ev.Members {
			if ev.Resolved[k] {
				continue
			}
			if !d.present[k] {
				ev.Resolved[k] = true
				delete(eventOfLink, k)
			}
		}
		if len(ev.Resolved) < len(ev.Members) {
			continue
		}
		toSend = append(toSend, eventRecoveryMsg(ev, d.now))
		saveEvent(ev, true)
		delete(openEvents, id)
		seelog.Info("[func:settleEvents] event ", id, " recovered")
	}
	if len(d.newBad) > 0 {
		ev := &alertEvent{
			ID: newEventID(d.now), Opened: d.now, LastNotify: d.now,
			Members: map[string]string{}, Resolved: map[string]bool{},
		}
		focus := map[string]bool{}
		for _, it := range d.newBad {
			k := masterIncidentKey(it.log.Fromip, it.log.Targetip)
			ev.Members[k] = endLabel(it.log.Fromip, it.log.Fromname) + " → " + endLabel(it.log.Targetip, it.log.Targetname)
			focus[k] = true
			eventOfLink[k] = ev.ID
		}
		view := localizeFocus(d.cells, focus)
		ev.Headline = view.Headline
		openEvents[ev.ID] = ev
		saveEvent(ev, false)
		toSend = append(toSend, eventAlertMsg(ev, view, d.now))
		seelog.Info("[func:settleEvents] event ", ev.ID, " opened: ", ev.Headline)
	}
	if d.remind > 0 {
		ids := make([]string, 0, len(openEvents))
		for id := range openEvents {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			ev := openEvents[id]
			if d.now.Before(ev.AckedUntil) || d.now.Sub(ev.LastNotify) < d.remind {
				continue
			}
			ev.LastNotify = d.now
			saveEvent(ev, false)
			toSend = append(toSend, eventReminderMsg(ev, d.now))
		}
	}
	eventMu.Unlock()
	for _, m := range toSend {
		sendEventMsg(m)
	}
}

var sendEventMsg = NotifyBuilt

func eventMembersText(ev *alertEvent, onlyOpen bool) []string {
	var out []string
	for k, name := range ev.Members {
		if onlyOpen && ev.Resolved[k] {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func eventAlertMsg(ev *alertEvent, view digestView, now time.Time) alertMsg {
	title := "🔴 故障点对"
	if strings.HasPrefix(view.Headline, "节点 ") {
		title = "🔴 故障节点"
	}
	fields := [][2]string{
		{"事件", "#" + ev.ID},
		{"故障点对", view.Headline},
		{"对照", view.Judgment},
		{"实测", strings.Join(view.Lines, "\n")},
		{"链路数", strconv.Itoa(len(ev.Members))},
		{"开始", now.Format("2006-01-02 15:04:05")},
	}
	return eventMsg("digest", title, view.Headline, fields, false)
}

func eventRecoveryMsg(ev *alertEvent, now time.Time) alertMsg {
	fields := [][2]string{
		{"事件", "#" + ev.ID},
		{"故障点对", ev.Headline},
		{"恢复链路", strconv.Itoa(len(ev.Resolved)) + " / " + strconv.Itoa(len(ev.Members))},
		{"持续", fmtDur(now.Sub(ev.Opened))},
		{"开始", ev.Opened.Format("2006-01-02 15:04:05")},
		{"恢复", now.Format("2006-01-02 15:04:05")},
	}
	return eventMsg("digest_recovery", "✅ 已恢复", ev.Headline, fields, true)
}

func eventReminderMsg(ev *alertEvent, now time.Time) alertMsg {
	open := eventMembersText(ev, true)
	fields := [][2]string{
		{"事件", "#" + ev.ID},
		{"故障点对", ev.Headline},
		{"未恢复链路", strconv.Itoa(len(open)) + " / " + strconv.Itoa(len(ev.Members)) + "\n" + strings.Join(open, "\n")},
		{"持续", fmtDur(now.Sub(ev.Opened))},
		{"开始", ev.Opened.Format("2006-01-02 15:04:05")},
	}
	return eventMsg("digest_reminder", "⏰ 仍未恢复", ev.Headline, fields, false)
}

func eventMsg(kind, title, link string, fields [][2]string, recovered bool) alertMsg {
	var b strings.Builder
	b.WriteString(title + " " + link + "\n")
	for _, f := range fields {
		b.WriteString(f[0] + ": " + f[1] + "\n")
	}
	return alertMsg{
		Kind: kind, Recovered: recovered, Title: title, Link: link,
		Fields: fields, Plain: strings.TrimRight(b.String(), "\n"),
	}
}
