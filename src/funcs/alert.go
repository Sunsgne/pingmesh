package funcs

import (
	"crypto/tls"
	"encoding/json"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/cihub/seelog"
	_ "modernc.org/sqlite"
	"github.com/zenlenet/pingmesh/src/g"
	"github.com/zenlenet/pingmesh/src/nettools"
)

func StartAlert() {
	seelog.Info("[func:StartAlert] ", "starting run AlertCheck ")
	for _, v := range g.SelfCfg.Topology {
		if v["Addr"] != g.SelfCfg.Addr {
			old, haskey := g.AlertStatus[v["Addr"]]
			sFlag := CheckAlertStatus(v)
			muted := IsMuted(v["Addr"])
			if sFlag {
				g.AlertStatus[v["Addr"]] = true
				// 状态由异常恢复为正常: 发送恢复通知(屏蔽中不打扰)
				if haskey && !old && !muted {
					seelog.Debug("[func:StartAlert] ", v["Addr"]+" Recovered!")
					l := newAlertLog(v)
					go NotifyAll(l, v, true)
				}
			} else if !haskey || old {
				seelog.Debug("[func:StartAlert] ", v["Addr"]+" Alert!")
				g.AlertStatus[v["Addr"]] = false
				l := newAlertLog(v)
				mtrString := ""
				hops, err := nettools.RunMtr(v["Addr"], time.Second, 64, 6)
				if nil != err {
					seelog.Error("[func:StartAlert] Traceroute error ", err)
					mtrString = err.Error()
				} else {
					jHops, err := json.Marshal(hops)
					if err != nil {
						mtrString = err.Error()
					} else {
						mtrString = string(jHops)
					}
				}
				l.Tracert = mtrString
				go AlertStorage(l)
				if muted {
					seelog.Info("[func:StartAlert] ", v["Addr"], " is muted, notification skipped")
				} else {
					go NotifyAll(l, v, false)
				}
			}

		}
	}
	seelog.Info("[func:StartAlert] ", "AlertCheck finish ")
}

// IsMuted 目标是否在屏蔽期内(屏蔽期间仍记录告警, 但不发通知)
func IsMuted(target string) bool {
	now := time.Now().Format("2006-01-02 15:04:05")
	var cnt int
	g.DLock.Lock()
	g.Db.QueryRow("SELECT count(1) FROM alertmute WHERE target = ? AND muteduntil > ?", target, now).Scan(&cnt)
	g.DLock.Unlock()
	return cnt > 0
}

func newAlertLog(v map[string]string) g.AlertLog {
	l := g.AlertLog{}
	l.Fromname = g.SelfCfg.Name
	l.Fromip = g.SelfCfg.Addr
	l.Logtime = time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04")
	l.Targetname = v["Name"]
	l.Targetip = v["Addr"]
	return l
}

func CheckAlertStatus(v map[string]string) bool {
	type Cnt struct {
		Cnt int
	}
	Thdchecksec, _ := strconv.Atoi(v["Thdchecksec"])
	timeStartStr := time.Unix((time.Now().Unix() - int64(Thdchecksec)), 0).Format("2006-01-02 15:04")
	// 达到阈值即计为异常分钟(>=): 丢包阈值设为100时, 100%丢包同样触发
	querysql := "SELECT count(1) cnt FROM pinglog where logtime > ? and target = ? and (cast(avgdelay as double) >= cast(? as double) or cast(losspk as double) >= cast(? as double)"
	args := []interface{}{timeStartStr, v["Addr"], v["Thdavgdelay"], v["Thdloss"]}
	// 抖动阈值为可选项, 配置后参与告警判定(IPLC/IEPL 场景)
	if v["Thdjitter"] != "" {
		querysql += " or cast(ifnull(jitter,0) as double) >= cast(? as double)"
		args = append(args, v["Thdjitter"])
	}
	querysql += ")"
	rows, err := g.Db.Query(querysql, args...)
	defer rows.Close()
	seelog.Debug("[func:StartAlert] ", querysql)
	if err != nil {
		seelog.Error("[func:StartAlert] Query Error ", err)
		return false
	}
	for rows.Next() {
		l := new(Cnt)
		err := rows.Scan(&l.Cnt)
		if err != nil {
			seelog.Error("[func:StartAlert]", err)
			return false
		}
		Thdoccnum, _ := strconv.Atoi(v["Thdoccnum"])
		// 异常分钟数"达到"触发次数即告警(与界面文案一致)
		if l.Cnt >= Thdoccnum {
			return false
		}
		return true
	}
	return false
}

func AlertStorage(t g.AlertLog) {
	seelog.Info("[func:AlertStorage] ", "(", t.Logtime, ")Starting AlertStorage ", t.Targetname)
	sql := "INSERT INTO alertlog (logtime, targetip, targetname, tracert) values(?,?,?,?)"
	g.DLock.Lock()
	_, err := g.Db.Exec(sql, t.Logtime, t.Targetip, t.Targetname, t.Tracert)
	if err != nil {
		seelog.Error("[func:StartPing] Sql Error ", err)
	}
	g.DLock.Unlock()
	seelog.Info("[func:AlertStorage] ", "(", t.Logtime, ") AlertStorage on ", t.Targetname, " finish!")
}

// SendMail 发送邮件。端口465走隐式TLS(QQ/163/企业邮等), 其他端口走明文+STARTTLS
func SendMail(user, pwd, host, to, subject, body string) error {
	if len(strings.Split(host, ":")) == 1 {
		host = host + ":25"
	}
	parts := strings.Split(host, ":")
	hostOnly, port := parts[0], parts[1]
	auth := smtp.PlainAuth("", user, pwd, hostOnly)
	contentType := "Content-Type: text/html; charset=UTF-8"
	msg := []byte("To: " + to + "\r\nFrom: " + user + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\n" + contentType + "\r\n\r\n" + body)
	sendTo := strings.Split(to, ";")
	if port != "465" {
		return smtp.SendMail(host, auth, user, sendTo, msg)
	}
	// 隐式 TLS (SMTPS)
	conn, err := tls.Dial("tcp", host, &tls.Config{ServerName: hostOnly})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, hostOnly)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if err = c.Auth(auth); err != nil {
		return err
	}
	if err = c.Mail(user); err != nil {
		return err
	}
	for _, t := range sendTo {
		if t = strings.TrimSpace(t); t == "" {
			continue
		} else if err = c.Rcpt(t); err != nil {
			return err
		}
	}
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = wc.Write(msg); err != nil {
		wc.Close()
		return err
	}
	if err = wc.Close(); err != nil {
		return err
	}
	return c.Quit()
}
