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
	Detail   string
}

// meshCell 矩阵里的一条有向探测。OK 为 false 表示这条当前超阈值。
type meshCell struct {
	FromIP, FromName, ToIP, ToName string
	OK                             bool
	Delay, Loss                    float64
	Detail                         string
}

// digestView 用整张矩阵对照后得到的故障点。
type digestView struct {
	Headline string
	Judgment string
	Lines    []string
}

func endKey(ip, name string) string {
	if ip != "" {
		return "ip:" + ip
	}
	return "name:" + name
}

func endLabel(ip, name string) string {
	if name != "" {
		return name
	}
	return ip
}

// localizeMatrix 用全矩阵把故障收到两个点，或收到一个节点。
// 某个点到其余方向都正常、只有和对端异常，故障就在这两个点之间。
// 某个点到多数对端都异常，故障就在这个点，而不是随便划两侧。
func localizeMatrix(cells []meshCell) digestView {
	type ns struct {
		label         string
		outBad, outOK int
		inBad, inOK   int
		peers         map[string]bool
	}
	nodes := map[string]*ns{}
	ensure := func(ip, name string) *ns {
		k := endKey(ip, name)
		n := nodes[k]
		if n == nil {
			n = &ns{label: endLabel(ip, name), peers: map[string]bool{}}
			nodes[k] = n
		}
		if name != "" {
			n.label = name
		}
		return n
	}
	badN := 0
	for _, c := range cells {
		a, b := ensure(c.FromIP, c.FromName), ensure(c.ToIP, c.ToName)
		bk := endKey(c.ToIP, c.ToName)
		ak := endKey(c.FromIP, c.FromName)
		if c.OK {
			a.outOK++
			b.inOK++
			continue
		}
		badN++
		a.outBad++
		b.inBad++
		a.peers[bk] = true
		b.peers[ak] = true
	}
	if badN == 0 {
		return digestView{Headline: "无异常点对", Judgment: "矩阵内没有超阈值的方向。"}
	}
	type cand struct {
		key   string
		score int
	}
	var best *cand
	for k, n := range nodes {
		incident := n.outBad + n.inBad
		if len(n.peers) < 3 || incident < 3 || incident*5 < badN*4 {
			continue
		}
		if best == nil || incident > best.score || (incident == best.score && n.label < nodes[best.key].label) {
			best = &cand{key: k, score: incident}
		}
	}
	if best != nil {
		n := nodes[best.key]
		lines := badLinesTouching(cells, best.key)
		return digestView{
			Headline: "节点 " + n.label,
			Judgment: n.label + " 发出 " + strconv.Itoa(n.outBad) + " 异常 / " + strconv.Itoa(n.outOK) + " 正常；收到 " + strconv.Itoa(n.inBad) + " 异常 / " + strconv.Itoa(n.inOK) + " 正常。",
			Lines:    lines,
		}
	}
	type pair struct {
		a, b         string
		aName, bName string
		abBad, baBad bool
	}
	seen := map[string]*pair{}
	var order []string
	for _, c := range cells {
		if c.OK {
			continue
		}
		a, b := endKey(c.FromIP, c.FromName), endKey(c.ToIP, c.ToName)
		lo, hi := a, b
		loName, hiName := endLabel(c.FromIP, c.FromName), endLabel(c.ToIP, c.ToName)
		fwd := true
		if hi < lo {
			lo, hi = hi, lo
			loName, hiName = hiName, loName
			fwd = false
		}
		k := lo + "|" + hi
		p := seen[k]
		if p == nil {
			p = &pair{a: lo, b: hi, aName: loName, bName: hiName}
			seen[k] = p
			order = append(order, k)
		}
		if fwd {
			p.abBad = true
		} else {
			p.baBad = true
		}
	}
	sort.Strings(order)
	var isolated, all []*pair
	for _, k := range order {
		p := seen[k]
		all = append(all, p)
		if restBad(cells, p.a, p.b) == 0 && restBad(cells, p.b, p.a) == 0 {
			isolated = append(isolated, p)
		}
	}
	use := all
	isolatedOnly := false
	if len(isolated) > 0 {
		use = isolated
		isolatedOnly = true
	}
	var heads, judges, lines []string
	for _, p := range use {
		if p.abBad && p.baBad {
			heads = append(heads, p.aName+" ↔ "+p.bName)
		} else if p.abBad {
			heads = append(heads, p.aName+" → "+p.bName)
		} else {
			heads = append(heads, p.bName+" → "+p.aName)
		}
		if isolatedOnly {
			judges = append(judges, p.aName+" 其余 "+restText(cells, p.a, p.b)+"；"+p.bName+" 其余 "+restText(cells, p.b, p.a))
		}
		lines = append(lines, pairDirectionLines(cells, p.a, p.b)...)
	}
	judgment := strings.Join(judges, "。")
	if !isolatedOnly {
		judgment = "这些点对同时超阈值，两端到其他节点也有异常，不能收成一个节点。"
	}
	if judgment != "" && !strings.HasSuffix(judgment, "。") {
		judgment += "。"
	}
	return digestView{Headline: strings.Join(heads, "；"), Judgment: judgment, Lines: lines}
}

