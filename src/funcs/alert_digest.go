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
	return localizeFocus(cells, nil)
}

// localizeFocus 只对 focus 里的异常格定位（同一事件的链路），对照时仍按整张矩阵的真实状态计数。
// focus 为 nil 时取全部异常格。
func localizeFocus(cells []meshCell, focus map[string]bool) digestView {
	inFocus := func(c meshCell) bool {
		return !c.OK && (focus == nil || focus[c.FromIP+"|"+c.ToIP])
	}
	type ns struct {
		label               string
		outBad, outOK       int
		inBad, inOK         int
		outBadAll, inBadAll int
		peers               map[string]bool
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
		a.outBadAll++
		b.inBadAll++
		if !inFocus(c) {
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
		lines := badLinesTouching(cells, best.key, inFocus)
		return digestView{
			Headline: "节点 " + n.label,
			Judgment: "行 " + n.label + " " + countText(n.outOK, n.outBadAll) + "；列 " + n.label + " " + countText(n.inOK, n.inBadAll) + "。",
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
		if !inFocus(c) {
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
			judges = append(judges, rowColText(cells, p.a, p.b, p.aName)+"；"+rowColText(cells, p.b, p.a, p.bName))
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

func rowColText(cells []meshCell, self, except, label string) string {
	rowOK, rowBad, colOK, colBad := 0, 0, 0, 0
	for _, c := range cells {
		a, b := endKey(c.FromIP, c.FromName), endKey(c.ToIP, c.ToName)
		if a == self && b != except {
			if c.OK {
				rowOK++
			} else {
				rowBad++
			}
		}
		if b == self && a != except {
			if c.OK {
				colOK++
			} else {
				colBad++
			}
		}
	}
	return "行 " + label + " " + countText(rowOK, rowBad) + "；列 " + label + " " + countText(colOK, colBad)
}

func countText(ok, bad int) string {
	return strconv.Itoa(ok) + " 正常 / " + strconv.Itoa(bad) + " 异常"
}

func restBad(cells []meshCell, self, other string) int {
	bad, _ := restCount(cells, self, other)
	return bad
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

func badLinesTouching(cells []meshCell, key string, inFocus func(meshCell) bool) []string {
	var lines []string
	for _, c := range cells {
		if !inFocus(c) {
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
