package funcs

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cihub/seelog"
	"github.com/smartping/smartping/src/g"
)

// JoinMaster 以 Agent 身份加入主节点:
// 1. 调用主节点 /api/join.json 完成注册(主节点自动完成全互联组网)
// 2. 切换到 cloud 模式, 从主节点拉取并保存全量配置(之后每分钟自动同步)
func JoinMaster(master, token, name, addr string) error {
	master = strings.TrimRight(master, "/")
	if !strings.HasPrefix(master, "http://") && !strings.HasPrefix(master, "https://") {
		master = "http://" + master
	}
	if name == "" {
		name = g.Cfg.Name
	}
	if addr == "" {
		detected, err := detectSelfAddr(master)
		if err != nil {
			return errors.New("无法自动识别本机IP, 请使用 -addr 指定: " + err.Error())
		}
		addr = detected
	}
	seelog.Info("[func:JoinMaster] joining ", master, " as ", name, "(", addr, ")")
	var lastErr error
	for i := 0; i < 5; i++ {
		if i > 0 {
			time.Sleep(3 * time.Second)
		}
		lastErr = joinOnce(master, token, name, addr)
		if lastErr == nil {
			break
		}
		seelog.Error("[func:JoinMaster] attempt ", i+1, " failed: ", lastErr)
	}
	if lastErr != nil {
		return lastErr
	}
	// 注册成功, 切换为 cloud 模式并同步配置
	g.Cfg.Name = name
	g.Cfg.Addr = addr
	endpoint := master + "/api/config.json"
	if g.Cfg.Mode == nil {
		g.Cfg.Mode = map[string]string{}
	}
	g.Cfg.Mode["Endpoint"] = endpoint
	if _, err := g.SaveCloudConfig(endpoint); err != nil {
		return errors.New("拉取主节点配置失败: " + err.Error())
	}
	if err := g.SaveConfig(); err != nil {
		return err
	}
	seelog.Info("[func:JoinMaster] joined mesh, config synced from ", endpoint)
	return nil
}

func joinOnce(master, token, name, addr string) error {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(master+"/api/join.json", url.Values{
		"token": {token},
		"name":  {name},
		"addr":  {addr},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var out struct {
		Status string `json:"status"`
		Info   string `json:"info"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return errors.New("主节点响应异常: " + string(body))
	}
	if out.Status != "true" {
		return errors.New(out.Info)
	}
	return nil
}

// detectSelfAddr 通过与主节点建立连接获取本机出口IP
func detectSelfAddr(master string) (string, error) {
	u, err := url.Parse(master)
	if err != nil {
		return "", err
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	local := conn.LocalAddr().String()
	if i := strings.LastIndex(local, ":"); i >= 0 {
		local = local[:i]
	}
	return local, nil
}
