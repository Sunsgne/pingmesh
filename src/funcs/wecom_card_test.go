package funcs

import (
	"strings"
	"testing"
)

func TestWecomCardDelay(t *testing.T) {
	got := wecomText(alertMsg{
		Kind: "alert",
		Link: "TYO-EQTY7 → PVG-GDS",
		Fields: [][2]string{
			{"告警类型", "延迟"},
			{"故障 Tag", "DELAY:10.100.1.9→10.100.1.20"},
			{"触发原因", "延迟 41.6ms ≥ 33ms（窗口 60s / 1 次）"},
			{"近15分钟实测", "平均延迟 41.6 ms / 丢包 0% / 抖动 0.4 ms"},
			{"链路", "TYO-EQTY7 → PVG-GDS"},
			{"源节点", "TYO-EQTY7 (10.100.1.9)"},
			{"目标", "PVG-GDS (10.100.1.20)"},
			{"触发规则", "延迟 > 33ms 或 丢包 > 1%"},
			{"时间", "2026-11-20 15:56"},
			{"平台", "ZENLENET PingMesh"},
		},
	})
	want := strings.TrimSpace(`
<font color="comment">— ZENLENET PingMesh —</font>
<font color="warning">级别：重要 (MAJOR) · 延迟</font>
类型：质量告警

## <font color="warning">【延迟】TYO-EQTY7 → PVG-GDS</font>

<font color="comment">▌ 详情</font>
源：TYO-EQTY7 (10.100.1.9)
目标：PVG-GDS (10.100.1.20)
原因：延迟 41.6ms ≥ 33ms
窗口：60s / 1 次
近15分钟：延迟 41.6ms
丢包 0% · 抖动 0.4ms
规则：延迟 > 33ms
或 丢包 > 1%
时间：2026-11-20 15:56

<font color="comment">▌ 影响评估</font>
这条链路延迟已超阈值，探测质量下降。

<font color="comment">▌ 建议处置</font>
查看该链路曲线，确认是否持续。
持续超标再查两端和中间网络。

<font color="comment">— 本消息由 ZENLENET PingMesh 自动发送 —</font>`)
	if got != want {
		t.Fatalf("delay card:\n%s", got)
	}
	if strings.Count(got, `color="warning"`) != 2 {
		t.Fatal("only the level and the title should be orange")
	}
	if strings.Contains(got, "故障 Tag") || strings.Contains(got, "DELAY:") {
		t.Fatal("tag should stay off the WeCom card")
	}
	if strings.Contains(got, "\n>") {
		t.Fatal("quote bar would collapse the card on a phone")
	}
}

func TestWecomCardRecoveryReminderTest(t *testing.T) {
	recovery := wecomText(alertMsg{
		Kind: "recovery",
		Link: "PEK-BJHK → PVG-SJHL",
		Fields: [][2]string{
			{"故障持续", "18 分钟"},
			{"近15分钟实测", "平均延迟 16.2 ms / 丢包 0% / 抖动 0.3 ms"},
			{"源节点", "PEK-BJHK (10.100.1.15)"},
			{"目标", "PVG-SJHL (10.100.1.5)"},
			{"时间", "2026-11-20 15:56"},
		},
	})
	for _, part := range []string{
		"级别：恢复 (OK)",
		"类型：恢复通知",
		"【已恢复】PEK-BJHK → PVG-SJHL",
		"持续：18 分钟",
		"近15分钟：延迟 16.2ms",
		"丢包 0% · 抖动 0.3ms",
		"链路已回到阈值以内，故障持续 18 分钟。",
		"无需处理。",
		"可抽查曲线确认已经稳住。",
	} {
		if !strings.Contains(recovery, part) {
			t.Fatalf("recovery missing %q\n%s", part, recovery)
		}
	}
	if strings.Contains(recovery, "规则：") || strings.Contains(recovery, "原因：") {
		t.Fatal("recovery should not repeat the trigger rule")
	}
	if !strings.Contains(recovery, `<font color="info">`) || strings.Contains(recovery, `color="warning"`) {
		t.Fatal("recovery should be green, not the fault color")
	}

	reminder := wecomText(alertMsg{
		Kind: "reminder",
		Link: "SIN-EQSG2 → TYO-EQTY8",
		Fields: [][2]string{
			{"故障持续", "2 小时 15 分钟"},
			{"源节点", "SIN-EQSG2 (10.100.1.8)"},
			{"目标", "TYO-EQTY8 (10.100.1.2)"},
			{"近15分钟实测", "平均延迟 46.8 ms / 丢包 2% / 抖动 1.4 ms"},
			{"触发规则", "延迟 > 96ms 或 丢包 > 1%"},
			{"时间", "2026-11-20 16:00"},
		},
	})
	for _, part := range []string{
		"级别：重要 (MAJOR) · 仍未恢复",
		"类型：持续提醒",
		"【仍未恢复】SIN-EQSG2 → TYO-EQTY8",
		"这是重复提醒，不是新故障。",
		"确认是否已有人处理，不必重复建单。",
		"规则：延迟 > 96ms",
		"或 丢包 > 1%",
	} {
		if !strings.Contains(reminder, part) {
			t.Fatalf("reminder missing %q\n%s", part, reminder)
		}
	}

	testMsg := wecomText(alertMsg{
		Kind: "test",
		Link: "SIN-EQSG2",
		Fields: [][2]string{
			{"节点", "SIN-EQSG2 (10.100.1.8)"},
			{"时间", "2026-11-20 15:49:01"},
			{"说明", "如收到此消息说明通道配置正确"},
			{"平台", "ZENLENET PingMesh"},
		},
	})
	for _, part := range []string{
		"级别：通知 (INFO) · 系统自检",
		"类型：测试通知",
		"【通道测试】企业微信告警联通测试",
		"收到此消息说明通道可用，",
		"生产告警可以正常送达。",
		"节点：SIN-EQSG2 (10.100.1.8)",
		"这是一条测试通知，不代表真实故障。",
		"无需处理。误收请检查通道配置。",
	} {
		if !strings.Contains(testMsg, part) {
			t.Fatalf("test card missing %q\n%s", part, testMsg)
		}
	}
	if strings.Contains(testMsg, "如收到此消息说明通道配置正确") || strings.Contains(testMsg, "平台：") {
		t.Fatal("test card should replace the old filler lines")
	}
	if !strings.Contains(testMsg, `<font color="comment">`) {
		t.Fatal("test card should be gray so it is not mistaken for a recovery")
	}
}

