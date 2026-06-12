package g

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"hash/fnv"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cihub/seelog"
	pingmesh "github.com/zenlenet/pingmesh"
)

var (
	Root string
	Cfg  Config
	//CLock	       sync.Mutex
	SelfCfg        NetworkMember
	AlertStatus    map[string]bool
	AuthUserIpMap  map[string]bool
	AuthAgentIpMap map[string]bool
	ToolLimit      map[string]int
	Db             *sql.DB
	DLock          sync.Mutex

	// 命令行参数(main 中赋值)
	FlagWorkDir string
	FlagPort    int
	FlagListen  string
	FlagWebUI   bool

	// FreshInstall 首次运行(尚无 config.json, 由内置默认配置初始化)
	FreshInstall bool

	// ToolLimit 并发访问保护
	ToolLimitLock sync.Mutex
	// 配置热更新保护(整体替换 Cfg/SelfCfg 时持有)
	CfgLock sync.Mutex
)

func IsExist(fp string) bool {
	_, err := os.Stat(fp)
	return err == nil || os.IsExist(err)
}

// healSkipPaths 配置自愈跳过的键:
//   - 顶层用户数据/身份信息(节点表、探测点、告警通道、本机身份、密码等)绝不从模板注入;
//   - Base.Apisign 升级时保持代码迁移默认(0, 兼容老集群), 与模板的全新安装默认(1)不同。
var healSkipPaths = map[string]bool{
	"Ver": true, "Name": true, "Addr": true, "Port": true,
	"Password": true, "Authiplist": true,
	"Network": true, "Chinamap": true, "Channels": true,
	"Base.Apisign": true,
}

// HealConfig 配置自愈: 以内嵌的 conf/config-base.json 为模板, 递归补全用户配置中
// 缺失的配置项(通常由二进制升级引入新功能开关导致), 不覆盖任何已有值。
// 返回补全后的 JSON 与新增键路径列表; 解析失败时原样返回, 交由后续流程报错。
func HealConfig(raw []byte) ([]byte, []string) {
	var user, base map[string]interface{}
	if err := json.Unmarshal(raw, &user); err != nil {
		return raw, nil
	}
	tpl, err := pingmesh.Assets.ReadFile("conf/config-base.json")
	if err != nil || json.Unmarshal(tpl, &base) != nil {
		return raw, nil
	}
	added := []string{}
	for k, v := range base {
		healMergeMissing(user, k, v, k, &added)
	}
	if len(added) == 0 {
		return raw, nil
	}
	sort.Strings(added)
	out, err := json.Marshal(user)
	if err != nil {
		return raw, nil
	}
	return out, added
}

// healMergeMissing 把模板里 dst 缺失的键补进去; 两边都是对象时逐层下钻
func healMergeMissing(dst map[string]interface{}, key string, val interface{}, path string, added *[]string) {
	if healSkipPaths[path] {
		return
	}
	cur, ok := dst[key]
	if !ok || cur == nil {
		dst[key] = val
		*added = append(*added, path)
		return
	}
	curMap, okCur := cur.(map[string]interface{})
	valMap, okVal := val.(map[string]interface{})
	if okCur && okVal {
		for k, v := range valMap {
			healMergeMissing(curMap, k, v, path+"."+k, added)
		}
	}
}

func GetRoot() string {
	if FlagWorkDir != "" {
		abs, err := filepath.Abs(FlagWorkDir)
		if err != nil {
			log.Fatal("Get Root Path Error:", err)
		}
		return strings.Replace(abs, "\\", "/", -1)
	}
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal("Get Root Path Error:", err)
	}
	dir = strings.Replace(dir, "\\", "/", -1)
	// 传统目录结构: 二进制位于 <root>/bin/ 下; 否则以二进制所在目录为工作目录(单文件部署)
	if filepath.Base(dir) == "bin" {
		return filepath.Dir(dir)
	}
	return dir
}

