package http

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCsrfGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := csrfGuard(ok)
	cases := []struct {
		method, url string
		hdr         map[string]string
		want        int
	}{
		{"GET", "/api/user/save.json?username=eve&role=admin", nil, 405},
		{"POST", "/api/user/save.json", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"POST", "/api/user/save.json", map[string]string{"Origin": "https://evil.example"}, 403},
		{"POST", "/api/user/save.json", map[string]string{"Origin": "https://pm.example", "Sec-Fetch-Site": "same-origin"}, 200},
		{"GET", "/api/mute.json?action=add&target=x", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"GET", "/api/mute.json?action=list", map[string]string{"Sec-Fetch-Site": "cross-site"}, 200},
		{"GET", "/api/alertack.json?id=1", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"GET", "/api/alertack.json?id=1", nil, 200}, // 节点间转发不带 Sec-Fetch-Site
		{"GET", "/api/ping.json?ip=1.1.1.1", map[string]string{"Sec-Fetch-Site": "cross-site"}, 200},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, "https://pm.example"+c.url, nil)
		req.Host = "pm.example"
		for k, v := range c.hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s %s %v: got %d want %d", c.method, c.url, c.hdr, rec.Code, c.want)
		}
	}
}

func TestIPTrust(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:5555"
	if ipTrustable(r) {
		t.Error("loopback (nginx) must not be IP-trusted")
	}
	r.Header.Set("X-Real-IP", "203.0.113.9")
	if got := clientIP(r); got != "203.0.113.9" {
		t.Errorf("clientIP via local proxy = %s", got)
	}
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "10.100.1.5:40000"
	r2.Header.Set("X-Real-IP", "1.2.3.4")
	if ipTrustable(r2) {
		t.Error("forwarded headers must disable IP trust")
	}
	if got := clientIP(r2); got != "10.100.1.5" {
		t.Errorf("X-Real-IP must be ignored from non-loopback peers, got %s", got)
	}
	r3 := httptest.NewRequest("GET", "/", nil)
	r3.RemoteAddr = "10.100.1.5:40000"
	if !ipTrustable(r3) {
		t.Error("direct internal peer should be IP-trustable")
	}
}

func TestProxyWritesTo(t *testing.T) {
	for u, want := range map[string]bool{
		"http://10.100.1.5:8899/api/mute.json?action=add&target=x": true,
		"http://10.100.1.5:8899/api/mute.json?action=del&target=x": true,
		"http://10.100.1.5:8899/api/mute.json?action=list":         false,
		"http://10.100.1.5:8899/api/alertack.json?id=3":            true,
		"http://10.100.1.5:8899/api/ping.json?ip=1.1.1.1":          false,
	} {
		if got := proxyWritesTo(u); got != want {
			t.Errorf("%s: got %v want %v", u, got, want)
		}
	}
}

func TestProbeIPBlocked(t *testing.T) {
	for ip, want := range map[string]bool{
		"127.0.0.1": true, "169.254.169.254": true, "0.0.0.0": true, "::1": true,
		"10.100.1.8": false, "8.8.8.8": false,
	} {
		if got := probeIPBlocked(net.ParseIP(ip)); got != want {
			t.Errorf("%s: got %v want %v", ip, got, want)
		}
	}
	if _, err := tcpProbeDial("127.0.0.1:22", 0); err == nil {
		t.Error("tcp probe to loopback must be refused")
	}
}
