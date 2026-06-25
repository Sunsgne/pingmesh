package funcs

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// clusterPeerInfo 主候选节点对外暴露的轻量集群信息(/api/clusterinfo.json)
type clusterPeerInfo struct {
	Endpoint  string `json:"-"`
	Addr      string `json:"addr"`
	Name      string `json:"name"`
	Epoch     int64  `json:"epoch"`
	EpochTime string `json:"epochtime"`
	Mode      string `json:"mode"`
	Acting    bool   `json:"acting"`
	UserRev   int64  `json:"userrev"` // 用户数据版本(账户密码随主同步的 LWW 依据)
	// Legacy: 节点 HTTP 可达但不支持集群接口(旧版本二进制/未互信),
	// 滚动升级期间视为"在线但不参与容灾", 避免误判离线触发接管。
	Legacy bool `json:"-"`
}

// probeClusterInfo 拉取某候选节点的集群信息(带 HMAC 签名, 短超时)。
// 返回值 ok 表示节点 HTTP 可达; 旧版本节点(404/鉴权失败/响应不可解析)
// 标记 Legacy=true, 不作为配置同步源, 但参与"在线"判定。
func probeClusterInfo(endpoint string, timeout time.Duration) (clusterPeerInfo, bool) {
	info := clusterPeerInfo{Endpoint: endpoint}
	url := "http://" + endpoint + "/api/clusterinfo.json"
	client := http.Client{Timeout: timeout}
	resp, err := client.Get(g.SignURL(url, g.Cfg.Password))
	if err != nil {
		return info, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != 200 {
		info.Legacy = true
		return info, true
	}
	if err := json.Unmarshal(body, &info); err != nil {
		info.Legacy = true
		return info, true
	}
	info.Endpoint = endpoint
	return info, true
}

// ClusterSync 周期性集群容灾同步(每分钟由 cron 触发):
//  1. 探测所有"其他"主候选节点的纪元与可达性
//  2. 选出代理主节点(优先级最高且可达者), 仅用于展示与运维指引
//  3. 采用 LWW: 从纪元最新的可达候选采纳配置, 实现全网收敛与主挂自动接管
func ClusterSync() {
	candidates := g.MasterList()
	// 拆出"其他"候选(排除自己)
	others := make([]string, 0, len(candidates))
	for _, ep := range candidates {
		if !g.IsSelfEndpoint(ep) {
			others = append(others, ep)
		}
	}
	// 无候选且非 cloud 模式: 单机独立运行, 本节点即权威主节点
	if len(others) == 0 {
		if g.Cfg.Mode["Type"] == "cloud" && g.Cfg.Mode["Endpoint"] != "" {
			// cloud 模式但候选列表只有自己(配置尚未带 Master 列表): 退回原始 Endpoint 同步
			syncFromEndpoint(g.Cfg.Mode["Endpoint"])
		}
		g.SetActingMaster(true)
		return
	}

	// 并发探测其他候选(限流, 避免大集群瞬时压力)
	infos := probeAll(others, 4*time.Second)

	// 选举代理主节点: 候选列表中优先级最高且可达者(自己始终视为可达)
	acting := ""
	for _, ep := range candidates {
		if g.IsSelfEndpoint(ep) {
			acting = ep
			break
		}
		if _, ok := infos[ep]; ok {
			acting = ep
			break
		}
	}
	selfActing := acting == "" || g.IsSelfEndpoint(acting)
	g.SetActingMaster(selfActing)

	// LWW: 在可达候选中挑选最新配置源(旧版本节点无纪元概念, 不作为同步源)
	local := g.LocalVersion()
	bestEp := ""
	best := local
	for ep, info := range infos {
		if info.Legacy {
			continue
		}
		v := g.CfgVersion{Epoch: info.Epoch, Time: info.EpochTime}
		if g.Fresher(v, best) {
			best = v
			bestEp = ep
		}
	}
	if bestEp != "" {
		seelog.Info("[func:ClusterSync] adopting fresher config from ", bestEp,
			" (epoch ", best.Epoch, " > local ", local.Epoch, ")")
		syncFromEndpoint("http://" + bestEp + "/api/config.json")
	} else if selfActing {
		seelog.Debug("[func:ClusterSync] self is acting master, config is authoritative (epoch ", local.Epoch, ")")
	} else {
		seelog.Debug("[func:ClusterSync] following acting master ", acting, ", local config up to date (epoch ", local.Epoch, ")")
	}

	// 用户账户随主同步(LWW): 可达候选中用户版本最高者胜出,
	// 主节点改密码/增删用户后, 全部 Agent 在下一周期跟随生效。
	localURev := g.UserRev()
	bestURev, bestUEp := localURev, ""
	for ep, info := range infos {
		if info.Legacy {
			continue
		}
		if info.UserRev > bestURev {
			bestURev = info.UserRev
			bestUEp = ep
		}
	}
	if bestUEp != "" {
		seelog.Info("[func:ClusterSync] adopting newer user table from ", bestUEp,
			" (rev ", bestURev, " > local ", localURev, ")")
		SyncUsersFrom(bestUEp, false)
	}
}

// SyncUsersFrom 从指定节点拉取完整用户表(签名请求 + AES-256-GCM 加密传输)。
// force=true 时无视版本号直接采纳(节点初次 join 时以主节点为准)。
func SyncUsersFrom(endpoint string, force bool) error {
	url := "http://" + endpoint + "/api/users.json"
	client := http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get(g.SignURL(url, g.Cfg.Password))
	if err != nil {
		seelog.Error("[func:SyncUsersFrom] pull users from ", endpoint, " failed: ", err)
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		seelog.Error("[func:SyncUsersFrom] ", endpoint, " returned HTTP ", resp.StatusCode)
		return errors.New("users sync: unexpected status")
	}
	// 用户哈希属敏感数据: 仅接受加密载荷
	if !g.IsEncryptedPayload(body) {
		seelog.Error("[func:SyncUsersFrom] ", endpoint, " returned non-encrypted payload, reject")
		return errors.New("users sync: payload not encrypted")
	}
	plain, err := g.DecryptPayload(body, g.Cfg.Password)
	if err != nil {
		seelog.Error("[func:SyncUsersFrom] decrypt failed (token mismatch?): ", err)
		return err
	}
	var out struct {
		Rev   int64        `json:"rev"`
		Users []g.UserFull `json:"users"`
	}
	if err := json.Unmarshal(plain, &out); err != nil {
		return err
	}
	if !force && out.Rev <= g.UserRev() {
		return nil
	}
	if force && out.Rev < 1 {
		out.Rev = 1
	}
	return g.ReplaceUsers(out.Users, out.Rev)
}

// probeAll 并发探测候选节点, 返回可达者的信息(endpoint -> info)
func probeAll(eps []string, timeout time.Duration) map[string]clusterPeerInfo {
	out := map[string]clusterPeerInfo{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for _, ep := range eps {
		wg.Add(1)
		sem <- struct{}{}
		go func(endpoint string) {
			defer func() { <-sem; wg.Done() }()
			if info, ok := probeClusterInfo(endpoint, timeout); ok {
				mu.Lock()
				out[endpoint] = info
				mu.Unlock()
			}
		}(ep)
	}
	wg.Wait()
	return out
}

// syncFromEndpoint 从指定 /api/config.json 拉取并落盘配置(沿用加密同步通道)
func syncFromEndpoint(endpoint string) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return
	}
	if _, err := g.SaveCloudConfig(endpoint); err != nil {
		seelog.Error("[func:syncFromEndpoint] pull config from ", endpoint, " failed: ", err)
		return
	}
	if err := g.SaveConfig(); err != nil {
		seelog.Error("[func:syncFromEndpoint] save synced config failed: ", err)
		return
	}
	seelog.Info("[func:syncFromEndpoint] config synced from ", endpoint)
}

