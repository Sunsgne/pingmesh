package g

import "testing"

func TestCyclePacketCount(t *testing.T) {
	// 67ms×820/min → 每10秒约136包, 仍能在9.5s内发完
	got := CyclePacketCount(67, 820)
	if got != 136 {
		t.Fatalf("CyclePacketCount(67,820)=%d want 136", got)
	}
	if got*67 > ProbeCycleMaxMs() {
		t.Fatalf("cycle exceeds budget: %dms", got*67)
	}
	// 每分钟总量近似守恒
	if got*6 != 816 {
		t.Fatalf("per-minute total %d want ~816", got*6)
	}
}
