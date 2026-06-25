package funcs

import (
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

// 防止探测周期重叠(超时较多时一轮可能超过60s)
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
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentTargets)
	results := make(chan targetResult, len(targets))
	for _, target := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(addr string) {
			defer func() { <-sem; wg.Done() }()
			results <- targetResult{Addr: addr, Stat: PingTask(addr)}
		}(target)
	}
	wg.Wait()
	close(results)
	batch := []targetResult{}
	for r := range results {
		batch = append(batch, r)
	}
	PingStorageBatch(batch)
	go StartAlert()
}

type targetResult struct {
	Addr string
	Stat g.PingSt
}

// probeParams 探测引擎参数(毫秒级, 适配 IPLC/IEPL 专线监控)
func probeParams() (interval, count, timeout, size int) {
	interval = g.Cfg.Base["Pinginterval"]
	count = g.Cfg.Base["Pingcount"]
	timeout = g.Cfg.Base["Pingtimeout"]
	size = g.Cfg.Base["Pingsize"]
	if interval < 10 {
		interval = 3000
	}
	if count < 1 {
		count = 20
	}
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
	ovr("Pcount", 1, 1000, &count)
	ovr("Ptimeout", 50, 10000, &timeout)
	ovr("Psize", 24, 1472, &size)
	return
}

// PingTask 对单个目标按配置的间隔/包数/超时/包长连续探测, 返回统计(含抖动)
func PingTask(addr string) g.PingSt {
	seelog.Debug("[func:PingTask] start ", addr)
	interval, count, timeout, size, srcip := probeParamsFor(addr)
	stat := g.PingSt{}
	stat.MinDelay = -1
	lossPK := 0
	ipaddr, err := net.ResolveIPAddr("ip", addr)
	if err != nil {
		stat.LossPk = 100
		seelog.Debug("[func:PingTask] ", addr, " unable to resolve")
		return stat
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
			// 抖动: 相邻成功样本 RTT 差
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
	stat.LossPk = int((float64(lossPK) / float64(stat.SendPk)) * 100)
	if stat.RevcPk > 0 {
		stat.AvgDelay = stat.AvgDelay / float64(stat.RevcPk)
	} else {
		stat.AvgDelay = 0.0
	}
	if jitterCnt > 0 {
		stat.Jitter = jitterSum / float64(jitterCnt)
	}
	seelog.Debug("[func:PingTask] finish ", addr, " avg:", stat.AvgDelay, " loss:", stat.LossPk, " jitter:", stat.Jitter)
	return stat
}

// PingStorageBatch 单事务批量落库(替代逐条INSERT+逐条fsync)
func PingStorageBatch(batch []targetResult) {
	if len(batch) == 0 {
		return
	}
	logtime := time.Now().Format("2006-01-02 15:04")
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
