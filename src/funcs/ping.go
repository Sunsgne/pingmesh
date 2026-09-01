package funcs

import (
	"hash/fnv"
	"math"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
	"github.com/zenlenet/pingmesh/src/nettools"
	_ "modernc.org/sqlite"
)

// 防止探测周期重叠(一轮可能超过10s)
var pingCycleRunning int32

// 单轮探测的最大并发目标数
const maxConcurrentTargets = 256

func Ping() {
	if !atomic.CompareAndSwapInt32(&pingCycleRunning, 0, 1) {
		seelog.Info("[func:Ping] previous cycle still running, skip this tick")
		return
	}
	defer atomic.StoreInt32(&pingCycleRunning, 0)

	targets := g.SelfCfg.Ping
	if len(targets) == 0 {
		go StartAlert()
		return
	}

	// 节点间错峰: 各 agent 按 Addr 哈希错开启动, 避免整网在同一秒打满对端 ICMP 回复配额
	if off := nodePhaseOffset(); off > 0 {
		time.Sleep(off)
	}

	var batch []targetResult
	if canPaceUniform(targets) {
		batch = pingCyclePaced(targets)
	} else {
		batch = pingCycleParallel(targets)
	}
	PingStorageBatch(batch)
	go StartAlert()
}

type targetResult struct {
	Addr string
	Stat g.PingSt
}

// lossPercent 真实丢包率(保留两位小数), 避免 int 截断把 0.4%~0.9% 记成 0
func lossPercent(lost, sent int) float64 {
	if sent <= 0 {
		return 100
	}
	return math.Round(float64(lost)/float64(sent)*10000) / 100
}

// nodePhaseOffset 在周期余量内错开本节点启动时刻。
func nodePhaseOffset() time.Duration {
	interval, count, timeout, _ := probeParams()
	cycleMs := count*interval + timeout + 200
	spare := g.ProbeCycleMaxMs() - cycleMs
	if spare <= 0 {
		return 0
	}
	if spare > 800 {
		spare = 800
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(g.Cfg.Addr))
	return time.Duration(h.Sum32()%uint32(spare)) * time.Millisecond
}

// canPaceUniform 全部目标共用同一套全局探测参数时可走平滑轮转发包
func canPaceUniform(targets []string) bool {
	if len(targets) <= 1 {
		return true
	}
	baseI, baseC, baseT, baseS := probeParams()
	for _, addr := range targets {
		i, c, t, s, src := probeParamsFor(addr)
		if src != "" || i != baseI || c != baseC || t != baseT || s != baseS {
			return false
		}
	}
	return true
}

// pingCyclePaced 全目标轮转平滑发包:
// 每目标仍保持原 interval/count(测量精度不变), 但把「同一时刻对 N 个目标齐射」
// 摊成 interval/N 的均匀节拍, 显著降低本机突发与对端 icmp_msgs 限速误判丢包。
func pingCyclePaced(targets []string) []targetResult {
	interval, count, timeout, size := probeParams()
	n := len(targets)
	ipaddrs := make([]*net.IPAddr, n)
	rtts := make([][]float64, n)
	for i, addr := range targets {
		rtts[i] = make([]float64, count)
		ip, err := net.ResolveIPAddr("ip", addr)
		if err != nil {
			for j := 0; j < count; j++ {
				rtts[i][j] = -1
			}
			seelog.Debug("[func:pingCyclePaced] ", addr, " unable to resolve")
			continue
		}
		ipaddrs[i] = ip
	}

	slot := time.Duration(interval) * time.Millisecond / time.Duration(n)
	if slot < time.Millisecond {
		slot = time.Millisecond
	}
	to := time.Duration(timeout) * time.Millisecond

	// 绝对时间轴节拍: 避免 time.Sleep 累计误差把整轮拖过周期上限。
	var pwg sync.WaitGroup
	start := time.Now()
	k := 0
	for pi := 0; pi < count; pi++ {
		for ti := 0; ti < n; ti++ {
			if wait := time.Until(start.Add(time.Duration(k) * slot)); wait > 0 {
				time.Sleep(wait)
			}
			k++
			ip := ipaddrs[ti]
			if ip == nil {
				continue
			}
			pwg.Add(1)
			go func(ti, pi int, ip *net.IPAddr) {
				defer pwg.Done()
				delay, err := nettools.RunPingFrom(ip, to, size, "")
				if err == nil {
					rtts[ti][pi] = delay
				} else {
					rtts[ti][pi] = -1
				}
			}(ti, pi, ip)
		}
	}
	pwg.Wait()

	batch := make([]targetResult, 0, n)
	for i, addr := range targets {
		batch = append(batch, targetResult{Addr: addr, Stat: finalizePingStat(rtts[i])})
	}
	seelog.Info("[func:pingCyclePaced] paced ", n, " targets x ", count, " pkts, slot=", slot)
	return batch
}

