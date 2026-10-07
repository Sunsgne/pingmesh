package g

import "testing"

func TestChartStepSec(t *testing.T) {
	cases := []struct {
		rangeSec int64
		want     int64
	}{
		{3600, 10},           // 1h
		{6 * 3600, 10},       // 6h → 2160 points @10s
		{12 * 3600, 30},      // 12h
		{24 * 3600, 60},      // 1d
		{3 * 24 * 3600, 120}, // 3d
		{7 * 24 * 3600, 300}, // 7d
		{0, 10},
	}
	for _, c := range cases {
		got := ChartStepSec(c.rangeSec)
		if got != c.want {
			t.Fatalf("range=%d got=%d want=%d", c.rangeSec, got, c.want)
		}
		if got%int64(ProbeCycleSec) != 0 {
			t.Fatalf("step %d not multiple of ProbeCycleSec", got)
		}
		pts := c.rangeSec / got
		if c.rangeSec > 0 && pts > int64(MaxPingChartPoints)+1 {
			t.Fatalf("range=%d step=%d still yields %d points", c.rangeSec, got, pts)
		}
	}
}

func TestAlignUnixStep(t *testing.T) {
	if AlignUnixStep(1005, 60) != 960 {
		t.Fatal(AlignUnixStep(1005, 60))
	}
	if AlignUnixStep(-1, 10) != 0 {
		t.Fatal("neg")
	}
}
