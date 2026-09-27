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

	"golang.org/x/crypto/argon2"

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
)

// authState 是登录认证状态（基于会话 Cookie + HMAC 签名）。
type authState struct {
	user     string
	pass     string
	enabled  bool
	secret   []byte
	key      []byte // 与密码绑定的 HMAC 密钥：改密码后旧会话全部失效
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
	// firstFail 记录窗口起点：未触顶条目的遗忘时限依据。
	firstFail time.Time
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
	now := time.Now()
	for k, e := range t.fails {
		// 清理所有过了锁定窗口的条目（含从未触顶的低次数失败）：旧实现只清
		// count>=上限的条目，未触顶的 IP 条目永久驻留，长期公网暴露下无界增长。
		// 未触顶条目的 until 为零值需要单独兜底，否则计数重置：
		// 给首次失败补一个窗口期时间戳（见 fail()），过期即视为可遗忘。
		if now.After(e.until) && e.count >= maxLoginFails {
			delete(t.fails, k)
		} else if e.count < maxLoginFails && (e.firstFail.IsZero() || now.Sub(e.firstFail) > lockWindow) {
			delete(t.fails, k)
		}
	}
	e := t.fails[ip]
	if e == nil {
		return true
	}
	if e.count >= maxLoginFails && now.After(e.until) {
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
		e = &failEntry{firstFail: time.Now()}
		t.fails[ip] = e
	} else if e.firstFail.IsZero() {
		e.firstFail = time.Now()
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
// 保证会话 cookie 在服务器重启后仍然有效（自动登录）。签名密钥与密码绑定：
// 修改密码后所有旧会话令牌自动失效。
func newAuthState(user, pass string, enabled bool) *authState {
	a := &authState{user: user, pass: pass, enabled: enabled, secret: loadSecret(), throttle: newLoginThrottle()}
	a.key = a.deriveKey()
	return a
}

// 会话密钥拉伸参数（Argon2id，RFC 9126 基准档）：单次派生 ~19MiB + 一次
// 迭代，成本参数编译期固定——可变参数意味着攻击者可按参数测算。
const (
	kdfTime    = 1
	kdfMemory  = 19 * 1024 // KiB
	kdfThreads = 1
	kdfKeyLen  = 32
)

// deriveKey 用 Argon2id 从 (secret, user, pass) 拉伸 HMAC 密钥。
// 旧实现为单轮 SHA-256，cookie 文件一旦泄露可按 ~10⁸/s 离线爆破口令；
// Argon2id 内存硬化后同硬件成本上升约 5 个数量级。salt 取 secret||user
// （secret 已持久化且随机）：改密码或改用户名后密钥必然变化，旧会话
// 令牌自动失效（与旧实现语义一致）。
func (a *authState) deriveKey() []byte {
	salt := append(append([]byte{}, a.secret...), a.user...)
	return argon2.Key([]byte(a.pass), salt, kdfTime, kdfMemory, kdfThreads, kdfKeyLen)
}

// checkPassword 常量时间密码比较（比较哈希避免长度差异泄露）。
func (a *authState) checkPassword(pass string) bool {
	h1 := sha256.Sum256([]byte(a.pass))
	h2 := sha256.Sum256([]byte(pass))
	return hmac.Equal(h1[:], h2[:])
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
	mac := hmac.New(sha256.New, a.key)
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
	mac := hmac.New(sha256.New, a.key)
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

func (a *authState) clearSession(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// handleLogout 注销当前会话：清 Cookie 并返回 JSON。仅接受 POST
// （跨站中间件对 POST /api/* 强制同源，避免被第三方页面强制登出）。
func (a *authState) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "仅支持 POST"})
		return
	}
	a.clearSession(w, r.TLS != nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
	// 令牌里的用户名必须与当前配置一致：否则改名后旧令牌仍可用
	// （改名按设计应吊销全部旧会话，但密钥即使不变也有此兜底才完整）。
	if u != a.user {
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
		if user == a.user && a.checkPassword(pass) {
			a.throttle.success(ip)
			a.setSession(w, user, r.TLS != nil)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		a.throttle.fail(ip)
		http.Redirect(w, r, "/login?error=1", http.StatusFound)
		return
	}
	// GET：渲染登录页。
	// 新前端是 Vite SPA，只产出单入口 index.html（没有 Nuxt 时代的 login/index.html
	// 预渲染路由），因此优先取预渲染页，缺失时回退 SPA 入口，由前端路由渲染 /login。
	login, err := web.ReadStatic("login/index.html")
	if err != nil {
		login, err = web.ReadStatic("index.html")
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(login)
}

func mustHex(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}
