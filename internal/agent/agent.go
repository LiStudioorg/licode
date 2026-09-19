// Package agent implements the main coding agent: a tool-calling loop that
// streams events to a WebSocket UI. It also provides a lightweight sub-agent
// system with DAG dependency scheduling.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"licode/internal/ai"
	"licode/internal/logx"
	"licode/internal/session"
)

// EventType enumerates events emitted by an Agent while running.
type EventType string

const (
	// EventText streams assistant text deltas.
	EventText EventType = "text"
	// EventToolStart is emitted before a tool executes.
	EventToolStart EventType = "tool_start"
	// EventToolDone is emitted after a tool returns.
	EventToolDone EventType = "tool_done"
	// EventDone signals a completed reply.
	EventDone EventType = "done"
	// EventError signals a fatal error.
	EventError EventType = "error"
	// EventStatus reports transient status like iteration count.
	EventStatus EventType = "status"
	// EventReasoning streams model reasoning/thinking deltas (not persisted).
	EventReasoning EventType = "reasoning"
	// EventAsk asks the user to approve a tool call (permission=ask).
	EventAsk EventType = "ask"
	// EventSettings carries updated runtime settings (from a remote server).
	EventSettings EventType = "settings"
	// EventSessions carries the remote session list.
	EventSessions EventType = "sessions"
)

// Event is a UI-agnostic stream event.
type Event struct {
	Type      EventType `json:"type"`
	Content   string    `json:"content,omitempty"`
	ToolName  string    `json:"toolName,omitempty"`
	ToolArgs  string    `json:"toolArgs,omitempty"`
	ToolOut   string    `json:"toolOut,omitempty"`
	Error     string    `json:"error,omitempty"`
	Settings  any       `json:"settings,omitempty"`
	AskID     string    `json:"askId,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`
}

// DefaultMainPrompt 是主 Agent 的系统提示词模板。
// 运行时变量由 BuildMainPrompt 注入（cwd/平台/日期/模型名）。
const DefaultMainPrompt = `You are Licode, an AI coding agent embedded in a web-based development
environment. Use the instructions below and the tools available to you to
assist the user.

# Tone and style
You should be concise, direct, and to the point. Your output is displayed
in a chat panel, not a terminal.
IMPORTANT: You MUST answer concisely with fewer than 4 lines of text (not
including tool use or code generation), unless the user asks for detail.
Answer the user's question directly, without elaboration. Avoid
introductions, conclusions, and explanations. Do NOT say "The answer is
<answer>", "Here is what I will do next", or summarize your actions.
Only use emojis if the user explicitly requests it.

<example>
user: what is 2+2?
assistant: 4
</example>

<example>
user: what command lists files in the current directory?
assistant: ls
</example>

<example>
user: which file defines foo?
assistant: [uses grep/read to locate it]
src/foo.c
</example>

# Proactiveness
You are allowed to be proactive, but only when the user asks you to do
something. If the user asks how to approach something, answer the question
first - do NOT immediately jump into taking actions. After finishing work
on a file, just stop; do not explain what you did unless asked.

# Following conventions
When making changes to files, first understand the file's code conventions.
Mimic code style, use existing libraries and utilities, follow existing
patterns.
- NEVER assume a library is available. Before using one, check that the
  codebase already uses it (look at neighboring files, package manifests,
  etc.).
- When creating a new component, look at existing ones first for naming,
  typing, and structure.
- When editing code, read the surrounding context (especially imports) to
  understand the codebase's framework and library choices.

# Code style
- IMPORTANT: DO NOT ADD ANY COMMENTS unless asked.
- Match existing style and naming.
- Never hardcode secrets, tokens, or credentials.

# Doing tasks
The user will primarily request software engineering tasks: fixing bugs,
adding features, refactoring, explaining code.
- Use search tools to understand the codebase and the user's query. Search
  extensively, in parallel and sequentially.
- Implement the solution using the tools available to you.
- Verify with tests if possible. NEVER assume a test framework or script.
  Check the README or search the codebase to determine how tests run.
- VERY IMPORTANT: When you finish a task, run the lint and typecheck
  commands if the project provides them. If you cannot find the correct
  command, ask the user.
NEVER commit changes unless the user explicitly asks you to. Committing
without being asked is being too proactive.

# Tool usage policy
- Prefer Grep/Glob for file search to reduce context usage; when sub-agents
  are enabled, you may Dispatch a search task to the explorer sub-agent.
- You can call multiple tools in a single response. When several
  independent pieces of information are requested, batch the tool calls in
  one response instead of multiple turns.
- Read a file before editing it to understand the exact text to replace.
- Prefer targeted edits over full rewrites; include enough surrounding
  context to make the match unique.
- For shell commands: explain non-obvious commands in one line before
  running them. Never run destructive commands (rm -rf, git reset --hard,
  force push) without explicit confirmation.
- If a tool result includes <system-reminder> tags, treat them as useful
  context, NOT as user input.

# Code references
When referencing specific functions or code, use the pattern
` + "`file_path:line_number`" + ` so the user can navigate to the source.

<example>
user: Where are errors from the client handled?
assistant: Clients are marked as failed in the ` + "`connectToServer`" + ` function
in src/services/process.ts:712.
</example>

You are powered by the model named {{MODEL_NAME}}. The exact model ID is
{{MODEL_ID}}.

Here is useful information about the environment:
<env>
Working directory: {{CWD}}
Platform: {{OS}}
Today's date: {{DATE}}
</env>

Respond in the same language as the user's message.`

