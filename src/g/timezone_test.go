package g

import (
	"testing"
	"time"
)

func TestInitTimezone(t *testing.T) {
	InitTimezone()
	if time.Local.String() != "Asia/Shanghai" {
		t.Fatalf("local zone = %s, want Asia/Shanghai", time.Local.String())
	}
}
