package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSecurityHeadersCSPStrict 默认（严格）模式下 script-src 不得含
// unsafe-inline。是否附带 sha256 白名单取决于嵌入产物：Vite SPA 产物
// 不含任何内联脚本，此时严格模式收敛为 "'self'"（比带哈希更紧），
// 因此这里只断言"无 unsafe-inline"，不断言哈希必然存在。
func TestSecurityHeadersCSPStrict(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), false, true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("missing CSP")
	}
	for _, d := range strings.Split(csp, ";") {
		d = strings.TrimSpace(d)
		if !strings.HasPrefix(d, "script-src") {
			continue
		}
		if strings.Contains(d, "unsafe-inline") {
			t.Fatalf("strict mode must drop unsafe-inline: %q", d)
		}
		if !strings.Contains(d, "'self'") {
			t.Fatalf("strict mode must at least allow same-origin scripts: %q", d)
		}
		return
	}
	t.Fatal("script-src directive missing")
}

// TestSecurityHeadersCSPLegacyEscape 逃生门：unsafeInline=true 保持旧行为。
func TestSecurityHeadersCSPLegacyEscape(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), false, false)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("escape hatch must restore legacy CSP, got %q", csp)
	}
}
