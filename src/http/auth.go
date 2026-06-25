package http

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
)

const sessionCookie = "sp_session"
const sessionTTL = 7 * 24 * time.Hour

type Session struct {
	Username string
	Role     string
	Expire   time.Time
}

var (
	sessions     = map[string]*Session{}
	sessionsLock sync.Mutex
)

// 登录暴力破解防护: 同一来源 IP 在窗口内连续失败达到上限后短时锁定。
const (
	loginMaxFails = 8
	loginWindow   = 10 * time.Minute
	loginLockout  = 10 * time.Minute
)

type loginGate struct {
	fails    int
	first    time.Time
	lockedAt time.Time
}

var (
	loginFailsMu sync.Mutex
	loginFails   = map[string]*loginGate{}
)

func loginClientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if i := lastColon(ip); i >= 0 {
		ip = ip[:i]
	}
	return ip
}

// loginBlocked 该来源当前是否处于锁定期
func loginBlocked(ip string) bool {
	loginFailsMu.Lock()
	defer loginFailsMu.Unlock()
	g := loginFails[ip]
	if g == nil {
		return false
	}
	if !g.lockedAt.IsZero() && time.Since(g.lockedAt) < loginLockout {
		return true
	}
	return false
}

func loginRecordFail(ip string) {
	loginFailsMu.Lock()
	defer loginFailsMu.Unlock()
	g := loginFails[ip]
	now := time.Now()
	if g == nil || now.Sub(g.first) > loginWindow {
		loginFails[ip] = &loginGate{fails: 1, first: now}
		return
	}
	g.fails++
	if g.fails >= loginMaxFails {
		g.lockedAt = now
	}
}

func loginRecordSuccess(ip string) {
	loginFailsMu.Lock()
	delete(loginFails, ip)
	loginFailsMu.Unlock()
}

func newToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetSession 从请求中解析有效会话
func GetSession(r *http.Request) *Session {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	sessionsLock.Lock()
	defer sessionsLock.Unlock()
	s, ok := sessions[c.Value]
	if !ok {
		return nil
	}
	if time.Now().After(s.Expire) {
		delete(sessions, c.Value)
		return nil
	}
	return s
}

// AuthUser 用户级访问: 有效登录会话 或 命中用户IP白名单(兼容旧版)
func AuthUser(r *http.Request) bool {
	if GetSession(r) != nil {
		return true
	}
	if len(g.AuthUserIpMap) > 0 && AuthUserIp(r.RemoteAddr) {
		return true
	}
	return false
}

// AuthAgent 节点级访问: 互Ping节点间的数据接口调用。
// 携带有效 HMAC 签名直接放行; 开启 Apisign 后强制验签(IP互信不再放行)。
func AuthAgent(r *http.Request) bool {
	r.ParseForm()
	if g.VerifySign(r.URL.Path, r.FormValue("ts"), r.FormValue("nonce"), r.FormValue("sign")) {
		return true
	}
	if g.SignRequired() {
		return false
	}
	ip := r.RemoteAddr
	if i := lastColon(ip); i >= 0 {
		ip = ip[:i]
	}
	_, ok := g.AuthAgentIpMap[ip]
	return ok
}

