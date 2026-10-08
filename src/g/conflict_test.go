package g

import (
	"os"
	"testing"
)

func editAs(addr string, base Config, edit func(*Config)) Config {
	// 模拟在 addr 节点上基于 base 做一次管理员修改
	Cfg = base
	Cfg.Addr = addr
	c := base
	c.Base = map[string]int{}
	for k, v := range base.Base {
		c.Base[k] = v
	}
	c.Mode = map[string]string{}
	for k, v := range base.Mode {
		c.Mode[k] = v
	}
	edit(&c)
	BumpEpochInPlace(&c)
	c.Addr = addr
	return c
}

func TestConflictDetection(t *testing.T) {
	Root = t.TempDir()
	os.MkdirAll(Root+"/conf", 0700)
	base := Config{Port: 8899, Mode: map[string]string{"Epoch": "10"}, Base: map[string]int{"Archive": 30},
		Network: map[string]NetworkMember{"10.0.0.1": {Addr: "10.0.0.1"}, "10.0.0.2": {Addr: "10.0.0.2"}}}

	// A 与 B 基于同一版本各改一处
	a := editAs("10.0.0.1", base, func(c *Config) { c.Base["Archive"] = 60 })
	b := editAs("10.0.0.2", base, func(c *Config) { c.Base["Refresh"] = 5 })
	if !Fresher(LocalVersionOf(b), LocalVersionOf(a)) && !Fresher(LocalVersionOf(a), LocalVersionOf(b)) {
		t.Fatalf("one of the concurrent versions must win")
	}
	winner, loser := a, b
	if Fresher(LocalVersionOf(b), LocalVersionOf(a)) {
		winner, loser = b, a
	}
	// 失败方采纳胜方: 应记录冲突
	Cfg = loser
	checkOverwrite(&Cfg, &winner)
	list := ListConflicts()
	if len(list) != 1 || len(list[0].Sections) != 1 || list[0].Sections[0] != "Base" {
		t.Fatalf("expected one conflict on Base, got %+v", list)
	}
	if _, err := os.Stat(Root + "/conf/backups/" + list[0].File); err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
	DismissConflict(list[0].ID)

	// 基于对方版本继续修改后再被采纳: 不算冲突
	next := editAs(winner.Addr, loser, func(c *Config) { c.Base["Timeout"] = 9 })
	Cfg = loser
	checkOverwrite(&Cfg, &next)
	if n := len(ListConflicts()); n != 0 {
		t.Fatalf("descendant version must not be a conflict, got %d", n)
	}
}

func LocalVersionOf(c Config) CfgVersion {
	saved := Cfg
	Cfg = c
	defer func() { Cfg = saved }()
	return LocalVersion()
}
