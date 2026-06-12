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
	"github.com/zenlenet/pingmesh/src/g"
	"github.com/zenlenet/pingmesh/src/nettools"
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

// alertMsg 结构化告警内容, 各通道按自身格式渲染
type alertMsg struct {
	Recovered bool
	Title     string
	Link      string       // A → B
	Fields    [][2]string  // {标签, 值}
	Plain     string       // 纯文本回退
}

// recentStat 查询目标最近N分钟的实测均值(用于告警附带实际指标)
func recentStat(target string, mins int) (avg float64, loss float64, jitter float64, ok bool) {
	since := time.Unix(time.Now().Unix()-int64(mins)*60, 0).Format("2006-01-02 15:04")
	g.DLock.Lock()
	row := g.Db.QueryRow("select ifnull(avg(avgdelay),0), ifnull(avg(losspk),0), ifnull(avg(ifnull(jitter,0)),0), count(1) from pinglog where target = ? and logtime >= ?", target, since)
	var cnt int
	err := row.Scan(&avg, &loss, &jitter, &cnt)
	g.DLock.Unlock()
	return avg, loss, jitter, err == nil && cnt > 0
}

func buildAlertMsg(l g.AlertLog, rule map[string]string, kind string, extras [][2]string) alertMsg {
	recovered := kind == "recovery"
	m := alertMsg{Recovered: recovered}
	m.Link = l.Fromname + " → " + l.Targetname
	switch kind {
	case "recovery":
		m.Title = "✅ 网络质量已恢复"
	case "reminder":
		m.Title = "⏰ 故障持续提醒"
	case "mute_expired":
		m.Title = "🔔 屏蔽到期, 目标仍异常"
	default:
		m.Title = "🔴 网络质量告警"
	}
	m.Fields = append(m.Fields, extras...)
	m.Fields = append(m.Fields, [2]string{"链路", m.Link})
	m.Fields = append(m.Fields, [2]string{"源节点", l.Fromname + " (" + l.Fromip + ")"})
	m.Fields = append(m.Fields, [2]string{"目标", l.Targetname + " (" + l.Targetip + ")"})
	if avg, loss, jitter, ok := recentStat(l.Targetip, 15); ok {
		m.Fields = append(m.Fields, [2]string{"近15分钟实测", fmt.Sprintf("平均延迟 %.1f ms / 丢包 %.0f%% / 抖动 %.1f ms", avg, loss, jitter)})
	}
	if rule != nil && !recovered {
		ruleTxt := "延迟 > " + rule["Thdavgdelay"] + "ms 或 丢包 > " + rule["Thdloss"] + "%"
		if rule["Thdjitter"] != "" {
			ruleTxt += " 或 抖动 > " + rule["Thdjitter"] + "ms"
		}
		m.Fields = append(m.Fields, [2]string{"触发规则", ruleTxt})
	}
	m.Fields = append(m.Fields, [2]string{"时间", l.Logtime})
	m.Fields = append(m.Fields, [2]string{"平台", "ZENLENET PingMesh"})

	var b strings.Builder
	b.WriteString(m.Title + " " + m.Link + "\n")
	for _, f := range m.Fields {
		b.WriteString(f[0] + ": " + f[1] + "\n")
	}
	m.Plain = strings.TrimRight(b.String(), "\n")
	return m
}

// NotifyAll 通过所有已启用通道发送告警/恢复/提醒类通知
// kind: alert | recovery | reminder | mute_expired
func NotifyAll(l g.AlertLog, rule map[string]string, kind string, extras [][2]string) {
	m := buildAlertMsg(l, rule, kind, extras)
	// 邮件
	if g.Cfg.Alert["SendEmailAccount"] != "" && g.Cfg.Alert["SendEmailPassword"] != "" &&
		g.Cfg.Alert["EmailHost"] != "" && g.Cfg.Alert["RevcEmailList"] != "" {
		go func() {
			title := m.Title + " " + m.Link + " - ZENLENET PingMesh"
			if err := SendMail(g.Cfg.Alert["SendEmailAccount"], g.Cfg.Alert["SendEmailPassword"],
				g.Cfg.Alert["EmailHost"], g.Cfg.Alert["RevcEmailList"], title, htmlMail(m, l)); err != nil {
				seelog.Error("[func:NotifyAll] send mail error ", err)
			}
		}()
	}
	// 其他通道
	payload := map[string]interface{}{
		"event":      kind,
		"title":      m.Title + " " + m.Link,
		"content":    m.Plain,
		"fromname":   l.Fromname,
		"fromip":     l.Fromip,
		"targetname": l.Targetname,
		"targetip":   l.Targetip,
		"time":       l.Logtime,
	}
	for _, f := range m.Fields {
		if f[0] == "近15分钟实测" {
			payload["metrics"] = f[1]
		}
		if f[0] == "触发规则" {
			payload["rule"] = f[1]
		}
	}
	for _, ch := range g.Cfg.Channels {
		if !ch.Enabled {
			continue
		}
		c := ch
		go func() {
			if body, err := sendRich(c, m, payload); err != nil {
				seelog.Error("[func:NotifyAll] channel ", c.Type, "(", c.Name, ") error: ", err, " resp: ", body)
			} else {
				seelog.Info("[func:NotifyAll] channel ", c.Type, "(", c.Name, ") sent")
			}
		}()
	}
}

