package funcs

import "strings"

// wecomText 把告警排成企业微信 markdown。不用引用条，避免手机上挤成灰边。
// 故障为橙色，恢复为绿色，测试为灰色，方便在群里一眼分开。
func wecomText(m alertMsg) string {
	f := firstFields(m.Fields)
	lines := []string{
		"— ZENLENET PingMesh —",
		"级别：" + wecomLevel(m, f),
		"类型：" + wecomType(m),
		"",
		wecomHeadline(m, f),
		"",
		"▌ 详情",
	}
	lines = append(lines, wecomDetail(m, f)...)
	lines = append(lines, "", "▌ 影响评估")
	lines = append(lines, wecomImpact(m, f)...)
	lines = append(lines, "", "▌ 建议处置")
	lines = append(lines, wecomAction(m)...)
	lines = append(lines, "", "— 本消息由 ZENLENET PingMesh 自动发送 —")
	return paintWecom(wecomColor(m.Kind), lines, 4)
}

func wecomColor(kind string) string {
	switch kind {
	case "recovery":
		return "info"
	case "test":
		return "comment"
	default:
		return "warning"
	}
}

func paintWecom(color string, lines []string, head int) string {
	var b strings.Builder
	for i, ln := range lines {
		if ln == "" {
			b.WriteByte('\n')
			continue
		}
		if i == head {
			b.WriteString("## ")
		}
		b.WriteString(`<font color="` + color + `">` + ln + "</font>\n")
	}
	return strings.TrimRight(b.String(), "\n")
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
	case "recovery":
		return "恢复 (OK)"
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
	case "recovery":
		return "恢复通知"
	case "reminder":
		return "持续提醒"
	case "mute_expired":
		return "屏蔽到期"
	default:
		return "质量告警"
	}
}

func wecomHeadline(m alertMsg, f map[string]string) string {
	switch m.Kind {
	case "test":
		return "【通道测试】企业微信告警联通测试"
	case "recovery":
		return "【已恢复】" + m.Link
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
