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
			if sFlag {
				g.AlertStatus[v["Addr"]] = true
				// 状态由异常恢复为正常: 发送恢复通知
				if haskey && !old {
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
				go NotifyAll(l, v, false)
			}

		}
	}
	seelog.Info("[func:StartAlert] ", "AlertCheck finish ")
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
	querysql := "SELECT count(1) cnt FROM pinglog where logtime > ? and target = ? and (cast(avgdelay as double) > cast(? as double) or cast(losspk as double) > cast(? as double))"
	rows, err := g.Db.Query(querysql, timeStartStr, v["Addr"], v["Thdavgdelay"], v["Thdloss"])
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
		if l.Cnt <= Thdoccnum {
			return true
		} else {
			return false
		}
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
