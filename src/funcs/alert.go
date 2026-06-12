package funcs

import (
	"crypto/tls"
	"encoding/json"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cihub/seelog"
	_ "modernc.org/sqlite"
	"github.com/zenlenet/pingmesh/src/g"
	"github.com/zenlenet/pingmesh/src/nettools"
)

// incidentState 每条链路本次故障的过程状态(用于提醒/确认联动/时长统计)
type incidentState struct {
	BadSince    time.Time
	LastNotify  time.Time
	MutedSkip   bool // 屏蔽期间漏发过告警
	Acked       bool // 本次故障已被确认, 抑制重复提醒
}

var (
	incidentMu sync.Mutex
	incidents  = map[string]*incidentState{}
)

func incidentOf(addr string) *incidentState {
	incidentMu.Lock()
	defer incidentMu.Unlock()
	st, ok := incidents[addr]
	if !ok {
		st = &incidentState{}
		incidents[addr] = st
	}
	return st
}

// AckIncident 告警被确认时调用: 本次故障期间不再重复提醒(恢复后自动重置)
func AckIncident(target string) {
	incidentMu.Lock()
	defer incidentMu.Unlock()
	if st, ok := incidents[target]; ok {
		st.Acked = true
	}
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return strconv.Itoa(h) + " 小时 " + strconv.Itoa(m) + " 分钟"
	}
	return strconv.Itoa(m) + " 分钟"
}

func StartAlert() {
	seelog.Info("[func:StartAlert] ", "starting run AlertCheck ")
	remindMin := g.Cfg.Base["Remindmin"] // 持续故障重复提醒间隔(分钟), 0=关闭
	for _, v := range g.SelfCfg.Topology {
		if v["Addr"] == g.SelfCfg.Addr {
			continue
		}
		old, haskey := g.AlertStatus[v["Addr"]]
		sFlag := CheckAlertStatus(v)
		muted := IsMuted(v["Addr"])
		st := incidentOf(v["Addr"])
		if sFlag {
			g.AlertStatus[v["Addr"]] = true
			// 状态由异常恢复为正常: 发送恢复通知(屏蔽中不打扰), 附故障时长
			if haskey && !old {
				seelog.Debug("[func:StartAlert] ", v["Addr"]+" Recovered!")
				if !muted {
					l := newAlertLog(v)
					extras := [][2]string{}
					if !st.BadSince.IsZero() {
						extras = append(extras, [2]string{"故障持续", fmtDur(time.Since(st.BadSince))})
					}
					go NotifyAll(l, v, "recovery", extras)
				}
				st.BadSince = time.Time{}
				st.MutedSkip = false
				st.Acked = false
			}
			continue
		}
		if !haskey || old {
			// 新故障: 记录 + 告警(屏蔽中只记录)
			seelog.Debug("[func:StartAlert] ", v["Addr"]+" Alert!")
			g.AlertStatus[v["Addr"]] = false
			st.BadSince = time.Now()
			st.Acked = false
			l := newAlertLog(v)
			mtrString := ""
			hops, err := nettools.RunMtr(v["Addr"], time.Second, 64, 6)
			if nil != err {
				seelog.Error("[func:StartAlert] Traceroute error ", err)
				mtrString = err.Error()
			} else {
				jHops, jerr := json.Marshal(hops)
				if jerr != nil {
					mtrString = jerr.Error()
				} else {
					mtrString = string(jHops)
				}
			}
			l.Tracert = mtrString
			go AlertStorage(l)
			if muted {
				st.MutedSkip = true
				seelog.Info("[func:StartAlert] ", v["Addr"], " is muted, notification skipped")
			} else {
				st.LastNotify = time.Now()
				go NotifyAll(l, v, "alert", nil)
			}
			continue
		}
		// 持续故障中
		dur := [2]string{"故障持续", fmtDur(time.Since(st.BadSince))}
		if muted {
			st.MutedSkip = true
		} else if st.MutedSkip {
			// 屏蔽到期复查: 仍异常则补一条通知, 故障不会静默挂着
			seelog.Info("[func:StartAlert] ", v["Addr"], " mute expired but still down, notifying")
			st.MutedSkip = false
			st.LastNotify = time.Now()
			go NotifyAll(newAlertLog(v), v, "mute_expired", [][2]string{dur})
		} else if remindMin > 0 && !st.Acked && !st.LastNotify.IsZero() &&
			time.Since(st.LastNotify) >= time.Duration(remindMin)*time.Minute {
			// 持续故障重复提醒(已确认的不再提醒)
			seelog.Info("[func:StartAlert] ", v["Addr"], " still down, periodic reminder")
			st.LastNotify = time.Now()
			go NotifyAll(newAlertLog(v), v, "reminder", [][2]string{dur})
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
	// 达到阈值即计为异常分钟(>=); 窗口起点用 >=(起点截断到分钟, 严格大于会把
	// 边界分钟排除, 小窗口在一分钟的大部分时间内会变成空窗口 → 误判正常)
	querysql := "SELECT count(1) cnt FROM pinglog where logtime >= ? and target = ? and (cast(avgdelay as double) >= cast(? as double) or cast(losspk as double) >= cast(? as double)"
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
