package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// state 是插件的持久化状态（plugins.state.json）。
type state struct {
	Enabled  map[string]bool            `json:"enabled"`
	Acked    map[string]bool            `json:"acked"` // 权限已确认
	Settings map[string]json.RawMessage `json:"settings"`
}

// ErrNeedAck 表示启用前需要用户确认权限。
var ErrNeedAck = errors.New("需要先确认插件权限")

// Info 是插件列表项（管理界面用）。
type Info struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Version           string          `json:"version"`
	APIVersion        int             `json:"apiVersion"`
	Description       string          `json:"description"`
	Author            string          `json:"author"`
	Capabilities      []string        `json:"capabilities"`
	PermissionSummary []string        `json:"permission_summary"`
	Permissions       Permission      `json:"permissions"`
	Enabled           bool            `json:"enabled"`
	Acked             bool            `json:"acked"`
	Running           bool            `json:"running"`
	Error             string          `json:"error,omitempty"`
	Tools             []ToolDef       `json:"tools"`
	Commands          []CommandDef    `json:"commands"`
	Panels            []PanelDef      `json:"panels"`
	SettingsSchema    json.RawMessage `json:"settings_schema,omitempty"`
	Settings          json.RawMessage `json:"settings,omitempty"`
	Prompt            string          `json:"prompt,omitempty"`
	Logs              []string        `json:"logs"`
	Dir               string          `json:"dir"`
}

// Manager 管理插件目录、进程与状态。
type Manager struct {
	mu        sync.Mutex
	dirs      []string
	plugins   map[string]*Plugin
	state     state
	statePath string
	watcher   *fsnotify.Watcher
	stopCh    chan struct{}
}

// NewManager 创建插件管理器并读取持久化状态。
func NewManager(statePath string, dirs ...string) *Manager {
	m := &Manager{
		dirs:      dirs,
		plugins:   map[string]*Plugin{},
		statePath: statePath,
		state: state{
			Enabled:  map[string]bool{},
			Acked:    map[string]bool{},
			Settings: map[string]json.RawMessage{},
		},
		stopCh: make(chan struct{}),
	}
	m.loadState()
	return m
}

func (m *Manager) loadState() {
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		return
	}
	var st state
	if json.Unmarshal(data, &st) != nil {
		return
	}
	if st.Enabled == nil {
		st.Enabled = map[string]bool{}
	}
	if st.Acked == nil {
		st.Acked = map[string]bool{}
	}
	if st.Settings == nil {
		st.Settings = map[string]json.RawMessage{}
	}
	m.state = st
}

func (m *Manager) saveState() {
	if m.statePath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(m.statePath), 0o700)
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.statePath, data, 0o600)
}

// Start 扫描插件目录、启动已启用的插件并开启热加载监听。
func (m *Manager) Start(ctx context.Context) {
	for _, dir := range m.dirs {
		_ = os.MkdirAll(dir, 0o700)
	}
	m.scanAll(ctx, true)
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[plugin] 监听失败: %v", err)
		return
	}
	m.mu.Lock()
	m.watcher = w
	m.mu.Unlock()
	for _, dir := range m.dirs {
		_ = w.Add(dir)
	}
	// fsnotify 不递归：为每个插件子目录单独加监听（插件内的文件修改才能触发重载）
	m.mu.Lock()
	for _, p := range m.plugins {
		_ = w.Add(p.Manifest.Dir)
	}
	m.mu.Unlock()
	go m.watchLoop(ctx)
	go m.pollLoop(ctx)
}

// pollLoop 周期兜底扫描：防止某些文件系统/挂载点漏事件导致热加载失效。
func (m *Manager) pollLoop(ctx context.Context) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-t.C:
			m.scanAll(ctx, false)
		}
	}
}

