package funcs

import (
	"sort"
	"strconv"
	"strings"

	"github.com/zenlenet/pingmesh/src/g"
)

// pendingNotify 一轮巡检里待发出的单条通知。凑够一批再决定是逐条发还是合成一条。
type pendingNotify struct {
	log    g.AlertLog
	rule   map[string]string
	kind   string
	extras [][2]string
}

// digestLink 参与归并判断的一条有向链路。
type digestLink struct {
	FromIP   string
	FromName string
	ToIP     string
	ToName   string
}

// digestView 同一批告警的结论：出在哪，以及要列出来的链路。
type digestView struct {
	Headline string
	Judgment string
	Lines    []string
}

// diagnoseLinks 判断这批同时出现的链路是不是同一处故障。
// 某一台出现在每一条上，就归到这台或它的接入；否则归到这几台之间的线路。
func diagnoseLinks(links []digestLink) digestView {
	n := len(links)
	type end struct {
		ip, name string
		in, out  int
	}
	nodes := map[string]*end{}
	keyOf := func(ip, name string) string {
		if ip != "" {
			return "ip:" + ip
		}
		return "name:" + name
	}
	get := func(ip, name string) *end {
		k := keyOf(ip, name)
		e := nodes[k]
		if e == nil {
			e = &end{ip: ip, name: name}
			nodes[k] = e
		}
		if name != "" {
			e.name = name
		}
		return e
	}
	for _, l := range links {
		get(l.FromIP, l.FromName).out++
		get(l.ToIP, l.ToName).in++
	}
	label := func(e *end) string {
		if e.name != "" {
			return e.name
		}
		return e.ip
	}
	var universal []*end
	for _, e := range nodes {
		if e.in+e.out == n {
			universal = append(universal, e)
		}
	}
	sort.Slice(universal, func(i, j int) bool { return label(universal[i]) < label(universal[j]) })
	lines := digestLinkLines(links)
	count := strconv.Itoa(n)

	switch len(universal) {
	case 1:
		e := universal[0]
		name := label(e)
		switch {
		case e.out > 0 && e.in > 0:
			return digestView{
				Headline: name + " 进出方向的 " + count + " 条链路",
				Judgment: "同一轮里这些链路一起越过阈值，进出都经过 " + name + "。优先查这台机器或它所在机房的接入，不必按每条链路分开处理。",
				Lines:    lines,
			}
		case e.out > 0:
			return digestView{
				Headline: "从 " + name + " 出去的 " + count + " 条链路",
				Judgment: "同一轮里这些链路一起越过阈值，都是从 " + name + " 发出的。优先查这台机器的出口或它接入的线路。",
				Lines:    lines,
			}
		default:
			return digestView{
				Headline: "打到 " + name + " 的 " + count + " 条链路",
				Judgment: "同一轮里这些链路一起越过阈值，都是打到 " + name + " 的。优先查这台机器或它的入口线路。",
				Lines:    lines,
			}
		}
	case 2:
		a, b := label(universal[0]), label(universal[1])
		return digestView{
			Headline: a + " 与 " + b + " 之间的 " + count + " 条链路",
			Judgment: "全部发生在这两台之间。更像这一段线路，不像整台机器故障。",
			Lines:    lines,
		}
	default:
		var names []string
		for _, e := range nodes {
			if e.in+e.out >= 2 {
				names = append(names, label(e))
			}
		}
		sort.Strings(names)
		if len(names) >= 2 {
			total := len(names)
			shown := names
			tail := ""
			if total > 4 {
				shown = names[:4]
				tail = " 等 " + strconv.Itoa(total) + " 台"
			}
			return digestView{
				Headline: strings.Join(shown, "、") + tail + " 之间的 " + count + " 条链路",
				Judgment: "没有单独一台机器出现在每一条链路上。更像这一段网络一起变差，不是某一台自己坏了。",
				Lines:    lines,
			}
		}
		return digestView{
			Headline: count + " 条互不相关的链路",
			Judgment: "这几条没有共同的机器，可能只是同时发生。告警记录里可以逐条看。",
			Lines:    lines,
		}
	}
}

func digestLinkLines(links []digestLink) []string {
	cp := append([]digestLink(nil), links...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].FromName != cp[j].FromName {
			return cp[i].FromName < cp[j].FromName
		}
		if cp[i].ToName != cp[j].ToName {
			return cp[i].ToName < cp[j].ToName
		}
		return cp[i].FromIP < cp[j].FromIP
	})
	const maxLines = 8
	limit := len(cp)
	if limit > maxLines {
		limit = maxLines
	}
	out := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		out = append(out, digestEnd(cp[i].FromName, cp[i].FromIP)+" → "+digestEnd(cp[i].ToName, cp[i].ToIP))
	}
	if len(cp) > maxLines {
		out = append(out, "还有 "+strconv.Itoa(len(cp)-maxLines)+" 条，告警记录里可以逐条看")
	}
	return out
}

func digestEnd(name, ip string) string {
	if name != "" {
		return name
	}
	return ip
}

// buildDigest 把同一轮、同一类的多条通知收成一条。
func buildDigest(kind string, items []pendingNotify) alertMsg {
	links := make([]digestLink, 0, len(items))
	for _, it := range items {
		links = append(links, digestLink{
			FromIP: it.log.Fromip, FromName: it.log.Fromname,
			ToIP: it.log.Targetip, ToName: it.log.Targetname,
		})
	}
	view := diagnoseLinks(links)
	msgKind, title := "digest", "🔴 同一故障"
	recovered := false
	switch kind {
	case "recovery":
		msgKind, title, recovered = "digest_recovery", "✅ 这批已恢复", true
	case "reminder":
		msgKind, title = "digest_reminder", "⏰ 这批仍未恢复"
	case "mute_expired":
		msgKind, title = "digest_mute", "🔔 这批屏蔽已到期"
	}
	when := ""
	if len(items) > 0 {
		when = items[0].log.Logtime
	}
	fields := [][2]string{
		{"判断", view.Judgment},
		{"链路", strings.Join(view.Lines, "\n")},
		{"时间", when},
	}
	var b strings.Builder
	b.WriteString(title + " " + view.Headline + "\n")
	for _, f := range fields {
		b.WriteString(f[0] + ": " + f[1] + "\n")
	}
	return alertMsg{
		Kind: msgKind, Recovered: recovered, Title: title, Link: view.Headline,
		Fields: fields, Plain: strings.TrimRight(b.String(), "\n"),
	}
}

// sendPending 一轮结束时发送。只有一条时保持原来的单链路卡片，两条及以上收成一条结论。
func sendPending(items []pendingNotify) {
	if len(items) == 0 {
		return
	}
	groups := map[string][]pendingNotify{}
	order := []string{}
	for _, it := range items {
		if _, ok := groups[it.kind]; !ok {
			order = append(order, it.kind)
		}
		groups[it.kind] = append(groups[it.kind], it)
	}
	for _, kind := range order {
		batch := groups[kind]
		if len(batch) == 1 {
			it := batch[0]
			NotifyAll(it.log, it.rule, it.kind, it.extras)
			continue
		}
		NotifyBuilt(buildDigest(kind, batch))
	}
}
