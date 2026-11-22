package funcs

import "testing"

func TestNodeDisplayNamePrefersTable(t *testing.T) {
	if got := nodeDisplayName("HKG-EQHK1", "hkg1"); got != "HKG-EQHK1" {
		t.Fatalf("table name lost, got %q", got)
	}
	if got := nodeDisplayName("  HKG-EQHK1  ", "hkg1"); got != "HKG-EQHK1" {
		t.Fatalf("table name not trimmed, got %q", got)
	}
	if got := nodeDisplayName("", "hkg1"); got != "hkg1" {
		t.Fatalf("missing table name should fall back, got %q", got)
	}
	if got := nodeDisplayName("   ", "SIN-EQSG2"); got != "SIN-EQSG2" {
		t.Fatalf("blank table name should fall back, got %q", got)
	}
}

func TestCloudEndpointHostPort(t *testing.T) {
	cases := map[string]string{
		"http://10.100.1.8:8899/api/config.json": "10.100.1.8:8899",
		"10.100.1.8:8899":                        "10.100.1.8:8899",
		"":                                       "",
	}
	for in, want := range cases {
		if got := cloudEndpointHostPort(in); got != want {
			t.Fatalf("cloudEndpointHostPort(%q) = %q, want %q", in, got, want)
		}
	}
}
