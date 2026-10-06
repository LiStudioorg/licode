package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ListModels 获取某厂商可用模型列表（各厂商原生接口）。
//   - openai: GET {base}/models（含 Ollama 等 OpenAI 兼容）
//   - google: GET {base}/v1beta/models（x-goog-api-key 头）
//   - claude: 无公开列表接口，返回空
func ListModels(ctx context.Context, cfg Config) ([]string, error) {
	if err := cfg.Resolve(); err != nil {
		return nil, err
	}
	switch cfg.Type {
	case "openai":
		return listOpenAIModels(ctx, cfg)
	case "google":
		return listGeminiModels(ctx, cfg)
	case "claude":
		return nil, nil
	}
	return nil, fmt.Errorf("不支持的协议类型 %q", cfg.Type)
}

func httpGetJSON(ctx context.Context, url, apiKey string, cfg Config) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := cfg.NewLLMHTTPClient(15 * time.Second)
	return client.Do(req)
}

// httpGetGoog 与 httpGetJSON 相同，但按 Google 规范用 x-goog-api-key 头传密钥。
func httpGetGoog(ctx context.Context, url, apiKey string, cfg Config) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("x-goog-api-key", apiKey)
	}
	client := cfg.NewLLMHTTPClient(15 * time.Second)
	return client.Do(req)
}

func listOpenAIModels(ctx context.Context, cfg Config) ([]string, error) {
	resp, err := httpGetJSON(ctx, strings.TrimRight(cfg.BaseURL, "/")+"/models", cfg.APIKey, cfg)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("openai %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	var names []string
	for _, d := range out.Data {
		names = append(names, d.ID)
	}
	return names, nil
}

func listGeminiModels(ctx context.Context, cfg Config) ([]string, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	url := base + "/v1beta/models"
	// 密钥走 x-goog-api-key 请求头而非 ?key= 查询串：URL 会进入服务端访问日志、
	// 中间代理日志与 http 错误信息（后者还会被回显到前端），等于泄露密钥。
	resp, err := httpGetGoog(ctx, url, cfg.APIKey, cfg)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("gemini %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	var names []string
	for _, m := range out.Models {
		if strings.Contains(m.Name, "gemini") {
			names = append(names, strings.TrimPrefix(m.Name, "models/"))
		}
	}
	return names, nil
}
