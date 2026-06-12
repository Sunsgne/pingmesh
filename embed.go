// Package smartping 通过 go:embed 将前端资源与默认配置打包进二进制,
// 实现单文件部署: 首次启动时自动释放到工作目录。
package smartping

import "embed"

//go:embed all:html conf/seelog.xml conf/config-base.json
var Assets embed.FS
