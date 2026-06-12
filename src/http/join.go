package http

import (
	"net/http"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/funcs"
	"github.com/zenlenet/pingmesh/src/g"
)

func configJoinRoutes() {

	// 节点接入API: 新节点凭接入令牌(配置密码)自动加入全互联 Pingmesh
	http.HandleFunc("/api/join.json", func(w http.ResponseWriter, r *http.Request) {
		preout := make(map[string]string)
		preout["status"] = "false"
		if r.Method != "POST" {
			preout["info"] = "仅支持POST"
			RenderJson(w, preout)
			return
		}
		r.ParseForm()
		token := r.FormValue("token")
		name := r.FormValue("name")
		addr := r.FormValue("addr")
		group := r.FormValue("group")
		if token == "" || token != g.Cfg.Password {
			seelog.Info("[func:/api/join.json] invalid token from ", r.RemoteAddr)
			preout["info"] = "接入令牌错误"
			RenderJson(w, preout)
			return
		}
		if name == "" {
			preout["info"] = "节点名称不能为空"
			RenderJson(w, preout)
			return
		}
		if !ValidHost(addr) {
			preout["info"] = "非法节点地址(需为IPv4或域名): " + addr
			RenderJson(w, preout)
			return
		}
		g.AddMeshNode(name, addr)
		if group != "" {
			m := g.Cfg.Network[addr]
			m.Group = group
			g.Cfg.Network[addr] = m
		}
		if err := g.SaveConfig(); err != nil {
			preout["info"] = "保存配置失败: " + err.Error()
			RenderJson(w, preout)
			return
		}
		seelog.Info("[func:/api/join.json] node ", name, "(", addr, ") joined the mesh from ", r.RemoteAddr)
		preout["status"] = "true"
		preout["info"] = "节点已加入"
		RenderJson(w, preout)
	})

	// 节点模式切换API: standalone=切回独立/主节点模式, join=加入主节点
	http.HandleFunc("/api/mode.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthAdmin(r) {
			deny(w)
			return
		}
		preout := map[string]string{"status": "false"}
		r.ParseForm()
		switch r.FormValue("action") {
		case "standalone":
			g.CfgLock.Lock()
			if g.Cfg.Mode == nil {
				g.Cfg.Mode = map[string]string{}
			}
			g.Cfg.Mode["Type"] = "local"
			g.Cfg.Mode["Endpoint"] = ""
			g.Cfg.Mode["Status"] = "true"
			g.CfgLock.Unlock()
			if err := g.SaveConfig(); err != nil {
				preout["info"] = err.Error()
				RenderJson(w, preout)
				return
			}
			seelog.Info("[func:/api/mode.json] switched to standalone mode")
			preout["status"] = "true"
			preout["info"] = "已切换为独立/主节点模式, 不再从主节点同步配置"
		case "join":
			master := r.FormValue("master")
			token := r.FormValue("token")
			if master == "" || token == "" {
				preout["info"] = "主节点地址与接入令牌不能为空"
				RenderJson(w, preout)
				return
			}
			if err := funcs.JoinMasterN(master, token, g.Cfg.Name, g.Cfg.Addr, "", 1); err != nil {
				preout["info"] = "加入失败: " + err.Error()
				RenderJson(w, preout)
				return
			}
			seelog.Info("[func:/api/mode.json] joined master ", master)
			preout["status"] = "true"
			preout["info"] = "已加入主节点并同步配置"
		default:
			preout["info"] = "未知操作"
		}
		RenderJson(w, preout)
	})
}
