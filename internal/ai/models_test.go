package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListGeminiModelsKeyInHeader 验证 Gemini 模型列表请求把 API Key 放在
// x-goog-api-key 头里，URL 查询串中不出现密钥（防止进入访问/代理日志）。
func TestListGeminiModelsKeyInHeader(t *testing.T) {
	var gotHeader, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-goog-api-key")
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-2.5-pro"}]}`))
	}))
	defer srv.Close()
	names, err := listGeminiModels(context.Background(), Config{BaseURL: srv.URL, APIKey: "SECRET-123"})
	if err != nil {
		t.Fatal(err)
	}
	if gotHeader != "SECRET-123" {
		t.Fatalf("x-goog-api-key header missing: %q", gotHeader)
	}
	if gotQuery != "" {
		t.Fatalf("query must not carry key, got %q", gotQuery)
	}
	if len(names) != 1 || names[0] != "gemini-2.5-pro" {
		t.Fatalf("names=%v", names)
	}
}