// pingCycleParallel 链路级参数不一致时的回退: 每目标独立探测, 但仍做目标间相位错开
func pingCycleParallel(targets []string) []targetResult {
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentTargets)
	results := make(chan targetResult, len(targets))
	n := len(targets)
	for i, target := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, addr string) {
			defer func() { <-sem; wg.Done() }()
			results <- targetResult{Addr: addr, Stat: PingTask(addr, idx, n)}
		}(i, target)
	}
	wg.Wait()
	close(results)
	batch := make([]targetResult, 0, n)
	for r := range results {
		batch = append(batch, r)
	}
	return batch
}

// probeParams 探测引擎参数(毫秒级); count 为当前10秒周期的发包数
func probeParams() (interval, count, timeout, size int) {
	interval = g.Cfg.Base["Pinginterval"]
	perMin := g.Cfg.Base["Pingcount"]
	timeout = g.Cfg.Base["Pingtimeout"]
	size = g.Cfg.Base["Pingsize"]
	if interval < 10 {
		interval = 3000
	}
	if perMin < 1 {
		perMin = 20
	}
	count = g.CyclePacketCount(interval, perMin)
	if timeout < 50 {
		timeout = 3000
	}
	if size < 24 {
		size = 56
	}
	return
}

// linkRule 该目标在本节点监测规则中的条目
func linkRule(addr string) map[string]string {
	for _, t := range g.SelfCfg.Topology {
		if t["Addr"] == addr {
			return t
		}
	}
	return nil
}

// probeParamsFor 链路级探测参数: 在全局默认基础上应用该链路的覆盖值
func probeParamsFor(addr string) (interval, count, timeout, size int, srcip string) {
	interval, count, timeout, size = probeParams()
	perMin := g.Cfg.Base["Pingcount"]
	t := linkRule(addr)
	if t == nil {
		return
	}
	srcip = t["Srcip"]
	ovr := func(key string, min, max int, def *int) {
		if v, err := strconv.Atoi(t[key]); err == nil && v >= min && v <= max {
			*def = v
		}
	}
	ovr("Pinterval", 10, 60000, &interval)
	if v, err := strconv.Atoi(t["Pcount"]); err == nil && v >= 1 && v <= 1000 {
		perMin = v
	}
	ovr("Ptimeout", 50, 10000, &timeout)
	ovr("Psize", 24, 1472, &size)
	count = g.CyclePacketCount(interval, perMin)
	return
}

