package g

// MaxPingChartPoints 单次 ping.json 返回的最大采样点数。
// 超过后按更大步长聚合, 避免长跨度(≥2~3天)超时/前端空白。
const MaxPingChartPoints = 2160

// ChartStepSec 按时间跨度选择图表分桶步长(秒), 且为 ProbeCycleSec 的整数倍。
func ChartStepSec(rangeSec int64) int64 {
	base := int64(ProbeCycleSec)
	if rangeSec <= 0 {
		return base
	}
	want := rangeSec / int64(MaxPingChartPoints)
	if want <= base {
		return base
	}
	// 优先使用较「整齐」的档位, 便于读轴
	// 5 分钟以上的档位都是 300 的整数倍, 才能直接由 5 分钟汇总表合并
	for _, c := range []int64{10, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400} {
		if c >= want && c%base == 0 {
			return c
		}
	}
	return ((want + 86400 - 1) / 86400) * 86400
}

// AlignUnixStep 将 Unix 时间戳对齐到 step 秒边界。
func AlignUnixStep(unix, step int64) int64 {
	if unix < 0 {
		return 0
	}
	if step <= 0 {
		step = int64(ProbeCycleSec)
	}
	return unix - unix%step
}
