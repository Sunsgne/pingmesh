package main

import (
	"flag"
	"fmt"
	"log"
	"os"

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
	token := flag.String("token", "", "join token (master's config password)")
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
	c := cron.New()
	c.AddFunc("*/60 * * * * *", func() {
		go funcs.Ping()
		go funcs.Mapping()
		// 集群容灾同步: cloud 模式或已组建集群时启用(主挂自动接管 + 配置全网收敛)
		if g.Cfg.Mode["Type"] == "cloud" || g.ClusterActive() {
			go funcs.ClusterSync()
		}
	}, "ping")
	c.AddFunc("0 0 * * * *", func() {
		go funcs.ClearArchive()
	}, "mtc")
	c.Start()
	http.StartHttp()
}
