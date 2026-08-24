package funcs

import "testing"

func TestClassifyAlert(t *testing.T) {
	rule := map[string]string{
		"Thdavgdelay": "100",
		"Thdloss":     "1",
		"Thdchecksec": "60",
		"Thdoccnum":   "1",
	}
	typ, reason, tag := ClassifyAlert("10.100.1.15", "10.100.1.18", rule, 50, 5, 0)
	if typ != "loss" {
		t.Fatalf("type=%s want loss", typ)
	}
	if tag != "LOSS:10.100.1.15→10.100.1.18" {
		t.Fatalf("tag=%s", tag)
	}
	if reason == "" || !containsAll(reason, "丢包", "5%", "1%") {
		t.Fatalf("reason=%s", reason)
	}

	typ, _, tag = ClassifyAlert("10.100.1.1", "10.100.1.7", rule, 200, 0, 0)
	if typ != "delay" || tag != "DELAY:10.100.1.1→10.100.1.7" {
		t.Fatalf("delay typ=%s tag=%s", typ, tag)
	}

	typ, _, _ = ClassifyAlert("a", "b", rule, 200, 8, 0)
	if typ != "delay+loss" {
		t.Fatalf("combo typ=%s", typ)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !containsStr(s, p) {
			return false
		}
	}
	return true
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
