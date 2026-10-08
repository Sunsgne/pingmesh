package g

import (
	"testing"
	"time"
)

func TestChartStepAlignsWithRollup(t *testing.T) {
	for _, days := range []int64{1, 3, 7, 30, 90, 180, 365, 730} {
		step := ChartStepSec(days * 86400)
		if step >= RollupBucketSec && step%RollupBucketSec != 0 {
			t.Fatalf("%d days: step %d is not a multiple of the 5-minute rollup", days, step)
		}
		if pts := days * 86400 / step; pts > MaxPingChartPoints {
			t.Fatalf("%d days: %d points exceeds limit", days, pts)
		}
	}
}

func TestRollupBucketMatchesSQLTruncation(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	cases := map[string]string{
		"2026-10-08 08:47:30": "2026-10-08 08:45:00",
		"2026-10-08 08:44:50": "2026-10-08 08:40:00",
		"2026-10-08 08:50:00": "2026-10-08 08:50:00",
		"2026-10-08 00:04:59": "2026-10-08 00:00:00",
	}
	for in, want := range cases {
		tm, _ := time.ParseInLocation("2006-01-02 15:04:05", in, loc)
		if got := RollupBucket(tm); got != want {
			t.Fatalf("RollupBucket(%s) = %s, want %s", in, got, want)
		}
	}
}
