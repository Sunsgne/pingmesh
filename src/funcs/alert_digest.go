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

// digestView 同一批告警的两端划分。
type digestView struct {
	Headline string
	Judgment string
	Lines    []string
}

// diagnoseLinks 把同一批异常链路划成两端。
// 能二分的，故障区间就是这两端；有奇数环的，不编造一对端点，改列节点对。
func diagnoseLinks(links []digestLink) digestView {
	labelOf := map[string]string{}
	adj := map[string]map[string]bool{}
	touch := func(ip, name string) string {
		k := ip
		if k == "" {
			k = "name:" + name
		} else {
			k = "ip:" + k
		}
		if name != "" {
			labelOf[k] = name
		} else if labelOf[k] == "" {
			labelOf[k] = ip
		}
		if adj[k] == nil {
			adj[k] = map[string]bool{}
		}
		return k
	}
	for _, l := range links {
		a := touch(l.FromIP, l.FromName)
		b := touch(l.ToIP, l.ToName)
		if a == b {
			continue
		}
		adj[a][b] = true
		adj[b][a] = true
	}
	seen := map[string]bool{}
	var keys []string
	for k := range labelOf {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return labelOf[keys[i]] < labelOf[keys[j]] })
	var comps [][]string
	for _, k := range keys {
		if seen[k] {
			continue
		}
		var comp []string
		stack := []string{k}
		seen[k] = true
		for len(stack) > 0 {
			u := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			comp = append(comp, u)
			var nxt []string
			for v := range adj[u] {
				nxt = append(nxt, v)
			}
			sort.Strings(nxt)
			for _, v := range nxt {
				if !seen[v] {
					seen[v] = true
					stack = append(stack, v)
				}
			}
		}
		comps = append(comps, comp)
	}
	var intervals, evidences []string
	for _, comp := range comps {
		left, right, ok := colorComponent(comp, adj)
		if !ok {
			intervals = append(intervals, "无法二分")
			evidences = append(evidences, oddCycleEvidence(comp, adj, labelOf))
			continue
		}
		left, right = orderSides(left, right, labelOf)
		intervals = append(intervals, sideText(left, labelOf)+" — "+sideText(right, labelOf))
		evidences = append(evidences, cutEvidence(links, left, right, labelOf))
	}
	return digestView{
		Headline: strings.Join(intervals, "；"),
		Judgment: strings.Join(evidences, " "),
		Lines:    digestLinkLines(links),
	}
}

func colorComponent(comp []string, adj map[string]map[string]bool) (left, right []string, ok bool) {
	col := map[string]int{}
	ok = true
	sorted := append([]string(nil), comp...)
	sort.Strings(sorted)
	var dfs func(u string, c int)
	dfs = func(u string, c int) {
		if prev, has := col[u]; has {
			if prev != c {
				ok = false
			}
			return
		}
		col[u] = c
		var nxt []string
		for v := range adj[u] {
			nxt = append(nxt, v)
		}
		sort.Strings(nxt)
		for _, v := range nxt {
			dfs(v, 1-c)
		}
	}
	for _, u := range sorted {
		if _, has := col[u]; !has {
			dfs(u, 0)
		}
	}
	if !ok {
		return nil, nil, false
	}
	for _, u := range sorted {
		if col[u] == 0 {
			left = append(left, u)
		} else {
			right = append(right, u)
		}
	}
	return left, right, true
}

func orderSides(left, right []string, labelOf map[string]string) ([]string, []string) {
	if len(right) < len(left) || (len(right) == len(left) && sideText(right, labelOf) < sideText(left, labelOf)) {
		return right, left
	}
	return left, right
}

func sideText(keys []string, labelOf map[string]string) string {
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, labelOf[k])
	}
	sort.Strings(names)
	if len(names) <= 4 {
		return strings.Join(names, "、")
	}
	return strings.Join(names[:4], "、") + " 等 " + strconv.Itoa(len(names)) + " 台"
}

func cutEvidence(links []digestLink, left, right []string, labelOf map[string]string) string {
	inLeft := map[string]bool{}
	for _, k := range left {
		inLeft[k] = true
	}
	keyOf := func(ip, name string) string {
		if ip != "" {
			return "ip:" + ip
		}
		return "name:" + name
	}
	l2r, r2l := 0, 0
	for _, l := range links {
		a, b := keyOf(l.FromIP, l.FromName), keyOf(l.ToIP, l.ToName)
		aLeft, bLeft := inLeft[a], inLeft[b]
		if aLeft == bLeft {
			continue
		}
		if aLeft {
			l2r++
		} else {
			r2l++
		}
	}
	return "该区间 " + strconv.Itoa(l2r+r2l) + " 条。两端为 " + sideText(left, labelOf) + "（" + strconv.Itoa(len(left)) + " 台）与 " + sideText(right, labelOf) + "（" + strconv.Itoa(len(right)) + " 台）。前一侧 → 后一侧 " + strconv.Itoa(l2r) + " 条，后一侧 → 前一侧 " + strconv.Itoa(r2l) + " 条。"
}

func oddCycleEvidence(comp []string, adj map[string]map[string]bool, labelOf map[string]string) string {
	var pairs []string
	seen := map[string]bool{}
	for _, u := range comp {
		for v := range adj[u] {
			a, b := labelOf[u], labelOf[v]
			if a > b {
				a, b = b, a
			}
			k := a + "—" + b
			if seen[k] {
				continue
			}
			seen[k] = true
			pairs = append(pairs, k)
		}
	}
	sort.Strings(pairs)
	if len(pairs) > 8 {
		pairs = append(pairs[:8], "等 "+strconv.Itoa(len(seen))+" 对")
	}
	return "异常链路含奇数环，不能划成唯一的两端。节点对：" + strings.Join(pairs, "、") + "。"
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
		line := digestEnd(cp[i].FromName, cp[i].FromIP) + " → " + digestEnd(cp[i].ToName, cp[i].ToIP)
		if d := strings.TrimSpace(cp[i].Detail); d != "" {
			line += "  " + d
		}
		out = append(out, line)
	}
	if len(cp) > maxLines {
		out = append(out, "其余 "+strconv.Itoa(len(cp)-maxLines)+" 条见告警记录")
	}
	return out
}

func digestEnd(name, ip string) string {
	if name != "" {
		return name
	}
	return ip
}

// buildDigest 把同一轮、同一类的多条通知收成一条，并写明两端和每条链路的触发值。
func buildDigest(kind string, items []pendingNotify) alertMsg {
	links := make([]digestLink, 0, len(items))
	for _, it := range items {
		links = append(links, digestLink{
			FromIP: it.log.Fromip, FromName: it.log.Fromname,
			ToIP: it.log.Targetip, ToName: it.log.Targetname,
			Detail: it.log.Reason,
		})
	}
	view := diagnoseLinks(links)
	msgKind, title := "digest", "🔴 故障区间"
	recovered := false
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
		{"故障区间", view.Headline},
		{"依据", view.Judgment},
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
