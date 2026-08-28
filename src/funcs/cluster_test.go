package funcs

import "testing"

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