// PingTask 对单个目标按配置的间隔/包数/超时/包长连续探测, 返回统计(含抖动)。
// targetIndex/targetCount 用于目标间相位错开(回退路径); 单目标探测可传 0,1。
func PingTask(addr string, targetIndex, targetCount int) g.PingSt {
	seelog.Debug("[func:PingTask] start ", addr)
	interval, count, timeout, size, srcip := probeParamsFor(addr)
	ipaddr, err := net.ResolveIPAddr("ip", addr)
	if err != nil {
		stat := g.PingSt{LossPk: 100}
		seelog.Debug("[func:PingTask] ", addr, " unable to resolve")
		return stat
	}
	if targetCount > 1 && targetIndex >= 0 {
		phase := time.Duration(targetIndex) * time.Duration(interval) * time.Millisecond / time.Duration(targetCount)
		if phase > 0 {
			time.Sleep(phase)
		}
	}
	// 按间隔节拍异步发包: 发送节奏不受超时影响,
	// 整轮耗时 ≈ (count-1)×interval + timeout, 丢包不会拖长周期导致跳轮断点
	rtts := make([]float64, count)
	var pwg sync.WaitGroup
	for i := 0; i < count; i++ {
		pwg.Add(1)
		go func(idx int) {
			defer pwg.Done()
			delay, err := nettools.RunPingFrom(ipaddr, time.Duration(timeout)*time.Millisecond, size, srcip)
			if err == nil {
				rtts[idx] = delay
			} else {
				rtts[idx] = -1
			}
		}(i)
		if i < count-1 {
			time.Sleep(time.Duration(interval) * time.Millisecond)
		}
	}
	pwg.Wait()
	stat := finalizePingStat(rtts)
	seelog.Debug("[func:PingTask] finish ", addr, " avg:", stat.AvgDelay, " loss:", stat.LossPk, " jitter:", stat.Jitter)
	return stat
}

func finalizePingStat(rtts []float64) g.PingSt {
	stat := g.PingSt{}
	stat.MinDelay = -1
	lossPK := 0
	prev := -1.0
	var jitterSum float64
	jitterCnt := 0
	for _, delay := range rtts {
		stat.SendPk++
		if delay >= 0 {
			stat.AvgDelay += delay
			if stat.MaxDelay < delay {
				stat.MaxDelay = delay
			}
			if stat.MinDelay == -1 || stat.MinDelay > delay {
				stat.MinDelay = delay
			}
			stat.RevcPk++
			if prev >= 0 {
				d := delay - prev
				if d < 0 {
					d = -d
				}
				jitterSum += d
				jitterCnt++
			}
			prev = delay
		} else {
			lossPK++
		}
	}
	stat.LossPk = lossPercent(lossPK, stat.SendPk)
	if stat.RevcPk > 0 {
		stat.AvgDelay = stat.AvgDelay / float64(stat.RevcPk)
	} else {
		stat.AvgDelay = 0.0
	}
	if jitterCnt > 0 {
		stat.Jitter = jitterSum / float64(jitterCnt)
	}
	return stat
}

// PingStorageBatch 单事务批量落库(替代逐条INSERT+逐条fsync)
func PingStorageBatch(batch []targetResult) {
	if len(batch) == 0 {
		return
	}
	logtime := g.ProbeLogTime(time.Now())
	g.DLock.Lock()
	defer g.DLock.Unlock()
	tx, err := g.Db.Begin()
	if err != nil {
		seelog.Error("[func:PingStorageBatch] Begin ", err)
		return
	}
	stmt, err := tx.Prepare("INSERT INTO pinglog (logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk, jitter) values(?,?,?,?,?,?,?,?,?)")
	if err != nil {
		seelog.Error("[func:PingStorageBatch] Prepare ", err)
		tx.Rollback()
		return
	}
	for _, r := range batch {
		if _, err := stmt.Exec(logtime, r.Addr, r.Stat.MaxDelay, r.Stat.MinDelay, r.Stat.AvgDelay, r.Stat.SendPk, r.Stat.RevcPk, r.Stat.LossPk, r.Stat.Jitter); err != nil {
			seelog.Error("[func:PingStorageBatch] Exec ", r.Addr, " ", err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		seelog.Error("[func:PingStorageBatch] Commit ", err)
		return
	}
	seelog.Info("[func:PingStorageBatch] (", logtime, ") stored ", len(batch), " targets in one tx")
}
