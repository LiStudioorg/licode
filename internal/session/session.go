// Package session manages multi-turn conversation history with context-window
// truncation. It stores provider-agnostic messages and produces a trimmed
// message list that fits the configured token budget.
package session

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	"licode/internal/ai"
)

// DefaultMaxTokens is the per-session context budget used when unset.
const DefaultMaxTokens = 128_000

// TokenEstimator approximates token count. ~4 chars per token is a reasonable
// heuristic for mixed English/Chinese text.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	n := utf8.RuneCountInString(s)
	return n/4 + 1
}

// Session holds the rolling conversation history.
type Session struct {
	mu          sync.Mutex
	id          string
	title       string
	messages    []ai.Message
	maxTok      int
	summary     string
	usage       ai.Usage // 累计 token 用量（含缓存读取）
	mode        string   // 运行模式：build/plan（plan=只读）；空按 build 处理
	onChange    func()
	alwaysAllow map[string]bool
}

// SetOnChange 设置消息变化回调（用于实时落盘）。
func (s *Session) SetOnChange(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = fn
}

// NewSession creates a session with a generated id.
func NewSession(maxTokens int) *Session {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	return &Session{id: genID(), title: "新对话", maxTok: maxTokens, alwaysAllow: map[string]bool{}}
}

// AlwaysAllowed 返回某工具是否在本对话被"始终允许"。
func (s *Session) AlwaysAllowed(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.alwaysAllow[name]
}

// SetAlwaysAllowed 记录某工具在本对话始终允许。
func (s *Session) SetAlwaysAllowed(name string) {
	s.mu.Lock()
	if s.alwaysAllow == nil {
		s.alwaysAllow = map[string]bool{}
	}
	s.alwaysAllow[name] = true
	onChange := s.onChange
	s.mu.Unlock()
	if onChange != nil {
		go onChange()
	}
}

// AlwaysAllowedList 返回本对话"始终允许"的工具列表。
func (s *Session) AlwaysAllowedList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for name := range s.alwaysAllow {
		if s.alwaysAllow[name] {
			out = append(out, name)
		}
	}
	return out
}

// ID returns the session identifier.
func (s *Session) ID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

// Title returns the session title.
func (s *Session) Title() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.title
}

// SetTitle updates the session title.
func (s *Session) SetTitle(t string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t != "" {
		s.title = t
	}
}

// Restore 导入时恢复会话内容（标题/消息/摘要）。
func (s *Session) Restore(title string, msgs []ai.Message, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if title != "" {
		s.title = title
	}
	s.messages = append(s.messages, msgs...)
	if summary != "" {
		s.summary = summary
	}
}

// Summary returns the compacted context summary, if any.
func (s *Session) Summary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.summary
}

// SetSummary stores a compacted context summary.
func (s *Session) SetSummary(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary = v
}

// Mode 返回会话运行模式（"" 视为 "build"）。
func (s *Session) Mode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mode == "" {
		return "build"
	}
	return s.mode
}

// SetMode 设置会话运行模式（build/plan），并触发落盘。
func (s *Session) SetMode(m string) {
	if m != "build" && m != "plan" {
		return
	}
	s.mu.Lock()
	s.mode = m
	onChange := s.onChange
	s.mu.Unlock()
	if onChange != nil {
		go onChange()
	}
}

func genID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// SetID 覆盖会话 ID（加载存档时使用）。
func (s *Session) SetID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != "" {
		s.id = id
	}
}

// fileRecord 是会话的磁盘存档格式。
type fileRecord struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Summary     string       `json:"summary"`
	Mode        string       `json:"mode,omitempty"`
	MaxTok      int          `json:"max_tokens"`
	Messages    []ai.Message `json:"messages"`
	Usage       ai.Usage     `json:"usage"`
	AlwaysAllow []string     `json:"always_allow,omitempty"`
}

// SaveToFile 将会话写入磁盘（对话记录）。
func (s *Session) SaveToFile(path string) error {
	s.mu.Lock()
	rec := fileRecord{ID: s.id, Title: s.title, Summary: s.summary, Mode: s.mode, MaxTok: s.maxTok, Usage: s.usage}
	rec.Messages = make([]ai.Message, len(s.messages))
	copy(rec.Messages, s.messages)
	for name, allow := range s.alwaysAllow {
		if allow {
			rec.AlwaysAllow = append(rec.AlwaysAllow, name)
		}
	}
	s.mu.Unlock()
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	// 原子写：先写同目录临时文件再 rename。直接 WriteFile 在写入中途断电/崩溃
	// 会留下截断的 JSON，下次启动该会话静默丢失。
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后为 no-op
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// LoadSessionFile 从磁盘加载会话存档。
func LoadSessionFile(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rec fileRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	s := NewSession(rec.MaxTok)
	s.id = rec.ID
	s.title = rec.Title
	s.summary = rec.Summary
	s.mode = rec.Mode
	s.usage = rec.Usage
	s.messages = rec.Messages
	for _, name := range rec.AlwaysAllow {
		s.alwaysAllow[name] = true
	}
	return s, nil
}

func (s *Session) Add(m ai.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, m)
}

func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = s.messages[:0]
}

