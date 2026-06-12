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
	"github.com/zenlenet/pingmesh/src/g"
)

// JoinMaster 以 Agent 身份加入主节点(带重试, 用于启动参数场景)
func JoinMaster(master, token, name, addr, group string) error {
	return JoinMasterN(master, token, name, addr, group, 5)
}

// JoinMasterN 以 Agent 身份加入主节点:
// 1. 调用主节点 /api/join.json 完成注册(主节点自动完成全互联组网)
// 2. 切换到 cloud 模式, 从主节点拉取并保存全量配置(之后每分钟自动同步)
func JoinMasterN(master, token, name, addr, group string, attempts int) error {
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
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(3 * time.Second)
		}
		lastErr = joinOnce(master, token, name, addr, group)
		if lastErr == nil {
			break
		}
		seelog.Error("[func:JoinMaster] attempt ", i+1, " failed: ", lastErr)
	}
	if lastErr != nil {
		return lastErr
	}
	// 注册成功: 统一集群令牌(签名/加密密钥对齐), 切换为 cloud 模式并同步配置
	g.Cfg.Password = token
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

func joinOnce(master, token, name, addr, group string) error {
	client := http.Client{Timeout: 10 * time.Second}
	form := url.Values{
		"name":  {name},
		"addr":  {addr},
		"group": {group},
	}
	// HMAC 签名代替明文令牌传输
	for k, v := range g.SignFormFields("/api/join.json", token) {
		form.Set(k, v)
	}
	resp, err := client.PostForm(master+"/api/join.json", form)
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
