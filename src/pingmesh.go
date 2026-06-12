package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/jakecoffman/cron"
	"github.com/zenlenet/pingmesh/src/funcs"
	"github.com/zenlenet/pingmesh/src/g"
	"github.com/zenlenet/pingmesh/src/http"
)

// Init config
var Version = "1.0.0"

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	version := flag.Bool("v", false, "show version")
	port := flag.Int("p", 0, "http port (override config)")
	listen := flag.String("l", "", "listen address, e.g. 127.0.0.1:8899 (override config)")
	workdir := flag.String("w", "", "work directory (default: binary directory)")
	join := flag.String("join", "", "join a master node, e.g. http://10.0.0.1:8899")
	token := flag.String("token", "", "join token (master's config password)")
	name := flag.String("name", "", "node name used when joining")
	addr := flag.String("addr", "", "node ip used when joining (auto-detect if empty)")
	group := flag.String("group", "", "node group used when joining (optional)")
	flag.Parse()
	if *version {
		fmt.Println(Version)
		os.Exit(0)
	}
	g.FlagWorkDir = *workdir
	g.FlagPort = *port
	g.FlagListen = *listen
	g.ParseConfig(Version)
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
		if g.Cfg.Mode["Type"] == "cloud" {
			go funcs.StartCloudMonitor()
		}
	}, "ping")
	c.AddFunc("0 0 * * * *", func() {
		go funcs.ClearArchive()
	}, "mtc")
	c.Start()
	http.StartHttp()
}
