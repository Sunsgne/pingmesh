package http

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/zenlenet/pingmesh/src/g"
)

// 无需登录即可访问的页面
var publicPages = map[string]bool{
	"/login.html": true,
}

func isPageRequest(path string) bool {
	return path == "/" || strings.HasSuffix(path, ".html") || strings.HasSuffix(path, "/")
}

// renderUIDisabled Agent 节点默认关闭 Web 页面时的提示页
func renderUIDisabled(w http.ResponseWriter) {
	master := g.MasterEndpoint()
	link := ""
	if master != "" {
		link = `<p><a href="http://` + master + `/">前往主节点管理页面 http://` + master + ` &rarr;</a></p>`
	}
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprint(w, `<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><title>ZENLENET PingMesh Agent</title></head>
<body style="font-family:-apple-system,'PingFang SC','Microsoft YaHei',sans-serif;background:#0f172a;color:#e2e8f0;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0">
<div style="max-width:560px;padding:36px;background:#1e293b;border-radius:14px;line-height:1.9">
<h2 style="margin-top:0">本节点为 PingMesh Agent</h2>
<p>出于安全考虑，Agent 节点的 Web 管理页面<b>默认关闭</b>，监控数据与集群同步不受影响。</p>
`+link+`
<p style="color:#94a3b8;font-size:13px">如需开启本节点页面：启动参数追加 <code style="background:#0f172a;padding:2px 6px;border-radius:4px">-webui</code>
（systemd 部署可编辑 <code style="background:#0f172a;padding:2px 6px;border-radius:4px">pingmesh.env</code> 后重启服务）。<br>
主节点故障时本节点若自动接管为代理主节点，页面会自动开放。</p>
</div></body></html>`)
}

func configIndexRoutes() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Agent 默认关闭 Web 页面(-webui 开启; 容灾接管期间自动开放)
		if !g.WebUIEnabled() {
			renderUIDisabled(w)
			return
		}
		// 静态页面访问控制: 未登录跳转到登录页
		if isPageRequest(r.URL.Path) && !publicPages[r.URL.Path] {
			if !AuthUser(r) {
				http.Redirect(w, r, "/login.html", http.StatusFound)
				return
			}
		}
		if strings.HasSuffix(r.URL.Path, "/") {
			if !g.IsExist(filepath.Join(g.Root, "/html", r.URL.Path, "index.html")) {
				http.NotFound(w, r)
				return
			}
		}
		http.FileServer(http.Dir(filepath.Join(g.Root, "/html"))).ServeHTTP(w, r)
	})

}
