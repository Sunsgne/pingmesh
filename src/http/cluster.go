package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/funcs"
	"github.com/zenlenet/pingmesh/src/g"
)

// 集群容灾相关接口:
//   /api/clusterinfo.json   节点间互探的轻量信息(纪元/模式/是否代理主)
//   /api/cluster/status.json 聚合全集群容灾态势(界面用)
//   /api/cluster/masters.json 配置主节点/备选/自动容灾(管理员)

func configClusterRoutes() {

	// 轻量集群信息: 供其余节点探测纪元与可达性(集群互信鉴权)
	http.HandleFunc("/api/clusterinfo.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		v := g.LocalVersion()
		RenderJson(w, map[string]interface{}{
			"addr":      g.Cfg.Addr,
			"name":      g.Cfg.Name,
			"epoch":     v.Epoch,
			"epochtime": v.Time,
			"mode":      g.Cfg.Mode["Type"],
			"acting":    g.IsActingMaster(),
			"userrev":   g.UserRev(),
		})
	})

	// 用户表同步(节点间): 登录账户密码随主节点。含 bcrypt 哈希,
	// 强制 HMAC 验签 + AES-256-GCM 加密传输, 不接受 IP 互信明文访问。
	http.HandleFunc("/api/users.json", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if !g.VerifySign(r.URL.Path, r.FormValue("ts"), r.FormValue("nonce"), r.FormValue("sign")) {
			deny(w)
			return
		}
		users, err := g.ListUsersFull()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"rev":   g.UserRev(),
			"users": users,
		})
		enc, err := g.EncryptPayload(payload, g.Cfg.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		fmt.Fprintln(w, enc)
	})

	// 聚合集群态势(界面用): 探测全网节点的在线/纪元/主从角色
	http.HandleFunc("/api/cluster/status.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		RenderJson(w, funcs.ClusterStatus())
	})

	// 配置主节点策略: 指定主节点、备选主节点、是否自动容灾(管理员)
	http.HandleFunc("/api/cluster/masters.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthAdmin(r) {
			deny(w)
			return
		}
		r.ParseForm()
		preout := map[string]string{"status": "false"}
		g.CfgLock.Lock()
		if g.Cfg.Mode == nil {
			g.Cfg.Mode = map[string]string{}
		}
		if v := r.FormValue("master"); v != "" {
			g.Cfg.Mode["Master"] = strings.TrimSpace(v)
		}
		if _, ok := r.Form["standbys"]; ok {
			g.Cfg.Mode["Standbys"] = strings.TrimSpace(r.FormValue("standbys"))
		}
		if _, ok := r.Form["masterauto"]; ok {
			if r.FormValue("masterauto") == "true" {
				g.Cfg.Mode["MasterAuto"] = "true"
			} else {
				g.Cfg.Mode["MasterAuto"] = "false"
			}
		}
		g.BumpEpochInPlace(&g.Cfg)
		g.CfgLock.Unlock()
		if err := g.SaveConfig(); err != nil {
			preout["info"] = err.Error()
			RenderJson(w, preout)
			return
		}
		seelog.Info("[func:/api/cluster/masters.json] master policy updated: master=",
			g.Cfg.Mode["Master"], " standbys=", g.Cfg.Mode["Standbys"], " auto=", g.Cfg.Mode["MasterAuto"])
		preout["status"] = "true"
		preout["info"] = "主节点策略已更新, 将在下一同步周期生效"
		RenderJson(w, preout)
	})
}
