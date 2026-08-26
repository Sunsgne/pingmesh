package http

import (
	"net"
	"testing"
)

func TestIsUnsafeProbeIP(t *testing.T) {
	cases := []struct {
		ip     string
		unsafe bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"169.254.169.254", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"10.100.1.8", false},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if got := isUnsafeProbeIP(ip); got != c.unsafe {
			t.Fatalf("%s: got %v want %v", c.ip, got, c.unsafe)
		}
	}
}

func TestProxyPathAllow(t *testing.T) {
	if !proxyPathAllow["/api/pingmesh.json"] {
		t.Fatal("pingmesh should be allowed")
	}
	if proxyPathAllow["/api/saveconfig.json"] {
		t.Fatal("saveconfig must not be proxyable")
	}
}
