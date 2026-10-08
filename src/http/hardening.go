package http

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// 仅接受 POST 的状态变更接口(前端全部用 POST 调用)。
// GET 写操作会被跨站顶层跳转利用: SameSite=Lax 的会话 Cookie 仍会随 GET 导航发送。
var postOnlyPaths = map[string]bool{
	"/api/user/save.json":       true,
	"/api/user/delete.json":     true,
	"/api/user/passwd.json":     true,
	"/api/saveconfig.json":      true,
	"/api/alerttest.json":       true,
	"/api/sendmailtest.json":    true,
	"/api/cluster/masters.json": true,
	"/api/mode.json":            true,
}

// 以 GET 参数执行写操作、又需兼容节点间转发的接口: 只拦截浏览器发起的跨站请求
var crossSiteWritePaths = map[string]bool{
	"/api/alertack.json": true,
	"/api/mute.json":     true,
	"/api/logout.json":   true,
}

func isWriteRequest(r *http.Request) bool {
	if postOnlyPaths[r.URL.Path] {
		return true
	}
	if r.URL.Path == "/api/mute.json" {
		a := r.URL.Query().Get("action")
		return a == "add" || a == "del"
	}
	return crossSiteWritePaths[r.URL.Path]
}

// sameOrigin 浏览器请求是否来自本站页面。节点间(服务端)请求不带 Sec-Fetch-Site / Origin, 视为同源。
func sameOrigin(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return false
	}
	if o := r.Header.Get("Origin"); o != "" {
		u, err := url.Parse(o)
		if err != nil || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	return true
}

// csrfGuard 拦截跨站伪造的状态变更请求
func csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if postOnlyPaths[r.URL.Path] && r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if isWriteRequest(r) && !sameOrigin(r) {
			http.Error(w, "Forbidden: cross-site request", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// remoteIP 连接对端 IP(不含端口)
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// viaLocalProxy 请求经本机反向代理(nginx)转入: 对端是回环地址。
// 这类请求的真实来源是公网, 不能按来源 IP 授信。
func viaLocalProxy(r *http.Request) bool {
	ip := net.ParseIP(remoteIP(r))
	return ip != nil && ip.IsLoopback()
}

// ipTrustable 是否允许按来源 IP 做白名单授信: 经反向代理或带转发头的请求一律不信
func ipTrustable(r *http.Request) bool {
	if viaLocalProxy(r) {
		return false
	}
	return r.Header.Get("X-Forwarded-For") == "" && r.Header.Get("X-Real-IP") == ""
}

// clientIP 客户端真实 IP: 仅当请求来自本机反向代理时才采信 X-Real-IP
func clientIP(r *http.Request) string {
	if viaLocalProxy(r) {
		if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(xr) != nil {
			return xr
		}
	}
	return remoteIP(r)
}

/* ---------- 检测工具出站限制 ---------- */

// probeIPBlocked 检测工具禁止连接的地址: 回环、链路本地(含云厂商元数据 169.254.169.254)、组播、未指定。
// 私网(含 10.100.1.0/24 节点内网)保留, 便于排查内网连通性。
func probeIPBlocked(ip net.IP) bool {
	return ip == nil || isUnsafeProbeIP(ip)
}

// probeDialer 在完成 DNS 解析后、建连前校验目标 IP, 同时覆盖重定向与 DNS 重绑定
var probeDialer = &net.Dialer{
	Timeout: 5 * time.Second,
	Control: func(network, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		if probeIPBlocked(net.ParseIP(host)) {
			return fmt.Errorf("禁止探测该地址(%s)", host)
		}
		return nil
	},
}

// probeHTTPClient HTTP 检测用客户端: 出站地址受限, 最多跟随 5 次跳转
func probeHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: probeDialer.DialContext, TLSHandshakeTimeout: 10 * time.Second, DisableKeepAlives: true},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("跳转次数过多")
			}
			return nil
		},
	}
}

// tcpProbeDial TCP 检测建连(同样受出站地址限制)
func tcpProbeDial(hostport string, timeout time.Duration) (net.Conn, error) {
	d := *probeDialer
	d.Timeout = timeout
	return d.Dial("tcp", hostport)
}