func (m *Manager) watchLoop(ctx context.Context) {
	m.mu.Lock()
	w := m.watcher
	m.mu.Unlock()
	if w == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			// 事件可能发生在插件子目录内（如新建 main.py），仅扫其父目录会漏掉；
			// 统一重新扫描所有根目录（插件数量少，代价可忽略）。
			_ = ev
			// 防抖：等待写入完成
			time.Sleep(150 * time.Millisecond)
			m.scanAll(ctx, false)
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

// scanAll 扫描全部目录。startEnabled 为 true 时启动已启用的插件。
func (m *Manager) scanAll(ctx context.Context, startEnabled bool) {
	for _, dir := range m.dirs {
		m.scanDir(ctx, dir)
	}
	if startEnabled {
		m.mu.Lock()
		ids := make([]string, 0, len(m.plugins))
		for id := range m.plugins {
			ids = append(ids, id)
		}
		m.mu.Unlock()
		for _, id := range ids {
			m.mu.Lock()
			enabled := m.state.Enabled[id]
			m.mu.Unlock()
			if enabled {
				if err := m.startPlugin(id); err != nil {
					log.Printf("[plugin] 启动 %s 失败: %v", id, err)
				}
			}
		}
	}
}

// scanDir 扫描单个目录：新增/更新清单，移除已删除的插件。
func (m *Manager) scanDir(ctx context.Context, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pdir := filepath.Join(dir, e.Name())
		man, err := LoadManifest(pdir)
		if err != nil {
			continue
		}
		seen[man.ID] = true
		m.mu.Lock()
		existing, ok := m.plugins[man.ID]
		if !ok {
			m.plugins[man.ID] = &Plugin{Manifest: man}
			w := m.watcher
			m.mu.Unlock()
			if w != nil {
				_ = w.Add(pdir)
			}
			log.Printf("[plugin] 已发现: %s（%s）", man.Name, man.ID)
			if m.isEnabled(man.ID) {
				if err := m.startPlugin(man.ID); err != nil {
					log.Printf("[plugin] 启动 %s 失败: %v", man.ID, err)
				}
			}
			continue
		}
		changed := existing.Manifest.Version != man.Version
		existing.Manifest = man
		running := existing.Running()
		m.mu.Unlock()
		if changed && running {
			if err := m.Reload(ctx, man.ID); err != nil {
				log.Printf("[plugin] 热重载 %s 失败: %v", man.ID, err)
			}
		}
	}
	// 卸载本目录中已删除的插件
	m.mu.Lock()
	var removed []string
	for id, p := range m.plugins {
		if filepath.Dir(p.Manifest.Dir) == dir && !seen[id] {
			removed = append(removed, id)
		}
	}
	m.mu.Unlock()
	for _, id := range removed {
		m.mu.Lock()
		p := m.plugins[id]
		delete(m.plugins, id)
		m.mu.Unlock()
		if p != nil {
			p.stop()
		}
		log.Printf("[plugin] 已移除: %s", id)
	}
}

func (m *Manager) isEnabled(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.Enabled[id]
}

// startPlugin 启动指定插件（内部使用，带锁保护状态读取）。
func (m *Manager) startPlugin(id string) error {
	m.mu.Lock()
	p := m.plugins[id]
	settings := m.state.Settings[id]
	m.mu.Unlock()
	if p == nil {
		return fmt.Errorf("插件不存在")
	}
	return p.start(settings)
}

// List 返回全部插件的管理信息。
func (m *Manager) List() []Info {
	m.mu.Lock()
	list := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		list = append(list, p)
	}
	enabled := make(map[string]bool, len(m.state.Enabled))
	for k, v := range m.state.Enabled {
		enabled[k] = v
	}
	acked := make(map[string]bool, len(m.state.Acked))
	for k, v := range m.state.Acked {
		acked[k] = v
	}
	settings := make(map[string]json.RawMessage, len(m.state.Settings))
	for k, v := range m.state.Settings {
		settings[k] = v
	}
	m.mu.Unlock()

	out := make([]Info, 0, len(list))
	for _, p := range list {
		man := p.Manifest
		out = append(out, Info{
			ID: man.ID, Name: man.Name, Version: man.Version, APIVersion: man.APIVersion,
			Description: man.Description, Author: man.Author,
			Capabilities: man.Capabilities, Permissions: man.Permissions,
			PermissionSummary: man.Permissions.Summary(),
			Enabled:           enabled[man.ID], Acked: acked[man.ID], Running: p.Running(),
			Error: p.LastError(), Tools: man.Contributes.Tools, Commands: man.Contributes.Commands,
			Panels: man.Contributes.Panels, SettingsSchema: man.Contributes.Settings,
			Settings: settings[man.ID], Prompt: man.Prompt, Logs: p.Logs(), Dir: man.Dir,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Running 返回正在运行的插件（工具/提示词注册用）。
func (m *Manager) Running() []*Plugin {
	m.mu.Lock()
	list := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		list = append(list, p)
	}
	m.mu.Unlock()
	out := make([]*Plugin, 0, len(list))
	for _, p := range list {
		if p.Running() {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.ID < out[j].Manifest.ID })
	return out
}

// Enable 启用插件；未确认权限时返回 ErrNeedAck。
func (m *Manager) Enable(ctx context.Context, id string, ack bool) error {
	m.mu.Lock()
	p := m.plugins[id]
	if p == nil {
		m.mu.Unlock()
		return fmt.Errorf("插件不存在")
	}
	if !m.state.Acked[id] && !ack {
		m.mu.Unlock()
		return ErrNeedAck
	}
	if ack {
		m.state.Acked[id] = true
	}
	m.state.Enabled[id] = true
	m.saveState()
	settings := m.state.Settings[id]
	m.mu.Unlock()
	return p.start(settings)
}

// Disable 停用插件。
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	p := m.plugins[id]
	if p == nil {
		m.mu.Unlock()
		return fmt.Errorf("插件不存在")
	}
	m.state.Enabled[id] = false
	m.saveState()
	m.mu.Unlock()
	p.stop()
	return nil
}

// Reload 重新读取清单并重启（若在运行）。
func (m *Manager) Reload(ctx context.Context, id string) error {
	m.mu.Lock()
	p := m.plugins[id]
	if p == nil {
		m.mu.Unlock()
		return fmt.Errorf("插件不存在")
	}
	dir := p.Manifest.Dir
	wasRunning := p.Running()
	m.mu.Unlock()
	if wasRunning {
		p.stop()
	}
	man, err := LoadManifest(dir)
	if err != nil {
		return err
	}
	m.mu.Lock()
	p.Manifest = man
	settings := m.state.Settings[id]
	m.mu.Unlock()
	if wasRunning {
		return p.start(settings)
	}
	return nil
}

// SetSettings 保存插件设置并通知运行中的插件。
func (m *Manager) SetSettings(id string, raw json.RawMessage) error {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	if !json.Valid(raw) {
		return fmt.Errorf("设置不是合法 JSON")
	}
	m.mu.Lock()
	p := m.plugins[id]
	if p == nil {
		m.mu.Unlock()
		return fmt.Errorf("插件不存在")
	}
	m.state.Settings[id] = raw
	m.saveState()
	m.mu.Unlock()
	p.UpdateSettings(raw)
	return nil
}

// Delete 停用并移除插件目录（移入 .trash 防误删）。
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	p := m.plugins[id]
	if p == nil {
		m.mu.Unlock()
		return fmt.Errorf("插件不存在")
	}
	delete(m.plugins, id)
	delete(m.state.Enabled, id)
	delete(m.state.Acked, id)
	delete(m.state.Settings, id)
	m.saveState()
	m.mu.Unlock()
	p.stop()
	trash := filepath.Join(filepath.Dir(p.Manifest.Dir), ".trash")
	if err := os.MkdirAll(trash, 0o700); err == nil {
		dst := filepath.Join(trash, id+"-"+time.Now().Format("20060102150405"))
		if os.Rename(p.Manifest.Dir, dst) == nil {
			return nil
		}
	}
	return os.RemoveAll(p.Manifest.Dir)
}

