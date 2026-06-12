package funcs

import (
	"net"
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

// PingTask 对单个目标连续探测20次(3s间隔), 返回统计
func PingTask(addr string) g.PingSt {
	seelog.Debug("[func:PingTask] start ", addr)
	stat := g.PingSt{}
	stat.MinDelay = -1
	lossPK := 0
	ipaddr, err := net.ResolveIPAddr("ip", addr)
	if err != nil {
		stat.LossPk = 100
		seelog.Debug("[func:PingTask] ", addr, " unable to resolve")
		return stat
	}
	for i := 0; i < 20; i++ {
		starttime := time.Now().UnixNano()
		delay, err := nettools.RunPing(ipaddr, 3*time.Second, 64, i)
		if err == nil {
			stat.AvgDelay = stat.AvgDelay + delay
			if stat.MaxDelay < delay {
				stat.MaxDelay = delay
			}
			if stat.MinDelay == -1 || stat.MinDelay > delay {
				stat.MinDelay = delay
			}
			stat.RevcPk = stat.RevcPk + 1
		} else {
			lossPK = lossPK + 1
		}
		stat.SendPk = stat.SendPk + 1
		stat.LossPk = int((float64(lossPK) / float64(stat.SendPk)) * 100)
		duringtime := time.Now().UnixNano() - starttime
		if sleep := 3000*1000000 - duringtime; sleep > 0 {
			time.Sleep(time.Duration(sleep) * time.Nanosecond)
		}
	}
	if stat.RevcPk > 0 {
		stat.AvgDelay = stat.AvgDelay / float64(stat.RevcPk)
	} else {
		stat.AvgDelay = 0.0
	}
	seelog.Debug("[func:PingTask] finish ", addr, " avg:", stat.AvgDelay, " loss:", stat.LossPk)
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
	stmt, err := tx.Prepare("INSERT INTO pinglog (logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk) values(?,?,?,?,?,?,?,?)")
	if err != nil {
		seelog.Error("[func:PingStorageBatch] Prepare ", err)
		tx.Rollback()
		return
	}
	for _, r := range batch {
		if _, err := stmt.Exec(logtime, r.Addr, r.Stat.MaxDelay, r.Stat.MinDelay, r.Stat.AvgDelay, r.Stat.SendPk, r.Stat.RevcPk, r.Stat.LossPk); err != nil {
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
