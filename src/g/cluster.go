package g

import (
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// 集群容灾(去中心化多主)模型
// ------------------------------------------------------------------
// 设计目标: 主节点宕机后配置不丢、集群可继续读写、并自动收敛。
//
//   - 配置在每个节点本地持久化(原有 cloud 同步即已实现完整副本),
//     因此任意单点(含主节点)宕机都不会丢失配置。
//   - 每条配置带一个单调递增的"纪元"(Epoch)+ 时间戳, 任何权威写入
//     (管理员保存 / 节点加入 / 模式切换)都会自增纪元。
//   - 每个节点周期性探测其余"主候选"节点, 采用 LWW(最后写入胜出):
//     纪元更高者胜出, 纪元相同则时间戳更晚者胜出。节点据此采纳最新配置,
//     无论该改动在哪个节点发起, 都会在全网收敛。
//   - "代理主节点"(Acting Master) = 候选列表中优先级最高且当前可达者,
//     仅用于界面展示与运维指引(提示在哪个节点改配置), 不限制写入,
//     避免主挂时把人锁在门外。
//
// 开启 MasterAuto 后, 所有具备探测能力的 mesh 节点都是主候选, 主挂时
// 下一优先级节点自动成为代理主节点, 真正做到无人值守容灾。

var actingMaster int32 // 1 = 本节点当前为代理主节点(优先级最高且可达)

// SetActingMaster 设置本节点是否为代理主节点(运行期状态, 不持久化)
func SetActingMaster(v bool) {
	if v {
		atomic.StoreInt32(&actingMaster, 1)
	} else {
		atomic.StoreInt32(&actingMaster, 0)
	}
}

// IsActingMaster 本节点当前是否为代理主节点
func IsActingMaster() bool { return atomic.LoadInt32(&actingMaster) == 1 }

// CfgVersion 配置版本(纪元 + 时间戳), 用于 LWW 比较
type CfgVersion struct {
	Epoch int64
	Time  string
}

// Fresher 报告 a 是否严格比 b 更新(纪元优先, 其次时间戳)
func Fresher(a, b CfgVersion) bool {
	if a.Epoch != b.Epoch {
		return a.Epoch > b.Epoch
	}
	return a.Time > b.Time
}

// LocalVersion 当前本地配置版本
func LocalVersion() CfgVersion {
	if Cfg.Mode == nil {
		return CfgVersion{}
	}
	e, _ := strconv.ParseInt(Cfg.Mode["Epoch"], 10, 64)
	return CfgVersion{Epoch: e, Time: Cfg.Mode["EpochTime"]}
}

// GetEpoch 读取当前配置纪元(权威配置版本号)
func GetEpoch() int64 {
	return LocalVersion().Epoch
}

// BumpEpochInPlace 在给定配置上自增纪元并记录时间(权威写入时调用)。
// 取 max(本地纪元, 传入纪元) + 1, 避免并发或回环导致纪元倒退。
func BumpEpochInPlace(c *Config) {
	if c.Mode == nil {
		c.Mode = map[string]string{}
	}
	cur := GetEpoch()
	sub, _ := strconv.ParseInt(c.Mode["Epoch"], 10, 64)
	next := cur
	if sub > next {
		next = sub
	}
	next++
	c.Mode["Epoch"] = strconv.FormatInt(next, 10)
	c.Mode["EpochTime"] = time.Now().Format("2006-01-02 15:04:05")
}

// BumpEpoch 对全局配置自增纪元(调用方在 SaveConfig 前调用)
func BumpEpoch() {
	CfgLock.Lock()
	BumpEpochInPlace(&Cfg)
	CfgLock.Unlock()
}

// SelfEndpoint 本节点对外 host:port
func SelfEndpoint() string {
	return Cfg.Addr + ":" + strconv.Itoa(Cfg.Port)
}

// MasterEndpoint 集群主节点(权威写入节点)的 host:port; 为空表示尚未确立
func MasterEndpoint() string {
	if Cfg.Mode == nil {
		return ""
	}
	return Cfg.Mode["Master"]
}

// MasterAutoEnabled 是否开启自动容灾(全节点皆为主候选)
func MasterAutoEnabled() bool {
	return Cfg.Mode != nil && Cfg.Mode["MasterAuto"] == "true"
}

// IsSelfEndpoint 判断 host:port 是否指向本节点(仅比较主机部分)
func IsSelfEndpoint(ep string) bool {
	ep = strings.TrimSpace(ep)
	if ep == "" {
		return false
	}
	host := ep
	if i := strings.LastIndex(ep, ":"); i >= 0 {
		host = ep[:i]
	}
	return host == Cfg.Addr
}

// ClusterActive 是否已组建集群(需要参与容灾同步与选举)
func ClusterActive() bool {
	if Cfg.Mode == nil {
		return false
	}
	return Cfg.Mode["Type"] == "cloud" ||
		strings.TrimSpace(Cfg.Mode["Master"]) != "" ||
		strings.TrimSpace(Cfg.Mode["Standbys"]) != ""
}

// SetStandbyMasters 由命令行 -masters 设置备选主节点(逗号分隔), 并持久化
func SetStandbyMasters(list string) {
	CfgLock.Lock()
	if Cfg.Mode == nil {
		Cfg.Mode = map[string]string{}
	}
	Cfg.Mode["Standbys"] = strings.TrimSpace(list)
	CfgLock.Unlock()
	SaveConfig()
}

// MasterList 返回有序的主候选列表(host:port), 优先级从高到低:
//  1. 显式主节点 Mode["Master"]
//  2. 手动指定的备选主节点 Mode["Standbys"](逗号分隔)
//  3. 开启 MasterAuto 时, 其余具备探测能力的 mesh 节点按地址排序补入
func MasterList() []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(ep string) {
		ep = strings.TrimSpace(ep)
		if ep == "" || seen[ep] {
			return
		}
		seen[ep] = true
		out = append(out, ep)
	}
	add(MasterEndpoint())
	if Cfg.Mode != nil {
		for _, s := range strings.Split(Cfg.Mode["Standbys"], ",") {
			add(s)
		}
	}
	if MasterAutoEnabled() {
		addrs := []string{}
		for addr, m := range Cfg.Network {
			if m.Pingmesh {
				addrs = append(addrs, addr)
			}
		}
		sort.Strings(addrs)
		for _, addr := range addrs {
			add(addr + ":" + strconv.Itoa(Cfg.Port))
		}
	}
	return out
}
