package g

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 配置冲突处理
// ------------------------------------------------------------------
// 两个节点在同步前各自改了配置(同时保存、或网络分区期间), 只能有一个版本胜出:
//   1. 胜负规则是全序的: 纪元大者胜; 纪元相同比修改时间; 再相同比修改节点地址。
//      各节点因此总会收敛到同一版本, 不会长期分裂。
//   2. 每个版本记录"修改节点"和最近若干个祖先版本(EpochHist), 随配置一起同步。
//      节点采纳远端配置时, 若自己最后一次修改不在对方的祖先里, 说明这次修改被覆盖了:
//      先把本地版本备份到 conf/backups/conflict-*.json, 记入冲突列表, 在集群页提示管理员,
//      由管理员决定重新发布被覆盖的版本(作为新的修改, 再次全网同步)或忽略。

const maxEpochHist = 32

// VersionID 版本标识: 纪元@修改节点
func VersionID(mode map[string]string) string {
	if mode == nil {
		return ""
	}
	return mode["Epoch"] + "@" + mode["EpochBy"]
}

func epochHist(mode map[string]string) []string {
	if mode == nil || mode["EpochHist"] == "" {
		return nil
	}
	return strings.Split(mode["EpochHist"], ",")
}

func appendHist(hist []string, id string) string {
	for _, h := range hist {
		if h == id {
			return strings.Join(hist, ",")
		}
	}
	hist = append(hist, id)
	if len(hist) > maxEpochHist {
		hist = hist[len(hist)-maxEpochHist:]
	}
	return strings.Join(hist, ",")
}

// ConfigConflict 一次被覆盖的本地修改
type ConfigConflict struct {
	ID         string   `json:"id"`
	Time       string   `json:"time"`
	Local      string   `json:"local"`
	LocalTime  string   `json:"localtime"`
	Remote     string   `json:"remote"`
	RemoteTime string   `json:"remotetime"`
	Sections   []string `json:"sections"`
	File       string   `json:"file"`
}

// 节点自身属性与同步状态, 不参与冲突比较
var nodeLocalFields = map[string]bool{"Name": true, "Addr": true, "Ver": true, "Port": true, "Password": true}
var modeVersionKeys = map[string]bool{"Epoch": true, "EpochTime": true, "EpochBy": true, "EpochHist": true, "LastSuccTime": true, "Status": true, "Type": true, "Endpoint": true}

