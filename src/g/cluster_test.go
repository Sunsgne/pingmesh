package g

import (
	"reflect"
	"testing"
)

func resetCfg() {
	Cfg = Config{
		Addr: "10.0.0.1",
		Port: 8899,
		Mode: map[string]string{"Epoch": "0"},
		Network: map[string]NetworkMember{
			"10.0.0.1": {Name: "A", Addr: "10.0.0.1", Pingmesh: true},
			"10.0.0.2": {Name: "B", Addr: "10.0.0.2", Pingmesh: true},
			"10.0.0.3": {Name: "C", Addr: "10.0.0.3", Pingmesh: true},
			"10.0.0.9": {Name: "D", Addr: "10.0.0.9", Pingmesh: false}, // 非探测节点不入候选
		},
	}
}

func TestFresher(t *testing.T) {
	cases := []struct {
		a, b CfgVersion
		want bool
	}{
		{CfgVersion{2, "2026-01-01 00:00:00"}, CfgVersion{1, "2030-01-01 00:00:00"}, true},  // 纪元优先
		{CfgVersion{1, "2026-01-01 00:00:01"}, CfgVersion{1, "2026-01-01 00:00:00"}, true},  // 同纪元比时间
		{CfgVersion{1, "2026-01-01 00:00:00"}, CfgVersion{1, "2026-01-01 00:00:00"}, false}, // 完全相同非更新
		{CfgVersion{0, ""}, CfgVersion{1, ""}, false},
	}
	for i, c := range cases {
		if got := Fresher(c.a, c.b); got != c.want {
			t.Errorf("case %d: Fresher(%+v,%+v)=%v want %v", i, c.a, c.b, got, c.want)
		}
	}
}

func TestBumpEpochMonotonic(t *testing.T) {
	resetCfg()
	if GetEpoch() != 0 {
		t.Fatalf("initial epoch want 0 got %d", GetEpoch())
	}
	BumpEpoch()
	if GetEpoch() != 1 {
		t.Fatalf("after bump want 1 got %d", GetEpoch())
	}
	// 传入配置携带更高纪元时, 取 max+1 防止倒退
	nc := Config{Mode: map[string]string{"Epoch": "5"}}
	BumpEpochInPlace(&nc)
	if nc.Mode["Epoch"] != "6" {
		t.Fatalf("max+1 want 6 got %s", nc.Mode["Epoch"])
	}
	if nc.Mode["EpochTime"] == "" {
		t.Fatalf("EpochTime should be set on bump")
	}
}

func TestIsSelfEndpoint(t *testing.T) {
	resetCfg()
	if !IsSelfEndpoint("10.0.0.1:8899") {
		t.Error("10.0.0.1:8899 should be self")
	}
	if !IsSelfEndpoint("10.0.0.1") {
		t.Error("bare self host should match")
	}
	if IsSelfEndpoint("10.0.0.2:8899") {
		t.Error("10.0.0.2 should not be self")
	}
	if IsSelfEndpoint("") {
		t.Error("empty should not be self")
	}
}

func TestMasterListPriorityOrder(t *testing.T) {
	resetCfg()
	// 显式主节点 + 自动容灾: 主节点最高优先, 其余 mesh 节点按地址排序补入
	Cfg.Mode["Master"] = "10.0.0.2:8899"
	Cfg.Mode["MasterAuto"] = "true"
	got := MasterList()
	want := []string{"10.0.0.2:8899", "10.0.0.1:8899", "10.0.0.3:8899"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MasterList auto want %v got %v", want, got)
	}

	// 关闭自动容灾: 仅主节点 + 手动备选, 顺序保持
	Cfg.Mode["MasterAuto"] = "false"
	Cfg.Mode["Standbys"] = "10.0.0.3:8899, 10.0.0.1:8899"
	got = MasterList()
	want = []string{"10.0.0.2:8899", "10.0.0.3:8899", "10.0.0.1:8899"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MasterList manual want %v got %v", want, got)
	}
}

func TestClusterActive(t *testing.T) {
	resetCfg()
	if ClusterActive() {
		t.Error("fresh local node should not be cluster-active")
	}
	Cfg.Mode["Master"] = "10.0.0.1:8899"
	if !ClusterActive() {
		t.Error("node with Master set should be cluster-active")
	}
	resetCfg()
	Cfg.Mode["Type"] = "cloud"
	if !ClusterActive() {
		t.Error("cloud node should be cluster-active")
	}
}

func TestWebUIEnabled(t *testing.T) {
	resetCfg()
	defer func() { FlagWebUI = false; SetActingMaster(false) }()

	// 主节点/独立节点(local 模式)始终开放
	Cfg.Mode["Type"] = "local"
	FlagWebUI = false
	SetActingMaster(false)
	if !WebUIEnabled() {
		t.Error("local node should always serve web UI")
	}

	// Agent(cloud 模式)默认关闭
	Cfg.Mode["Type"] = "cloud"
	if WebUIEnabled() {
		t.Error("cloud agent should disable web UI by default")
	}

	// -webui 强制开启
	FlagWebUI = true
	if !WebUIEnabled() {
		t.Error("-webui flag should force-enable web UI")
	}
	FlagWebUI = false

	// 容灾接管期间(代理主节点)自动开放
	SetActingMaster(true)
	if !WebUIEnabled() {
		t.Error("acting master agent should serve web UI during failover")
	}
}

// TestElectionPromotionOnPrimaryDown 验证选举核心: 主节点不可达时,
// 下一优先级的可达候选自动成为代理主节点。这里用纯函数复刻 ClusterSync
// 的选举片段进行断言, 不依赖网络。
func TestElectionPromotionOnPrimaryDown(t *testing.T) {
	resetCfg()
	Cfg.Mode["Master"] = "10.0.0.2:8899" // 主节点为 B
	Cfg.Mode["MasterAuto"] = "true"
	// 候选优先级: B, A(self), C
	candidates := MasterList()

	elect := func(reachable map[string]bool) string {
		for _, ep := range candidates {
			if IsSelfEndpoint(ep) {
				return ep // 自己始终可达
			}
			if reachable[ep] {
				return ep
			}
		}
		return ""
	}

	// 主节点 B 在线: 代理主为 B, self(A) 不是
	acting := elect(map[string]bool{"10.0.0.2:8899": true, "10.0.0.3:8899": true})
	if acting != "10.0.0.2:8899" {
		t.Fatalf("with primary up, acting want B got %s", acting)
	}
	if IsSelfEndpoint(acting) {
		t.Fatal("self should not be acting while primary up")
	}

	// 主节点 B 宕机: 下一优先级为 self(A), A 自动接管
	acting = elect(map[string]bool{"10.0.0.2:8899": false, "10.0.0.3:8899": true})
	if !IsSelfEndpoint(acting) {
		t.Fatalf("with primary down, self(A) should take over, got %s", acting)
	}
}