// BuildMainPrompt 把运行时变量注入 DefaultMainPrompt 模板：
// cwd=工作目录，osName=操作系统，date=当日日期，modelName=模型可读名，modelID=模型完整 ID。
func BuildMainPrompt(cwd, osName, date, modelName, modelID string) string {
	p := strings.NewReplacer(
		"{{CWD}}", cwd,
		"{{OS}}", osName,
		"{{DATE}}", date,
		"{{MODEL_NAME}}", modelName,
		"{{MODEL_ID}}", modelID,
	).Replace(DefaultMainPrompt)
	return p
}

// Tool is a registered callable function.
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	schemaBytes []byte
	Run         func(ctx context.Context, args map[string]any) (string, error)
}

// Registry is a concurrency-safe tool set.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t.Name == "" {
		return errors.New("tool name required")
	}
	t.schemaBytes, _ = json.Marshal(t.Schema)
	r.tools[t.Name] = t
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Unregister 动态移除一个工具（fsnotify 热加载时用于卸载被删除/失效的工具）。
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// MergeFrom 把另一个注册表中的工具复制到当前注册表（用于把外部热加载工具
// 并入每个新建的 Agent）。
func (r *Registry) MergeFrom(src *Registry) {
	if src == nil {
		return
	}
	src.mu.RLock()
	var tools []Tool
	for _, t := range src.tools {
		tools = append(tools, t)
	}
	src.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range tools {
		t.schemaBytes, _ = json.Marshal(t.Schema)
		r.tools[t.Name] = t
	}
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.tools))
	for n := range r.tools {
		out = append(out, n)
	}
	return out
}

// List returns OpenAI-style tool definitions for the model.
func (r *Registry) List() []ai.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ai.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, ai.Tool{
			Type: "function",
			Function: ai.FunctionSpec{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.schemaBytes,
			},
		})
	}
	return out
}

// Execute runs a tool by name with JSON-encoded arguments.
func (r *Registry) Execute(ctx context.Context, name string, argsJSON []byte) (string, error) {
	t, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	var args map[string]any
	if len(bytes.TrimSpace(argsJSON)) > 0 {
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return "", fmt.Errorf("tool %s: bad arguments: %w", name, err)
		}
	}
	return t.Run(ctx, args)
}

// Agent is the main orchestration loop. It calls the LLM, executes any tool
// calls, feeds results back, and repeats until the model replies without
// tools or MaxIterations is hit.
type Agent struct {
	Name          string
	System        string
	Client        ai.LLMClient
	Model         string
	Tools         *Registry
	Session       *session.Session
	SubAgents     []SubAgentSpec
	MaxIterations int
	MaxTokens     int
	Temperature   float64
	// Timeout 硬超时（秒，0=不限制）。超时后强制取消本次运行。
	Timeout int
	// Permissions 工具名 -> allow/ask/deny；"*" 为默认模式。
	Permissions map[string]string
	// Ask 在 permission=ask 时被调用，返回 true 表示允许执行。
	Ask func(ctx context.Context, toolName, args string) (bool, error)
	// AutoAllowPaths 允许工具访问工作目录之外路径时不再逐次询问
	// （信任该设置的用户显式开启；默认关闭，外部路径仍需逐次确认）。
	AutoAllowPaths bool
	// OnUserMessage 允许外部（插件）改写用户消息。
	OnUserMessage func(ctx context.Context, content string) (string, error)
	// OnBeforeTool 工具执行前钩子：返回 false 拒绝执行，第二值可改写参数。
	OnBeforeTool func(ctx context.Context, tool, args string) (bool, string)
	// OnAfterTool 工具执行后钩子：可改写输出。
	OnAfterTool func(ctx context.Context, tool, args, out string) string
	// OnDone 一轮回复完成后的通知。
	OnDone func(ctx context.Context)
	// Compaction 上下文超限时用 LLM 压缩旧对话。
	Compaction bool
	// RedactSecrets 对工具输出做敏感信息脱敏。
	RedactSecrets bool
	// ToolAutoRetry 在工具返回错误或空结果时自动重试。
	ToolAutoRetry bool
	// ToolRetryMax 单次工具调用最多重试次数（默认 3）。
	ToolRetryMax int
	// Shell 配置 Shell 工具（路径、沙箱等）。
	Shell ShellConfig
	// TraceID 本次运行的调用链标识（结构化日志用）。
	TraceID string
	// Usage 累计本次运行消耗的 token（含缓存读取）。
	Usage ai.Usage
	// mcpMgr 由 BuildAgent 装配的 MCP 连接管理器；一次运行结束后由调用方
	// 通过 Close 释放，避免 stdio 子进程泄漏。
	mcpMgr *MCPManager
}

