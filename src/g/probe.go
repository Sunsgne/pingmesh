package g

import "time"

// ProbeCycleSec 探测汇总入库周期(秒)。Base.Pingcount 语义为每分钟总发包数。
const ProbeCycleSec = 10

// ProbeCycleMaxMs 单轮探测可用时间上限(毫秒, 留 500ms 余量)。
func ProbeCycleMaxMs() int {
	return ProbeCycleSec*1000 - 500
}

// CyclePacketCount 按周期折算每轮发包数, 保持包间隔不变、每分钟总包数不变。
func CyclePacketCount(interval, perMinuteCount int) int {
	if interval < 1 {
		interval = 1
	}
	if perMinuteCount < 1 {
		perMinuteCount = 1
	}
	perCycle := perMinuteCount * ProbeCycleSec / 60
	if perCycle < 1 {
		perCycle = 1
	}
	maxFit := ProbeCycleMaxMs() / interval
	if maxFit < 1 {
		maxFit = 1
	}
	if perCycle > maxFit {
		perCycle = maxFit
	}
	return perCycle
}

// ProbeLogTime 对齐到探测周期的入库时间戳。
func ProbeLogTime(t time.Time) string {
	return t.Truncate(time.Duration(ProbeCycleSec) * time.Second).Format("2006-01-02 15:04:05")
}

// AlignUnixProbe 将 Unix 时间戳对齐到探测周期起点。
func AlignUnixProbe(unix int64) int64 {
	if unix < 0 {
		return 0
	}
	step := int64(ProbeCycleSec)
	return unix - unix%step
}

// ParseProbeTime 解析查询起止时间(支持到秒或仅到分钟)。
func ParseProbeTime(s string) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	return time.ParseInLocation("2006-01-02 15:04", s, time.Local)
}
