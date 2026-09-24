package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"licode/internal/ai"
)

// 工具目录必须按名字典序，且两次调用字节一致：map 遍历随机化会击穿 Provider 前缀缓存。
func TestRegistryListDeterministic(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"Write", "Read", "Grep", "Glob", "Edit", "Shell"} {
		_ = r.Register(Tool{Name: n, Description: "d", Schema: map[string]any{"type": "object"}})
	}
	first := r.List()
	for i := 0; i < 20; i++ {
		got := r.List()
		if len(got) != len(first) {
			t.Fatalf("list length changed")
		}
		for j := range got {
			if got[j].Function.Name != first[j].Function.Name {
				t.Fatalf("non-deterministic order: %v vs %v", names(got), names(first))
			}
		}
	}
	// 必须升序
	for j := 1; j < len(first); j++ {
		if first[j-1].Function.Name > first[j].Function.Name {
			t.Fatalf("not sorted: %v", names(first))
		}
	}
}

func names(ts []ai.Tool) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Function.Name
	}
	return out
}

// 冻结前缀必须：确定性（同输入同字节）、不含日期、无未替换占位符。
func TestAnchorDeterministicAndDateFree(t *testing.T) {
	o := AnchorOpts{ModelName: "gpt-4o", ModelID: "openai/gpt-4o", Cwd: "/tmp/proj", OS: "linux", IsGit: true, ProjectSummary: "PG", AppendPrompt: "AP"}
	a := BuildAnchor(o)
	b := BuildAnchor(o)
	if a.System != b.System {
		t.Fatal("anchor not byte-stable across builds")
	}
	if a.Hash != b.Hash || len(a.Hash) != 64 {
		t.Fatalf("bad hash: %q", a.Hash)
	}
	if strings.Contains(a.System, "Today's date") {
		t.Fatal("frozen prefix must not embed the volatile date")
	}
	if strings.Contains(a.System, "{{") {
		t.Fatal("unreplaced placeholder left in system prompt")
	}
	if !strings.Contains(a.System, "/tmp/proj") || !strings.Contains(a.System, "openai/gpt-4o") {
		t.Fatal("B-zone env values missing")
	}
	// 模型覆盖 + 项目摘要 + 附加提示词都应进入 A 区
	if !strings.Contains(a.System, "PG") || !strings.Contains(a.System, "AP") {
		t.Fatal("project summary / append prompt not folded into anchor")
	}
}

func TestSelectModelPrompt(t *testing.T) {
	want := map[string]string{
		"openai/gpt-4o":        "prompts/model/gpt.txt",
		"anthropic/claude-3-5": "prompts/model/anthropic.txt",
		"provider/anthropic-x": "prompts/model/anthropic.txt",
		"google/gemini-pro":    "prompts/model/gemini.txt",
		"openai/codex-mini":    "prompts/model/codex.txt",
		"qwen2.5-coder":        "prompts/model/qwen.txt",
		"mistral-large":        "", // 未知模型 → 空（仅用基础模板）
	}
	for id, file := range want {
		got := selectModelPrompt(id, "")
		exp := loadPromptFile("", file)
		if got != exp {
			t.Fatalf("model %s mapped wrong: got %d bytes want %d bytes (%q)", id, len(got), len(exp), file)
		}
	}
	// 不同厂商映射到不同内容（codex 与 gpt 不能撞车）
	if selectModelPrompt("openai/codex", "") == selectModelPrompt("openai/gpt-4o", "") {
		t.Fatal("codex and gpt overlays must differ")
	}
}

// C 区上下文必须并入最后一条 user 消息（角色不变，兼容 Anthropic 的交替约束）。
func TestMergeContextTailPreservesRole(t *testing.T) {
	in := []ai.Message{
		{Role: ai.RoleUser, Content: "问题"},
		{Role: ai.RoleAssistant, Content: "回答"},
		{Role: ai.RoleUser, Content: "第二个问题"},
	}
	out := mergeContextTail(in, "Today's date: 2020-01-01\nMode: build")
	if len(out) != len(in) {
		t.Fatal("must not add a new message")
	}
	if out[len(out)-1].Role != ai.RoleUser {
		t.Fatal("last message role must stay user")
	}
	if !strings.Contains(out[len(out)-1].Content, "<context>") || !strings.Contains(out[len(out)-1].Content, "第二个问题") {
		t.Fatal("context not merged into last user message")
	}
	// 原切片不被修改（返回副本）
	if strings.Contains(in[len(in)-1].Content, "<context>") {
		t.Fatal("input slice mutated")
	}
}

// 溢出：短内容原样返回；长内容有界化并落盘、预览指向完整文件。
func TestSpillOutput(t *testing.T) {
	dir := t.TempDir()
	if got := spillOutput(dir, "Shell", "short", 100); got != "short" {
		t.Fatalf("short content should pass through, got %q", got)
	}
	long := strings.Repeat("x", 5000)
	got := spillOutput(dir, "Shell", long, 1000)
	if len(got) >= len(long) {
		t.Fatalf("expected truncation, kept %d bytes", len(got))
	}
	if !strings.Contains(got, "完整内容见") {
		t.Fatal("missing spill reference")
	}
	// 预览指向的文件确实存在且含完整内容
	var spilled string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".txt") {
			spilled = p
		}
		return nil
	})
	if spilled == "" {
		t.Fatal("spilled file not written")
	}
	data, _ := os.ReadFile(spilled)
	if string(data) != long {
		t.Fatal("spilled file does not contain full content")
	}
}

// plan 模式只暴露只读工具（按需激活）；build 模式暴露全部（除 deny）。
func TestVisibleToolsPlanMode(t *testing.T) {
	a := NewAgent(&mockClient{model: "m"}, "sys")
	a.Permissions = map[string]string{"*": "ask"}
	a.Mode = ModePlan
	got := map[string]bool{}
	for _, tool := range a.visibleTools() {
		got[tool.Function.Name] = true
	}
	for _, ro := range []string{"Read", "Glob", "Grep", "ListDirectory"} {
		if !got[ro] {
			t.Fatalf("plan mode should keep read-only tool %s", ro)
		}
	}
	for _, mutating := range []string{"Write", "Edit", "Delete", "Move", "Shell"} {
		if got[mutating] {
			t.Fatalf("plan mode must hide mutating tool %s", mutating)
		}
	}
	// build 模式应重新看到写工具
	a.Mode = ModeBuild
	seen := false
	for _, tool := range a.visibleTools() {
		if tool.Function.Name == "Write" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("build mode should expose Write")
	}
}

// deny 工具即使在 build 模式也不进入目录。
func TestVisibleToolsHidesDenied(t *testing.T) {
	a := NewAgent(&mockClient{model: "m"}, "sys")
	a.Permissions = map[string]string{"*": "allow", "Shell": "deny"}
	a.Mode = ModeBuild
	for _, tool := range a.visibleTools() {
		if tool.Function.Name == "Shell" {
			t.Fatal("denied tool must not appear in catalog")
		}
	}
}

// 运行结束后应发出一次 EventStats，且请求计数与累计用量可用。
func TestRunEmitsStats(t *testing.T) {
	ag := buildMockAgent([]func(req mockReq) []mockStep{
		func(req mockReq) []mockStep { return []mockStep{{content: "done"}} },
	})
	var sawStats bool
	_ = ag.Run(context.Background(), "hi", func(e Event) {
		if e.Type == EventStats && e.Stats != nil {
			sawStats = true
		}
	})
	if !sawStats {
		t.Fatal("expected an EventStats before done")
	}
}
