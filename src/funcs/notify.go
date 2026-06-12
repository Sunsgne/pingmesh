package funcs

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cihub/seelog"
	"github.com/smartping/smartping/src/g"
)

// ChannelTypes 支持的告警通道类型
var ChannelTypes = map[string]bool{
	"webhook":  true,
	"dingtalk": true,
	"wecom":    true,
	"feishu":   true,
	"telegram": true,
	"slack":    true,
	"discord":  true,
}

// NotifyAll 通过所有已启用通道发送告警/恢复通知
func NotifyAll(l g.AlertLog, rule map[string]string, recovered bool) {
	title, text := buildMessage(l, rule, recovered)
	// 邮件(沿用原配置)
	if g.Cfg.Alert["SendEmailAccount"] != "" && g.Cfg.Alert["SendEmailPassword"] != "" &&
		g.Cfg.Alert["EmailHost"] != "" && g.Cfg.Alert["RevcEmailList"] != "" {
		if recovered {
			go func() {
				if err := SendMail(g.Cfg.Alert["SendEmailAccount"], g.Cfg.Alert["SendEmailPassword"],
					g.Cfg.Alert["EmailHost"], g.Cfg.Alert["RevcEmailList"], title, strings.Replace(text, "\n", "<br>", -1)); err != nil {
					seelog.Error("[func:NotifyAll] send recovery mail error ", err)
				}
			}()
		} else {
			go AlertSendMail(l)
		}
	}
	// 其他通道
	event := "alert"
	if recovered {
		event = "recovery"
	}
	payload := map[string]interface{}{
		"event":      event,
		"title":      title,
		"content":    text,
		"fromname":   l.Fromname,
		"fromip":     l.Fromip,
		"targetname": l.Targetname,
		"targetip":   l.Targetip,
		"time":       l.Logtime,
	}
	for _, ch := range g.Cfg.Channels {
		if !ch.Enabled {
			continue
		}
		c := ch
		go func() {
			if body, err := SendChannelMessage(c, title, text, payload); err != nil {
				seelog.Error("[func:NotifyAll] channel ", c.Type, "(", c.Name, ") error: ", err, " resp: ", body)
			} else {
				seelog.Info("[func:NotifyAll] channel ", c.Type, "(", c.Name, ") sent")
			}
		}()
	}
}

func buildMessage(l g.AlertLog, rule map[string]string, recovered bool) (string, string) {
	reason := ""
	if rule != nil {
		reason = "规则: 平均延迟 > " + rule["Thdavgdelay"] + "ms 或 丢包率 > " + rule["Thdloss"] + "%"
	}
	var title string
	if recovered {
		title = "【恢复】" + l.Fromname + " → " + l.Targetname + " 网络质量已恢复"
	} else {
		title = "【告警】" + l.Fromname + " → " + l.Targetname + " 网络质量异常"
	}
	text := title + "\n" +
		"时间: " + l.Logtime + "\n" +
		"源节点: " + l.Fromname + " (" + l.Fromip + ")\n" +
		"目标节点: " + l.Targetname + " (" + l.Targetip + ")"
	if !recovered && reason != "" {
		text += "\n" + reason
	}
	text += "\n来自 SmartPing"
	return title, text
}

// SendChannelMessage 向指定通道发送消息, 返回响应内容(便于测试诊断)
func SendChannelMessage(ch g.AlertChannel, title, text string, payload map[string]interface{}) (string, error) {
	p := ch.Params
	if p == nil {
		p = map[string]string{}
	}
	switch ch.Type {
	case "webhook":
		if p["Url"] == "" {
			return "", errors.New("Webhook Url 不能为空")
		}
		return postJson(p["Url"], payload)
	case "dingtalk":
		if p["Url"] == "" {
			return "", errors.New("钉钉机器人 Url 不能为空")
		}
		u := p["Url"]
		if p["Secret"] != "" {
			ts := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
			sign := hmacSha256Base64(p["Secret"], ts+"\n"+p["Secret"])
			sep := "&"
			if !strings.Contains(u, "?") {
				sep = "?"
			}
			u = u + sep + "timestamp=" + ts + "&sign=" + url.QueryEscape(sign)
		}
		body, err := postJson(u, map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": text},
		})
		return body, checkErrcode(body, err)
	case "wecom":
		if p["Url"] == "" {
			return "", errors.New("企业微信机器人 Url 不能为空")
		}
		body, err := postJson(p["Url"], map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": text},
		})
		return body, checkErrcode(body, err)
	case "feishu":
		if p["Url"] == "" {
			return "", errors.New("飞书机器人 Url 不能为空")
		}
		msg := map[string]interface{}{
			"msg_type": "text",
			"content":  map[string]string{"text": text},
		}
		if p["Secret"] != "" {
			ts := strconv.FormatInt(time.Now().Unix(), 10)
			msg["timestamp"] = ts
			msg["sign"] = hmacSha256Base64(ts+"\n"+p["Secret"], "")
		}
		body, err := postJson(p["Url"], msg)
		return body, checkFeishu(body, err)
	case "telegram":
		if p["Token"] == "" || p["ChatId"] == "" {
			return "", errors.New("Telegram Token 与 ChatId 不能为空")
		}
		body, err := postJson("https://api.telegram.org/bot"+p["Token"]+"/sendMessage", map[string]interface{}{
			"chat_id": p["ChatId"],
			"text":    text,
		})
		return body, err
	case "slack":
		if p["Url"] == "" {
			return "", errors.New("Slack Webhook Url 不能为空")
		}
		return postJson(p["Url"], map[string]interface{}{"text": text})
	case "discord":
		if p["Url"] == "" {
			return "", errors.New("Discord Webhook Url 不能为空")
		}
		return postJson(p["Url"], map[string]interface{}{"content": text})
	}
	return "", errors.New("未知通道类型: " + ch.Type)
}

func postJson(u string, payload interface{}) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(u, "application/json", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return string(body), fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return string(body), nil
}

// 钉钉/企业微信返回 {"errcode":0,...} 才算成功
func checkErrcode(body string, err error) error {
	if err != nil {
		return err
	}
	var r struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if json.Unmarshal([]byte(body), &r) == nil && r.Errcode != 0 {
		return fmt.Errorf("errcode %d: %s", r.Errcode, r.Errmsg)
	}
	return nil
}

// 飞书返回 {"code":0} / 旧版 {"StatusCode":0} 才算成功
func checkFeishu(body string, err error) error {
	if err != nil {
		return err
	}
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal([]byte(body), &r) == nil && r.Code != 0 {
		return fmt.Errorf("code %d: %s", r.Code, r.Msg)
	}
	return nil
}

func hmacSha256Base64(key, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ValidateChannels 配置保存时的通道校验
func ValidateChannels(chs []g.AlertChannel) error {
	for _, ch := range chs {
		if !ChannelTypes[ch.Type] {
			return errors.New("未知告警通道类型: " + ch.Type)
		}
		p := ch.Params
		if p == nil {
			p = map[string]string{}
		}
		switch ch.Type {
		case "telegram":
			if p["Token"] == "" || p["ChatId"] == "" {
				return errors.New("Telegram 通道需要 Token 与 ChatId")
			}
		default:
			if p["Url"] == "" {
				return errors.New(ch.Type + " 通道需要 Url")
			}
		}
	}
	return nil
}
