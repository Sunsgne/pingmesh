package http

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"github.com/zenlenet/pingmesh/src/g"
	"github.com/zenlenet/pingmesh/src/nettools"
)

// 检测工具: 支持主流探测手段 icmp / tcp / http / mtr / dns

type httpProbeRes struct {
	Code      int     `json:"code"`
	DnsMs     float64 `json:"dnsms"`
	ConnectMs float64 `json:"connectms"`
	TlsMs     float64 `json:"tlsms"`
	TtfbMs    float64 `json:"ttfbms"`
	TotalMs   float64 `json:"totalms"`
	Size      int64   `json:"size"`
	FinalUrl  string  `json:"finalurl"`
}

type dnsProbeRes struct {
	Ips []string `json:"ips"`
	Ms  float64  `json:"ms"`
}

func configToolsRoutes() {

	http.HandleFunc("/api/tools.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		out := map[string]interface{}{"status": "false"}
		target := strings.TrimSpace(r.FormValue("t"))
		ttype := r.FormValue("type")
		if ttype == "" {
			ttype = "icmp"
		}
		out["type"] = ttype
		if target == "" {
			out["error"] = "target empty!"
			RenderJson(w, out)
			return
		}
		// 频率限制
		nowtime := int(time.Now().Unix())
		g.ToolLimitLock.Lock()
		if last, ok := g.ToolLimit[r.RemoteAddr]; ok && (nowtime-last) <= g.Cfg.Toollimit {
			g.ToolLimitLock.Unlock()
			out["error"] = "Time Limit Exceeded!"
			RenderJson(w, out)
			return
		}
		g.ToolLimit[r.RemoteAddr] = nowtime
		g.ToolLimitLock.Unlock()

		switch ttype {
		case "icmp":
			toolIcmp(target, out)
		case "tcp":
			toolTcp(target, out)
		case "http":
			toolHttp(target, out)
		case "mtr":
			toolMtr(target, out)
		case "dns":
			toolDns(target, out)
		default:
			out["error"] = "未知探测类型: " + ttype
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, out)
	})

	// 可达状态: 对单个 IP/域名做轻量 ICMP 探测, 供节点管理/全球延迟列表展示。
	// 结果短时缓存(避免列表重渲染时反复探测), 不受检测工具限频(Toollimit)影响。
	http.HandleFunc("/api/reach.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		host := stripScheme(strings.TrimSpace(r.FormValue("ip")))
		if host == "" {
			renderErr(w, "参数错误: 需要 ip")
			return
		}
		RenderJson(w, reachCheck(host))
	})
}

/* ---------- 可达状态探测(短缓存) ---------- */

type reachEntry struct {
	ok        bool // 是否成功解析
	ip        string
	reachable bool
	rtt       float64
	loss      int
	at        time.Time
}

var (
	reachCache = map[string]reachEntry{}
	reachMu    sync.Mutex
)

const reachTTL = 12 * time.Second

func reachCheck(host string) map[string]interface{} {
	reachMu.Lock()
	if c, ok := reachCache[host]; ok && time.Since(c.at) < reachTTL {
		reachMu.Unlock()
		return reachOut(c)
	}
	reachMu.Unlock()

	e := reachEntry{at: time.Now()}
	ipaddr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		reachMu.Lock()
		reachCache[host] = e
		reachMu.Unlock()
		return map[string]interface{}{"status": "false", "info": "无法解析: " + host}
	}
	e.ok = true
	e.ip = ipaddr.String()
	recv, sum := 0, 0.0
	const probes = 3
	for i := 0; i < probes; i++ {
		d, perr := nettools.RunPingSize(ipaddr, 2*time.Second, 56)
		if perr == nil {
			recv++
			sum += d
		}
		time.Sleep(80 * time.Millisecond)
	}
	e.reachable = recv > 0
	if recv > 0 {
		e.rtt = sum / float64(recv)
	}
	e.loss = int(float64(probes-recv) / float64(probes) * 100)
	reachMu.Lock()
	reachCache[host] = e
	reachMu.Unlock()
	return reachOut(e)
}

func reachOut(e reachEntry) map[string]interface{} {
	return map[string]interface{}{
		"status":    "true",
		"ip":        e.ip,
		"reachable": e.reachable,
		"rtt":       e.rtt,
		"loss":      e.loss,
	}
}

func stripScheme(t string) string {
	t = strings.Replace(t, "https://", "", 1)
	t = strings.Replace(t, "http://", "", 1)
	if i := strings.Index(t, "/"); i > 0 {
		t = t[:i]
	}
	return t
}

/* ---------- ICMP ---------- */

func toolIcmp(target string, out map[string]interface{}) {
	host := stripScheme(target)
	ipaddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		out["error"] = "Unable to resolve destination host"
		return
	}
	out["ip"] = ipaddr.String()
	stat := g.PingSt{}
	stat.MinDelay = -1
	loss := 0
	var wg sync.WaitGroup
	ch := make(chan float64, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			delay, err := nettools.RunPingSize(ipaddr, 5*time.Second, 56)
			if err != nil {
				ch <- -1.0
			} else {
				ch <- delay
			}
		}()
		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
	close(ch)
	for delay := range ch {
		if delay >= 0 {
			stat.AvgDelay += delay
			if stat.MaxDelay < delay {
				stat.MaxDelay = delay
			}
			if stat.MinDelay == -1 || stat.MinDelay > delay {
				stat.MinDelay = delay
			}
			stat.RevcPk++
		} else {
			loss++
		}
		stat.SendPk++
	}
	stat.LossPk = int(float64(loss) / float64(stat.SendPk) * 100)
	if stat.RevcPk > 0 {
		stat.AvgDelay = stat.AvgDelay / float64(stat.RevcPk)
	} else {
		stat.AvgDelay, stat.MinDelay, stat.MaxDelay = 0, 0, 0
	}
	out["ping"] = stat
	out["status"] = "true"
}

