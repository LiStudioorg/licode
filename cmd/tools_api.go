package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"licode/internal/agent"
	"licode/internal/settings"
)

// toolsAPI 是工具管理接口（独立页面使用）：
//   - GET  /api/tools         列出全部工具及来源、当前权限
//   - POST /api/tools/rule    设置某工具的权限（allow/ask/deny）
//   - POST /api/tools/delete  删除 MCP 服务器 / 外部命令工具 / 技能
type toolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"` // builtin | subagent | mcp | external | skill
	Server      string `json:"server,omitempty"`
	File        string `json:"file,omitempty"`
	Removable   bool   `json:"removable"`
	Rule        string `json:"rule"` // allow | ask | deny
}

func (st *serverState) handleToolsList(w http.ResponseWriter, r *http.Request) {
	st.mu.RLock()
	s := st.settings.Snapshot()
	st.mu.RUnlock()

	var tools []toolInfo

	// 内置工具
	reg := agent.NewRegistry()
	agent.RegisterDefaultTools(reg, agent.ShellConfig{Path: s.ShellPath, Sandbox: s.Sandbox, Image: s.SandboxImage})
	for _, name := range reg.Names() {
		t, _ := reg.Get(name)
		tools = append(tools, toolInfo{
			Name: name, Description: t.Description, Source: "builtin",
			Rule: s.EffectiveToolRule(name),
		})
	}
	if s.SubAgents {
		tools = append(tools, toolInfo{
			Name: "Dispatch", Description: "把子任务分派给专用子代理并行执行（DAG 依赖调度）",
			Source: "subagent", Rule: s.EffectiveToolRule("Dispatch"),
		})
	}

	// 外部命令工具（~/.licode/tools/*.json）
	for _, t := range agent.ListExternalTools(settings.ToolsDir()) {
		tools = append(tools, toolInfo{
			Name: t.Name, Description: t.Description, Source: "external",
			File: filepath.Base(t.File), Removable: true, Rule: s.EffectiveToolRule(t.Name),
		})
	}

	// 技能（skills/*.md）
	for _, sk := range agent.LoadSkills(agent.SkillDirs()...) {
		tools = append(tools, toolInfo{
			Name: "skill_" + sk.Name, Description: sk.Description, Source: "skill",
			File: sk.File, Removable: true, Rule: s.EffectiveToolRule("skill_" + sk.Name),
		})
	}

	// MCP 工具：短超时连接枚举；失败的服务器仍列出（可删除配置）
	mcpErr := ""
	if len(s.MCPServers) > 0 {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		mreg := agent.NewRegistry()
		mgr := agent.NewMCPManager()
		if err := mgr.RegisterContext(ctx, mreg, s.MCPServers); err != nil {
			mcpErr = err.Error()
		}
		for _, name := range mreg.Names() {
			t, _ := mreg.Get(name)
			server := mcpServerOf(name)
			tools = append(tools, toolInfo{
				Name: name, Description: t.Description, Source: "mcp",
				Server: server, Rule: s.EffectiveToolRule(name),
			})
		}
		mgr.Close()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tools":       tools,
		"mcp_servers": s.MCPServers,
		"mcp_error":   mcpErr,
	})
}

// mcpServerOf 从 mcp__<server>__<tool> 提取服务器名。
func mcpServerOf(toolName string) string {
	parts := strings.SplitN(toolName, "__", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func (st *serverState) handleToolsSetRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Rule string `json:"rule"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "工具名不能为空"})
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	next := st.settings.Snapshot()
	if err := next.SetToolRule(body.Name, body.Rule); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := next.Save(""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "保存失败: " + err.Error()})
		return
	}
	st.settings = next
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": body.Name, "rule": body.Rule})
}

func (st *serverState) handleToolsDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type string `json:"type"` // mcp | external | skill
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "名称不能为空"})
		return
	}
	switch body.Type {
	case "mcp":
		st.deleteMCPServer(w, body.Name)
	case "external":
		st.deleteExternalTool(w, body.Name)
	case "skill":
		st.deleteSkill(w, body.Name)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "不支持删除该类型（仅支持 MCP 服务器、外部命令工具、技能）"})
	}
}

func (st *serverState) deleteMCPServer(w http.ResponseWriter, name string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	next := st.settings.Snapshot()
	var kept []settings.MCPServer
	found := false
	for _, srv := range next.MCPServers {
		if srv.Name == name {
			found = true
			continue
		}
		kept = append(kept, srv)
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "未找到该 MCP 服务器"})
		return
	}
	next.MCPServers = kept
	// 清理该服务器下 MCP 工具的权限规则
	prefix := "mcp__" + name + "__"
	for tool := range next.ToolRules {
		if strings.HasPrefix(tool, prefix) {
			delete(next.ToolRules, tool)
		}
	}
	if err := next.Save(""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "保存失败: " + err.Error()})
		return
	}
	st.settings = next
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (st *serverState) deleteExternalTool(w http.ResponseWriter, name string) {
	toolsDir := settings.ToolsDir()
	var target string
	for _, t := range agent.ListExternalTools(toolsDir) {
		if t.Name == name {
			target = t.File
			break
		}
	}
	if target == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "未找到该外部命令工具"})
		return
	}
	// 只允许删除工具目录内的文件
	if filepath.Clean(filepath.Dir(target)) != filepath.Clean(toolsDir) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "工具文件不在允许的目录内"})
		return
	}
	if err := os.Remove(target); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "删除失败: " + err.Error()})
		return
	}
	st.removeToolRule(name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (st *serverState) deleteSkill(w http.ResponseWriter, name string) {
	var target string
	for _, sk := range agent.LoadSkills(agent.SkillDirs()...) {
		if sk.Name == name {
			target = sk.File
			break
		}
	}
	if target == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "未找到该技能"})
		return
	}
	if !pathInDirs(target, agent.SkillDirs()) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "技能文件不在允许的目录内"})
		return
	}
	if err := os.Remove(target); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "删除失败: " + err.Error()})
		return
	}
	st.removeToolRule("skill_" + name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// removeToolRule 删除某工具的权限规则（文件型工具删除后同步清理）。
func (st *serverState) removeToolRule(name string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	next := st.settings.Snapshot()
	if _, ok := next.ToolRules[name]; ok {
		delete(next.ToolRules, name)
		_ = next.Save("")
	}
	st.settings = next
}

// pathInDirs 判断文件是否位于允许的目录内（用于删除前防护）。
func pathInDirs(path string, dirs []string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, d := range dirs {
		adir, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(adir, abs)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			return true
		}
	}
	return false
}