func restBad(cells []meshCell, self, other string) int {
	bad, _ := restCount(cells, self, other)
	return bad
}

func restText(cells []meshCell, self, other string) string {
	bad, ok := restCount(cells, self, other)
	return strconv.Itoa(ok) + " 正常 / " + strconv.Itoa(bad) + " 异常"
}

func restCount(cells []meshCell, self, other string) (bad, ok int) {
	for _, c := range cells {
		a, b := endKey(c.FromIP, c.FromName), endKey(c.ToIP, c.ToName)
		if a != self && b != self {
			continue
		}
		otherEnd := b
		if b == self {
			otherEnd = a
		}
		if otherEnd == other {
			continue
		}
		if c.OK {
			ok++
		} else {
			bad++
		}
	}
	return bad, ok
}

func pairDirectionLines(cells []meshCell, a, b string) []string {
	var lines []string
	for _, c := range cells {
		ak, bk := endKey(c.FromIP, c.FromName), endKey(c.ToIP, c.ToName)
		if !((ak == a && bk == b) || (ak == b && bk == a)) {
			continue
		}
		lines = append(lines, cellLine(c))
	}
	sort.Strings(lines)
	return lines
}

func badLinesTouching(cells []meshCell, key string) []string {
	var lines []string
	for _, c := range cells {
		if c.OK {
			continue
		}
		if endKey(c.FromIP, c.FromName) != key && endKey(c.ToIP, c.ToName) != key {
			continue
		}
		lines = append(lines, cellLine(c))
	}
	sort.Strings(lines)
	const maxLines = 8
	if len(lines) > maxLines {
		extra := len(lines) - maxLines
		lines = append(lines[:maxLines], "其余 "+strconv.Itoa(extra)+" 条见告警记录")
	}
	return lines
}

func cellLine(c meshCell) string {
	line := endLabel(c.FromIP, c.FromName) + " → " + endLabel(c.ToIP, c.ToName)
	if c.OK {
		return line + "  正常  延迟 " + formatMetric(c.Delay) + "ms  丢包 " + formatMetric(c.Loss) + "%"
	}
	if strings.TrimSpace(c.Detail) != "" {
		return line + "  " + c.Detail
	}
	return line + "  异常  延迟 " + formatMetric(c.Delay) + "ms  丢包 " + formatMetric(c.Loss) + "%"
}

// localizeLinks 没有全矩阵时，只根据这批异常链路本身找点对，供测试和兜底。
func localizeLinks(links []digestLink) digestView {
	cells := make([]meshCell, 0, len(links))
	for _, l := range links {
		cells = append(cells, meshCell{
			FromIP: l.FromIP, FromName: l.FromName, ToIP: l.ToIP, ToName: l.ToName,
			Detail: l.Detail,
		})
	}
	return localizeMatrix(cells)
}

// buildDigest 用矩阵把同一轮通知收成「哪两个点」或「哪一个节点」。
func buildDigest(kind string, items []pendingNotify, cells []meshCell) alertMsg {
	view := localizeMatrix(cells)
	if kind == "recovery" || len(cells) == 0 {
		links := make([]digestLink, 0, len(items))
		for _, it := range items {
			links = append(links, digestLink{
				FromIP: it.log.Fromip, FromName: it.log.Fromname,
				ToIP: it.log.Targetip, ToName: it.log.Targetname,
				Detail: it.log.Reason,
			})
		}
		view = localizeLinks(links)
	}
	msgKind, title := "digest", "🔴 故障点对"
	recovered := false
	if strings.HasPrefix(view.Headline, "节点 ") {
		title = "🔴 故障节点"
	}
	switch kind {
	case "recovery":
		msgKind, title, recovered = "digest_recovery", "✅ 已恢复", true
	case "reminder":
		msgKind, title = "digest_reminder", "⏰ 仍未恢复"
	case "mute_expired":
		msgKind, title = "digest_mute", "🔔 屏蔽到期"
	}
	when := ""
	if len(items) > 0 {
		when = items[0].log.Logtime
	}
	fields := [][2]string{
		{"故障点对", view.Headline},
		{"对照", view.Judgment},
		{"实测", strings.Join(view.Lines, "\n")},
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

// sendPending 一轮结束时发送。告警用整张矩阵定位到两个点或一个节点。
func sendPending(items []pendingNotify, cells []meshCell) {
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
		if kind != "alert" && kind != "reminder" && len(batch) == 1 {
			it := batch[0]
			NotifyAll(it.log, it.rule, it.kind, it.extras)
			continue
		}
		NotifyBuilt(buildDigest(kind, batch, cells))
	}
}