// StartCloudMonitor 兼容旧入口: 统一走集群容灾同步
func StartCloudMonitor() {
	ClusterSync()
}

// ClusterNode 集群状态视图中的单个节点(供界面渲染容灾态势)
type ClusterNode struct {
	Endpoint  string `json:"endpoint"`
	Addr      string `json:"addr"`
	Name      string `json:"name"`
	Epoch     int64  `json:"epoch"`
	EpochTime string `json:"epochtime"`
	Mode      string `json:"mode"`
	Online    bool   `json:"online"`
	Self      bool   `json:"self"`
	Acting    bool   `json:"acting"`    // 该节点自报为代理主节点
	Candidate bool   `json:"candidate"` // 是否为主候选
	Priority  int    `json:"priority"`  // 候选优先级(越小越高), -1 表示非候选
	Legacy    bool   `json:"legacy"`    // 在线但运行旧版本(不支持集群接口), 升级后消失
}

// ClusterStatus 聚合全集群容灾态势: 本节点 + 所有主候选/ mesh 节点的可达性与纪元。
// 供「集群容灾」页面展示当前主节点、备选、各节点在线与配置版本是否收敛。
func ClusterStatus() map[string]interface{} {
	candidates := g.MasterList()
	prio := map[string]int{}
	for i, ep := range candidates {
		if _, ok := prio[ep]; !ok {
			prio[ep] = i
		}
	}
	// 待探测目标 = 候选 ∪ 所有 mesh 节点(去重, 排除自己)
	targetSet := map[string]bool{}
	for _, ep := range candidates {
		if !g.IsSelfEndpoint(ep) {
			targetSet[ep] = true
		}
	}
	for addr, m := range g.Cfg.Network {
		ep := addr + ":" + itoa(g.Cfg.Port)
		if m.Pingmesh && !g.IsSelfEndpoint(ep) {
			targetSet[ep] = true
		}
	}
	targets := make([]string, 0, len(targetSet))
	for ep := range targetSet {
		targets = append(targets, ep)
	}
	// 界面查询用较短超时, 保证页面秒级可用(离线节点不拖慢整页)
	infos := probeAll(targets, 2*time.Second)

	nodes := []ClusterNode{}
	// 本节点
	local := g.LocalVersion()
	selfEp := g.SelfEndpoint()
	selfPrio := -1
	if p, ok := prio[selfEp]; ok {
		selfPrio = p
	}
	nodes = append(nodes, ClusterNode{
		Endpoint: selfEp, Addr: g.Cfg.Addr, Name: g.Cfg.Name,
		Epoch: local.Epoch, EpochTime: local.Time, Mode: g.Cfg.Mode["Type"],
		Online: true, Self: true, Acting: g.IsActingMaster(),
		Candidate: selfPrio >= 0, Priority: selfPrio,
	})
	// 其他节点
	for _, ep := range targets {
		n := ClusterNode{Endpoint: ep, Online: false, Priority: -1}
		if p, ok := prio[ep]; ok {
			n.Candidate = true
			n.Priority = p
		}
		host := ep
		if i := strings.LastIndex(ep, ":"); i >= 0 {
			host = ep[:i]
		}
		n.Addr = host
		if m, ok := g.Cfg.Network[host]; ok {
			n.Name = m.Name
		}
		if info, ok := infos[ep]; ok {
			n.Online = true
			n.Legacy = info.Legacy
			n.Epoch = info.Epoch
			n.EpochTime = info.EpochTime
			n.Mode = info.Mode
			n.Acting = info.Acting
			if info.Name != "" {
				n.Name = info.Name
			}
		}
		nodes = append(nodes, n)
	}

	// 推导集群层面的代理主节点(优先级最高且在线者)
	actingEp := ""
	for _, ep := range candidates {
		if g.IsSelfEndpoint(ep) {
			actingEp = ep
			break
		}
		if _, ok := infos[ep]; ok {
			actingEp = ep
			break
		}
	}
	// 收敛判定: 在线节点纪元是否一致(旧版本节点无纪元概念, 不参与判定)
	maxEpoch := local.Epoch
	converged := true
	first := true
	var firstEpoch int64
	for _, n := range nodes {
		if !n.Online || n.Legacy {
			continue
		}
		if n.Epoch > maxEpoch {
			maxEpoch = n.Epoch
		}
		if first {
			firstEpoch = n.Epoch
			first = false
		} else if n.Epoch != firstEpoch {
			converged = false
		}
	}

	return map[string]interface{}{
		"status":     "true",
		"self":       selfEp,
		"acting":     actingEp,
		"masterauto": g.MasterAutoEnabled(),
		"primary":    g.MasterEndpoint(),
		"standbys":   g.Cfg.Mode["Standbys"],
		"maxepoch":   maxEpoch,
		"converged":  converged,
		"nodes":      nodes,
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
