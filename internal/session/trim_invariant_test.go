package session

import (
	"strings"
	"testing"

	"licode/internal/ai"
)

// fill 生成 n 轮 user/assistant 往返，内容足够长以触发预算裁剪。
func fill(s *Session, n int) {
	for i := 0; i < n; i++ {
		s.Add(ai.Message{Role: ai.RoleUser, Content: strings.Repeat("u", 400)})
		s.Add(ai.Message{Role: ai.RoleAssistant, Content: strings.Repeat("a", 400), ToolCalls: []ai.ToolCall{{ID: "tc1"}}})
		s.Add(ai.Message{Role: ai.RoleTool, ToolCallID: "tc1", Content: strings.Repeat("t", 400)})
	}
}

// TestMessagesForLLMTrimNoOrphans 验证裁剪产物的 provider 不变式：
// 首条必须是 user；不得出现失去父 assistant 的 tool 消息；
// 末尾不得是带 tool_calls 却无结果的 assistant（该形态会让后续请求永久 400）。
func TestMessagesForLLMTrimNoOrphans(t *testing.T) {
	s := NewSession(1000) // 小预算强制裁剪
	fill(s, 30)
	// 末尾停在带 tool_calls 无结果的 assistant（工具中断场景）。
	s.Add(ai.Message{Role: ai.RoleAssistant, Content: "will call", ToolCalls: []ai.ToolCall{{ID: "tcX"}}})

	out := s.MessagesForLLM("system prompt")
	if len(out) == 0 {
		t.Fatal("trim produced empty context")
	}
	if out[0].Role != ai.RoleUser {
		t.Fatalf("first message must be user, got %s", out[0].Role)
	}
	seen := map[string]bool{} // 已见 assistant 的 tool_call id
	for _, m := range out {
		if m.Role == ai.RoleTool {
			if !seen[m.ToolCallID] {
				t.Fatalf("orphan tool message (parent assistant trimmed): %v", m.ToolCallID)
			}
		}
		if m.Role == ai.RoleAssistant {
			for _, tc := range m.ToolCalls {
				seen[tc.ID] = true
			}
		}
	}
	if last := out[len(out)-1]; last.Role == ai.RoleAssistant && len(last.ToolCalls) > 0 {
		// 检查其结果是否缺失：tool 结果应紧跟其后，因此它是最后一条即为孤儿。
		t.Fatal("trailing assistant with tool_calls but no results")
	}
}

// TestRestoreReplacesNotAppends 验证 Restore 为替换语义：重复调用不复制消息。
func TestRestoreReplacesNotAppends(t *testing.T) {
	s := NewSession(0)
	msgs := []ai.Message{{Role: ai.RoleUser, Content: "a"}}
	s.Restore("t1", msgs, "")
	s.Restore("t1", msgs, "")
	if s.Len() != 1 {
		t.Fatalf("Restore must replace, len=%d", s.Len())
	}
}
