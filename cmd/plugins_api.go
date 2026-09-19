package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"licode/internal/plugin"
	"licode/internal/websocket"
)

// 插件管理接口（设置页「插件」页签使用）。

func (st *serverState) requirePlugins(w http.ResponseWriter) bool {
	if st.plugins == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "插件系统未启用"})
		return false
	}
	return true
}

func (st *serverState) handlePluginsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "仅支持 GET"})
		return
	}
	if !st.requirePlugins(w) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plugins": st.plugins.List()})
}

func (st *serverState) handlePluginsEnable(w http.ResponseWriter, r *http.Request) {
	if !st.requirePlugins(w) {
		return
	}
	var body struct {
		ID  string `json:"id"`
		Ack bool   `json:"ack"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	err := st.plugins.Enable(r.Context(), body.ID, body.Ack)
	if errors.Is(err, plugin.ErrNeedAck) {
		// 返回权限清单，前端弹确认后再带 ack 调用
		var summary []string
		var perms plugin.Permission
		for _, info := range st.plugins.List() {
			if info.ID == body.ID {
				summary = info.PermissionSummary
				perms = info.Permissions
				break
			}
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "需要确认插件权限", "need_ack": true,
			"permissions": perms, "permission_summary": summary,
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (st *serverState) handlePluginsDisable(w http.ResponseWriter, r *http.Request) {
	if !st.requirePlugins(w) {
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if err := st.plugins.Disable(body.ID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (st *serverState) handlePluginsReload(w http.ResponseWriter, r *http.Request) {
	if !st.requirePlugins(w) {
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if err := st.plugins.Reload(r.Context(), body.ID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (st *serverState) handlePluginsSettings(w http.ResponseWriter, r *http.Request) {
	if !st.requirePlugins(w) {
		return
	}
	var body struct {
		ID       string          `json:"id"`
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if err := st.plugins.SetSettings(body.ID, body.Settings); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (st *serverState) handlePluginsDelete(w http.ResponseWriter, r *http.Request) {
	if !st.requirePlugins(w) {
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if err := st.plugins.Delete(body.ID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handlePluginsInstall 接收 zip 上传并安装插件。
func (st *serverState) handlePluginsInstall(w http.ResponseWriter, r *http.Request) {
	if !st.requirePlugins(w) {
		return
	}
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "文件过大或格式错误"})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "缺少 file 字段"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "读取上传失败"})
		return
	}
	id, err := st.plugins.Install(data)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// handlePluginsPanel 渲染插件的声明式面板。
func (st *serverState) handlePluginsPanel(w http.ResponseWriter, r *http.Request) {
	if !st.requirePlugins(w) {
		return
	}
	id := r.URL.Query().Get("id")
	panel := r.URL.Query().Get("panel")
	var target *plugin.Plugin
	for _, p := range st.plugins.Running() {
		if p.Manifest.ID == id {
			target = p
			break
		}
	}
	if target == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "插件未运行"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	raw, err := target.RenderPanel(ctx, panel)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(raw)
}

// runPluginCommand 处理插件斜杠命令（/name 或 /pluginid:name）。
func (st *serverState) runPluginCommand(ctx context.Context, c *websocket.Client, content string) bool {
	if st.plugins == nil || !strings.HasPrefix(content, "/") {
		return false
	}
	body := strings.TrimSpace(strings.TrimPrefix(content, "/"))
	if body == "" || body == "clear" {
		return false
	}
	name, args := body, ""
	if i := strings.IndexAny(body, " \t"); i >= 0 {
		name = body[:i]
		args = strings.TrimSpace(body[i+1:])
	}
	for _, p := range st.plugins.Running() {
		for _, cmd := range p.Manifest.Contributes.Commands {
			if cmd.Name != name && p.Manifest.ID+":"+cmd.Name != name {
				continue
			}
			c.SendEvent(websocket.ServerEvent{Type: websocket.EvtStatus, Content: "插件命令执行中…"})
			callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			out, err := p.CallCommand(callCtx, cmd.Name, args)
			cancel()
			if err != nil {
				out = "插件命令执行失败: " + err.Error()
			}
			c.SendEvent(websocket.ServerEvent{Type: websocket.EvtPluginOutput, Content: out})
			return true
		}
	}
	return false
}
