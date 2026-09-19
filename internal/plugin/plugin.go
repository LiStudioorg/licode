package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Plugin 是一个已加载的插件（可能尚未启用/运行）。
type Plugin struct {
	Manifest *Manifest

	mu      sync.Mutex // 保护以下字段
	startMu sync.Mutex // 串行化启动/停止（避免持有 mu 时调用 appendLog 导致自死锁）
	proc    *process
	running bool
	lastErr string
	logs    []string
}

const maxLogs = 200

// Running 报告插件进程是否在运行。
func (p *Plugin) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// LastError 返回最近一次启动/调用错误。
func (p *Plugin) LastError() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}

// Logs 返回插件日志（最新在后）。
func (p *Plugin) Logs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.logs))
	copy(out, p.logs)
	return out
}

func (p *Plugin) appendLog(level, msg string) {
	if level == "" {
		level = "info"
	}
	line := time.Now().Format("15:04:05") + " [" + level + "] " + msg
	p.mu.Lock()
	p.logs = append(p.logs, line)
	if len(p.logs) > maxLogs {
		p.logs = p.logs[len(p.logs)-maxLogs:]
	}
	p.mu.Unlock()
}

// start 启动插件进程并完成 initialize 握手。
func (p *Plugin) start(settings json.RawMessage) error {
	p.startMu.Lock()
	defer p.startMu.Unlock()

	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}
	entry := p.Manifest.Entry
	args := p.Manifest.Args
	env := p.Manifest.Env
	dir := p.Manifest.Dir
	inheritEnv := p.Manifest.Permissions.Env
	id := p.Manifest.ID
	version := p.Manifest.Version
	p.mu.Unlock()

	proc, err := startProcess(dir, entry, args, env, inheritEnv)
	if err != nil {
		p.setErr(err.Error())
		return err
	}
	proc.notify = func(method string, params json.RawMessage) {
		var m struct {
			Level   string `json:"level"`
			Message string `json:"message"`
			Text    string `json:"text"`
		}
		_ = json.Unmarshal(params, &m)
		msg := m.Message
		if msg == "" {
			msg = m.Text
		}
		if msg != "" {
			p.appendLog(m.Level, msg)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = proc.call(ctx, "initialize", map[string]any{
		"apiVersion": APIVersion,
		"plugin":     map[string]any{"id": id, "version": version},
		"settings":   settings,
	})
	if err != nil {
		proc.close()
		p.setErr("initialize: " + err.Error())
		return fmt.Errorf("initialize: %w", err)
	}

	p.mu.Lock()
	p.proc = proc
	p.running = true
	p.lastErr = ""
	p.mu.Unlock()
	p.appendLog("info", "插件已启动")
	return nil
}

func (p *Plugin) setErr(msg string) {
	p.mu.Lock()
	p.lastErr = msg
	p.mu.Unlock()
}

// stop 关闭插件进程。
func (p *Plugin) stop() {
	p.startMu.Lock()
	defer p.startMu.Unlock()
	p.mu.Lock()
	proc := p.proc
	p.proc = nil
	p.running = false
	p.mu.Unlock()
	if proc != nil {
		proc.close()
		p.appendLog("info", "插件已停止")
	}
}

// call 向运行中的插件发起 RPC 调用。
func (p *Plugin) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	p.mu.Lock()
	proc := p.proc
	running := p.running
	p.mu.Unlock()
	if !running || proc == nil {
		return nil, fmt.Errorf("插件未运行")
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return proc.call(callCtx, method, params)
}

// CallTool 调用插件工具。
func (p *Plugin) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	raw, err := p.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return "", err
	}
	return resultContent(raw)
}

// CallCommand 调用插件斜杠命令。
func (p *Plugin) CallCommand(ctx context.Context, name, args string) (string, error) {
	raw, err := p.call(ctx, "command/run", map[string]any{"name": name, "args": args})
	if err != nil {
		return "", err
	}
	return resultContent(raw)
}

// RenderPanel 获取声明式面板内容。
func (p *Plugin) RenderPanel(ctx context.Context, panel string) (json.RawMessage, error) {
	return p.call(ctx, "panel/render", map[string]any{"panel": panel})
}

// UpdateSettings 通知插件设置已变更。
func (p *Plugin) UpdateSettings(settings json.RawMessage) {
	p.mu.Lock()
	proc := p.proc
	p.mu.Unlock()
	if proc != nil {
		_ = proc.notifyHost("settings/update", map[string]any{"settings": settings})
	}
}

// RunHook 执行插件钩子（由管理工作器按超时调用）。
func (p *Plugin) RunHook(ctx context.Context, event string, payload any) (json.RawMessage, error) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	p.mu.Lock()
	proc := p.proc
	p.mu.Unlock()
	if proc == nil {
		return nil, fmt.Errorf("插件未运行")
	}
	return proc.call(callCtx, "hook/run", map[string]any{"event": event, "payload": payload})
}

// resultContent 从插件返回结果中提取文本（支持字符串或 {content} 对象）。
func resultContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	var obj struct {
		Content string `json:"content"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return string(raw), nil
	}
	if obj.Error != "" {
		return "", fmt.Errorf("%s", obj.Error)
	}
	return obj.Content, nil
}