// ensureAssets 从二进制内嵌资源释放默认配置与前端文件。
// 前端资源以内容哈希为版本戳: 二进制升级后自动重新释放, 避免页面停留在旧版本。
func ensureAssets() {
	for _, d := range []string{"conf", "db", "logs", "html"} {
		os.MkdirAll(filepath.Join(Root, d), 0755)
	}
	for _, f := range []string{"conf/seelog.xml"} {
		dst := filepath.Join(Root, f)
		if !IsExist(dst) {
			if data, err := pingmesh.Assets.ReadFile(f); err == nil {
				os.WriteFile(dst, data, 0644)
				log.Println("[init] extracted", f)
			}
		}
	}
	// config-base.json 是默认配置模板(配置自愈的数据源), 随二进制升级保持最新
	if data, err := pingmesh.Assets.ReadFile("conf/config-base.json"); err == nil {
		dst := filepath.Join(Root, "conf", "config-base.json")
		if old, rerr := os.ReadFile(dst); rerr != nil || !bytes.Equal(old, data) {
			os.WriteFile(dst, data, 0644)
			log.Println("[init] refreshed conf/config-base.json template")
		}
	}
	stamp := embeddedHtmlHash()
	stampFile := filepath.Join(Root, "html", ".assets-ver")
	if old, err := os.ReadFile(stampFile); err == nil && string(old) == stamp && IsExist(filepath.Join(Root, "html", "index.html")) {
		return
	}
	cnt := 0
	fs.WalkDir(pingmesh.Assets, "html", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(Root, p)
		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}
		data, err := pingmesh.Assets.ReadFile(p)
		if err != nil {
			return err
		}
		cnt++
		return os.WriteFile(dst, data, 0644)
	})
	os.WriteFile(stampFile, []byte(stamp), 0644)
	log.Println("[init] extracted html assets:", cnt, "files (ver", stamp[:12], ")")
}

// embeddedHtmlHash 计算内嵌前端资源的内容哈希
func embeddedHtmlHash() string {
	h := fnv.New64a()
	fs.WalkDir(pingmesh.Assets, "html", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		h.Write([]byte(p))
		if data, err := pingmesh.Assets.ReadFile(p); err == nil {
			h.Write(data)
		}
		return nil
	})
	return strconv.FormatUint(h.Sum64(), 16)
}

