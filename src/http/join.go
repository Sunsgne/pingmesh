package http

import (
	"net/http"

	"github.com/cihub/seelog"
	"github.com/smartping/smartping/src/g"
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
		if !ValidIP4(addr) {
			preout["info"] = "非法节点IP: " + addr
			RenderJson(w, preout)
			return
		}
		g.AddMeshNode(name, addr)
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
}
