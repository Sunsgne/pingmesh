package g

import (
	"os"
	"time"

	"github.com/cihub/seelog"
)

const defaultTZ = "Asia/Shanghai"

// InitTimezone 将进程时区固定为 Asia/Shanghai(优先 TZ 环境变量)。
func InitTimezone() {
	name := os.Getenv("TZ")
	if name == "" {
		name = defaultTZ
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		seelog.Warn("[g:InitTimezone] load ", name, " failed: ", err)
		return
	}
	time.Local = loc
	seelog.Info("[g:InitTimezone] using timezone ", name)
}