// Messages returns a copy of all messages (no truncation).
func (s *Session) Messages() []ai.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ai.Message, len(s.messages))
	copy(out, s.messages)
	return out
}

// Len returns the number of stored messages.
func (s *Session) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}

// LastRole 返回最后一条消息的角色（空会话返回 ""），
// 供调用方判断会话是否以孤立的 user/tool 消息收尾。
func (s *Session) LastRole() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return ""
	}
	return s.messages[len(s.messages)-1].Role
}

// MaxTokens 返回本会话的上下文预算。
func (s *Session) MaxTokens() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxTok
}

// SetMaxTokens 设置会话上下文预算（0=不裁剪），用于上下文窗口滑动保护。
func (s *Session) SetMaxTokens(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxTok = n
}

// AddUsage 累计此对话消耗的 token（含缓存读取）。
func (s *Session) AddUsage(u ai.Usage) {
	s.mu.Lock()
	s.usage.InputTokens += u.InputTokens
	s.usage.OutputTokens += u.OutputTokens
	s.usage.CachedTokens += u.CachedTokens
	s.mu.Unlock()
}

// Usage 返回累计用量快照。
func (s *Session) Usage() ai.Usage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.usage
}

// MessagesForLLM returns the message list trimmed to the context budget.
// Truncation strategy: keep the most recent messages that fit, dropping the
// oldest tool-call pairs first so the tail of the conversation always survives.
// 若存在压缩摘要（SetSummary），会把摘要作为一条 user 消息前置。
func (s *Session) MessagesForLLM(system string) []ai.Message {
	s.mu.Lock()
	msgs := make([]ai.Message, len(s.messages))
	copy(msgs, s.messages)
	summary := s.summary
	maxTok := s.maxTok
	s.mu.Unlock()

	// maxTok<=0 表示不裁剪（SetMaxTokens 文档承诺的语义）：
	// 若不特判，预算比较 total+cost > 0 恒成立，会话会被裁到只剩 1 条消息。
	if maxTok <= 0 {
		out := msgs
		if summary != "" {
			out = append([]ai.Message{{Role: ai.RoleUser, Content: "【之前对话的压缩摘要】\n" + summary}}, msgs...)
		}
		return out
	}

	var budget int
	if system != "" {
		budget = EstimateTokens(system)
	}

	// Walk backwards, collecting messages until the budget is exceeded.
	var tail []ai.Message
	total := budget
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		cost := EstimateTokens(m.Content)
		for _, tc := range m.ToolCalls {
			cost += EstimateTokens(tc.Function.Arguments)
		}
		if total+cost > maxTok && len(tail) > 0 {
			break
		}
		tail = append(tail, m)
		total += cost
		if total >= maxTok {
			break
		}
	}
	// Reverse to restore chronological order.
	for i, j := 0, len(tail)-1; i < j; i, j = i+1, j-1 {
		tail[i], tail[j] = tail[j], tail[i]
	}
	// 裁剪后开头必须是 user：
	//   - 开头的 tool 消息失去了父 assistant(tool_calls)（OpenAI 直接 400）；
	//   - 开头的 assistant 消息违反 Anthropic「首条必须为 user」的约束（同样 400）。
	// 逐条丢弃开头非 user 消息即可同时满足（丢弃 assistant 后其 tool 结果也成
	// 为开头 tool 消息，会被同一循环继续丢弃）。
	for len(tail) > 0 && tail[0].Role != ai.RoleUser {
		tail = tail[1:]
	}
	// 末尾不能是带 tool_calls 却没有结果的 assistant：若工具执行中断，
	// 这样一条消息会让后续所有请求 400。开头已是 user，删除它不会再造孤儿。
	for len(tail) > 0 && tail[len(tail)-1].Role == ai.RoleAssistant && len(tail[len(tail)-1].ToolCalls) > 0 {
		tail = tail[:len(tail)-1]
	}
	if summary != "" {
		head := make([]ai.Message, 0, len(tail)+1)
		head = append(head, ai.Message{Role: ai.RoleUser, Content: "【之前对话的压缩摘要】\n" + summary})
		head = append(head, tail...)
		return head
	}
	return tail
}

// TrimHead 移除最旧的 n 条消息（压缩摘要覆盖后释放空间）。
func (s *Session) TrimHead(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n >= len(s.messages) {
		s.messages = nil
		return
	}
	s.messages = s.messages[n:]
}

// Dropped returns the messages that would be trimmed by MessagesForLLM.
// 用于判断是否需要触发上下文压缩。
func (s *Session) Dropped(system string) int {
	s.mu.Lock()
	msgs := make([]ai.Message, len(s.messages))
	copy(msgs, s.messages)
	maxTok := s.maxTok
	s.mu.Unlock()

	if maxTok <= 0 {
		return 0 // 不裁剪 => 永远不会触发压缩
	}
	total := 0
	if system != "" {
		total = EstimateTokens(system)
	}
	dropped := 0
	for _, m := range msgs {
		cost := EstimateTokens(m.Content)
		for _, tc := range m.ToolCalls {
			cost += EstimateTokens(tc.Function.Arguments)
		}
		if total+cost > maxTok {
			dropped += cost
		} else {
			total += cost
		}
	}
	return dropped
}
