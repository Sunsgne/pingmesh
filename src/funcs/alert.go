package funcs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/cihub/seelog"
	_ "modernc.org/sqlite"
	"github.com/smartping/smartping/src/g"
	"github.com/smartping/smartping/src/nettools"
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
	querysql := "SELECT count(1) cnt FROM  `pinglog` where logtime > '" + timeStartStr + "' and target = '" + v["Addr"] + "' and (cast(avgdelay as double) > " + v["Thdavgdelay"] + " or cast(losspk as double) > " + v["Thdloss"] + ") "
	rows, err := g.Db.Query(querysql)
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
	sql := "INSERT INTO [alertlog] (logtime, targetip, targetname, tracert) values('" + t.Logtime + "','" + t.Targetip + "','" + t.Targetname + "','" + t.Tracert + "')"
	g.DLock.Lock()
	//g.Db.Exec(sql)
	_, err := g.Db.Exec(sql)
	if err != nil {
		seelog.Error("[func:StartPing] Sql Error ", err)
	}
	g.DLock.Unlock()
	seelog.Info("[func:AlertStorage] ", "(", t.Logtime, ") AlertStorage on ", t.Targetname, " finish!")
}

func AlertSendMail(t g.AlertLog) {
	hops := []nettools.Mtr{}
	err := json.Unmarshal([]byte(t.Tracert), &hops)
	if err != nil {
		seelog.Error("[func:AlertSendMail] json Error ", err)
		return
	}
	mtrstr := bytes.NewBufferString("")
	fmt.Fprintf(mtrstr, "<table>")
	fmt.Fprintf(mtrstr, "<tr><td>Host</td><td>Loss</td><td>Snt</td><td>Last</td><td>Avg</td><td>Best</td><td>Wrst</td><td>StDev</td></tr>")
	for i, hop := range hops {
		fmt.Fprintf(mtrstr, "<tr><td>%d %s</td><td>%.2f</td><td>%d</td><td>%v</td><td>%v</td><td>%v</td><td>%v</td><td>%.2f</td></tr>", i+1, hop.Host, ((float64(hop.Loss) / float64(hop.Send)) * 100), hop.Send, hop.Last, hop.Avg, hop.Best, hop.Wrst, hop.StDev)
	}
	fmt.Fprintf(mtrstr, "</table>")
	title := "【" + t.Fromname + "->" + t.Targetname + "】延迟大于200ms且丢包率大于30%（" + t.Logtime + "）- ZENLENET PingMesh"
	content := "报警时间：" + t.Logtime + " <br> 源IP：" + t.Fromname + "(" + t.Fromip + ") <br>  目IP：" + t.Targetname + "(" + t.Targetip + ") <br> <br> <br>"
	SendEmailAccount := g.Cfg.Alert["SendEmailAccount"]
	SendEmailPassword := g.Cfg.Alert["SendEmailPassword"]
	EmailHost := g.Cfg.Alert["EmailHost"]
	RevcEmailList := g.Cfg.Alert["RevcEmailList"]
	err = SendMail(SendEmailAccount, SendEmailPassword, EmailHost, RevcEmailList, title, content+mtrstr.String())
	if err != nil {
		seelog.Error("[func:AlertSendMail] SendMail Error ", err)
	}
}

func SendMail(user, pwd, host, to, subject, body string) error {
	if len(strings.Split(host, ":")) == 1 {
		host = host + ":25"
	}
	auth := smtp.PlainAuth("", user, pwd, strings.Split(host, ":")[0])
	content_type := "Content-Type: text/html" + "; charset=UTF-8"
	msg := []byte("To: " + to + "\r\nFrom: " + user + "\r\nSubject: " + subject + "\r\n" + content_type + "\r\n\r\n" + body)
	send_to := strings.Split(to, ";")
	err := smtp.SendMail(host, auth, user, send_to, msg)
	if err != nil {
		return err
	}
	return nil
}
