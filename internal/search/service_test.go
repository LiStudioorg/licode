package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 本地测试页（SSRF 校验只拦非 http/https，允许 loopback）。
var testPage = `<html><head><title>Licode 自建搜索测试页</title></head>
<body><h1>标题</h1><p>百度 必应 是常见的搜索引擎；本文介绍自建索引与倒排结构。</p>
<ul><li>第一项：goroutine channel</li><li>第二项：BM25 打分</li></ul></body></html>`

func TestServiceRoundtrip(t *testing.T) {
	// 本测试用 127.0.0.1 的 httptest 服务，临时关闭私网拦截。
	old := blockPrivateHosts
	blockPrivateHosts = false
	defer func() { blockPrivateHosts = old }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testPage))
	}))
	defer srv.Close()

	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(store, nil)

	ctx := context.Background()

	// ValidateURL 拒绝非 http(s)
	if _, err := ValidateURL("file:///etc/passwd"); err == nil {
		t.Error("file:// 应被拒绝")
	}

	title, text, err := svc.Fetch(ctx, srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(title, "自建搜索测试页") {
		t.Errorf("标题解析失败：%q", title)
	}
	if !strings.Contains(text, "倒排结构") || !strings.Contains(text, "BM25") {
		t.Errorf("正文解析失败：%q", text)
	}

	d, err := svc.Save(ctx, srv.URL)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if d.Title != title {
		t.Errorf("保存标题不一致：%q vs %q", d.Title, title)
	}

	hits := store.Search("自建索引", 5)
	if len(hits) == 0 {
		t.Fatal("本地检索应命中")
	}
	if hits[0].URL != srv.URL {
		t.Errorf("命中 URL 错误：%s", hits[0].URL)
	}

	// 关闭 meta（不联网）仍能搜到本地
	rs, err := svc.Search(ctx, "索引 倒排", nil, false, true, 5)
	if err != nil || len(rs) == 0 {
		t.Fatalf("本地-only 搜索失败：err=%v rs=%d", err, len(rs))
	}
	if !rs[0].Local {
		t.Error("本地命中应标记 local")
	}

	// 删除后清空
	if err := store.Remove(srv.URL); err != nil {
		t.Fatal(err)
	}
	if hits := store.Search("索引", 5); len(hits) != 0 {
		t.Errorf("删除后仍命中：%d", len(hits))
	}
}

func TestValidateURL(t *testing.T) {
	good := []string{"https://example.com/a?b=1"}
	for _, u := range good {
		if _, err := ValidateURL(u); err != nil {
			t.Errorf("%q 应为合法：%v", u, err)
		}
	}
	bad := []string{"", "ftp://x.com/a", "javascript:alert(1)", "/relative/path", "not-a-url", "http://localhost:8000/x"}
	for _, u := range bad {
		if _, err := ValidateURL(u); err == nil {
			t.Errorf("%q 应判为非法", u)
		}
	}
}

// TestBlockPrivateHosts 验证默认开启的 SSRF 私网拦截生效。
func TestBlockPrivateHosts(t *testing.T) {
	for _, u := range []string{
		"http://127.0.0.1/x",
		"http://localhost/x",
		"http://169.254.169.254/latest/meta-data",
		"http://10.0.0.1/x",
		"http://192.168.1.1/x",
	} {
		if _, err := ValidateURL(u); err == nil {
			t.Errorf("%q 应被私网拦截拒绝", u)
		}
	}
	if _, err := ValidateURL("https://example.com/x"); err != nil {
		t.Errorf("公网域名不应被拦截：%v", err)
	}
}