func TestWecomMentionAllOnImportantOnly(t *testing.T) {
	for _, kind := range []string{"alert", "digest", "reminder", "digest_reminder", "mute_expired", "digest_mute"} {
		msgs := wecomMessages(alertMsg{
			Kind:   kind,
			Link:   "HKG-EQHK2 ↔ PEK-BJHK",
			Fields: [][2]string{{"事件", "#1124-110000"}},
		})
		if len(msgs) != 2 || msgs[0]["msgtype"] != "markdown" || msgs[1]["msgtype"] != "text" {
			t.Fatalf("%s should send the card and then @all, got %#v", kind, msgs)
		}
		text := msgs[1]["text"].(map[string]interface{})
		mentioned := text["mentioned_list"].([]string)
		if len(mentioned) != 1 || mentioned[0] != "@all" {
			t.Fatalf("%s mention = %v", kind, mentioned)
		}
		content := text["content"].(string)
		if !strings.Contains(content, "HKG-EQHK2 ↔ PEK-BJHK") {
			t.Fatalf("%s mention text %q", kind, content)
		}
		if (kind == "digest" || kind == "digest_reminder") && !strings.Contains(content, "#1124-110000") {
			t.Fatalf("%s mention should carry the event id, got %q", kind, content)
		}
	}
	for _, kind := range []string{"recovery", "digest_recovery", "test"} {
		msgs := wecomMessages(alertMsg{Kind: kind, Link: "HKG-EQHK2 ↔ PEK-BJHK"})
		if len(msgs) != 1 || msgs[0]["msgtype"] != "markdown" {
			t.Fatalf("%s should stay a single card without @all", kind)
		}
	}
}

func TestWecomCardEdges(t *testing.T) {
	muted := wecomText(alertMsg{
		Kind: "mute_expired",
		Link: "HKG-EQHK2 → LAX-CORESITE",
		Fields: [][2]string{
			{"故障持续", "6 小时 0 分钟"},
			{"源节点", "HKG-EQHK2 (10.100.1.12)"},
			{"目标", "LAX-CORESITE (10.100.1.10)"},
		},
	})
	if !strings.Contains(muted, "屏蔽已结束，目标仍异常。") || !strings.Contains(muted, "【屏蔽到期】HKG-EQHK2 → LAX-CORESITE") {
		t.Fatalf("mute card:\n%s", muted)
	}

	dup := wecomText(alertMsg{
		Kind: "alert",
		Link: "A → B",
		Fields: [][2]string{
			{"告警类型", "丢包"},
			{"近15分钟实测", "平均延迟 18.6 ms / 丢包 4% / 抖动 1.1 ms"},
			{"近15分钟实测", "平均延迟 99.0 ms / 丢包 9% / 抖动 9.0 ms"},
			{"触发规则", "延迟 > 50ms 或 丢包 > 1% 或 抖动 > 20ms"},
			{"触发原因", "丢包 4.2% ≥ 1%"},
		},
	})
	if strings.Contains(dup, "99.0") || strings.Contains(dup, "丢包 9%") {
		t.Fatal("duplicate metrics should keep the first value")
	}
	if !strings.Contains(dup, "近15分钟：延迟 18.6ms") || !strings.Contains(dup, "或 抖动 > 20ms") {
		t.Fatalf("edges:\n%s", dup)
	}
	if !strings.Contains(dup, "原因：丢包 4.2% ≥ 1%") || strings.Contains(dup, "窗口：") {
		t.Fatal("reason without a window marker should stay one line")
	}

	odd := wecomText(alertMsg{
		Kind: "alert",
		Link: "A → B",
		Fields: [][2]string{
			{"近15分钟实测", "自定义指标"},
		},
	})
	if !strings.Contains(odd, "近15分钟：自定义指标") || !strings.Contains(odd, "级别：重要 (MAJOR) · 质量异常") {
		t.Fatalf("fallback:\n%s", odd)
	}

	for _, card := range []string{muted, dup, odd} {
		if len(card) > 2048 {
			t.Fatalf("card is %d bytes, WeCom text limit is 2048", len(card))
		}
	}
}
