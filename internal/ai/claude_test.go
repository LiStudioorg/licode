package ai

import (
	"encoding/json"
	"testing"
)

func decodeBody(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	return m
}

func TestClaudeBuildBodyCacheControl(t *testing.T) {
	p := &ClaudeProvider{model: "claude-3-5-sonnet"}
	tools := []Tool{
		{Type: "function", Function: FunctionSpec{Name: "Read", Parameters: json.RawMessage(`{"type":"object"}`)}},
		{Type: "function", Function: FunctionSpec{Name: "Write", Parameters: json.RawMessage(`{"type":"object"}`)}},
	}

	// 关闭缓存：system 为字符串（与改动前逐字节一致的形态）。
	b, err := p.buildBody(ChatRequest{System: "SYS", Messages: []Message{{Role: RoleUser, Content: "hi"}}, Tools: tools}, true)
	if err != nil {
		t.Fatal(err)
	}
	m := decodeBody(t, b)
	if s, ok := m["system"].(string); !ok || s != "SYS" {
		t.Fatalf("cache off: system should be string, got %#v", m["system"])
	}
	if tt, ok := m["tools"].([]any); !ok || len(tt) != 2 {
		t.Fatalf("tools missing: %#v", m["tools"])
	} else if _, has := tt[1].(map[string]any)["cache_control"]; has {
		t.Fatal("cache off: tools must not carry cache_control")
	}

	// system 为空且关闭缓存：system 字段应被省略（omitempty 行为）。
	b, _ = p.buildBody(ChatRequest{System: "", Messages: []Message{{Role: RoleUser, Content: "hi"}}}, true)
	if _, has := decodeBody(t, b)["system"]; has {
		t.Fatal("empty system must be omitted when cache off")
	}

	// 开启缓存 + system：system 为数组块且带 ephemeral 断点。
	b, err = p.buildBody(ChatRequest{System: "SYS", PromptCache: true, Messages: []Message{{Role: RoleUser, Content: "hi"}}, Tools: tools}, true)
	if err != nil {
		t.Fatal(err)
	}
	m = decodeBody(t, b)
	arr, ok := m["system"].([]any)
	if !ok || len(arr) != 1 {
		t.Fatalf("cache on: system should be 1-element array, got %#v", m["system"])
	}
	blk := arr[0].(map[string]any)
	if cc, ok := blk["cache_control"].(map[string]any); !ok || cc["type"] != "ephemeral" {
		t.Fatalf("cache on: system block missing ephemeral cache_control: %#v", blk)
	}
	if tt := m["tools"].([]any); len(tt) != 2 {
		t.Fatal("tools should be preserved with cache on")
	}

	// 开启缓存但 system 为空 + 有工具：断点退到最后一个工具。
	b, _ = p.buildBody(ChatRequest{System: "", PromptCache: true, Messages: []Message{{Role: RoleUser, Content: "hi"}}, Tools: tools}, true)
	m = decodeBody(t, b)
	if _, has := m["system"]; has {
		t.Fatal("empty system still omitted even with cache on")
	}
	tt := m["tools"].([]any)
	if _, has := tt[len(tt)-1].(map[string]any)["cache_control"]; !has {
		t.Fatal("cache on + empty system: last tool should carry cache_control")
	}
}