// Close 停止全部插件并关闭监听。
func (m *Manager) Close() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	m.mu.Lock()
	w := m.watcher
	m.watcher = nil
	list := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		list = append(list, p)
	}
	m.mu.Unlock()
	if w != nil {
		_ = w.Close()
	}
	for _, p := range list {
		p.stop()
	}
}

// RunHook 依次调用声明了 hooks 能力的运行中插件；返回首个 modify 内容。
func (m *Manager) RunHook(ctx context.Context, event string, payload any) (json.RawMessage, bool) {
	for _, p := range m.Running() {
		if !p.Manifest.HasCapability("hooks") {
			continue
		}
		raw, err := p.RunHook(ctx, event, payload)
		if err != nil {
			p.appendLog("error", "hook "+event+": "+err.Error())
			continue
		}
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var res struct {
			Action  string          `json:"action"`
			Content string          `json:"content"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(raw, &res) != nil {
			continue
		}
		switch res.Action {
		case "modify":
			if len(res.Payload) > 0 {
				return res.Payload, true
			}
			if res.Content != "" {
				b, _ := json.Marshal(res.Content)
				return b, true
			}
		}
	}
	return nil, false
}

// HookUserMessage 允许插件改写用户消息。
func (m *Manager) HookUserMessage(ctx context.Context, content string) string {
	raw, ok := m.RunHook(ctx, "user_message", map[string]any{"content": content})
	if !ok {
		return content
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return content
}

// HookBeforeTool 返回是否允许执行与（可改写的）参数 JSON。
// 插件可返回 {"action":"deny"} 或者 {"action":"modify","payload":"<新参数JSON>"}。
func (m *Manager) HookBeforeTool(ctx context.Context, tool, args string) (bool, string) {
	for _, p := range m.Running() {
		if !p.Manifest.HasCapability("hooks") {
			continue
		}
		raw, err := p.RunHook(ctx, "before_tool", map[string]any{"tool": tool, "args": args})
		if err != nil || len(raw) == 0 {
			continue
		}
		var res struct {
			Action  string          `json:"action"`
			Content string          `json:"content"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(raw, &res) != nil {
			continue
		}
		switch res.Action {
		case "deny":
			if res.Content != "" {
				return false, res.Content
			}
			return false, "插件拒绝执行 " + tool
		case "modify":
			if len(res.Payload) > 0 {
				var s string
				if json.Unmarshal(res.Payload, &s) == nil {
					args = s
				} else {
					args = string(res.Payload)
				}
			}
		}
	}
	return true, args
}

// HookAfterTool 允许插件改写工具输出。
func (m *Manager) HookAfterTool(ctx context.Context, tool, args, out string) string {
	raw, ok := m.RunHook(ctx, "after_tool", map[string]any{"tool": tool, "args": args, "output": out})
	if !ok {
		return out
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return out
}

// HookDone 通知插件一轮回复结束（忽略结果）。
func (m *Manager) HookDone(ctx context.Context) {
	m.RunHook(ctx, "done", map[string]any{})
}
