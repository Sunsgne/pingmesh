package funcs

import "strings"

// wecomText 把告警排成企业微信 markdown。不用引用条，避免手机上挤成灰边。
// 只有级别和标题上色：故障橙色，恢复绿色，测试灰色。正文保持黑色。
func wecomText(m alertMsg) string {
	f := firstFields(m.Fields)
	accent := wecomColor(m.Kind)
	var b strings.Builder
	write := func(color, text string, head bool) {
		if text == "" {
			b.WriteByte('\n')
			return
		}
		if head {
			b.WriteString("## ")
		}
		if color == "" {
			b.WriteString(text)
			b.WriteByte('\n')
			return
		}
		b.WriteString(`<font color="` + color + `">` + text + "</font>\n")
	}
	writePlain := func(lines []string) {
		for _, ln := range lines {
			write("", ln, false)
		}
	}
	write("comment", "— ZENLENET PingMesh —", false)
	write(accent, "级别："+wecomLevel(m, f), false)
	write("", "类型："+wecomType(m), false)
	write("", "", false)
	write(accent, wecomHeadline(m, f), true)
	write("", "", false)
	write("comment", "▌ 详情", false)
	writePlain(wecomDetail(m, f))
	if impact := wecomImpact(m, f); len(impact) > 0 {
		write("", "", false)
		write("comment", "▌ 影响评估", false)
		writePlain(impact)
	}
	if action := wecomAction(m); len(action) > 0 {
		write("", "", false)
		write("comment", "▌ 建议处置", false)
		writePlain(action)
	}
	write("", "", false)
	write("comment", "— 本消息由 ZENLENET PingMesh 自动发送 —", false)
	return strings.TrimRight(b.String(), "\n")
}

func wecomColor(kind string) string {
	switch kind {
	case "recovery", "digest_recovery":
		return "info"
	case "test":
		return "comment"
	default:
		return "warning"
	}
}

func firstFields(fields [][2]string) map[string]string {
	out := make(map[string]string, len(fields))
	for _, f := range fields {
		if _, ok := out[f[0]]; !ok {
			out[f[0]] = f[1]
		}
	}
	return out
}

func wecomLevel(m alertMsg, f map[string]string) string {
	switch m.Kind {
	case "test":
		return "通知 (INFO) · 系统自检"
	case "recovery", "digest_recovery":
		return "恢复 (OK)"
	case "digest":
		if strings.HasPrefix(m.Link, "节点 ") {
			return "重要 (MAJOR) · 节点"
		}
		return "重要 (MAJOR) · 点对"
	case "digest_reminder":
		return "重要 (MAJOR) · 仍未恢复"
	case "digest_mute":
		return "重要 (MAJOR) · 屏蔽已到期"
	case "reminder":
		return "重要 (MAJOR) · 仍未恢复"
	case "mute_expired":
		return "重要 (MAJOR) · 屏蔽已到期"
	default:
		t := f["告警类型"]
		if t == "" {
			t = "质量异常"
		}
		return "重要 (MAJOR) · " + t
	}
}

func wecomType(m alertMsg) string {
	switch m.Kind {
	case "test":
		return "测试通知"
	case "recovery", "digest_recovery":
		return "恢复通知"
	case "digest":
		if strings.HasPrefix(m.Link, "节点 ") {
			return "故障节点"
		}
		return "故障点对"
	case "digest_reminder":
		return "持续提醒"
	case "digest_mute":
		return "屏蔽到期"
	case "reminder":
		return "持续提醒"
	case "mute_expired":
		return "屏蔽到期"
	default:
		return "质量告警"
	}
}

func wecomHeadline(m alertMsg, f map[string]string) string {
	id := ""
	if v := f["事件"]; v != "" {
		id = " " + v
	}
	switch m.Kind {
	case "test":
		return "【通道测试】企业微信告警联通测试"
	case "digest_recovery":
		return "【已恢复】" + m.Link + id
	case "recovery":
		return "【已恢复】" + m.Link
	case "digest":
		return "【" + m.Link + "】" + id
	case "digest_reminder":
		return "【仍未恢复】" + m.Link + id
	case "digest_mute":
		return "【屏蔽到期】" + m.Link
	case "reminder":
		return "【仍未恢复】" + m.Link
	case "mute_expired":
		return "【屏蔽到期】" + m.Link
	default:
		t := f["告警类型"]
		if t == "" {
			t = "质量告警"
		}
		return "【" + t + "】" + m.Link
	}
}

