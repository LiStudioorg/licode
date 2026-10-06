package ai

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestHTTPClientConcurrentLazyInit 并发首调 httpClient 不得产生数据竞态
// （go test -race 下验证），且所有 goroutine 必须拿到同一个 client。
func TestHTTPClientConcurrentLazyInit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	providers := []interface{ httpClient() *http.Client }{
		&OpenAIProvider{baseURL: srv.URL, model: "m"},
		&ClaudeProvider{baseURL: srv.URL, model: "m"},
		&GeminiProvider{baseURL: srv.URL, model: "m"},
		&OllamaProvider{baseURL: srv.URL, model: "m"},
	}
	for _, p := range providers {
		var wg sync.WaitGroup
		results := make([]*http.Client, 16)
		for i := range results {
			wg.Add(1)
			go func(i int) { defer wg.Done(); results[i] = p.httpClient() }(i)
		}
		wg.Wait()
		for i, c := range results {
			if c != results[0] {
				t.Fatalf("provider returned different clients at index %d", i)
			}
		}
	}
}
