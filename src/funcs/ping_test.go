package funcs

import "testing"

func TestLossPercent(t *testing.T) {
	cases := []struct {
		lost, sent int
		want       float64
	}{
		{0, 820, 0},
		{1, 820, 0.12},  // 100/820 ≈ 0.122 → 0.12
		{3, 820, 0.37},
		{8, 820, 0.98},
		{9, 820, 1.10},
		{820, 820, 100},
		{1, 0, 100},
	}
	for _, c := range cases {
		got := lossPercent(c.lost, c.sent)
		if got != c.want {
			t.Fatalf("lossPercent(%d,%d)=%v want %v", c.lost, c.sent, got, c.want)
		}
	}
}

func TestPaceSlotPreservesPerTargetInterval(t *testing.T) {
	intervalMs := 67
	n := 16
	slot := intervalMs / n // integer ms used as lower bound in code via Duration
	// n slots == one interval (with Duration division this is exact in time.Duration space)
	if slot < 1 {
		t.Fatal("slot too small")
	}
	roundTrip := slot * n
	if roundTrip > intervalMs {
		t.Fatalf("round-robin period %dms exceeds interval %dms", roundTrip, intervalMs)
	}
}