// InitDbSchema 创建数据表(替代旧版随包分发的 database-base.db)
func InitDbSchema() {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS alertlog (
			logtime    VARCHAR (16),
			targetip   VARCHAR (16),
			targetname VARCHAR (15),
			tracert    TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS pinglog (
			logtime  VARCHAR (16),
			target   VARCHAR (15),
			maxdelay FLOAT,
			mindelay FLOAT,
			avgdelay FLOAT,
			sendpk   INT,
			revcpk   INT,
			losspk   INT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pinglog_target_time ON pinglog (target, logtime)`,
		`CREATE INDEX IF NOT EXISTS idx_pinglog_time ON pinglog (logtime)`,
		`CREATE TABLE IF NOT EXISTS mappinglog (logtime VARCHAR (16) PRIMARY KEY, mapjson TEXT)`,
		`CREATE TABLE IF NOT EXISTS alertmute (
			target     VARCHAR (64) PRIMARY KEY,
			reason     TEXT,
			muteduntil VARCHAR (32),
			createdby  VARCHAR (64),
			created_at VARCHAR (32)
		)`,
	}
	DLock.Lock()
	defer DLock.Unlock()
	for _, s := range stmts {
		if _, err := Db.Exec(s); err != nil {
			log.Fatalln("[Fault]db schema init fail.", err)
		}
	}
	// 告警确认字段(老库升级, 已存在时报错忽略)
	for _, s := range []string{
		`ALTER TABLE pinglog ADD COLUMN jitter FLOAT DEFAULT 0`,
		`ALTER TABLE alertlog ADD COLUMN ack INT DEFAULT 0`,
		`ALTER TABLE alertlog ADD COLUMN ackby VARCHAR(64)`,
		`ALTER TABLE alertlog ADD COLUMN ackreason TEXT`,
		`ALTER TABLE alertlog ADD COLUMN acktime VARCHAR(32)`,
	} {
		Db.Exec(s)
	}
}

func ParseConfig(ver string) {
	Root = GetRoot()
	ensureAssets()
	// seelog 中的日志路径为相对路径, 切换到工作目录保证日志落在 <root>/logs 下
	os.Chdir(Root)
	cfile := "config.json"
	if !IsExist(Root + "/conf/" + "config.json") {
		if !IsExist(Root + "/conf/" + "config-base.json") {
			log.Fatalln("[Fault]config file:", Root+"/conf/"+"config(-base).json", "both not existent.")
		}
		cfile = "config-base.json"
		FreshInstall = true
	}
	logger, err := seelog.LoggerFromConfigAsFile(Root + "/conf/" + "seelog.xml")
	if err != nil {
		log.Fatalln("[Fault]log config open fail .", err)
	}
	seelog.ReplaceLogger(logger)
	raw, err := os.ReadFile(Root + "/conf/" + cfile)
	if err != nil {
		log.Fatalln("[Fault]config file read fail.", err)
	}
	// 配置自愈: 升级后用内置模板补全新增配置项, 否则新功能因配置缺失无法启用
	healedKeys := []string{}
	if cfile == "config.json" {
		raw, healedKeys = HealConfig(raw)
	}
	Cfg = Config{}
	if err := json.Unmarshal(raw, &Cfg); err != nil {
		log.Fatalln("[Fault]config file parse fail.", err)
	}
	if Cfg.Name == "" {
		Cfg.Name, _ = os.Hostname()
	}
	if Cfg.Addr == "" {
		Cfg.Addr = "127.0.0.1"
	}
	Cfg.Ver = ver
	if FlagPort > 0 {
		Cfg.Port = FlagPort
	}
	// 旧配置迁移: 功能开关与探测参数默认值(Base 缺失时也要补齐, 前端依赖这些键)
	if Cfg.Base == nil {
		Cfg.Base = map[string]int{}
	}
	baseDefaults := map[string]int{
		"Archive":      10,
		"Refresh":      1,
		"Timeout":      5,
		"Chinamap":     1,
		"Pinginterval": 2500, // 包间隔(ms); 间隔×包数 ≤ 55秒(每分钟一轮)
		"Pingcount":    20,   // 每轮包数
		"Pingtimeout":  3000, // 单包超时(ms)
		"Pingsize":     56,   // 探测包大小(字节)
		"Apisign":      0,    // 老集群默认关闭, 全部升级并统一令牌后可开启
		"Remindmin":    60,   // 持续故障重复提醒间隔(分钟), 0=关闭; 默认每小时提醒未确认的持续故障
	}
	for k, v := range baseDefaults {
		if _, ok := Cfg.Base[k]; !ok {
			Cfg.Base[k] = v
		}
	}
	// 配置自愈: 修复非法探测参数组合。旧版默认 3000ms×20包=60秒 超过55秒上限,
	// 会导致前后端校验拒绝所有配置保存(看起来就是"升级后什么都改不了/启用不了")。
	if Cfg.Base["Pingcount"] < 1 || Cfg.Base["Pingcount"] > 1000 {
		Cfg.Base["Pingcount"] = 20
		healedKeys = append(healedKeys, "Base.Pingcount(fix)")
	}
	if Cfg.Base["Pinginterval"] < 10 || Cfg.Base["Pinginterval"] > 60000 {
		Cfg.Base["Pinginterval"] = 2500
		healedKeys = append(healedKeys, "Base.Pinginterval(fix)")
	}
	if Cfg.Base["Pinginterval"]*Cfg.Base["Pingcount"] > 55000 {
		// 保留用户的包数意图, 压缩间隔使一轮能在55秒内发完
		Cfg.Base["Pinginterval"] = 55000 / Cfg.Base["Pingcount"]
		if Cfg.Base["Pinginterval"] < 10 {
			Cfg.Base["Pinginterval"] = 2500
			Cfg.Base["Pingcount"] = 20
		}
		healedKeys = append(healedKeys, "Base.Pinginterval(fix: 间隔×包数>55s)")
	}
	// 集群容灾默认值: 纪元从 0 起, 默认开启自动容灾(主挂时备选自动接管)
	if Cfg.Mode == nil {
		Cfg.Mode = map[string]string{}
	}
	if _, ok := Cfg.Mode["Epoch"]; !ok {
		Cfg.Mode["Epoch"] = "0"
	}
	if _, ok := Cfg.Mode["MasterAuto"]; !ok {
		Cfg.Mode["MasterAuto"] = "true"
	}
	// 配置自愈: Network 缺失/为空会让前端拿到 null 直接崩页面, 补全为含本机的最小拓扑
	if Cfg.Network == nil {
		Cfg.Network = map[string]NetworkMember{}
	}
	if _, ok := Cfg.Network[Cfg.Addr]; !ok && Cfg.Addr != "" {
		Cfg.Network[Cfg.Addr] = NetworkMember{
			Name: Cfg.Name, Addr: Cfg.Addr, Pingmesh: true,
			Ping: []string{}, Topology: []map[string]string{},
		}
		log.Println("[init] config healed: added self node", Cfg.Addr, "to empty network")
	}
	if Cfg.Alert == nil {
		Cfg.Alert = map[string]string{}
	}
	if Cfg.Chinamap == nil {
		Cfg.Chinamap = map[string]map[string][]string{}
	}
	// 旧配置迁移: 拓扑展示参数默认值(缺省或空 map 时补齐, 避免前端拓扑页无法渲染)
	if Cfg.Topology == nil {
		Cfg.Topology = map[string]string{}
	}
	topoDefaults := map[string]string{
		"Tline":       "1",
		"Tsound":      "/alert-soft.wav",
		"Tsymbolsize": "70",
	}
	for k, v := range topoDefaults {
		if Cfg.Topology[k] == "" {
			Cfg.Topology[k] = v
		}
	}
	if Cfg.Topology["Tsound"] == "/alert.mp3" {
		Cfg.Topology["Tsound"] = "/alert-soft.wav"
	}
	// 配置自愈落盘: 补全/修复的配置写回 config.json, 页面「高级 JSON」与后续升级都能看到完整配置
	if len(healedKeys) > 0 {
		log.Println("[init] config healed:", strings.Join(healedKeys, ", "))
		if err := SaveConfig(); err != nil {
			log.Println("[init] config heal persist fail:", err)
		}
	}
	seelog.Info("Config loaded")
	// PRAGMA 写入 DSN: 连接池中每条连接都生效(busy_timeout/synchronous 为连接级,
	// 旧写法仅作用于单条连接, 池化后其余连接仍可能 database is locked)。
	dsn := "file:" + Root + "/db/database.db" +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
	Db, err = sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalln("[Fault]db open fail .", err)
	}
	// 连接池上限: 兼顾读并发与 SQLite 写串行, 避免句柄无限增长
	Db.SetMaxOpenConns(8)
	Db.SetMaxIdleConns(8)
	Db.SetConnMaxIdleTime(5 * time.Minute)
	InitDbSchema()
	SelfCfg = Cfg.Network[Cfg.Addr]
	AlertStatus = map[string]bool{}
	ToolLimit = map[string]int{}
	saveAuth()
	InitUserTable()
}

func SaveCloudConfig(url string) (Config, error) {
	config := Config{}
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	// 节点间同步: 携带 HMAC 签名并请求加密载荷
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	resp, err := client.Get(SignURL(url+sep+"enc=1", Cfg.Password))
	if err != nil {
		return config, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if IsEncryptedPayload(body) {
		body, err = DecryptPayload(body, Cfg.Password)
		if err != nil {
			return config, errors.New("配置解密失败(各节点接入令牌需一致): " + err.Error())
		}
	}
	err = json.Unmarshal(body, &config)
	if err != nil {
		config.Name = string(body)
		return config, err
	}
	// 远端配置完整性校验: 残缺配置(空拓扑)一律拒绝采纳, 防止覆盖本地好配置后全网扩散
	if len(config.Network) == 0 {
		return config, errors.New("远端配置不完整(Network 为空), 拒绝采纳")
	}
	CfgLock.Lock()
	defer CfgLock.Unlock()
	Name := Cfg.Name
	Addr := Cfg.Addr
	Ver := Cfg.Ver
	Password := Cfg.Password
	Port := Cfg.Port
	Endpoint := Cfg.Mode["Endpoint"]
	Cfg = config
	Cfg.Name = Name
	Cfg.Addr = Addr
	// 集群模式以主节点的节点列表为命名权威: 主节点改名后 Agent 自动采用
	if m, ok := Cfg.Network[Addr]; ok && m.Name != "" {
		Cfg.Name = m.Name
	}
	Cfg.Ver = Ver
	Cfg.Port = Port
	Cfg.Password = Password
	Cfg.Mode["LastSuccTime"] = time.Now().Format("2006-01-02 15:04:05")
	Cfg.Mode["Status"] = "true"
	Cfg.Mode["Endpoint"] = Endpoint
	Cfg.Mode["Type"] = "cloud"
	SelfCfg = Cfg.Network[Cfg.Addr]
	saveAuth()
	return config, nil
}

func SaveConfig() error {
	saveAuth()
	rrs, _ := json.Marshal(Cfg)
	var out bytes.Buffer
	errjson := json.Indent(&out, rrs, "", "\t")
	if errjson != nil {
		seelog.Error("[func:SaveConfig] Json Parse ", errjson)
		return errjson
	}
	err := os.WriteFile(Root+"/conf/"+"config.json", []byte(out.String()), 0644)
	if err != nil {
		seelog.Error("[func:SaveConfig] Config File Write", err)
		return err
	}
	// 自动快照: 保留最近若干份配置历史, 误改/故障后可回滚, 兼作本地灾备
	go snapshotConfig([]byte(out.String()))
	return nil
}

const maxConfigSnapshots = 30

// snapshotConfig 将配置写入 conf/backups/ 并滚动保留最近 maxConfigSnapshots 份
func snapshotConfig(data []byte) {
	dir := filepath.Join(Root, "conf", "backups")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	name := "config-" + time.Now().Format("20060102-150405") + ".json"
	if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		seelog.Error("[func:snapshotConfig] write snapshot", err)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "config-") && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	if len(names) <= maxConfigSnapshots {
		return
	}
	sort.Strings(names) // 文件名按时间戳排序, 删除最旧的
	for _, old := range names[:len(names)-maxConfigSnapshots] {
		os.Remove(filepath.Join(dir, old))
	}
}

func saveAuth() {
	AuthUserIpMap = map[string]bool{}
	AuthAgentIpMap = map[string]bool{}
	for _, k := range Cfg.Network {
		AuthAgentIpMap[k.Addr] = true
		// 域名节点: 解析出的 IP 一并加入互信表(请求来源是 IP)
		if net.ParseIP(k.Addr) == nil {
			if ips, err := net.LookupHost(k.Addr); err == nil {
				for _, ip := range ips {
					AuthAgentIpMap[ip] = true
				}
			}
		}
	}
	Cfg.Authiplist = strings.Replace(Cfg.Authiplist, " ", "", -1)
	if Cfg.Authiplist != "" {
		authiplist := strings.Split(Cfg.Authiplist, ",")
		for _, ip := range authiplist {
			AuthUserIpMap[ip] = true
		}
	}
}
