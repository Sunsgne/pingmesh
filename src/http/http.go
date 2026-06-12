package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

func ValidIP4(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")
	re, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	if re.MatchString(ipAddress) {
		return true
	}
	return false
}

var hostRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)+$`)

// ValidHost 合法的 IPv4 或域名(节点地址支持域名, 探测时自动解析)
func ValidHost(addr string) bool {
	addr = strings.Trim(addr, " ")
	return ValidIP4(addr) || hostRe.MatchString(addr)
}

func RenderJson(w http.ResponseWriter, v interface{}) {
	bs, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(bs)
}

func AuthUserIp(RemoteAddr string) bool {
	if len(g.AuthUserIpMap) == 0 {
		return true
	}
	ips := strings.Split(RemoteAddr, ":")
	if len(ips) == 2 {
		if _, ok := g.AuthUserIpMap[ips[0]]; ok {
			return true
		}
	}
	return false
}

func AuthAgentIp(RemoteAddr string, drt bool) bool {
	if drt {
		if len(g.AuthUserIpMap) == 0 {
			return true
		}
	}
	if len(g.AuthAgentIpMap) == 0 {
		return true
	}
	ips := strings.Split(RemoteAddr, ":")
	if len(ips) == 2 {
		if _, ok := g.AuthAgentIpMap[ips[0]]; ok {
			return true
		}
	}
	return false
}

// securityHeaders 统一注入安全响应头(防 MIME 嗅探/点击劫持/Referer 泄露),
// 所有静态资源均为同源, 因此 CSP 仅放行 self + 必要的内联脚本/样式。
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; connect-src 'self'; font-src 'self' data:; "+
				"media-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'self'")
		next.ServeHTTP(w, r)
	})
}

// configHealthRoutes 注册无需鉴权的健康检查端点, 便于负载均衡/容器探活。
func configHealthRoutes() {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		acting := "false"
		if g.IsActingMaster() {
			acting = "true"
		}
		RenderJson(w, map[string]interface{}{
			"status":  "ok",
			"version": g.Cfg.Ver,
			"node":    g.Cfg.Name,
			"addr":    g.Cfg.Addr,
			"mode":    g.Cfg.Mode["Type"],
			"epoch":   g.GetEpoch(),
			"master":  acting,
			"time":    time.Now().Format("2006-01-02 15:04:05"),
		})
	})
}

func StartHttp() {
	configAuthRoutes()
	configApiRoutes()
	configPingmeshRoutes()
	configJoinRoutes()
	configOpsRoutes()
	configToolsRoutes()
	configClusterRoutes()
	configHealthRoutes()
	configIndexRoutes()
	s := g.FlagListen
	if s == "" {
		s = fmt.Sprintf(":%d", g.Cfg.Port)
	}

	srv := &http.Server{
		Addr:    s,
		Handler: securityHeaders(http.DefaultServeMux),
		// 防慢速攻击: 限制请求头读取时间; 节点间探测/MTR 可能较慢, 故不设过紧的总写超时
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 优雅停机: 收到 SIGINT/SIGTERM 时停止接收新连接并放行在途请求
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		seelog.Info("[func:StartHttp] shutting down gracefully ...")
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	seelog.Info("[func:StartHttp] starting to listen on ", s)
	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		seelog.Error("[func:StartHttp] ", err)
		seelog.Flush()
		os.Exit(1)
	}
	seelog.Flush()
	os.Exit(0)
}
