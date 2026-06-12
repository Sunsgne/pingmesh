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

func ReadConfig(filename string) Config {
	config := Config{}
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		log.Fatal("Config Not Found!")
	} else {
		err = json.NewDecoder(file).Decode(&config)
		if err != nil {
			log.Fatal(err)
		}
	}
	return config
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
	for _, f := range []string{"conf/seelog.xml", "conf/config-base.json"} {
		dst := filepath.Join(Root, f)
		if !IsExist(dst) {
			if data, err := pingmesh.Assets.ReadFile(f); err == nil {
				os.WriteFile(dst, data, 0644)
				log.Println("[init] extracted", f)
			}
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
	Cfg = ReadConfig(Root + "/conf/" + cfile)
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
	// 旧配置迁移: 功能开关与探测参数默认值
	if Cfg.Base != nil {
		baseDefaults := map[string]int{
			"Chinamap":     1,
			"Pinginterval": 3000, // 包间隔(ms)
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
	// 旧配置迁移: 已删除的旧提示音文件改为内置柔和提示音
	if Cfg.Topology != nil && Cfg.Topology["Tsound"] == "/alert.mp3" {
		Cfg.Topology["Tsound"] = "/alert-soft.wav"
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