/* ---------- TCP Ping ---------- */

func toolTcp(target string, out map[string]interface{}) {
	hostport := stripScheme(target)
	if !strings.Contains(hostport, ":") {
		hostport += ":80"
	}
	host := strings.Split(hostport, ":")[0]
	if ipaddr, err := net.ResolveIPAddr("ip", host); err == nil {
		out["ip"] = ipaddr.String()
	}
	out["port"] = strings.Split(hostport, ":")[1]
	stat := g.PingSt{}
	stat.MinDelay = -1
	loss := 0
	for i := 0; i < 5; i++ {
		t0 := time.Now()
		conn, err := net.DialTimeout("tcp", hostport, 3*time.Second)
		delay := float64(time.Since(t0).Nanoseconds()) / 1e6
		if err == nil {
			conn.Close()
			stat.AvgDelay += delay
			if stat.MaxDelay < delay {
				stat.MaxDelay = delay
			}
			if stat.MinDelay == -1 || stat.MinDelay > delay {
				stat.MinDelay = delay
			}
			stat.RevcPk++
		} else {
			loss++
		}
		stat.SendPk++
		time.Sleep(100 * time.Millisecond)
	}
	stat.LossPk = int(float64(loss) / float64(stat.SendPk) * 100)
	if stat.RevcPk > 0 {
		stat.AvgDelay = stat.AvgDelay / float64(stat.RevcPk)
	} else {
		stat.AvgDelay, stat.MinDelay, stat.MaxDelay = 0, 0, 0
		out["error"] = "TCP 连接失败(端口不通或被过滤)"
	}
	out["ping"] = stat
	out["status"] = "true"
}

/* ---------- HTTP (curl 式分段计时) ---------- */

func toolHttp(target string, out map[string]interface{}) {
	u := target
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "http://" + u
	}
	res := httpProbeRes{}
	var dnsStart, connStart, tlsStart, start time.Time
	var dnsDone, connDone, tlsDone, firstByte time.Time
	trace := &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:              func(httptrace.DNSDoneInfo) { dnsDone = time.Now() },
		ConnectStart:         func(string, string) { connStart = time.Now() },
		ConnectDone:          func(string, string, error) { connDone = time.Now() },
		TLSHandshakeStart:    func() { tlsStart = time.Now() },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { tlsDone = time.Now() },
		GotFirstResponseByte: func() { firstByte = time.Now() },
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		out["error"] = "URL 非法: " + err.Error()
		return
	}
	req.Header.Set("User-Agent", "ZENLENET-PingMesh/1.0 (probe)")
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	client := http.Client{Timeout: 15 * time.Second}
	start = time.Now()
	resp, err := client.Do(req)
	if err != nil {
		out["error"] = "请求失败: " + err.Error()
		return
	}
	defer resp.Body.Close()
	n, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, 10<<20))
	total := time.Since(start)
	ms := func(a, b time.Time) float64 {
		if a.IsZero() || b.IsZero() || b.Before(a) {
			return 0
		}
		return float64(b.Sub(a).Nanoseconds()) / 1e6
	}
	res.Code = resp.StatusCode
	res.DnsMs = ms(dnsStart, dnsDone)
	res.ConnectMs = ms(connStart, connDone)
	res.TlsMs = ms(tlsStart, tlsDone)
	res.TtfbMs = ms(start, firstByte)
	res.TotalMs = float64(total.Nanoseconds()) / 1e6
	res.Size = n
	res.FinalUrl = resp.Request.URL.String()
	if host := resp.Request.URL.Hostname(); host != "" {
		if ipaddr, err := net.ResolveIPAddr("ip", host); err == nil {
			out["ip"] = ipaddr.String()
		}
	}
	out["http"] = res
	out["status"] = "true"
}

/* ---------- MTR ---------- */

func toolMtr(target string, out map[string]interface{}) {
	host := stripScheme(target)
	ipaddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		out["error"] = "Unable to resolve destination host"
		return
	}
	out["ip"] = ipaddr.String()
	hops, err := nettools.RunMtr(ipaddr.String(), 800*time.Millisecond, 30, 5)
	if err != nil {
		out["error"] = "MTR 失败: " + err.Error()
		return
	}
	out["mtr"] = hops
	out["status"] = "true"
}

/* ---------- DNS ---------- */

func toolDns(target string, out map[string]interface{}) {
	host := stripScheme(target)
	t0 := time.Now()
	ips, err := net.LookupHost(host)
	elapsed := float64(time.Since(t0).Nanoseconds()) / 1e6
	if err != nil {
		out["error"] = "解析失败: " + err.Error()
		return
	}
	out["dns"] = dnsProbeRes{Ips: ips, Ms: elapsed}
	if len(ips) > 0 {
		out["ip"] = ips[0]
	}
	out["status"] = "true"
}
