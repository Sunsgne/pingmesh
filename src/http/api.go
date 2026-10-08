package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/funcs"
	"github.com/zenlenet/pingmesh/src/g"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func configApiRoutes() {

	firstForm := func(r *http.Request, keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(r.FormValue(k)); v != "" {
				return v
			}
		}
		return ""
	}

	//配置文件API
	http.HandleFunc("/api/config.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		nconf := g.Config{}
		cfgJson, _ := json.Marshal(g.Cfg)
		json.Unmarshal(cfgJson, &nconf)
		// 管理员可见接入令牌; 非管理员隐藏。
		// 敏感字段: 仅管理员明文可见; 节点加密同步(AgentSigned)拿完整配置后加密返回。
		// 注意: 仅凭节点 IP 不再下发明文密钥, 防止经 /api/proxy.json 被只读用户套取。
		if !AuthAdmin(r) {
			nconf.Password = ""
		}
		if !AuthAdmin(r) && !AgentSigned(r) {
			maskConfigSecrets(&nconf)
		}
		//fmt.Print(g.Cfg.Alert["SendEmailPassword"])
		onconf, _ := json.Marshal(nconf)
		// 节点签名请求且声明 enc=1: 配置同步走 AES-256-GCM 加密载荷
		if AgentSigned(r) {
			if enc, err := g.EncryptPayload(onconf, g.Cfg.Password); err == nil {
				w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
				fmt.Fprintln(w, enc)
				return
			}
		}
		var out bytes.Buffer
		json.Indent(&out, onconf, "", "\t")
		o := out.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, o)
	})

	//Ping数据API
	http.HandleFunc("/api/ping.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		if len(r.Form["ip"]) == 0 {
			o := "Missing Param !"
			http.Error(w, o, 406)
			return
		}
		tableip := r.Form["ip"][0]
		// 兼容 starttime/endtime 与 start/end
		timeStartStr := firstForm(r, "starttime", "start")
		timeEndStr := firstForm(r, "endtime", "end")

		var timeStart, timeEnd int64
		if timeStartStr != "" {
			if tms, err := g.ParseProbeTime(timeStartStr); err == nil {
				timeStart = g.AlignUnixProbe(tms.Unix())
			}
		}
		if timeEndStr != "" {
			if tmn, err := g.ParseProbeTime(timeEndStr); err == nil {
				timeEnd = g.AlignUnixProbe(tmn.Unix())
			}
		}
		if timeStart <= 0 || timeEnd <= 0 || timeEnd <= timeStart {
			timeEnd = g.AlignUnixProbe(time.Now().Unix())
			timeStart = timeEnd - 2*60*60
			timeStart = g.AlignUnixProbe(timeStart)
		}
		// 长跨度自适应加大分桶, 控制点数; 同时 O(1) 填桶, 避免三天以上超时空白
		step := g.ChartStepSec(timeEnd - timeStart)
		timeStart = g.AlignUnixStep(timeStart, step)
		timeEnd = g.AlignUnixStep(timeEnd, step)
		if timeEnd < timeStart {
			timeEnd = timeStart
		}
		timeStartStr = time.Unix(timeStart, 0).In(time.Local).Format("2006-01-02 15:04:05")
		timeEndStr = time.Unix(timeEnd, 0).In(time.Local).Format("2006-01-02 15:04:05")

		cnt := int((timeEnd-timeStart)/step) + 1
		if cnt < 1 {
			cnt = 1
		}
		lastcheck := make([]string, cnt)
		maxdelay := make([]string, cnt)
		mindelay := make([]string, cnt)
		avgdelay := make([]string, cnt)
		losspk := make([]string, cnt)
		jitter := make([]string, cnt)
		type agg struct {
			maxV, minV, sumAvg, sumLoss, sumJit float64
			nMax, nMin, nAvg, nLoss, nJit       int
		}
		aggs := make([]agg, cnt)
		for i := 0; i < cnt; i++ {
			ts := timeStart + int64(i)*step
			lastcheck[i] = time.Unix(ts, 0).In(time.Local).Format("2006-01-02 15:04:05")
			maxdelay[i], mindelay[i], avgdelay[i], losspk[i], jitter[i] = "-", "-", "-", "-", "-"
		}

		querySql := "SELECT logtime,maxdelay,mindelay,avgdelay,losspk,ifnull(jitter,0) FROM pinglog where target=? and logtime between ? and ?"
		rows, err := g.Db.Query(querySql, tableip, timeStartStr, timeEndStr)
		seelog.Debug("[func:/api/ping.json] Query ", querySql, " step=", step, " points=", cnt)
		if err != nil {
			// 查询失败如实返回错误, 不要伪装成"没有数据"的 200
			seelog.Error("[func:/api/ping.json] Query ", err)
			http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
			return
		} else {
			for rows.Next() {
				l := new(g.PingLog)
				if err := rows.Scan(&l.Logtime, &l.Maxdelay, &l.Mindelay, &l.Avgdelay, &l.Losspk, &l.Jitter); err != nil {
					seelog.Error("[/api/ping.json] Rows", err)
					continue
				}
				tm, err := g.ParseProbeTime(l.Logtime)
				if err != nil {
					continue
				}
				idx := int((g.AlignUnixStep(tm.Unix(), step) - timeStart) / step)
				if idx < 0 || idx >= cnt {
					continue
				}
				a := &aggs[idx]
				// 全丢包样本的延迟记为 0, 不参与延迟统计(否则中断被画成延迟变低); 丢包率照常统计
				lp, _ := strconv.ParseFloat(l.Losspk, 64)
				fullLoss := lp >= 100
				if v, e := strconv.ParseFloat(l.Maxdelay, 64); e == nil && !fullLoss {
					if a.nMax == 0 || v > a.maxV {
						a.maxV = v
					}
					a.nMax++
				}
				if v, e := strconv.ParseFloat(l.Mindelay, 64); e == nil && !fullLoss {
					if a.nMin == 0 || v < a.minV {
						a.minV = v
					}
					a.nMin++
				}
				if v, e := strconv.ParseFloat(l.Avgdelay, 64); e == nil && !fullLoss {
					a.sumAvg += v
					a.nAvg++
				}
				if v, e := strconv.ParseFloat(l.Losspk, 64); e == nil {
					a.sumLoss += v
					a.nLoss++
				}
				if v, e := strconv.ParseFloat(l.Jitter, 64); e == nil {
					a.sumJit += v
					a.nJit++
				}
			}
			rows.Close()
		}
		for i := range aggs {
			a := aggs[i]
			if a.nMax > 0 {
				maxdelay[i] = strconv.FormatFloat(a.maxV, 'f', 2, 64)
			}
			if a.nMin > 0 {
				mindelay[i] = strconv.FormatFloat(a.minV, 'f', 2, 64)
			}
			if a.nAvg > 0 {
				avgdelay[i] = strconv.FormatFloat(a.sumAvg/float64(a.nAvg), 'f', 2, 64)
			}
			if a.nLoss > 0 {
				losspk[i] = strconv.FormatFloat(a.sumLoss/float64(a.nLoss), 'f', 2, 64)
			}
			if a.nJit > 0 {
				jitter[i] = strconv.FormatFloat(a.sumJit/float64(a.nJit), 'f', 2, 64)
			}
		}
		preout := map[string]interface{}{
			"lastcheck": lastcheck,
			"maxdelay":  maxdelay,
			"mindelay":  mindelay,
			"avgdelay":  avgdelay,
			"losspk":    losspk,
			"jitter":    jitter,
			"step":      step,
			"points":    cnt,
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	//Ping拓扑API
	http.HandleFunc("/api/topology.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		r.ParseForm()
		rangeStart := r.FormValue("start")
		rangeEnd := r.FormValue("end")
		useRange := rangeStart != "" && rangeEnd != ""
		// 历史区间最长 31 天(每个目标都要扫一遍区间, 更长会算几十秒)
		if useRange {
			if s, e1 := g.ParseProbeTime(rangeStart); e1 == nil {
				if e, e2 := g.ParseProbeTime(rangeEnd); e2 == nil && e.Sub(s) > 31*24*time.Hour {
					rangeStart = e.Add(-31 * 24 * time.Hour).Format("2006-01-02 15:04")
				}
			}
		}
		preout := make(map[string]string)
		for _, v := range g.SelfCfg.Topology {
			var ok bool
			if useRange {
				ok = funcs.CheckAlertStatusInRange(v, rangeStart, rangeEnd)
			} else {
				ok = funcs.CheckAlertStatus(v)
			}
			if ok {
				preout[v["Addr"]] = "true"
			} else {
				preout[v["Addr"]] = "false"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	// 告警健康快照: 主节点汇总各探测节点状态后统一发通知
	http.HandleFunc("/api/alerthealth.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		RenderJson(w, funcs.LocalAlertHealth())
	})

	//报警API
	http.HandleFunc("/api/alert.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		type DateList struct {
			Ldate string
		}
		w.Header().Set("Content-Type", "application/json")
		r.ParseForm()
		dtb := time.Unix(time.Now().Unix(), 0).Format("2006-01-02")
		if len(r.Form["date"]) > 0 {
			dtb = strings.Replace(r.Form["date"][0], "alertlog-", "", -1)
		}
		// 支持自定义时间范围(优先于 date)
		rangeStart, rangeEnd := dtb+" 00:00:00", dtb+" 23:59:59"
		if len(r.Form["start"]) > 0 && len(r.Form["end"]) > 0 && r.Form["start"][0] != "" && r.Form["end"][0] != "" {
			rangeStart, rangeEnd = r.Form["start"][0], r.Form["end"][0]
		}
		listpreout := []string{}
		datapreout := []g.AlertLog{}
		querySql := "select date(logtime) as ldate from alertlog group by date(logtime) order by logtime desc"
		rows, err := g.Db.Query(querySql)
		seelog.Debug("[func:/api/alert.json] Query ", querySql)
		if err != nil {
			seelog.Error("[func:/api/alert.json] Query ", err)
		} else {
			for rows.Next() {
				l := new(DateList)
				err := rows.Scan(&l.Ldate)
				if err != nil {
					seelog.Error("[/api/alert.json] Rows", err)
					continue
				}
				listpreout = append(listpreout, l.Ldate)
			}
			rows.Close()
		}
		querySql = "select rowid,logtime,targetname,targetip,tracert,ifnull(ack,0),ifnull(ackby,''),ifnull(ackreason,''),ifnull(acktime,''),ifnull(alerttype,''),ifnull(reason,''),ifnull(tag,''),ifnull(avgdelay,0),ifnull(loss,0),ifnull(jitter,0) from alertlog where logtime between ? and ? order by logtime desc limit 1000"
		rows, err = g.Db.Query(querySql, rangeStart, rangeEnd)
		seelog.Debug("[func:/api/alert.json] Query ", querySql)
		if err != nil {
			seelog.Error("[func:/api/alert.json] Query ", err)
		} else {
			tagTotal := map[string]int{}
			trows, terr := g.Db.Query("select tag, count(1) from alertlog where ifnull(tag,'') != '' group by tag")
			if terr == nil {
				for trows.Next() {
					var tag string
					var cnt int
					if trows.Scan(&tag, &cnt) == nil {
						tagTotal[tag] = cnt
					}
				}
				trows.Close()
			}
			for rows.Next() {
				l := new(g.AlertLog)
				err := rows.Scan(&l.Id, &l.Logtime, &l.Targetname, &l.Targetip, &l.Tracert, &l.Ack, &l.Ackby, &l.Ackreason, &l.Acktime,
					&l.AlertType, &l.Reason, &l.Tag, &l.AvgDelay, &l.Loss, &l.Jitter)
				l.Fromname = g.Cfg.Name
				l.Fromip = g.Cfg.Addr
				// 名称按当前配置解析: 节点改名后历史告警同步显示新名
				if n, ok := g.Cfg.Network[l.Targetip]; ok && n.Name != "" {
					l.Targetname = n.Name
				}
				if err != nil {
					seelog.Error("[/api/alert.json] Rows", err)
					continue
				}
				// 老数据无类型时补全展示(不写回库)
				if l.AlertType == "" || l.Tag == "" {
					rule := map[string]string{}
					for _, t := range g.SelfCfg.Topology {
						if t["Addr"] == l.Targetip {
							rule = t
							break
						}
					}
					if l.AvgDelay == 0 && l.Loss == 0 && l.Jitter == 0 {
						l.AlertType = "quality"
						l.Reason = "历史告警(升级前未记录类型)"
						l.Tag = "QUALITY:" + l.Fromip + "→" + l.Targetip
					} else if len(rule) > 0 {
						l.AlertType, l.Reason, l.Tag = funcs.ClassifyAlert(l.Fromip, l.Targetip, rule, l.AvgDelay, l.Loss, l.Jitter)
					}
				}
				if l.Tag != "" {
					l.Occur = tagTotal[l.Tag]
				}
				datapreout = append(datapreout, *l)
			}
			rows.Close()
		}
		lout, _ := json.Marshal(listpreout)
		dout, _ := json.Marshal(datapreout)
		fmt.Fprintln(w, "["+string(lout)+","+string(dout)+"]")
	})

	//全国延迟API
	http.HandleFunc("/api/mapping.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		m, _ := time.ParseDuration("-1m")
		dataKey := time.Now().Add(m).Format("2006-01-02 15:04")
		r.ParseForm()
		if len(r.Form["d"]) > 0 {
			dataKey = r.Form["d"][0]
		}
		type Mapjson struct {
			Mapjson string
		}
		// 线路名称由配置动态决定(全球化: 地区 -> 线路 -> 探测IP)
		chinaMp := g.ChinaMp{}
		chinaMp.Text = g.Cfg.Name
		chinaMp.Subtext = dataKey
		chinaMp.Avgdelay = map[string][]g.MapVal{}
		querySql := "select mapjson from mappinglog where logtime = ?"
		rows, err := g.Db.Query(querySql, dataKey)
		seelog.Debug("[func:/api/mapping.json] Query ", querySql)
		if err != nil {
			seelog.Error("[func:/api/mapping.json] Query ", err)
		} else {
			for rows.Next() {
				l := new(Mapjson)
				err := rows.Scan(&l.Mapjson)
				if err != nil {
					seelog.Error("[/api/mapping.json] Rows", err)
					continue
				}
				json.Unmarshal([]byte(l.Mapjson), &chinaMp.Avgdelay)
			}
			rows.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, chinaMp)
	})

	//保存配置文件
	http.HandleFunc("/api/saveconfig.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthAdmin(r) {
			deny(w)
			return
		}
		preout := make(map[string]string)
		r.ParseForm()
		preout["status"] = "false"
		// 管理员会话可直接保存; 兼容旧版的配置密码方式
		if GetSession(r) == nil {
			if len(r.Form["password"]) == 0 || r.Form["password"][0] != g.Cfg.Password {
				preout["info"] = "密码错误!"
				RenderJson(w, preout)
				return
			}
		}
		if len(r.Form["config"]) == 0 {
			preout["info"] = "参数错误!"
			RenderJson(w, preout)
			return
		}
		nconfig := g.Config{}
		err := json.Unmarshal([]byte(r.Form["config"][0]), &nconfig)
		if err != nil {
			preout["info"] = "配置文件解析错误!" + err.Error()
			RenderJson(w, preout)
			return
		}
		if nconfig.Name == "" {
			preout["info"] = "本机节点名称为空!"
			RenderJson(w, preout)
			return
		}
		if !ValidIP4(nconfig.Addr) {
			preout["info"] = "非法本机节点IP!"
			RenderJson(w, preout)
			return
		}
		//Base
		if _, ok := nconfig.Base["Timeout"]; !ok || nconfig.Base["Timeout"] <= 0 {
			preout["info"] = "非法超时时间!(>0)"
			RenderJson(w, preout)
			return
		}
		if _, ok := nconfig.Base["Archive"]; !ok || nconfig.Base["Archive"] <= 0 {
			preout["info"] = "非法存档天数!(>0)"
			RenderJson(w, preout)
			return
		}
		if _, ok := nconfig.Base["Refresh"]; !ok || nconfig.Base["Refresh"] <= 0 {
			preout["info"] = "非法刷新频率!(>0)"
			RenderJson(w, preout)
			return
		}
		//Topology
		if _, ok := nconfig.Topology["Tline"]; !ok || nconfig.Topology["Tline"] <= "0" {
			preout["info"] = "非法拓扑连线粗细(>0)"
			RenderJson(w, preout)
			return
		}
		if _, ok := nconfig.Topology["Tsymbolsize"]; !ok || nconfig.Topology["Tsymbolsize"] <= "0" {
			preout["info"] = "非法拓扑形状大小!(>0)"
			RenderJson(w, preout)
			return
		}
		if nconfig.Toollimit < 0 {
			preout["info"] = "非法检测工具限定频率!(>=0)"
			RenderJson(w, preout)
			return
		}
		//探测参数(毫秒级)
		if v := nconfig.Base["Pinginterval"]; v < 10 || v > 60000 {
			preout["info"] = "非法探测间隔!(10~60000 毫秒)"
			RenderJson(w, preout)
			return
		}
		if v := nconfig.Base["Pingcount"]; v < 1 || v > 1000 {
			preout["info"] = "非法每分钟包数!(1~1000)"
			RenderJson(w, preout)
			return
		}
		if nconfig.Base["Pinginterval"]*nconfig.Base["Pingcount"] > 60000 {
			total := nconfig.Base["Pinginterval"] * nconfig.Base["Pingcount"] / 1000
			preout["info"] = "探测参数无效: 每分钟总发包时长超限。当前 间隔" +
				strconv.Itoa(nconfig.Base["Pinginterval"]) + "ms × " + strconv.Itoa(nconfig.Base["Pingcount"]) + "包/分钟 = " +
				strconv.Itoa(total) + "秒 > 60秒。请调小间隔或每分钟包数"
			RenderJson(w, preout)
			return
		}
		perCycle := g.CyclePacketCount(nconfig.Base["Pinginterval"], nconfig.Base["Pingcount"])
		if nconfig.Base["Pinginterval"]*perCycle > g.ProbeCycleMaxMs() {
			preout["info"] = "探测参数无效: 每" + strconv.Itoa(g.ProbeCycleSec) + "秒周期内发不完折算包数(间隔" +
				strconv.Itoa(nconfig.Base["Pinginterval"]) + "ms × " + strconv.Itoa(perCycle) + "包)。请调小每分钟包数或增大间隔"
			RenderJson(w, preout)
			return
		}
		if v := nconfig.Base["Pingtimeout"]; v < 50 || v > 10000 {
			preout["info"] = "非法单包超时!(50~10000 毫秒)"
			RenderJson(w, preout)
			return
		}
		if v := nconfig.Base["Pingsize"]; v < 24 || v > 1472 {
			preout["info"] = "非法探测包大小!(24~1472 字节)"
			RenderJson(w, preout)
			return
		}
		//Channels
		if err := funcs.ValidateChannels(nconfig.Channels); err != nil {
			preout["info"] = err.Error()
			RenderJson(w, preout)
			return
		}
		//Network
		for k, network := range nconfig.Network {
			if !ValidHost(network.Addr) || !ValidHost(k) {
				preout["info"] = "Ping节点测试网络信息错误!(非法节点地址 " + k + ", 需为IPv4或域名)"
				RenderJson(w, preout)
				return
			}
			if network.Name == "" {
				preout["info"] = "Ping节点测试网络信息错误!( " + k + " 节点名称为空)"
				RenderJson(w, preout)
				return
			}
			for _, topology := range network.Topology {
				if _, ok := topology["Thdchecksec"]; !ok {
					preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，秒) "
					RenderJson(w, preout)
					return
				} else {
					Thdchecksec, err := strconv.Atoi(topology["Thdchecksec"])
					if err != nil || Thdchecksec <= 0 {
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，>0 秒  ) "
						RenderJson(w, preout)
						return
					}
				}
				if _, ok := topology["Thdloss"]; !ok {
					preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，%) "
					RenderJson(w, preout)
					return
				} else {
					Thdloss, err := strconv.Atoi(topology["Thdloss"])
					if err != nil || (Thdloss < 0 || Thdloss > 100) {
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，0 <= % <=100  ) "
						RenderJson(w, preout)
						return
					}
				}
				if _, ok := topology["Thdavgdelay"]; !ok {
					preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，ms) "
					RenderJson(w, preout)
					return
				} else {
					Thdavgdelay, err := strconv.Atoi(topology["Thdavgdelay"])
					if err != nil || Thdavgdelay <= 0 {
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，> 0 ms  ) "
						RenderJson(w, preout)
						return
					}
				}
				{
					chk := func(key string, min, max int, label string) bool {
						v, ok := topology[key]
						if !ok || v == "" {
							return true
						}
						n, err := strconv.Atoi(v)
						if err != nil || n < min || n > max {
							preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法" + label + ", " + strconv.Itoa(min) + "~" + strconv.Itoa(max) + " 或留空 ) "
							return false
						}
						return true
					}
					if !chk("Pinterval", 10, 60000, "链路探测间隔(ms)") || !chk("Pcount", 1, 1000, "链路探测包数") ||
						!chk("Ptimeout", 50, 10000, "链路单包超时(ms)") || !chk("Psize", 24, 1472, "链路探测包大小") {
						RenderJson(w, preout)
						return
					}
					effI, effC := nconfig.Base["Pinginterval"], nconfig.Base["Pingcount"]
					if v, err := strconv.Atoi(topology["Pinterval"]); err == nil && v > 0 {
						effI = v
					}
					if v, err := strconv.Atoi(topology["Pcount"]); err == nil && v > 0 {
						effC = v
					}
					if effI*effC > 60000 {
						preout["info"] = k + "->" + topology["Addr"] + " 链路探测参数无效: 每分钟总时长 间隔" + strconv.Itoa(effI) + "ms × " + strconv.Itoa(effC) + "包 = " + strconv.Itoa(effI*effC/1000) + "秒 > 60秒"
						RenderJson(w, preout)
						return
					}
					if effI*g.CyclePacketCount(effI, effC) > g.ProbeCycleMaxMs() {
						pc := g.CyclePacketCount(effI, effC)
						preout["info"] = k + "->" + topology["Addr"] + " 链路探测参数无效: 每" + strconv.Itoa(g.ProbeCycleSec) + "秒周期内 间隔" + strconv.Itoa(effI) + "ms × " + strconv.Itoa(pc) + "包 超时"
						RenderJson(w, preout)
						return
					}
				}
				if sip, ok := topology["Srcip"]; ok && sip != "" && !ValidIP4(sip) {
					preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法探测源IP, 需为本机网口的IPv4地址或留空 ) "
					RenderJson(w, preout)
					return
				}
				if tj, ok := topology["Thdjitter"]; ok && tj != "" {
					if jv, err := strconv.ParseFloat(tj, 64); err != nil || jv <= 0 {
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法抖动阈值, > 0 ms 或留空 ) "
						RenderJson(w, preout)
						return
					}
				}
				{
					sec, _ := strconv.Atoi(topology["Thdchecksec"])
					num, _ := strconv.Atoi(topology["Thdoccnum"])
					if num > 0 && sec/g.ProbeCycleSec < num {
						cap := sec / g.ProbeCycleSec
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 检测窗口太小: 窗口" + topology["Thdchecksec"] + "秒最多累计" + strconv.Itoa(cap) + "个采样点, 无法达到触发次数" + topology["Thdoccnum"] + "; 需满足 窗口秒数 ≥ 次数×" + strconv.Itoa(g.ProbeCycleSec) + " ) "
						RenderJson(w, preout)
						return
					}
				}
				if _, ok := topology["Thdoccnum"]; !ok {
					preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，次) "
					RenderJson(w, preout)
					return
				} else {
					Thdoccnum, err := strconv.Atoi(topology["Thdoccnum"])
					if err != nil || Thdoccnum <= 0 {
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，> 0 次  ) "
						RenderJson(w, preout)
						return
					}
				}
			}
		}
		//ChinaMap
		for _, provVal := range nconfig.Chinamap {
			for _, telcomVal := range provVal {
				for _, ip := range telcomVal {
					if ip != "" && !ValidHost(ip) {
						preout["info"] = "全球延迟探测地址非法(需为IPv4或域名): " + ip
						RenderJson(w, preout)
						return
					}
				}
			}
		}
		//Brand: 限制 Logo(通常为 base64 data URI)体积, 防止配置文件膨胀拖慢同步
		if nconfig.Brand != nil {
			if len(nconfig.Brand["Logo"]) > 512*1024 {
				preout["info"] = "品牌 Logo 图片过大, 请压缩到 256KB 以内!"
				RenderJson(w, preout)
				return
			}
		}
		nconfig.Ver = g.Cfg.Ver
		nconfig.Port = g.Cfg.Port
		// 密码(接入令牌)仅在显式提供时更新
		if nconfig.Password == "" {
			nconfig.Password = g.Cfg.Password
		}
		if nconfig.Alert["SendEmailPassword"] == "samepasswordasbefore" {
			nconfig.Alert["SendEmailPassword"] = g.Cfg.Alert["SendEmailPassword"]
		}
		if nconfig.OAuth != nil && nconfig.OAuth["ClientSecret"] == "samepasswordasbefore" {
			if g.Cfg.OAuth != nil {
				nconfig.OAuth["ClientSecret"] = g.Cfg.OAuth["ClientSecret"]
			}
		}
		// 通道密钥占位还原
		for i := range nconfig.Channels {
			np := nconfig.Channels[i].Params
			if np == nil {
				continue
			}
			var old map[string]string
			for _, och := range g.Cfg.Channels {
				if och.Name == nconfig.Channels[i].Name && och.Type == nconfig.Channels[i].Type {
					old = och.Params
					break
				}
			}
			if old == nil {
				continue
			}
			for _, k := range []string{"Token", "Webhook", "Url", "Secret", "AccessToken", "AppSecret"} {
				if np[k] == "samepasswordasbefore" {
					np[k] = old[k]
				}
			}
		}
		// 管理员保存属权威写入: 自增纪元, 使本次改动在集群内 LWW 收敛
		g.BumpEpochInPlace(&nconfig)
		g.CfgLock.Lock()
		g.Cfg = nconfig
		g.SelfCfg = g.Cfg.Network[g.Cfg.Addr]
		g.CfgLock.Unlock()
		saveerr := g.SaveConfig()
		if saveerr != nil {
			preout["info"] = saveerr.Error()
			RenderJson(w, preout)
			return
		}
		preout["status"] = "true"
		RenderJson(w, preout)
	})

	//导出配置备份(管理员): 下载当前完整配置 JSON, 便于离线留存与跨集群迁移
	http.HandleFunc("/api/config/export.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthAdmin(r) {
			deny(w)
			return
		}
		data, err := json.MarshalIndent(g.Cfg, "", "\t")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fname := "pingmesh-config-" + g.Cfg.Name + "-" + time.Now().Format("20060102-150405") + ".json"
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+fname+"\"")
		w.Write(data)
	})

	//测试告警通道
	http.HandleFunc("/api/alerttest.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthAdmin(r) {
			deny(w)
			return
		}
		preout := make(map[string]string)
		preout["status"] = "false"
		r.ParseForm()
		if len(r.Form["channel"]) == 0 {
			preout["info"] = "参数错误!"
			RenderJson(w, preout)
			return
		}
		ch := g.AlertChannel{}
		if err := json.Unmarshal([]byte(r.Form["channel"][0]), &ch); err != nil {
			preout["info"] = "通道配置解析错误: " + err.Error()
			RenderJson(w, preout)
			return
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		title := "【测试】ZENLENET PingMesh 告警通道测试"
		text := title + "\n时间: " + now + "\n节点: " + g.Cfg.Name + " (" + g.Cfg.Addr + ")\n如收到此消息说明通道配置正确。"
		payload := map[string]interface{}{
			"event": "test", "title": title, "content": text,
			"fromname": g.Cfg.Name, "fromip": g.Cfg.Addr, "time": now,
		}
		body, err := funcs.SendChannelMessage(ch, title, text, payload)
		if err != nil {
			preout["info"] = err.Error()
			if body != "" {
				preout["info"] += " | " + body
			}
			RenderJson(w, preout)
			return
		}
		preout["status"] = "true"
		preout["info"] = body
		RenderJson(w, preout)
	})

	//发送测试邮件
	http.HandleFunc("/api/sendmailtest.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthAdmin(r) {
			deny(w)
			return
		}
		preout := make(map[string]string)
		r.ParseForm()
		preout["status"] = "false"
		if len(r.Form["EmailHost"]) == 0 {
			preout["info"] = "邮件服务器不能为空!"
			RenderJson(w, preout)
			return
		}
		if len(r.Form["SendEmailAccount"]) == 0 {
			preout["info"] = "发件邮件不能为空!"
			RenderJson(w, preout)
			return
		}
		if len(r.Form["SendEmailPassword"]) == 0 {
			preout["info"] = "发件邮箱密码不能为空!"
			RenderJson(w, preout)
			return
		}
		if len(r.Form["RevcEmailList"]) == 0 {
			preout["info"] = "收件邮箱列表不能为空!"
			RenderJson(w, preout)
			return
		}

		err := funcs.SendMail(r.Form["SendEmailAccount"][0], r.Form["SendEmailPassword"][0], r.Form["EmailHost"][0], r.Form["RevcEmailList"][0], "报警测试邮件 - ZENLENET PingMesh", "报警测试邮件")
		if err != nil {
			preout["info"] = err.Error()
			RenderJson(w, preout)
			return
		}
		preout["status"] = "true"
		RenderJson(w, preout)
	})

	//代理访问: 仅允许访问本集群节点白名单 API, 防止 SSRF / 密钥套取
	http.HandleFunc("/api/proxy.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthData(r) {
			deny(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		r.ParseForm()
		if len(r.Form["g"]) == 0 {
			http.Error(w, "Url Param Error!", 406)
			return
		}
		url := strings.Replace(strings.Replace(r.Form["g"][0], "%26", "&", -1), " ", "%20", -1)
		if err := proxyAllowed(url); err != nil {
			http.Error(w, "Proxy Denied: "+err.Error(), 403)
			return
		}
		// proxy 以节点身份签名转发, 远端会当成可信节点; 写操作(确认/屏蔽)只允许管理员经此转发
		if proxyWritesTo(url) && (!AuthAdmin(r) || !sameOrigin(r)) {
			http.Error(w, "Proxy Denied: 需要管理员权限", 403)
			return
		}
		to := strconv.Itoa(g.Cfg.Base["Timeout"])
		if len(r.Form["t"]) > 0 {
			to = r.Form["t"][0]
		}
		defaultto, err := strconv.Atoi(to)
		if err != nil || defaultto <= 0 {
			http.Error(w, "Timeout Param Error!", 406)
			return
		}
		if defaultto > 30 {
			defaultto = 30
		}
		url = g.SignURL(url, g.Cfg.Password)
		// 不跟随跳转: 白名单只校验了首个地址, 跳转目标可能是任意内网/元数据地址
		client := http.Client{
			Timeout:       time.Duration(defaultto) * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
		resp, err := client.Get(url)
		if err != nil {
			http.Error(w, "Request Remote Data Error:"+err.Error(), 503)
			return
		}
		defer resp.Body.Close()
		resCode := resp.StatusCode
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 最多 8MB
		if err != nil {
			http.Error(w, "Read Remote Data Error:"+err.Error(), 503)
			return
		}
		if resCode != 200 {
			http.Error(w, "Get Remote Data Status Error", resCode)
			return
		}
		// 原样转发(不再重新缩进 JSON: 多 14% 体积和每次一次完整解析)
		w.Write(body)
	})

}