func wecomDetail(m alertMsg, f map[string]string) []string {
	if strings.HasPrefix(m.Kind, "digest") {
		var lines []string
		for _, kv := range m.Fields {
			if kv[1] == "" {
				continue
			}
			parts := strings.Split(kv[1], "\n")
			if kv[0] == "实测" {
				for _, ln := range parts {
					if ln != "" {
						lines = append(lines, ln)
					}
				}
				continue
			}
			lines = append(lines, kv[0]+"："+parts[0])
			for _, ln := range parts[1:] {
				if ln != "" {
					lines = append(lines, ln)
				}
			}
		}
		return lines
	}
	if m.Kind == "test" {
		lines := []string{"收到此消息说明通道可用，", "生产告警可以正常送达。"}
		if v := f["节点"]; v != "" {
			lines = append(lines, "节点："+v)
		}
		if v := f["时间"]; v != "" {
			lines = append(lines, "时间："+v)
		}
		return lines
	}
	var lines []string
	if v := f["源节点"]; v != "" {
		lines = append(lines, "源："+v)
	}
	if v := f["目标"]; v != "" {
		lines = append(lines, "目标："+v)
	}
	if cause, window := splitReason(f["触发原因"]); cause != "" {
		lines = append(lines, "原因："+cause)
		if window != "" {
			lines = append(lines, "窗口："+window)
		}
	}
	if v := f["故障持续"]; v != "" {
		lines = append(lines, "持续："+v)
	}
	if v := f["近15分钟实测"]; v != "" {
		lines = append(lines, shortMetricLines(v)...)
	}
	if v := f["窗口均值"]; v != "" {
		lines = append(lines, "窗口均值："+v)
	}
	if v := f["触发规则"]; v != "" {
		lines = append(lines, ruleLines(v)...)
	}
	if v := f["时间"]; v != "" {
		lines = append(lines, "时间："+v)
	}
	return lines
}

func wecomImpact(m alertMsg, f map[string]string) []string {
	switch m.Kind {
	case "test":
		return []string{"这是一条测试通知，不代表真实故障。"}
	case "digest", "digest_recovery", "digest_reminder", "digest_mute":
		return nil
	case "recovery":
		if d := f["故障持续"]; d != "" {
			return []string{"链路已回到阈值以内，故障持续 " + d + "。"}
		}
		return []string{"这条链路已回到阈值以内。"}
	case "reminder":
		line := "故障尚未恢复。"
		if d := f["故障持续"]; d != "" {
			line = "故障尚未恢复，已持续 " + d + "。"
		}
		return []string{line, "这是重复提醒，不是新故障。"}
	case "mute_expired":
		return []string{"屏蔽已结束，目标仍异常。"}
	default:
		t := f["告警类型"]
		if t == "" || t == "质量异常" {
			return []string{"这条链路探测质量已超阈值。"}
		}
		return []string{"这条链路" + t + "已超阈值，探测质量下降。"}
	}
}

func wecomAction(m alertMsg) []string {
	switch m.Kind {
	case "test":
		return []string{"无需处理。误收请检查通道配置。"}
	case "digest", "digest_recovery", "digest_reminder", "digest_mute":
		return nil
	case "recovery":
		return []string{"无需处理。", "可抽查曲线确认已经稳住。"}
	case "reminder":
		return []string{"确认是否已有人处理，不必重复建单。"}
	case "mute_expired":
		return []string{"请重新确认该目标，屏蔽不会再挡住通知。"}
	default:
		return []string{"查看该链路曲线，确认是否持续。", "持续超标再查两端和中间网络。"}
	}
}

func splitReason(s string) (cause, window string) {
	if s == "" {
		return "", ""
	}
	const mark = "（窗口 "
	i := strings.Index(s, mark)
	if i < 0 {
		return s, ""
	}
	return s[:i], strings.TrimSuffix(s[i+len(mark):], "）")
}

func ruleLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, " 或 ")
	out := make([]string, 0, len(parts))
	out = append(out, "规则："+parts[0])
	for _, p := range parts[1:] {
		out = append(out, "或 "+p)
	}
	return out
}

func shortMetricLines(s string) []string {
	const p1, p2, p3 = "平均延迟 ", " ms / 丢包 ", " / 抖动 "
	rest, ok := strings.CutPrefix(s, p1)
	if !ok {
		return []string{"近15分钟：" + s}
	}
	avg, rest, ok := strings.Cut(rest, p2)
	if !ok {
		return []string{"近15分钟：" + s}
	}
	loss, jit, ok := strings.Cut(rest, p3)
	if !ok {
		return []string{"近15分钟：" + s}
	}
	jit = strings.TrimSuffix(jit, " ms")
	return []string{
		"近15分钟：延迟 " + avg + "ms",
		"丢包 " + loss + " · 抖动 " + jit + "ms",
	}
}
