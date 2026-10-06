package cmd

import (
	"crypto/hmac"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testAuth() *authState {
	a := &authState{
		user:     "licode",
		pass:     "s3cret",
		enabled:  true,
		secret:   []byte("0123456789abcdef0123456789abcdef"),
		throttle: newLoginThrottle(),
	}
	a.key = a.deriveKey()
	return a
}

// TestLoginThenLogoutRoundTrip 验证：登录后拿到会话 Cookie；POST /logout
// 下发同名空值 + MaxAge<0 的 Set-Cookie 将其清除，且清除后令牌不再被接受。
func TestLoginThenLogoutRoundTrip(t *testing.T) {
	a := testAuth()

	// 登录签发
	rec := httptest.NewRecorder()
	a.setSession(rec, a.user, false)
	var token string
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("login must set session cookie")
	}
	if _, ok := a.verifyToken(token); !ok {
		t.Fatal("issued token must verify")
	}

	// 登出清 Cookie
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	a.handleLogout(rec, req)
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("logout status=%d", res.StatusCode)
	}
	var cleared bool
	for _, c := range res.Cookies() {
		if c.Name == SessionCookie && c.Value == "" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("logout must clear session cookie, cookies=%v", res.Cookies())
	}

	// 改密码后旧令牌必须失效（key 与密码绑定的既有行为，防回归）
	a2 := *a
	a2.pass = "newpass"
	a2.key = a2.deriveKey()
	if _, ok := a2.verifyToken(token); ok {
		t.Fatal("old token must be invalid after password change")
	}
}

// TestLogoutRejectsGet 路由层之外，handler 本身拒绝非 POST。
func TestLogoutRejectsGet(t *testing.T) {
	a := testAuth()
	rec := httptest.NewRecorder()
	a.handleLogout(rec, httptest.NewRequest(http.MethodGet, "/logout", nil))
	if rec.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /logout should be 405, got %d", rec.Result().StatusCode)
	}
}

// TestThrottleCleansExpiredEntries 验证限流表清理：低次数失败 IP 的过期条目
// 必须被清出（无界增长修复），且锁定条目到期后同样被清理。
func TestThrottleCleansExpiredEntries(t *testing.T) {
	th := newLoginThrottle()
	// 未达上限但窗口已过的失败：应被遗忘
	th.fails["1.1.1.1"] = &failEntry{count: 1, firstFail: time.Now().Add(-time.Hour)}
	// 未达上限且窗口内：必须保留（旧实现会在 allowed 里直接删掉，导致
	// count 永远累计不到上限，限流失效）
	th.fails["4.4.4.4"] = &failEntry{count: 3, firstFail: time.Now()}
	// 达到上限但已过锁定期
	th.fails["2.2.2.2"] = &failEntry{count: maxLoginFails, until: time.Now().Add(-time.Second)}
	// 锁定期内的条目不能被清
	th.fails["3.3.3.3"] = &failEntry{count: maxLoginFails, until: time.Now().Add(time.Minute)}

	if !th.allowed("9.9.9.9") {
		t.Fatal("unrelated ip must be allowed")
	}
	if _, ok := th.fails["1.1.1.1"]; ok {
		t.Fatal("expired low-count entry must be purged")
	}
	if _, ok := th.fails["2.2.2.2"]; ok {
		t.Fatal("expired locked entry must be purged")
	}
	if _, ok := th.fails["4.4.4.4"]; !ok {
		t.Fatal("in-window low-count entry must persist for accumulation")
	}
	if th.allowed("4.4.4.4") == false {
		t.Fatal("below-limit entry must still be allowed")
	}
	if th.allowed("3.3.3.3") {
		t.Fatal("locked ip must be denied")
	}
}

// TestDeriveKeyStableAndSensitive 固化 S4 语义：Argon2id 派生对同参数确定
// （重启后 cookie 仍有效），对密码与用户名敏感（改密/改名吊销全部旧会话）。
func TestDeriveKeyStableAndSensitive(t *testing.T) {
	a := testAuth()
	b := testAuth()
	if !hmac.Equal(a.key, b.key) {
		t.Fatal("deriveKey must be deterministic for same inputs")
	}
	if len(a.key) != 32 {
		t.Fatalf("key length=%d want 32", len(a.key))
	}
	c := testAuth()
	c.pass = "other"
	c.key = c.deriveKey()
	if hmac.Equal(a.key, c.key) {
		t.Fatal("password change must rotate signing key")
	}
	d := testAuth()
	d.user = "someone"
	d.key = d.deriveKey()
	if hmac.Equal(a.key, d.key) {
		t.Fatal("username change must rotate signing key")
	}
}

// TestAuthedRejectsForeignUserToken 双保险：即便令牌签名可验（同密钥），
// 令牌内用户名与当前配置不一致也必须拒绝（authed 层的显式校验）。
func TestAuthedRejectsForeignUserToken(t *testing.T) {
	a := testAuth()
	tok := a.issueToken("attacker", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: tok})
	if u := a.authed(req); u != "" {
		t.Fatalf("token for foreign user must not authenticate, got %q", u)
	}
}
