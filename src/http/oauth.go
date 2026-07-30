package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cihub/seelog"
	"github.com/zenlenet/pingmesh/src/g"
	"golang.org/x/oauth2"
)

const oauthStateCookie = "sp_oauth_state"

type oauthStateEntry struct {
	Provider string
	Expire   time.Time
}

var (
	oauthStates     = map[string]*oauthStateEntry{}
	oauthStatesLock sync.Mutex
)

func requestScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return strings.ToLower(strings.TrimSpace(proto))
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func requestHost(r *http.Request) string {
	if host := r.Header.Get("X-Forwarded-Host"); host != "" {
		return strings.TrimSpace(strings.Split(host, ",")[0])
	}
	return r.Host
}

func oauthRedirectBase(r *http.Request) string {
	return requestScheme(r) + "://" + requestHost(r)
}

func oauthEnabled() bool {
	o := g.Cfg.OAuth
	return o != nil && (o["Enabled"] == "1" || strings.EqualFold(o["Enabled"], "true"))
}

func oauthAutoCreate() bool {
	o := g.Cfg.OAuth
	if o == nil {
		return false
	}
	return o["AutoCreate"] == "1" || strings.EqualFold(o["AutoCreate"], "true")
}

func oauthDefaultRole() string {
	role := "viewer"
	if g.Cfg.OAuth != nil && g.Cfg.OAuth["DefaultRole"] != "" {
		role = g.Cfg.OAuth["DefaultRole"]
	}
	if role != g.RoleAdmin && role != g.RoleViewer {
		role = g.RoleViewer
	}
	return role
}

func oauthAllowedDomains() []string {
	raw := ""
	if g.Cfg.OAuth != nil {
		raw = g.Cfg.OAuth["AllowedDomains"]
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func microsoftOAuthConfig(r *http.Request) (*oauth2.Config, error) {
	o := g.Cfg.OAuth
	if o == nil || strings.TrimSpace(o["ClientId"]) == "" || strings.TrimSpace(o["ClientSecret"]) == "" {
		return nil, fmt.Errorf("Microsoft OAuth 未配置 ClientId/ClientSecret")
	}
	tenant := strings.TrimSpace(o["TenantId"])
	if tenant == "" {
		tenant = "organizations"
	}
	redirectURL := oauthRedirectBase(r) + "/api/oauth/microsoft/callback"
	return &oauth2.Config{
		ClientID:     strings.TrimSpace(o["ClientId"]),
		ClientSecret: strings.TrimSpace(o["ClientSecret"]),
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "profile", "email", "User.Read"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", url.PathEscape(tenant)),
			TokenURL: fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", url.PathEscape(tenant)),
		},
	}, nil
}

func putOAuthState(token, provider string) {
	oauthStatesLock.Lock()
	defer oauthStatesLock.Unlock()
	now := time.Now()
	for k, v := range oauthStates {
		if now.After(v.Expire) {
			delete(oauthStates, k)
		}
	}
	oauthStates[token] = &oauthStateEntry{Provider: provider, Expire: now.Add(10 * time.Minute)}
}

func takeOAuthState(token, provider string) bool {
	oauthStatesLock.Lock()
	defer oauthStatesLock.Unlock()
	entry, ok := oauthStates[token]
	if !ok || entry.Provider != provider || time.Now().After(entry.Expire) {
		return false
	}
	delete(oauthStates, token)
	return true
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, username, role string) {
	token := newToken()
	sessionsLock.Lock()
	sessions[token] = &Session{Username: username, Role: role, Expire: time.Now().Add(sessionTTL)}
	sessionsLock.Unlock()
	secure := requestScheme(r) == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

type msGraphMe struct {
	Mail              string `json:"mail"`
	UserPrincipalName string `json:"userPrincipalName"`
	DisplayName       string `json:"displayName"`
}

func microsoftUserEmail(me *msGraphMe) string {
	email := strings.TrimSpace(me.Mail)
	if email == "" {
		email = strings.TrimSpace(me.UserPrincipalName)
	}
	return strings.ToLower(email)
}

func fetchMicrosoftProfile(ctx context.Context, client *http.Client) (*msGraphMe, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Microsoft Graph 返回 %d: %s", resp.StatusCode, string(body))
	}
	me := &msGraphMe{}
	if err := json.Unmarshal(body, me); err != nil {
		return nil, err
	}
	return me, nil
}

func oauthLoginErrorRedirect(w http.ResponseWriter, r *http.Request, msg string) {
	seelog.Info("[oauth] login failed: ", msg, " from ", r.RemoteAddr)
	u := "/login.html?oauth_error=" + url.QueryEscape(msg)
	http.Redirect(w, r, u, http.StatusFound)
}

func configOAuthRoutes() {
	http.HandleFunc("/api/oauth/microsoft/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if !oauthEnabled() {
			oauthLoginErrorRedirect(w, r, "Microsoft 登录未启用")
			return
		}
		cfg, err := microsoftOAuthConfig(r)
		if err != nil {
			oauthLoginErrorRedirect(w, r, err.Error())
			return
		}
		state := newToken()
		putOAuthState(state, "microsoft")
		secure := requestScheme(r) == "https"
		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookie,
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   600,
		})
		authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
		http.Redirect(w, r, authURL, http.StatusFound)
	})

	http.HandleFunc("/api/oauth/microsoft/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if !oauthEnabled() {
			oauthLoginErrorRedirect(w, r, "Microsoft 登录未启用")
			return
		}
		errMsg := r.URL.Query().Get("error_description")
		if errMsg == "" {
			errMsg = r.URL.Query().Get("error")
		}
		if errMsg != "" {
			oauthLoginErrorRedirect(w, r, errMsg)
			return
		}
		state := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")
		if state == "" || code == "" {
			oauthLoginErrorRedirect(w, r, "授权参数缺失")
			return
		}
		cookieState := ""
		if c, err := r.Cookie(oauthStateCookie); err == nil {
			cookieState = c.Value
		}
		if cookieState == "" || cookieState != state || !takeOAuthState(state, "microsoft") {
			oauthLoginErrorRedirect(w, r, "授权状态无效或已过期, 请重试")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/", MaxAge: -1})

		cfg, err := microsoftOAuthConfig(r)
		if err != nil {
			oauthLoginErrorRedirect(w, r, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		tok, err := cfg.Exchange(ctx, code)
		if err != nil {
			oauthLoginErrorRedirect(w, r, "获取访问令牌失败")
			return
		}
		me, err := fetchMicrosoftProfile(ctx, cfg.Client(ctx, tok))
		if err != nil {
			oauthLoginErrorRedirect(w, r, "读取 Microsoft 账户信息失败")
			return
		}
		email := microsoftUserEmail(me)
		if email == "" {
			oauthLoginErrorRedirect(w, r, "Microsoft 账户未返回可用邮箱")
			return
		}
		u, err := g.GetOrCreateOAuthUser(email, me.DisplayName, oauthAutoCreate(), oauthDefaultRole(), oauthAllowedDomains())
		if err != nil {
			oauthLoginErrorRedirect(w, r, err.Error())
			return
		}
		setSessionCookie(w, r, u.Username, u.Role)
		seelog.Info("[oauth] microsoft login success: ", u.Username, " from ", r.RemoteAddr)
		http.Redirect(w, r, "/index.html", http.StatusFound)
	})
}
