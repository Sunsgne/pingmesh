package http

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/zenlenet/pingmesh/src/g"
)

// 浏览器通过 proxy 可访问的只读/运维 API 白名单(主机必须是本集群节点)。
var proxyPathAllow = map[string]bool{
	"/api/config.json":      true,
	"/api/ping.json":        true,
	"/api/pingmesh.json":    true,
	"/api/topology.json":    true,
	"/api/alert.json":       true,
	"/api/alerthealth.json": true,
	"/api/mapping.json":     true,
	"/api/mute.json":        true,
	"/api/alertack.json":    true,
	"/api/alertdiag.json":   true,
	"/healthz":              true,
}

// proxyAllowed 校验代理目标: 仅 http(s)、无 userinfo、主机为本集群节点、路径在白名单。
func proxyAllowed(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("空 URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("URL 非法")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("仅允许 http/https")
	}
	if u.User != nil {
		return fmt.Errorf("不允许带凭据的 URL")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("缺少主机")
	}
	if !isMeshHost(host) {
		return fmt.Errorf("目标不在本集群节点列表")
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	// 去掉路径末尾多余斜杠再匹配基础 API
	base := path
	if i := strings.Index(base, "?"); i >= 0 {
		base = base[:i]
	}
	if !proxyPathAllow[base] {
		return fmt.Errorf("不允许代理该 API: %s", base)
	}
	return nil
}

func isMeshHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return true
	}
	if g.Cfg.Addr != "" && strings.EqualFold(host, g.Cfg.Addr) {
		return true
	}
	if g.AuthAgentIpMap != nil && g.AuthAgentIpMap[host] {
		return true
	}
	for _, m := range g.Cfg.Network {
		if m.Addr != "" && strings.EqualFold(host, m.Addr) {
			return true
		}
	}
	return false
}

// isUnsafeProbeIP 拒绝探测链路本地/回环/元数据等敏感地址(HTTP 工具用)
func isUnsafeProbeIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// AWS/GCP/Azure 等常见 metadata
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}

// httpProbeAllowed 检测工具 HTTP 探测: 解析主机并拒绝明显危险目标
func httpProbeAllowed(raw string) error {
	u := strings.TrimSpace(raw)
	if u == "" {
		return fmt.Errorf("目标为空")
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "http://" + u
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("URL 非法")
	}
	if parsed.User != nil {
		return fmt.Errorf("不允许带凭据的 URL")
	}
	host := parsed.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("无法解析主机")
	}
	for _, ip := range ips {
		if isUnsafeProbeIP(ip) {
			return fmt.Errorf("禁止探测该地址(%s)", ip.String())
		}
	}
	return nil
}

// maskConfigSecrets 对非管理员、非加密同步的配置副本脱敏
func maskConfigSecrets(nconf *g.Config) {
	if nconf.Alert != nil && nconf.Alert["SendEmailPassword"] != "" {
		nconf.Alert["SendEmailPassword"] = "samepasswordasbefore"
	}
	if nconf.OAuth != nil && nconf.OAuth["ClientSecret"] != "" {
		nconf.OAuth["ClientSecret"] = "samepasswordasbefore"
	}
	for i := range nconf.Channels {
		p := nconf.Channels[i].Params
		if p == nil {
			continue
		}
		for _, k := range []string{"Token", "Webhook", "Url", "Secret", "AccessToken", "AppSecret"} {
			if p[k] != "" {
				p[k] = "samepasswordasbefore"
			}
		}
	}
}
