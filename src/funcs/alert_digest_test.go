package funcs

import (
	"strings"
	"testing"

	"github.com/zenlenet/pingmesh/src/g"
)

func TestDiagnoseStarBothWays(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "SIN-EQSG2", ToName: "FRA-EQFRA5"},
		{FromName: "TYO-EQTY7", ToName: "FRA-EQFRA5"},
		{FromName: "FRA-EQFRA5", ToName: "SIN-EQSG2"},
		{FromName: "FRA-EQFRA5", ToName: "PEK-BJHK"},
	})
	if got.Headline != "FRA-EQFRA5 — PEK-BJHK、SIN-EQSG2、TYO-EQTY7" {
		t.Fatalf("headline=%s", got.Headline)
	}
	if !strings.Contains(got.Judgment, "前一侧 → 后一侧 2 条") || !strings.Contains(got.Judgment, "后一侧 → 前一侧 2 条") {
		t.Fatalf("judgment=%s", got.Judgment)
	}
}

func TestDiagnoseOutboundOnly(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "FRA-EQFRA5", ToName: "SIN-EQSG2"},
		{FromName: "FRA-EQFRA5", ToName: "PEK-BJHK"},
		{FromName: "FRA-EQFRA5", ToName: "PVG-GDS"},
	})
	if got.Headline != "FRA-EQFRA5 — PEK-BJHK、PVG-GDS、SIN-EQSG2" {
		t.Fatalf("headline=%s", got.Headline)
	}
	if !strings.Contains(got.Judgment, "前一侧 → 后一侧 3 条，后一侧 → 前一侧 0 条") {
		t.Fatalf("judgment=%s", got.Judgment)
	}
}

func TestDiagnosePair(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "PEK-BJHK", ToName: "CAN-HXY"},
		{FromName: "CAN-HXY", ToName: "PEK-BJHK"},
	})
	if got.Headline != "CAN-HXY — PEK-BJHK" {
		t.Fatalf("headline=%s", got.Headline)
	}
	if !strings.Contains(got.Judgment, "前一侧 → 后一侧 1 条，后一侧 → 前一侧 1 条") {
		t.Fatalf("judgment=%s", got.Judgment)
	}
}

func TestDiagnoseOddCycle(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "PEK-BJHK", ToName: "CAN-HXY"},
		{FromName: "CAN-HXY", ToName: "CAN-XXG"},
		{FromName: "CAN-XXG", ToName: "PEK-BJHK"},
	})
	if got.Headline != "无法二分" {
		t.Fatalf("headline=%s", got.Headline)
	}
	if !strings.Contains(got.Judgment, "奇数环") || !strings.Contains(got.Judgment, "CAN-HXY—PEK-BJHK") {
		t.Fatalf("judgment=%s", got.Judgment)
	}
}

func TestDigestCardStatesBothEnds(t *testing.T) {
	items := []pendingNotify{
		{log: g.AlertLog{Fromname: "SIN-EQSG2", Targetname: "FRA-EQFRA5", Logtime: "2026-11-21 17:12", Reason: "延迟 222ms ≥ 200ms（观测 2 次）（窗口 60s / 规则 2 次）"}},
		{log: g.AlertLog{Fromname: "TYO-EQTY7", Targetname: "FRA-EQFRA5", Logtime: "2026-11-21 17:12", Reason: "延迟 229ms ≥ 200ms（观测 2 次）（窗口 60s / 规则 2 次）"}},
		{log: g.AlertLog{Fromname: "FRA-EQFRA5", Targetname: "SIN-EQSG2", Logtime: "2026-11-21 17:12", Reason: "丢包 10.3% ≥ 1%（观测 1 次）（窗口 60s / 规则 2 次）"}},
	}
	card := wecomText(buildDigest("alert", items))
	for _, want := range []string{
		"【故障区间】FRA-EQFRA5 — SIN-EQSG2、TYO-EQTY7",
		"前一侧 → 后一侧 1 条，后一侧 → 前一侧 2 条",
		"SIN-EQSG2 → FRA-EQFRA5  延迟 222ms ≥ 200ms（观测 2 次）",
		"FRA-EQFRA5 → SIN-EQSG2  丢包 10.3% ≥ 1%（观测 1 次）",
	} {
		if !strings.Contains(card, want) {
			t.Fatalf("missing %q\n%s", want, card)
		}
	}
	if strings.Contains(card, "优先查") || strings.Contains(card, "建议处置") {
		t.Fatalf("digest should not use colloquial advice:\n%s", card)
	}
}

func TestClassifyBreachUsesPeakNotMean(t *testing.T) {
	rule := map[string]string{"Thdavgdelay": "42", "Thdloss": "1", "Thdchecksec": "60", "Thdoccnum": "2"}
	typ, reason, tag := classifyBreach("10.100.1.15", "10.100.1.19", rule, WindowBreach{
		DelayMax: 54.7, DelayN: 2, MeanDelay: 38.7,
	})
	if typ != "delay" || tag != "DELAY:10.100.1.15→10.100.1.19" {
		t.Fatalf("typ=%s tag=%s", typ, tag)
	}
	if !strings.Contains(reason, "延迟 54.7ms ≥ 42ms（观测 2 次）") || strings.Contains(reason, "38.7") {
		t.Fatalf("reason=%s", reason)
	}
}
