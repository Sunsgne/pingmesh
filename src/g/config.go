package g

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cihub/seelog"
	smartping "github.com/smartping/smartping"
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

// ensureAssets 首次启动时从二进制内嵌资源释放默认配置与前端文件
func ensureAssets() {
	for _, d := range []string{"conf", "db", "logs", "html"} {
		os.MkdirAll(filepath.Join(Root, d), 0755)
	}
	for _, f := range []string{"conf/seelog.xml", "conf/config-base.json"} {
		dst := filepath.Join(Root, f)
		if !IsExist(dst) {
			if data, err := smartping.Assets.ReadFile(f); err == nil {
				ioutil.WriteFile(dst, data, 0644)
				log.Println("[init] extracted", f)
			}
		}
	}
	if !IsExist(filepath.Join(Root, "html", "index.html")) {
		cnt := 0
		fs.WalkDir(smartping.Assets, "html", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			dst := filepath.Join(Root, p)
			if d.IsDir() {
				return os.MkdirAll(dst, 0755)
			}
			data, err := smartping.Assets.ReadFile(p)
			if err != nil {
				return err
			}
			cnt++
			return ioutil.WriteFile(dst, data, 0644)
		})
		log.Println("[init] extracted html assets:", cnt, "files")
	}
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
		`CREATE TABLE IF NOT EXISTS mappinglog (logtime VARCHAR (16) PRIMARY KEY, mapjson TEXT)`,
	}
	DLock.Lock()
	defer DLock.Unlock()
	for _, s := range stmts {
		if _, err := Db.Exec(s); err != nil {
			log.Fatalln("[Fault]db schema init fail.", err)
		}
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
	// 旧配置迁移: 全国延迟开关默认开启
	if Cfg.Base != nil {
		if _, ok := Cfg.Base["Chinamap"]; !ok {
			Cfg.Base["Chinamap"] = 1
		}
	}
	// 兼容旧版: 存在 database-base.db 时沿用拷贝方式, 否则由代码建表
	if !IsExist(Root+"/db/"+"database.db") && IsExist(Root+"/db/"+"database-base.db") {
		src, err := os.Open(Root + "/db/" + "database-base.db")
		if err != nil {
			log.Fatalln("[Fault]db-base file open error.")
		}
		defer src.Close()
		dst, err := os.OpenFile(Root+"/db/"+"database.db", os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			log.Fatalln("[Fault]db-base file copy error.")
		}
		defer dst.Close()
		io.Copy(dst, src)
	}
	seelog.Info("Config loaded")
	Db, err = sql.Open("sqlite3", Root+"/db/database.db")
	if err != nil {
		log.Fatalln("[Fault]db open fail .", err)
	}
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
	resp, err := client.Get(url)
	if err != nil {
		return config, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &config)
	if err != nil {
		config.Name = string(body)
		return config, err
	}
	Name := Cfg.Name
	Addr := Cfg.Addr
	Ver := Cfg.Ver
	Password := Cfg.Password
	Port := Cfg.Port
	Endpoint := Cfg.Mode["Endpoint"]
	Cfg = config
	Cfg.Name = Name
	Cfg.Addr = Addr
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
	err := ioutil.WriteFile(Root+"/conf/"+"config.json", []byte(out.String()), 0644)
	if err != nil {
		seelog.Error("[func:SaveConfig] Config File Write", err)
		return err
	}
	return nil
}

func saveAuth() {
	AuthUserIpMap = map[string]bool{}
	AuthAgentIpMap = map[string]bool{}
	for _, k := range Cfg.Network {
		AuthAgentIpMap[k.Addr] = true
	}
	Cfg.Authiplist = strings.Replace(Cfg.Authiplist, " ", "", -1)
	if Cfg.Authiplist != "" {
		authiplist := strings.Split(Cfg.Authiplist, ",")
		for _, ip := range authiplist {
			AuthUserIpMap[ip] = true
		}
	}
}
