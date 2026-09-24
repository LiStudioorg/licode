package cordis

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
)

// toolEntry 记录工具及其归属 Fiber。
type toolEntry struct {
	tool  Tool
	owner *Fiber
}

// toolStore 是并发安全的工具表；归属用于防止 Fiber 重建后被旧 Disposer 误删。
type toolStore struct {
	mu    sync.RWMutex
	tools map[string]toolEntry
}

func newToolStore() *toolStore {
	return &toolStore{tools: map[string]toolEntry{}}
}

func (ts *toolStore) add(t Tool, owner *Fiber) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.tools[t.Name] = toolEntry{tool: t, owner: owner}
}

func (ts *toolStore) remove(name string, owner *Fiber) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	e, ok := ts.tools[name]
	if !ok || (owner != nil && e.owner != owner) {
		return
	}
	delete(ts.tools, name)
}

func (ts *toolStore) get(name string) (Tool, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	e, ok := ts.tools[name]
	return e.tool, ok
}

func (ts *toolStore) list() []Tool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	out := make([]Tool, 0, len(ts.tools))
	for _, e := range ts.tools {
		out = append(out, e.tool)
	}
	return out
}

func (r *Runtime) addTool(t Tool, owner *Fiber) { r.tools.add(t, owner) }
func (r *Runtime) removeTool(name string, owner *Fiber) {
	r.tools.remove(name, owner)
}

// ToolRegistry 是通过 "tools" 服务暴露的工具执行管道入口。
//
// 执行路径是一条 waterfall 链：tools/pre-execute → 工具执行 → tools/post-execute。
// pre 处理器可改写参数、短路拒绝；post 处理器可脱敏/改写结果。
type ToolRegistry struct {
	r *Runtime
}

// Names 返回全部工具名。
func (tr *ToolRegistry) Names() []string {
	ts := tr.r.tools.list()
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Name)
	}
	return out
}

// List 返回全部工具定义。
func (tr *ToolRegistry) List() []Tool { return tr.r.tools.list() }

// Get 查找工具。
func (tr *ToolRegistry) Get(name string) (Tool, bool) { return tr.r.tools.get(name) }

// PermissionMode 返回工具的权限模式（空默认 allow）。
func PermissionMode(t Tool) string {
	if t.Permission == "" {
		return PermissionAllow
	}
	return t.Permission
}

// Execute 走完整 waterfall 管道执行工具。
//
// 返回的 error 仅在管道本身失败（如处理器报错、类型不合法）时非空；
// 工具自身错误在 ToolResult.Err 中，管道把它作为结果返回给调用方。
func (tr *ToolRegistry) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	res, err := tr.ExecuteResult(ctx, name, args)
	if err != nil {
		return "", err
	}
	return res.Output, res.Err
}

// ExecuteResult 执行工具并返回结构化结果。
func (tr *ToolRegistry) ExecuteResult(ctx context.Context, name string, args map[string]any) (ToolResult, error) {
	r := tr.r
	call := ToolCall{Name: name, Args: args}

	out, err := r.bus.Waterfall(r.detached, EventToolPreExecute, call, func(in any) (any, error) {
		tc, ok := in.(ToolCall)
		if !ok {
			return nil, fmt.Errorf("cordis: %s received %T, want ToolCall", EventToolPreExecute, in)
		}
		if tc.Output != nil { // pre 处理器短路：直接作为结果返回
			return ToolResult{Name: tc.Name, Output: *tc.Output}, nil
		}
		tool, ok := r.tools.get(tc.Name)
		if !ok {
			return ToolResult{Name: tc.Name, Err: fmt.Errorf("%w: %s", ErrToolUnknown, tc.Name)}, nil
		}
		out, terr := runTool(ctx, tool, tc.Args)
		post := ToolResult{Name: tc.Name, Output: out, Err: terr}
		res, perr := r.bus.Waterfall(r.detached, EventToolPostExecute, post, nil)
		if perr != nil {
			return ToolResult{Name: tc.Name, Err: perr}, nil
		}
		tr2, ok := res.(ToolResult)
		if !ok {
			return ToolResult{Name: tc.Name, Err: fmt.Errorf("cordis: %s received %T, want ToolResult", EventToolPostExecute, res)}, nil
		}
		return tr2, nil
	})
	if err != nil {
		return ToolResult{}, err
	}
	switch v := out.(type) {
	case ToolResult:
		return v, nil
	case ToolCall: // pre 处理器短路返回（带 Output）
		if v.Output != nil {
			return ToolResult{Name: v.Name, Output: *v.Output}, nil
		}
		return ToolResult{Name: v.Name, Err: errors.New("cordis: pre-execute short-circuit without Output")}, nil
	}
	return ToolResult{}, fmt.Errorf("cordis: tool pipeline returned %T, want ToolResult", out)
}

// runTool 调用工具本体，panic 转为错误。
func runTool(ctx context.Context, t Tool, args map[string]any) (out string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("tool %s panicked: %v\n%s", t.Name, p, debug.Stack())
		}
	}()
	return t.Execute(ctx, args)
}
