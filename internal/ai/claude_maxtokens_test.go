package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newFakeClaude 返回一个假 Anthropic 端点：当 max_tokens > 8000 时回 400
// （与真实 API 的 max_tokens 超限错误同形态），否则回成功响应。
// got 记录每次请求携带的 max_tokens。
func newFakeClaude(t *testing.T, got *[]int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(b, &body)
		mt, _ := body["max_tokens"].(float64)
		*got = append(*got, int(mt))
		if int(mt) > 8000 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens: too large, which is the maximum allowed number of output tokens"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"ok"}]}`)
	}))
}

// TestClaudeBuildBodyMaxTokensPassthrough buildBody 保持请求值语义（0→4096），
// 上限钳制发生在 do()/maxTokensFallback（由 TestDoWithMaxTokensFallback 验证）。
func TestClaudeBuildBodyMaxTokensPassthrough(t *testing.T) {
	p := &ClaudeProvider{model: "claude-sonnet-4-5"}
	req := ChatRequest{MaxTokens: 1000000, Messages: []Message{{Role: RoleUser, Content: "hi"}}}
	b, err := p.buildBody(req, true)
	if err != nil {
		t.Fatal(err)
	}
	m := decodeBody(t, b)
	if mt, _ := m["max_tokens"].(float64); mt != 1000000 {
		t.Fatalf("buildBody should pass through, got %v", mt)
	}
	// 0 回退 4096。
	req.MaxTokens = 0
	b, _ = p.buildBody(req, true)
	m = decodeBody(t, b)
	if mt, _ := m["max_tokens"].(float64); mt != 4096 {
		t.Fatalf("zero max_tokens should fallback 4096, got %v", mt)
	}
}

// TestIsMaxTokensError 验证"输出上限超限"类错误的识别。
func TestIsMaxTokensError(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{`claude 400 Bad Request: {"error":{"message":"max_tokens: 1000000 > 64000, which is the maximum allowed number of output tokens"}}`, true},
		{`openai 400 Bad Request: {"error":{"message":"max_tokens is too large: 1000000"}}`, true},
		{`gemini 400 Bad Request: {"error":{"message":"maxOutputTokens exceeds limit"}}`, true},
		{`openai 400 Bad Request: {"error":{"message":"invalid api key"}}`, false},
		{`claude 429 Too Many Requests: rate limit`, false},
	}
	for _, c := range cases {
		if got := isMaxTokensError(errors.New(c.msg)); got != c.want {
			t.Errorf("isMaxTokensError(%q)=%v want %v", c.msg, got, c.want)
		}
	}
}

// TestDoWithMaxTokensFallback 用 httptest 验证：默认 1e6 先被钳到 cap，
// 首次 400(max_tokens) 后自动以减半后的值重试并最终成功。
func TestDoWithMaxTokensFallback(t *testing.T) {
	var got []int
	srv := newFakeClaude(t, &got)
	defer srv.Close()
	p := &ClaudeProvider{name: "claude", baseURL: srv.URL, apiKey: "k", model: "m"}
	out, err := p.Chat(context.Background(), ChatRequest{MaxTokens: 1000000, Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if out != "ok" {
		t.Fatalf("out=%q", out)
	}
	if len(got) < 2 {
		t.Fatalf("expected retry after 400 max_tokens, requests=%v", got)
	}
	if got[0] != claudeMaxTokensCap {
		t.Fatalf("first request should send cap %d, got %v", claudeMaxTokensCap, got)
	}
	for i := 1; i < len(got); i++ {
		if got[i] >= got[i-1] {
			t.Fatalf("retry should decrease: %v", got)
		}
	}
}

// TestMaxTokensFallbackNoRetryOnOtherErrors 非上限类错误不得触发减半重试。
func TestMaxTokensFallbackNoRetryOnOtherErrors(t *testing.T) {
	calls := 0
	err := maxTokensFallback(1000000, 32000, func(mt int) error {
		calls++
		if mt != 32000 {
			t.Fatalf("should send capped value, got %d", mt)
		}
		return errors.New("claude 400 Bad Request: invalid api key")
	})
	if err == nil || calls != 1 {
		t.Fatalf("non-cap 400 must not retry: calls=%d err=%v", calls, err)
	}
}
