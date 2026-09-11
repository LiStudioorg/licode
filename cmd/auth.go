package cmd

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"licode/internal/settings"
	"licode/internal/web"
)

// 登录常量。
const (
	DefaultUsername = "licode"
	EnvUsername     = "LICODE_USERNAME"
	EnvPassword     = "LICODE_PASSWORD"
	SessionCookie   = "licode_auth"
	sessionLifetime = 7 * 24 * time.Hour
	csrfCookie      = "licode_csrf"
)

// authState 是登录认证状态（基于会话 Cookie + HMAC 签名）。
type authState struct {
	user     string
	pass     string
	enabled  bool
	secret   []byte
	throttle *loginThrottle
}

// 登录限流：单 IP 连续失败上限与锁定窗口，降低暴力破解风险。
const (
	maxLoginFails = 8
	lockWindow    = 5 * time.Minute
)

type failEntry struct {
	count int
	until time.Time
}

type loginThrottle struct {
	mu    sync.Mutex
	fails map[string]*failEntry
}

func newLoginThrottle() *loginThrottle {
	return &loginThrottle{fails: map[string]*failEntry{}}
}

func (t *loginThrottle) allowed(ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.fails[ip]
	if e == nil {
		return true
	}
	if time.Now().After(e.until) {
		delete(t.fails, ip)
		return true
	}
	return e.count < maxLoginFails
}

func (t *loginThrottle) fail(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.fails[ip]
	if e == nil {
		e = &failEntry{}
		t.fails[ip] = e
	}
	e.count++
	if e.count >= maxLoginFails {
		e.until = time.Now().Add(lockWindow)
	}
}

func (t *loginThrottle) success(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.fails, ip)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ResolveAuth 解析用户名与密码：环境变量优先，未设置时用户名默认 licode。
// 返回 (用户名, 密码, 是否启用登录)。未设置密码时登录关闭。
func ResolveAuth(username, password string) (string, string, bool) {
	if username == "" {
		username = os.Getenv(EnvUsername)
	}
	if username == "" {
		username = DefaultUsername
	}
	if password == "" {
		password = os.Getenv(EnvPassword)
	}
	return username, password, password != ""
}

// newAuthState 构造认证状态。HMAC 密钥持久化在 ~/.licode/session.key，
// 保证会话 cookie 在服务器重启后仍然有效（自动登录）。
func newAuthState(user, pass string, enabled bool) *authState {
	return &authState{user: user, pass: pass, enabled: enabled, secret: loadSecret(), throttle: newLoginThrottle()}
}

// loadSecret 读取或生成持久化会话密钥。
func loadSecret() []byte {
	path := filepath.Join(settings.BaseDir(), "session.key")
	if data, err := os.ReadFile(path); err == nil && len(data) >= 32 {
		return data
	}
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, secret, 0o600)
	return secret
}

// issueToken 签发签名会话令牌：base64url(用户名.过期时间戳.签名)。
func (a *authState) issueToken(user string, exp time.Time) string {
	raw := user + "." + strconv.FormatInt(exp.Unix(), 10)
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(raw))
	sig := hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(raw + "." + sig))
}

// verifyToken 校验令牌，返回用户名。
func (a *authState) verifyToken(token string) (string, bool) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", false
	}
	parts := strings.Split(string(b), ".")
	if len(parts) != 3 {
		return "", false
	}
	user, expStr, sig := parts[0], parts[1], parts[2]
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", false
	}
	if time.Now().Unix() > exp {
		return "", false
	}
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(user + "." + expStr))
	if !hmac.Equal(mac.Sum(nil), mustHex(sig)) {
		return "", false
	}
	return user, true
}

func (a *authState) setSession(w http.ResponseWriter, user string, secure bool) {
	tok := a.issueToken(user, time.Now().Add(sessionLifetime))
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionLifetime.Seconds()),
	})
}

func (a *authState) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
}

// authed 返回当前请求的登录用户名（未登录返回空）。
func (a *authState) authed(r *http.Request) string {
	if !a.enabled {
		return a.user
	}
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	u, ok := a.verifyToken(c.Value)
	if !ok {
		return ""
	}
	return u
}

// require 校验请求是否已登录。未登录时：页面跳转 /login，接口/WS 返回 401。
// 返回 true 表示允许继续处理。
func (a *authState) require(w http.ResponseWriter, r *http.Request) bool {
	if !a.enabled {
		return true
	}
	if a.authed(r) != "" {
		return true
	}
	if r.URL.Path == "/login" {
		return true
	}
	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("401 未登录"))
		return false
	}
	http.Redirect(w, r, "/login", http.StatusFound)
	return false
}

func isAPIRequest(r *http.Request) bool {
	if r.URL.Path == "/ws" {
		return true
	}
	if r.Header.Get("Accept") == "application/json" {
		return true
	}
	return false
}

// handleLogin 处理登录页与登录提交。
func (a *authState) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !a.enabled {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if r.Method == http.MethodPost {
		ip := clientIP(r)
		if !a.throttle.allowed(ip) {
			http.Redirect(w, r, "/login?error=rate", http.StatusFound)
			return
		}
		user := r.FormValue("username")
		pass := r.FormValue("password")
		if user == a.user && pass == a.pass {
			a.throttle.success(ip)
			a.setSession(w, user, r.TLS != nil)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		a.throttle.fail(ip)
		http.Redirect(w, r, "/login?error=1", http.StatusFound)
		return
	}
	// GET：渲染登录页（Nuxt 静态产物 login/index.html，SPA）
	login, err := web.ReadNuxt("login/index.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	token := generateCSRFToken()
	setCSRFCookie(w, token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(login)
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func setCSRFCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionLifetime.Seconds()),
	})
}

// basicAuthValue 生成 Authorization: Basic 头值（供远程脚本等使用）。
func basicAuthValue(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

// checkAuth 校验 HTTP Basic 认证；未启用认证时直接放行。
func checkAuth(r *http.Request, user, pass string, enabled bool) bool {
	if !enabled {
		return true
	}
	u, p, ok := r.BasicAuth()
	return ok && u == user && p == pass
}

func mustHex(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}
