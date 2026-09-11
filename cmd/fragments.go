// HTMX 片段路由：把页面局部（设置弹窗、文件树）在服务端用 Go
// 模板渲染成 HTML，由 /static/htmx.min.js 拉取并替换到页面。片段只读取与
// 既有 JSON API 相同的数据与逻辑，不改变任何 /api/* 行为。
package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"licode/internal/settings"
	"licode/internal/web"
	"licode/internal/websocket"
)

// registerFragmentRoutes 注册 /fragment/* 路由（需登录认证，由调用方传入 authState）。
func registerFragmentRoutes(mux *http.ServeMux, a *authState, st *serverState, ws *workspaceState, hub *websocket.Hub) {
	mux.HandleFunc("/fragment/settings", func(w http.ResponseWriter, r *http.Request) {
		if !a.require(w, r) {
			return
		}
		st.mu.RLock()
		s := st.settings.Snapshot()
		st.mu.RUnlock()
		renderHTMLFragment(w, "frag_settings.html", settingsFormData(s))
	})

	mux.HandleFunc("/fragment/files", func(w http.ResponseWriter, r *http.Request) {
		if !a.require(w, r) {
			return
		}
		p := r.URL.Query().Get("path")
		abs, err := ws.fsPath(p)
		if err != nil {
			renderHTMLFragment(w, "frag_files.html", map[string]any{"Error": err.Error()})
			return
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			renderHTMLFragment(w, "frag_files.html", map[string]any{"Error": "路径无效或不是目录"})
			return
		}
		entries, err := browseDir(abs)
		if err != nil {
			renderHTMLFragment(w, "frag_files.html", map[string]any{"Error": err.Error()})
			return
		}
		parent := filepath.Dir(abs)
		renderHTMLFragment(w, "frag_files.html", map[string]any{
			"Path": abs, "Entries": entries, "Parent": parent, "HasParent": parent != abs,
		})
	})
}

// renderHTMLFragment 以 text/html 渲染一个模板片段。
func renderHTMLFragment(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := web.RenderFragment(w, name, data); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
}

// settingsFormData 构造设置弹窗表单的模板数据（字段对应 templates/frag_settings.html）。
func settingsFormData(s settings.Settings) map[string]any {
	providers := make([]string, 0, len(settings.ProviderChoices)+len(s.Providers))
	seen := map[string]bool{}
	for _, p := range s.Providers {
		if !seen[p.Provider] {
			seen[p.Provider] = true
			providers = append(providers, p.Provider)
		}
	}
	for _, p := range settings.ProviderChoices {
		if !seen[p] {
			seen[p] = true
			providers = append(providers, p)
		}
	}
	mcpJSON, _ := json.MarshalIndent(s.MCPServers, "", "  ")
	provJSON, _ := json.MarshalIndent(s.Providers, "", "  ")
	return map[string]any{
		"Provider": s.Provider, "Providers": providers,
		"Model": s.Model, "APIKey": s.APIKey, "BaseURL": s.BaseURL,
		"Temperature": s.Temperature, "MaxTokens": s.MaxTokens, "MaxIterations": s.MaxIterations,
		"SubAgentsOn": s.SubAgents, "AutoAllowOn": s.AutoAllow,
		"ShellPath":    s.ShellPath,
		"RetryMax":     s.RetryMax,
		"SubTimeout":   s.SubTimeout,
		"MaxCtxTokens": s.MaxCtxTokens,
		"RedactOn":     s.RedactSecrets,
		"SandboxOn":    s.Sandbox,
		"SandboxImage": s.SandboxImage,
		"CacheOn":      s.CacheEnabled,
		"AutoRetryOn":  s.ToolAutoRetry,
		"RAGOn":        s.RAGEnabled,
		"RAGSource":    s.RAGSource,
		"MCPJSON":      string(mcpJSON),
		"ProvJSON":     string(provJSON),
	}
}