// diffSections 列出两份配置内容不同的顶层配置段
func diffSections(a, b *Config) []string {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	ma, mb := map[string]json.RawMessage{}, map[string]json.RawMessage{}
	json.Unmarshal(ja, &ma)
	json.Unmarshal(jb, &mb)
	keys := map[string]bool{}
	for k := range ma {
		keys[k] = true
	}
	for k := range mb {
		keys[k] = true
	}
	out := []string{}
	for k := range keys {
		if nodeLocalFields[k] {
			continue
		}
		if k == "Mode" {
			strip := func(m map[string]string) string {
				c := map[string]string{}
				for mk, mv := range m {
					if !modeVersionKeys[mk] {
						c[mk] = mv
					}
				}
				j, _ := json.Marshal(c)
				return string(j)
			}
			if strip(a.Mode) != strip(b.Mode) {
				out = append(out, k)
			}
			continue
		}
		if string(ma[k]) != string(mb[k]) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// checkOverwrite 在采纳 remote 前调用(已持 CfgLock): 本节点最后一次修改将被覆盖时备份并记录
func checkOverwrite(local *Config, remote *Config) {
	if local.Mode == nil || remote.Mode == nil {
		return
	}
	localBy := local.Mode["EpochBy"]
	if localBy == "" || !IsSelfEndpoint(localBy) {
		return // 本地版本不是本节点改的, 由修改它的节点负责发现冲突
	}
	if e, _ := strconv.ParseInt(local.Mode["Epoch"], 10, 64); e <= 0 {
		return
	}
	remoteHist := epochHist(remote.Mode)
	if len(remoteHist) == 0 {
		return // 对端是旧版本, 没有祖先记录, 无法判断
	}
	localID := VersionID(local.Mode)
	if VersionID(remote.Mode) == localID {
		return
	}
	for _, h := range remoteHist {
		if h == localID {
			return // 对端版本基于本地修改继续改的, 不算覆盖
		}
	}
	sections := diffSections(local, remote)
	if len(sections) == 0 {
		return
	}
	data, err := json.MarshalIndent(local, "", "\t")
	if err != nil {
		return
	}
	now := time.Now()
	c := ConfigConflict{
		ID:         now.Format("20060102-150405.000"),
		Time:       now.Format("2006-01-02 15:04:05"),
		Local:      localID,
		LocalTime:  local.Mode["EpochTime"],
		Remote:     VersionID(remote.Mode),
		RemoteTime: remote.Mode["EpochTime"],
		Sections:   sections,
	}
	c.File = "conflict-" + c.ID + ".json"
	dir := filepath.Join(Root, "conf", "backups")
	os.MkdirAll(dir, 0700)
	if err := os.WriteFile(filepath.Join(dir, c.File), data, 0600); err != nil {
		return
	}
	conflictMu.Lock()
	list := loadConflicts()
	list = append(list, c)
	if len(list) > 10 {
		list = list[len(list)-10:]
	}
	saveConflicts(list)
	conflictMu.Unlock()
}

var conflictMu sync.Mutex

func conflictsFile() string { return filepath.Join(Root, "conf", "conflicts.json") }

func loadConflicts() []ConfigConflict {
	out := []ConfigConflict{}
	if b, err := os.ReadFile(conflictsFile()); err == nil {
		json.Unmarshal(b, &out)
	}
	return out
}

func saveConflicts(list []ConfigConflict) {
	b, _ := json.MarshalIndent(list, "", "\t")
	writeFileAtomic(conflictsFile(), b, 0600)
}

// ListConflicts 尚未处理的配置冲突(最新在前)
func ListConflicts() []ConfigConflict {
	conflictMu.Lock()
	defer conflictMu.Unlock()
	list := loadConflicts()
	sort.Slice(list, func(i, j int) bool { return list[i].ID > list[j].ID })
	return list
}

// DismissConflict 从列表移除(备份文件保留)
func DismissConflict(id string) {
	conflictMu.Lock()
	defer conflictMu.Unlock()
	list := loadConflicts()
	out := list[:0]
	for _, c := range list {
		if c.ID != id {
			out = append(out, c)
		}
	}
	saveConflicts(out)
}

// RestoreConflict 把被覆盖的本地版本作为一次新的修改重新发布(基于当前版本, 全网再次同步)
func RestoreConflict(id string) error {
	var target *ConfigConflict
	for _, c := range ListConflicts() {
		if c.ID == id {
			cc := c
			target = &cc
			break
		}
	}
	if target == nil {
		return errors.New("冲突记录不存在")
	}
	data, err := os.ReadFile(filepath.Join(Root, "conf", "backups", target.File))
	if err != nil {
		return err
	}
	snap := Config{}
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if len(snap.Network) == 0 {
		return errors.New("备份内容不完整")
	}
	CfgLock.Lock()
	snap.Name, snap.Addr, snap.Ver, snap.Port, snap.Password = Cfg.Name, Cfg.Addr, Cfg.Ver, Cfg.Port, Cfg.Password
	mode := map[string]string{}
	for k, v := range snap.Mode {
		mode[k] = v
	}
	for k := range modeVersionKeys {
		if v, ok := Cfg.Mode[k]; ok {
			mode[k] = v
		} else {
			delete(mode, k)
		}
	}
	snap.Mode = mode
	BumpEpochInPlace(&snap)
	Cfg = snap
	SelfCfg = Cfg.Network[Cfg.Addr]
	CfgLock.Unlock()
	if err := SaveConfig(); err != nil {
		return err
	}
	DismissConflict(id)
	return nil
}