/* ---------- 各通道富文本渲染 ---------- */

func mdFields(m alertMsg, bullet string) string {
	var b strings.Builder
	for _, f := range m.Fields {
		b.WriteString(bullet + "**" + f[0] + "**: " + f[1] + "\n")
	}
	return b.String()
}

// sendRich 按通道原生格式发送结构化告警
func sendRich(ch g.AlertChannel, m alertMsg, payload map[string]interface{}) (string, error) {
	p := ch.Params
	if p == nil {
		p = map[string]string{}
	}
	switch ch.Type {
	case "dingtalk":
		if p["Url"] == "" {
			return "", errors.New("钉钉机器人 Url 不能为空")
		}
		u := dingtalkSign(p["Url"], p["Secret"])
		md := "### " + m.Title + "\n\n#### " + m.Link + "\n\n" + mdFields(m, "- ")
		body, err := postJson(u, map[string]interface{}{
			"msgtype":  "markdown",
			"markdown": map[string]string{"title": m.Title + " " + m.Link, "text": md},
		})
		return body, checkErrcode(body, err)
	case "wecom":
		if p["Url"] == "" {
			return "", errors.New("企业微信机器人 Url 不能为空")
		}
		color := "warning"
		if m.Recovered {
			color = "info"
		}
		md := "## <font color=\"" + color + "\">" + m.Title + "</font>\n**" + m.Link + "**\n" + mdFields(m, "> ")
		body, err := postJson(p["Url"], map[string]interface{}{
			"msgtype":  "markdown",
			"markdown": map[string]string{"content": md},
		})
		return body, checkErrcode(body, err)
	case "feishu":
		if p["Url"] == "" {
			return "", errors.New("飞书机器人 Url 不能为空")
		}
		// 富文本 post 消息
		lines := [][]map[string]interface{}{}
		for _, f := range m.Fields {
			lines = append(lines, []map[string]interface{}{
				{"tag": "text", "text": f[0] + ": ", "style": []string{"bold"}},
				{"tag": "text", "text": f[1]},
			})
		}
		msg := map[string]interface{}{
			"msg_type": "post",
			"content": map[string]interface{}{
				"post": map[string]interface{}{
					"zh_cn": map[string]interface{}{
						"title":   m.Title + " " + m.Link,
						"content": lines,
					},
				},
			},
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
		var b strings.Builder
		b.WriteString("<b>" + m.Title + " " + m.Link + "</b>\n")
		for _, f := range m.Fields {
			b.WriteString("<b>" + f[0] + "</b>: " + f[1] + "\n")
		}
		body, err := postJson("https://api.telegram.org/bot"+p["Token"]+"/sendMessage", map[string]interface{}{
			"chat_id":    p["ChatId"],
			"text":       b.String(),
			"parse_mode": "HTML",
		})
		return body, err
	case "slack":
		if p["Url"] == "" {
			return "", errors.New("Slack Webhook Url 不能为空")
		}
		var b strings.Builder
		b.WriteString("*" + m.Title + " " + m.Link + "*\n")
		for _, f := range m.Fields {
			b.WriteString("• *" + f[0] + "*: " + f[1] + "\n")
		}
		return postJson(p["Url"], map[string]interface{}{"text": b.String()})
	case "discord":
		if p["Url"] == "" {
			return "", errors.New("Discord Webhook Url 不能为空")
		}
		colorInt := 15548245 // 红
		if m.Recovered {
			colorInt = 1096065 // 绿
		}
		fields := []map[string]interface{}{}
		for _, f := range m.Fields {
			fields = append(fields, map[string]interface{}{"name": f[0], "value": f[1], "inline": false})
		}
		return postJson(p["Url"], map[string]interface{}{
			"embeds": []map[string]interface{}{{
				"title":  m.Title + " " + m.Link,
				"color":  colorInt,
				"fields": fields,
			}},
		})
	case "webhook":
		if p["Url"] == "" {
			return "", errors.New("Webhook Url 不能为空")
		}
		return postJson(p["Url"], payload)
	}
	return "", errors.New("未知通道类型: " + ch.Type)
}

// SendChannelMessage 测试消息(配置页"发送测试"使用), 复用富文本渲染
func SendChannelMessage(ch g.AlertChannel, title, text string, payload map[string]interface{}) (string, error) {
	m := alertMsg{
		Recovered: true,
		Title:     "🔔 测试消息",
		Link:      g.Cfg.Name,
		Fields: [][2]string{
			{"节点", g.Cfg.Name + " (" + g.Cfg.Addr + ")"},
			{"时间", time.Now().Format("2006-01-02 15:04:05")},
			{"说明", "如收到此消息说明通道配置正确"},
			{"平台", "ZENLENET PingMesh"},
		},
		Plain: text,
	}
	return sendRich(ch, m, payload)
}

/* ---------- 邮件 HTML 模板 ---------- */

func htmlMail(m alertMsg, l g.AlertLog) string {
	color, bg := "#dc2626", "#fef2f2"
	if m.Recovered {
		color, bg = "#047857", "#ecfdf5"
	}
	var rows strings.Builder
	for _, f := range m.Fields {
		rows.WriteString(`<tr><td style="padding:7px 12px;color:#64748b;font-size:13px;white-space:nowrap">` + f[0] +
			`</td><td style="padding:7px 12px;color:#0f172a;font-size:13px;font-weight:600">` + f[1] + `</td></tr>`)
	}
	mtr := ""
	if !m.Recovered && l.Tracert != "" {
		mtr = mtrHtmlTable(l.Tracert)
	}
	return `<div style="font-family:-apple-system,'PingFang SC','Microsoft YaHei',Arial,sans-serif;max-width:620px;margin:0 auto;padding:16px">
<div style="border:1px solid #e2e8f0;border-radius:14px;overflow:hidden">
  <div style="background:` + bg + `;padding:18px 22px;border-bottom:1px solid #e2e8f0">
    <div style="font-size:17px;font-weight:700;color:` + color + `">` + m.Title + `</div>
    <div style="font-size:14px;color:#334155;margin-top:4px">` + m.Link + `</div>
  </div>
  <table style="width:100%;border-collapse:collapse;background:#ffffff">` + rows.String() + `</table>
  ` + mtr + `
  <div style="padding:12px 22px;background:#f8fafc;color:#94a3b8;font-size:11.5px;border-top:1px solid #e2e8f0">
    本邮件由 ZENLENET PingMesh 自动发送
  </div>
</div></div>`
}

func mtrHtmlTable(tracert string) string {
	hops := []nettools.Mtr{}
	if err := json.Unmarshal([]byte(tracert), &hops); err != nil || len(hops) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div style="padding:14px 22px;border-top:1px solid #e2e8f0"><div style="font-size:13px;font-weight:700;color:#334155;margin-bottom:8px">MTR 路由快照</div>`)
	b.WriteString(`<table style="width:100%;border-collapse:collapse;font-size:12px;color:#334155">`)
	b.WriteString(`<tr style="color:#94a3b8"><td style="padding:4px 6px">#</td><td>Host</td><td>Loss%</td><td>Snt</td><td>Avg(ms)</td><td>Wrst(ms)</td></tr>`)
	for i, hop := range hops {
		loss := 0.0
		if hop.Send > 0 {
			loss = float64(hop.Loss) / float64(hop.Send) * 100
		}
		b.WriteString(fmt.Sprintf(`<tr style="border-top:1px solid #f1f5f9"><td style="padding:4px 6px">%d</td><td>%s</td><td>%.1f</td><td>%d</td><td>%.1f</td><td>%.1f</td></tr>`,
			i+1, hop.Host, loss, hop.Send, float64(hop.Avg.Nanoseconds())/1e6, float64(hop.Wrst.Nanoseconds())/1e6))
	}
	b.WriteString(`</table></div>`)
	return b.String()
}

/* ---------- 基础工具 ---------- */

func dingtalkSign(u, secret string) string {
	if secret == "" {
		return u
	}
	ts := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	sign := hmacSha256Base64(secret, ts+"\n"+secret)
	sep := "&"
	if !strings.Contains(u, "?") {
		sep = "?"
	}
	return u + sep + "timestamp=" + ts + "&sign=" + url.QueryEscape(sign)
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

// 飞书返回 {"code":0} 才算成功
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
