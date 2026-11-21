package funcs

import (
	"strings"
	"testing"

	"github.com/zenlenet/pingmesh/src/g"
)

func TestDiagnoseCommonNodeBothWays(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "SIN-EQSG2", ToName: "FRA-EQFRA5"},
		{FromName: "TYO-EQTY7", ToName: "FRA-EQFRA5"},
		{FromName: "FRA-EQFRA5", ToName: "SIN-EQSG2"},
		{FromName: "FRA-EQFRA5", ToName: "PEK-BJHK"},
	})
	if got.Headline != "FRA-EQFRA5 进出方向的 4 条链路" {
		t.Fatalf("headline=%s", got.Headline)
	}
	if !strings.Contains(got.Judgment, "进出都经过 FRA-EQFRA5") {
		t.Fatalf("judgment=%s", got.Judgment)
	}
}

func TestDiagnoseOutboundOnly(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "FRA-EQFRA5", ToName: "SIN-EQSG2"},
		{FromName: "FRA-EQFRA5", ToName: "PEK-BJHK"},
		{FromName: "FRA-EQFRA5", ToName: "PVG-GDS"},
	})
	if got.Headline != "从 FRA-EQFRA5 出去的 3 条链路" {
		t.Fatalf("headline=%s", got.Headline)
	}
}

func TestDiagnoseInboundOnly(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "SIN-EQSG2", ToName: "FRA-EQFRA5"},
		{FromName: "TYO-EQTY7", ToName: "FRA-EQFRA5"},
	})
	if got.Headline != "打到 FRA-EQFRA5 的 2 条链路" {
		t.Fatalf("headline=%s", got.Headline)
	}
}

func TestDiagnosePair(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "PEK-BJHK", ToName: "CAN-HXY"},
		{FromName: "CAN-HXY", ToName: "PEK-BJHK"},
	})
	if got.Headline != "CAN-HXY 与 PEK-BJHK 之间的 2 条链路" {
		t.Fatalf("headline=%s", got.Headline)
	}
	if !strings.Contains(got.Judgment, "这一段线路") {
		t.Fatalf("judgment=%s", got.Judgment)
	}
}

func TestDiagnoseRegion(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "PEK-BJHK", ToName: "CAN-HXY"},
		{FromName: "CAN-HXY", ToName: "PEK-BJHK"},
		{FromName: "CAN-HXY", ToName: "HKG-EQHK2"},
		{FromName: "HKG-EQHK2", ToName: "SZX-SZX"},
	})
	if got.Headline != "CAN-HXY、HKG-EQHK2、PEK-BJHK 之间的 4 条链路" {
		t.Fatalf("headline=%s", got.Headline)
	}
	if !strings.Contains(got.Judgment, "这一段网络") {
		t.Fatalf("judgment=%s", got.Judgment)
	}
}

func TestDiagnoseUnrelated(t *testing.T) {
	got := diagnoseLinks([]digestLink{
		{FromName: "SIN-EQSG2", ToName: "LAX-CORESITE"},
		{FromName: "TYO-EQTY7", ToName: "PVG-GDS"},
	})
	if got.Headline != "2 条互不相关的链路" {
		t.Fatalf("headline=%s", got.Headline)
	}
}

func TestDigestCardOneMessage(t *testing.T) {
	items := []pendingNotify{
		{log: g.AlertLog{Fromname: "SIN-EQSG2", Targetname: "FRA-EQFRA5", Logtime: "2026-11-21 04:56"}},
		{log: g.AlertLog{Fromname: "TYO-EQTY7", Targetname: "FRA-EQFRA5", Logtime: "2026-11-21 04:56"}},
		{log: g.AlertLog{Fromname: "PVG-GDS", Targetname: "FRA-EQFRA5", Logtime: "2026-11-21 04:56"}},
	}
	card := wecomText(buildDigest("alert", items))
	if strings.Count(card, "【同一故障】") != 1 {
		t.Fatalf("want one headline:\n%s", card)
	}
	if !strings.Contains(card, "打到 FRA-EQFRA5 的 3 条链路") || !strings.Contains(card, "SIN-EQSG2 → FRA-EQFRA5") {
		t.Fatalf("card missing judgment or links:\n%s", card)
	}
	if strings.Contains(card, "【质量异常】") {
		t.Fatalf("digest must not look like a single-link alert:\n%s", card)
	}
}
