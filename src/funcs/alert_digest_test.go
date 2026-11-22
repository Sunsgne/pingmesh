package funcs

import (
	"strings"
	"testing"
	"time"
)

func mesh(from, to string, ok bool, detail string) meshCell {
	return meshCell{FromName: from, ToName: to, OK: ok, Delay: 30, Loss: 0, Detail: detail}
}

func TestLocalizeIsolatedPair(t *testing.T) {
	// A、B 互相异常，两台到 C、D 都正常。问题只在 A 与 B 之间。
	var cells []meshCell
	nodes := []string{"HKG-EQHK2", "PEK-BJHK", "SIN-EQSG2", "CAN-HXY"}
	for _, a := range nodes {
		for _, b := range nodes {
			if a == b {
				continue
			}
			bad := (a == "HKG-EQHK2" && b == "PEK-BJHK") || (a == "PEK-BJHK" && b == "HKG-EQHK2")
			detail := ""
			if bad {
				detail = "延迟 58.5ms ≥ 46ms（观测 5 次）"
			}
			cells = append(cells, mesh(a, b, !bad, detail))
		}
	}
	got := localizeMatrix(cells)
	if got.Headline != "HKG-EQHK2 ↔ PEK-BJHK" {
		t.Fatalf("headline=%s", got.Headline)
	}
	if !strings.Contains(got.Judgment, "行 HKG-EQHK2 2 正常 / 0 异常") || !strings.Contains(got.Judgment, "列 PEK-BJHK 2 正常 / 0 异常") {
		t.Fatalf("judgment=%s", got.Judgment)
	}
	if !strings.Contains(strings.Join(got.Lines, "\n"), "延迟 58.5ms ≥ 46ms（观测 5 次）") {
		t.Fatalf("lines=%v", got.Lines)
	}
}

func TestLocalizeOneDirection(t *testing.T) {
	var cells []meshCell
	nodes := []string{"A", "B", "C"}
	for _, a := range nodes {
		for _, b := range nodes {
			if a == b {
				continue
			}
			bad := a == "A" && b == "B"
			cells = append(cells, mesh(a, b, !bad, "丢包 10% ≥ 1%（观测 2 次）"))
		}
	}
	got := localizeMatrix(cells)
	if got.Headline != "A → B" {
		t.Fatalf("headline=%s", got.Headline)
	}
}

func TestLocalizeNodeNotFakePair(t *testing.T) {
	// FRA 进出都异常，其余点之间正常。这是 FRA 这个点，不是随便两个点。
	var cells []meshCell
	nodes := []string{"FRA", "SIN", "TYO", "PVG"}
	for _, a := range nodes {
		for _, b := range nodes {
			if a == b {
				continue
			}
			bad := a == "FRA" || b == "FRA"
			cells = append(cells, mesh(a, b, !bad, "丢包 1.1% ≥ 1%（观测 1 次）"))
		}
	}
	got := localizeMatrix(cells)
	if got.Headline != "节点 FRA" {
		t.Fatalf("headline=%s", got.Headline)
	}
	if !strings.Contains(got.Judgment, "行 FRA 0 正常 / 3 异常") || !strings.Contains(got.Judgment, "列 FRA 0 正常 / 3 异常") {
		t.Fatalf("judgment=%s", got.Judgment)
	}
}

func TestDigestCardNamesThePair(t *testing.T) {
	cells := []meshCell{
		mesh("HKG-EQHK2", "PEK-BJHK", false, "延迟 58.5ms ≥ 46ms（观测 5 次）"),
		mesh("PEK-BJHK", "HKG-EQHK2", false, "延迟 58.5ms ≥ 46ms（观测 5 次）"),
		mesh("HKG-EQHK2", "SIN-EQSG2", true, ""),
		mesh("SIN-EQSG2", "HKG-EQHK2", true, ""),
		mesh("PEK-BJHK", "SIN-EQSG2", true, ""),
		mesh("SIN-EQSG2", "PEK-BJHK", true, ""),
	}
	ev := &alertEvent{ID: "1122-140000", Members: map[string]string{"a": "x", "b": "y"}}
	card := wecomText(eventAlertMsg(ev, localizeMatrix(cells), time.Date(2026, 11, 22, 14, 0, 0, 0, time.Local)))
	for _, want := range []string{"【HKG-EQHK2 ↔ PEK-BJHK】", "事件：#1122-140000", "行 HKG-EQHK2 1 正常 / 0 异常", "延迟 58.5ms ≥ 46ms（观测 5 次）"} {
		if !strings.Contains(card, want) {
			t.Fatalf("missing %q\n%s", want, card)
		}
	}
	if strings.Contains(card, "前一侧") || strings.Contains(card, "建议处置") {
		t.Fatalf("card still vague:\n%s", card)
	}
}