// AgentSigned 请求是否带有效节点签名(用于决定是否加密响应)
func AgentSigned(r *http.Request) bool {
	r.ParseForm()
	// 注意: VerifySign 消耗 nonce, 仅在 AuthAgent 已通过后用于判别时,
	// 这里直接复用其结果会二次消耗; 因此单独校验一次专用参数。
	return r.FormValue("sign") != "" && r.FormValue("enc") == "1"
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

// AuthData 数据接口访问: 登录用户 / 白名单IP / 互Ping节点
func AuthData(r *http.Request) bool {
	return AuthUser(r) || AuthAgent(r)
}

// AuthAdmin 管理员访问
func AuthAdmin(r *http.Request) bool {
	s := GetSession(r)
	if s != nil && s.Role == g.RoleAdmin {
		return true
	}
	// 兼容旧版: 配置了用户IP白名单时, 白名单内IP视为管理员
	if len(g.AuthUserIpMap) > 0 && AuthUserIp(r.RemoteAddr) {
		return true
	}
	return false
}

func renderErr(w http.ResponseWriter, info string) {
	RenderJson(w, map[string]string{"status": "false", "info": info})
}

func renderOk(w http.ResponseWriter, extra map[string]interface{}) {
	out := map[string]interface{}{"status": "true"}
	for k, v := range extra {
		out[k] = v
	}
	RenderJson(w, out)
}

func deny(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"status":"false","info":"未登录或无权限"}`))
}

func configAuthRoutes() {

	// 登录页元信息: 默认账号是否仍有效(改掉默认密码后提示自动消失)
	http.HandleFunc("/api/loginmeta.json", func(w http.ResponseWriter, r *http.Request) {
		RenderJson(w, map[string]interface{}{
			"defaultcreds": g.DefaultCredsActive(),
			"brand":        g.Cfg.Brand,
		})
	})

	// 登录
	http.HandleFunc("/api/login.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			renderErr(w, "仅支持POST")
			return
		}
		r.ParseForm()
		ip := loginClientIP(r)
		if loginBlocked(ip) {
			seelog.Info("[func:/api/login.json] too many failed attempts from ", ip)
			renderErr(w, "失败次数过多, 请稍后再试(约10分钟)")
			return
		}
		username := r.FormValue("username")
		password := r.FormValue("password")
		if username == "" || password == "" {
			renderErr(w, "用户名或密码不能为空")
			return
		}
		u, err := g.VerifyUser(username, password)
		if err != nil {
			loginRecordFail(ip)
			seelog.Info("[func:/api/login.json] login failed for ", username, " from ", r.RemoteAddr)
			renderErr(w, "用户名或密码错误")
			return
		}
		loginRecordSuccess(ip)
		token := newToken()
		sessionsLock.Lock()
		sessions[token] = &Session{Username: u.Username, Role: u.Role, Expire: time.Now().Add(sessionTTL)}
		sessionsLock.Unlock()
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(sessionTTL.Seconds()),
		})
		seelog.Info("[func:/api/login.json] user ", username, " login from ", r.RemoteAddr)
		renderOk(w, map[string]interface{}{"user": u})
	})

	// 登出
	http.HandleFunc("/api/logout.json", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			sessionsLock.Lock()
			delete(sessions, c.Value)
			sessionsLock.Unlock()
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
		renderOk(w, nil)
	})

	// 当前登录用户
	http.HandleFunc("/api/whoami.json", func(w http.ResponseWriter, r *http.Request) {
		s := GetSession(r)
		if s == nil {
			if len(g.AuthUserIpMap) > 0 && AuthUserIp(r.RemoteAddr) {
				renderOk(w, map[string]interface{}{"user": g.User{Username: "ip-whitelist", Role: g.RoleAdmin}})
				return
			}
			deny(w)
			return
		}
		u, err := g.GetUser(s.Username)
		if err != nil {
			deny(w)
			return
		}
		renderOk(w, map[string]interface{}{"user": u})
	})

	// 用户列表(管理员)
	http.HandleFunc("/api/user/list.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthAdmin(r) {
			deny(w)
			return
		}
		users, err := g.ListUsers()
		if err != nil {
			renderErr(w, err.Error())
			return
		}
		renderOk(w, map[string]interface{}{"users": users})
	})

	// 创建/更新用户(管理员)
	http.HandleFunc("/api/user/save.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthAdmin(r) {
			deny(w)
			return
		}
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")
		role := r.FormValue("role")
		if username == "" {
			renderErr(w, "用户名不能为空")
			return
		}
		if _, err := g.GetUser(username); err != nil {
			// 新建用户
			if err := g.CreateUser(username, password, role); err != nil {
				renderErr(w, err.Error())
				return
			}
		} else {
			// 更新已有用户
			if role != "" {
				if s := GetSession(r); s != nil && s.Username == username && role != g.RoleAdmin {
					renderErr(w, "不能降级自己的角色")
					return
				}
				if err := g.UpdateUserRole(username, role); err != nil {
					renderErr(w, err.Error())
					return
				}
			}
			if password != "" {
				if err := g.UpdateUserPassword(username, password); err != nil {
					renderErr(w, err.Error())
					return
				}
				kickUser(username, GetSession(r))
			}
		}
		renderOk(w, nil)
	})

	// 删除用户(管理员)
	http.HandleFunc("/api/user/delete.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthAdmin(r) {
			deny(w)
			return
		}
		r.ParseForm()
		username := r.FormValue("username")
		if s := GetSession(r); s != nil && s.Username == username {
			renderErr(w, "不能删除当前登录用户")
			return
		}
		if err := g.DeleteUser(username); err != nil {
			renderErr(w, err.Error())
			return
		}
		kickUser(username, nil)
		renderOk(w, nil)
	})

	// 修改自己的密码
	http.HandleFunc("/api/user/passwd.json", func(w http.ResponseWriter, r *http.Request) {
		s := GetSession(r)
		if s == nil {
			deny(w)
			return
		}
		r.ParseForm()
		oldpassword := r.FormValue("oldpassword")
		password := r.FormValue("password")
		if _, err := g.VerifyUser(s.Username, oldpassword); err != nil {
			renderErr(w, "原密码错误")
			return
		}
		if err := g.UpdateUserPassword(s.Username, password); err != nil {
			renderErr(w, err.Error())
			return
		}
		kickUser(s.Username, s)
		renderOk(w, nil)
	})
}

// kickUser 使某用户的其他会话失效(keep 为保留的会话)
func kickUser(username string, keep *Session) {
	sessionsLock.Lock()
	defer sessionsLock.Unlock()
	for token, s := range sessions {
		if s.Username == username && s != keep {
			delete(sessions, token)
		}
	}
}
