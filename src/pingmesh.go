package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jakecoffman/cron"
	"github.com/zenlenet/pingmesh/src/funcs"
	"github.com/zenlenet/pingmesh/src/g"
	"github.com/zenlenet/pingmesh/src/http"
)

// 版本信息(BuildTime / GitCommit 由 -ldflags 注入, 见 deploy/install.sh 与 Dockerfile)
var (
	Version   = "2.0.0"
	BuildTime = "unknown"
	GitCommit = "dev"
)

func main() {
	version := flag.Bool("v", false, "show version")
	port := flag.Int("p", 0, "http port (override config)")
	listen := flag.String("l", "", "listen address, e.g. 127.0.0.1:8899 (override config)")
	workdir := flag.String("w", "", "work directory (default: binary directory)")
	join := flag.String("join", "", "join a master node, e.g. http://10.0.0.1:8899")
	// 令牌优先从环境变量读(systemd EnvironmentFile, 0600), 避免出现在进程命令行和全员可读的 unit 文件里
	token := flag.String("token", os.Getenv("PINGMESH_JOIN_TOKEN"), "join token (master's config password); defaults to $PINGMESH_JOIN_TOKEN")
	name := flag.String("name", "", "node name used when joining")
	addr := flag.String("addr", "", "node ip used when joining (auto-detect if empty)")
	group := flag.String("group", "", "node group used when joining (optional)")
	masters := flag.String("masters", "", "comma-separated standby master endpoints for failover, e.g. 10.0.0.2:8899")
	webui := flag.Bool("webui", false, "force-enable web UI on agent nodes (agents disable it by default; master always on)")
	flag.Parse()
	if *version {
		fmt.Printf("ZENLENET PingMesh %s (commit %s, built %s)\n", Version, GitCommit, BuildTime)
		os.Exit(0)
	}
	g.FlagWorkDir = *workdir
	g.FlagPort = *port
	g.FlagListen = *listen
	g.FlagWebUI = *webui
	g.ParseConfig(Version)
	g.InitTimezone()
	// 首次安装的主节点: 用 -name/-addr 初始化节点身份(仅首次, 不覆盖后续在页面改名)
	if g.FreshInstall && *join == "" {
		if *name != "" {
			g.Cfg.Name = *name
		}
		if *addr != "" {
			g.Cfg.Addr = *addr
			g.SelfCfg = g.Cfg.Network[g.Cfg.Addr]
		}
		if *name != "" || *addr != "" {
			g.SaveConfig()
		}
	}
	if *masters != "" {
		g.SetStandbyMasters(*masters)
	}
	if *join != "" {
		if err := funcs.JoinMaster(*join, *token, *name, *addr, *group); err != nil {
			log.Fatalln("[Fault]join master fail:", err)
		}
	}
	go funcs.ClearArchive()
	funcs.StartDbMaintenance()
	sched := &scheduler{}
	sched.start()
	go sched.watchClock()
	http.StartHttp()
}

// scheduler 跑周期任务。jakecoffman/cron 用墙上时间算出下一次触发点再 time.After 等待。
// NTP 把时钟往回拨几个小时后，下一次触发点仍停在拨钟前的时刻，探测、告警和配置同步会一直停到时钟追上。
type scheduler struct {
	mu sync.Mutex
	c  *cron.Cron
}

func (s *scheduler) start() {
	c := cron.New()
	c.AddFunc("*/10 * * * * *", func() {
		go funcs.Ping()
		go func() {
			time.Sleep(3 * time.Second)
			funcs.MasterAlertSweep()
		}()
	}, "ping")
	c.AddFunc("*/60 * * * * *", func() {
		go funcs.Mapping()
		if g.Cfg.Mode["Type"] == "cloud" || g.ClusterActive() {
			go funcs.ClusterSync()
		}
	}, "mapping")
	c.AddFunc("0 0 * * * *", func() {
		go funcs.ClearArchive()
	}, "mtc")
	c.Start()
	s.mu.Lock()
	s.c = c
	s.mu.Unlock()
}

func (s *scheduler) restart() {
	s.mu.Lock()
	old := s.c
	s.mu.Unlock()
	if old != nil {
		old.Stop()
	}
	s.start()
}

func (s *scheduler) watchClock() {
	prevWall := time.Now().Round(0)
	prevMono := time.Now()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for now := range ticker.C {
		nowWall := now.Round(0)
		if clockSteppedBack(prevWall, nowWall, now.Sub(prevMono)) {
			log.Printf("[clock] wall clock stepped backward by %s, restarting scheduler", prevWall.Sub(nowWall).Round(time.Second))
			s.restart()
		}
		prevWall = time.Now().Round(0)
		prevMono = time.Now()
	}
}

// clockSteppedBack 报告墙上时间相对单调时钟是否往回跳了超过 2 秒。
func clockSteppedBack(prevWall, nowWall time.Time, monoElapsed time.Duration) bool {
	if monoElapsed <= 0 {
		return false
	}
	return monoElapsed-nowWall.Sub(prevWall) > 2*time.Second
}
