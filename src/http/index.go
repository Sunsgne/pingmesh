package http

import (
	"github.com/zenlenet/pingmesh/src/g"
	"net/http"
	"path/filepath"
	"strings"
)

// 无需登录即可访问的页面
var publicPages = map[string]bool{
	"/login.html": true,
}

func isPageRequest(path string) bool {
	return path == "/" || strings.HasSuffix(path, ".html") || strings.HasSuffix(path, "/")
}

func configIndexRoutes() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
