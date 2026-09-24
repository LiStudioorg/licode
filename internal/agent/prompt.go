package agent

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
	"strings"
)

// promptFS 内嵌默认提示词（作为磁盘覆盖缺失时的兜底）。
// 磁盘覆盖优先级更高：~/.licode/prompts/<relpath>（由 overrideDir 传入）。
//
//go:embed prompts
var promptFS embed.FS

// modelPromptOrder 按“具体 → 通用”顺序匹配模型 ID 子串，返回对应提示词文件。
// 匹配用 strings.Contains(小写 modelID)，因此 codex/beast 等要排在 gpt 之前。
var modelPromptOrder = []struct {
	sub  string
	file string
}{
	{"codex", "prompts/model/codex.txt"},
	{"beast", "prompts/model/beast.txt"},
	{"gemini", "prompts/model/gemini.txt"},
	{"claude", "prompts/model/anthropic.txt"},
	{"anthropic", "prompts/model/anthropic.txt"},
	{"trinity", "prompts/model/trinity.txt"},
	{"qwen", "prompts/model/qwen.txt"},
	{"gpt", "prompts/model/gpt.txt"},
}

// loadPromptFile 读取提示词：优先磁盘覆盖目录（overrideDir/<rel>），否则内嵌默认。
// 二者都取不到时返回空串。所有读取都是确定性的（无时间/随机/locale 依赖）。
func loadPromptFile(overrideDir, rel string) string {
	if overrideDir != "" {
		if b, err := os.ReadFile(filepath.Join(overrideDir, rel)); err == nil && len(bytes.TrimSpace(b)) > 0 {
			return strings.TrimSpace(string(b))
		}
	}
	if b, err := promptFS.ReadFile(rel); err == nil {
		return strings.TrimSpace(string(b))
	}
	return ""
}

// selectModelPrompt 依据 modelID 选择模型层提示词（A 区的一部分）。
// 匹配不到时返回空串（仅用基础模板）。
func selectModelPrompt(modelID, overrideDir string) string {
	id := strings.ToLower(modelID)
	for _, m := range modelPromptOrder {
		if strings.Contains(id, m.sub) {
			return loadPromptFile(overrideDir, m.file)
		}
	}
	return ""
}

// modePrompt 返回运行模式层提示词（C 区尾部追加；plan 模式为只读约束）。
func modePrompt(mode, overrideDir string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModePlan:
		return loadPromptFile(overrideDir, "prompts/mode/plan.txt")
	default:
		return ""
	}
}