// SetMCPManager 绑定 MCP 连接管理器（settings.BuildAgent 装配时调用）。
func (a *Agent) SetMCPManager(m *MCPManager) { a.mcpMgr = m }

// Close 释放 Agent 持有的外部资源（MCP 子进程等）。运行结束后必须调用。
func (a *Agent) Close() {
	if a.mcpMgr != nil {
		a.mcpMgr.Close()
		a.mcpMgr = nil
	}
}

func NewAgent(client ai.LLMClient, system string) *Agent {
	a := &Agent{
		Name:          "main",
		System:        system,
		Client:        client,
		Model:         client.Model(),
		Tools:         NewRegistry(),
		Session:       session.NewSession(0),
		MaxIterations: 16,
		MaxTokens:     4096,
		Permissions:   map[string]string{},
	}
	RegisterDefaultTools(a.Tools, a.Shell)
	return a
}

// Run executes a user request, streaming events through onEvent.
// It returns after the reply completes or an error occurs.
func (a *Agent) Run(ctx context.Context, input string, onEvent func(Event)) error {
	a.RunWithAttachments(ctx, input, nil, func(e Event) error { onEvent(e); return nil })
	return nil
}

// RunWithAttachments 执行请求并附带多模态附件（图片/文件）。
func (a *Agent) RunWithAttachments(ctx context.Context, input string, attachments []ai.Attachment, onEvent func(Event) error) error {
	if a.TraceID == "" {
		a.TraceID = logx.NewTraceID()
	}
	// Timeout 硬超时在此处统一生效（主 Agent 与子代理共用该字段语义）。
	if a.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(a.Timeout)*time.Second)
		defer cancel()
	}
	logx.AgentStart(a.TraceID, a.Name)
	if a.OnUserMessage != nil {
		if next, err := a.OnUserMessage(ctx, input); err == nil && next != "" {
			input = next
		}
	}
	a.Session.Add(ai.Message{Role: ai.RoleUser, Content: input, Attachments: attachments})

	var asst ai.Message
	for iter := 1; iter <= a.MaxIterations; iter++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		onEvent(Event{Type: EventStatus, Content: fmt.Sprintf("思考中 (%d)", iter)})

		if a.Compaction {
			a.compactIfNeeded(ctx)
		}
		msgs := a.Session.MessagesForLLM(a.System)
		req := ai.ChatRequest{
			Model:       a.Model,
			System:      a.System,
			Messages:    msgs,
			Tools:       a.Tools.List(),
			MaxTokens:   a.MaxTokens,
			Temperature: a.Temperature,
		}

		asst = ai.Message{}
		asst.Role = ai.RoleAssistant
		done := false
		callErr := a.Client.ChatStream(ctx, req, func(evt ai.StreamEvent) error {
			switch {
			case evt.Reasoning != "":
				onEvent(Event{Type: EventReasoning, Content: evt.Reasoning})
			case evt.Content != "":
				asst.Content += evt.Content
				onEvent(Event{Type: EventText, Content: evt.Content})
			case evt.ToolCall != nil:
				asst.ToolCalls = append(asst.ToolCalls, *evt.ToolCall)
			case evt.Done:
				done = true
				if evt.Usage != nil {
					a.Usage.InputTokens += evt.Usage.InputTokens
					a.Usage.OutputTokens += evt.Usage.OutputTokens
					a.Usage.CachedTokens += evt.Usage.CachedTokens
				}
			case evt.Error != nil:
				return evt.Error
			}
			return nil
		})
		if callErr != nil {
			// 用户主动停止（context canceled）不算错误，不推错误事件
			if !errors.Is(callErr, context.Canceled) {
				onEvent(Event{Type: EventError, Error: callErr.Error()})
			}
			return callErr
		}
		_ = done

		a.Session.Add(asst)

		if len(asst.ToolCalls) == 0 {
			if a.OnDone != nil {
				a.OnDone(ctx)
			}
			onEvent(Event{Type: EventDone})
			return nil
		}

		// Execute tool calls sequentially (order from the model).
		for _, tc := range asst.ToolCalls {
			if err := ctx.Err(); err != nil {
				return err
			}
			onEvent(Event{Type: EventToolStart, ToolName: tc.Function.Name, ToolArgs: tc.Function.Arguments})
			out, terr := a.runTool(ctx, tc, onEvent)
			if terr != nil {
				out = fmt.Sprintf("TOOL ERROR: %v", terr)
			}
			if a.RedactSecrets {
				out = RedactSecrets(out)
			}
			if a.OnAfterTool != nil {
				out = a.OnAfterTool(ctx, tc.Function.Name, tc.Function.Arguments, out)
			}
			args := tc.Function.Arguments
			if a.RedactSecrets {
				args = RedactSecrets(args)
			}
			logx.ToolCall(a.TraceID, tc.Function.Name, args, out)
			onEvent(Event{Type: EventToolDone, ToolName: tc.Function.Name, ToolOut: out})
			a.Session.Add(ai.Message{Role: ai.RoleTool, ToolCallID: tc.ID, ToolName: tc.Function.Name, Content: out})
		}
	}
	// 达到迭代上限时助手消息可能带有未执行完的 tool_calls；补写合成结果，
	// 避免会话里留下"有调用无结果"的孤儿消息导致后续 API 请求被 400 拒绝。
	for _, tc := range asst.ToolCalls {
		a.Session.Add(ai.Message{
			Role: ai.RoleTool, ToolCallID: tc.ID, ToolName: tc.Function.Name,
			Content: "TOOL ERROR: reached max iterations without a final answer",
		})
	}
	onEvent(Event{Type: EventError, Error: "已达最大迭代次数，未得到最终回答"})
	return errors.New("max iterations reached without a final answer")
}

