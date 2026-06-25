package funcs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

// joinFingerprint join 参数指纹(用于识别"启动时重放的旧参数")
func joinFingerprint(master, token, name, addr, group string) string {
	sum := sha256.Sum256([]byte(master + "|" + token + "|" + name + "|" + addr + "|" + group))
	return hex.EncodeToString(sum[:])
}

func joinMarkerPath() string { return g.Root + "/conf/.join-args" }

// JoinMaster 以 Agent 身份加入主节点(启动参数场景, 带重试)。
// 参数与上次成功 join 完全相同时跳过——服务重启时 systemd 固化的旧参数
// 不会再覆盖页面上的改名; 改了 -name 等参数则正常执行并生效。
func JoinMaster(master, token, name, addr, group string) error {
	fp := joinFingerprint(master, token, name, addr, group)
	if old, err := os.ReadFile(joinMarkerPath()); err == nil && strings.TrimSpace(string(old)) == fp {
		seelog.Info("[func:JoinMaster] join args unchanged since last successful join, skip re-join")
		return nil
	}
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
	// 登录账户随主节点: join 时全量同步主节点用户表(之后每分钟跟随变更)
	if u, err := url.Parse(master); err == nil && u.Host != "" {
		if err := SyncUsersFrom(u.Host, true); err != nil {
			seelog.Error("[func:JoinMaster] initial user sync failed (will retry each cycle): ", err)
		} else {
			seelog.Info("[func:JoinMaster] user accounts synced from master")
		}
	}
	os.WriteFile(joinMarkerPath(), []byte(joinFingerprint(master, token, name, addr, group)), 0600)
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
	body, _ := io.ReadAll(resp.Body)
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
