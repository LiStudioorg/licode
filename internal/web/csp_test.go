package web

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var tagRe = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)

// TestScriptSrcCSPMatchesArtifacts 独立复算一遍，
// 确认 CSP 哈希白名单覆盖产物中每个内联脚本体。
// 注意：Vite SPA 产物不含内联脚本（入口是外部 module script），
// 此时 script-src 收敛为 "'self'"——这比带哈希更严格，属于期望结果；
// 因此仅当产物确实存在内联脚本时才要求出现对应哈希。
func TestScriptSrcCSPMatchesArtifacts(t *testing.T) {
	src := ScriptSrcCSP()
	if !strings.HasPrefix(src, "'self'") {
		t.Fatalf("expected script-src to start with 'self', got %q", src)
	}
	if strings.Contains(src, "unsafe-inline") {
		t.Fatal("strict mode must not contain unsafe-inline")
	}
	present := map[string]bool{}
	for _, part := range strings.Split(src, " ") {
		present[part] = true
	}
	n := 0
	fs.WalkDir(staticFS, "dist", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".html" {
			return nil
		}
		data, _ := fs.ReadFile(staticFS, p)
		for _, m := range tagRe.FindAllSubmatch(data, -1) {
			attrs := strings.ToLower(string(m[1]))
			if strings.Contains(attrs, "src=") || strings.Contains(attrs, "json") || strings.TrimSpace(string(m[2])) == "" {
				continue
			}
			sum := sha256.Sum256(m[2])
			h := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
			if !present[h] {
				t.Errorf("%s inline script hash %s missing from CSP", p, h)
			}
			n++
		}
		return nil
	})
	t.Logf("verified %d inline script bodies covered (0 means artifact has none)", n)
}