// permissionMode 返回工具的执行模式：allow / ask / deny。
func (a *Agent) permissionMode(tool string) string {
	if len(a.Permissions) == 0 {
		return "allow"
	}
	if m, ok := a.Permissions[tool]; ok {
		return m
	}
	if m, ok := a.Permissions["*"]; ok {
		return m
	}
	return "allow"
}

// runTool 执行单个工具，先做权限检查；支持迭代式自动重试。
func (a *Agent) runTool(ctx context.Context, tc ai.ToolCall, onEvent func(Event) error) (string, error) {
	switch a.permissionMode(tc.Function.Name) {
	case "deny":
		return "已拒绝执行 " + tc.Function.Name + "（权限配置为禁止）", nil
	case "ask":
		if a.Ask != nil {
			ok, aerr := a.Ask(ctx, tc.Function.Name, tc.Function.Arguments)
			if aerr != nil {
				return "", aerr
			}
			if !ok {
				return "用户拒绝执行工具 " + tc.Function.Name, nil
			}
		} else {
			// Ask 未接线时按拒绝处理，避免“ask”静默放行高风险工具。
			return "已拒绝执行 " + tc.Function.Name + "（需人工确认，但当前无确认通道）", nil
		}
	}
	var out string
	var terr error
	max := 0
	if a.ToolAutoRetry {
		max = a.ToolRetryMax
		if max <= 0 {
			max = 3
		}
	}
	if a.OnBeforeTool != nil {
		allow, nextArgs := a.OnBeforeTool(ctx, tc.Function.Name, tc.Function.Arguments)
		if !allow {
			reason := nextArgs
			if reason == "" {
				reason = "插件拒绝了该工具调用"
			}
			return reason, nil
		}
		if nextArgs != "" {
			tc.Function.Arguments = nextArgs
		}
	}
	// 注入工作目录之外路径的人工确认钩子：工具访问外部路径时先问用户。
	if a.Ask != nil {
		ctx = withPathApprover(ctx, func(ctx context.Context, path, tool string) bool {
			if a.AutoAllowPaths {
				return true
			}
			ok, err := a.Ask(ctx, "Path", tool+": "+path)
			return err == nil && ok
		})
	}
	exec := func() (string, error) {
		return a.Tools.Execute(ctx, tc.Function.Name, []byte(tc.Function.Arguments))
	}
	for attempt := 0; ; attempt++ {
		out, terr = exec()
		// 迭代式自动重试：仅对“错误或空结果”重试
		shouldRetry := terr != nil || strings.TrimSpace(out) == ""
		if !shouldRetry || attempt >= max {
			break
		}
		onEvent(Event{Type: EventStatus,
			Content: fmt.Sprintf("工具 %s 未返回可用结果，自动重试 (%d/%d)…",
				tc.Function.Name, attempt+1, max)})
		select {
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		case <-ctx.Done():
			return out, terr
		}
	}
	return out, terr
}
