package agent

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
	"strings"
)

// promptFS 内嵌默认提示词（零配置即可用的模型/模式覆盖兜底）。
// 磁盘覆盖优先：~/.licode/prompts/model-<id>.md、mode-<mode>.md（见 prompts_disk.go）；
// 磁盘没有对应文件时，回退到这里按“模型家族”匹配的内嵌默认，保证开箱即用。
//
//go:embed prompts
var promptFS embed.FS

// modelPromptOrder 按“具体 → 通用”顺序匹配模型 ID 子串，返回对应内嵌提示词文件。
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

// loadPromptFile 读取提示词：overrideDir/<rel> 存在且非空则用它，否则回退内嵌默认。
// 都是确定性读取（无时间/随机/locale 依赖）。
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

// embeddedModelDefault 按 modelID 子串匹配返回内嵌模型层提示词（A 区，磁盘覆盖缺失时兜底）。
// 匹配不到返回空串（仅用基础模板）。
func embeddedModelDefault(modelID string) string {
	id := strings.ToLower(modelID)
	for _, m := range modelPromptOrder {
		if strings.Contains(id, m.sub) {
			return loadPromptFile("", m.file)
		}
	}
	return ""
}

// embeddedModeDefault 返回内嵌模式提示词（C 区尾部；目前仅 plan 有默认）。
func embeddedModeDefault(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModePlan:
		return loadPromptFile("", "prompts/mode/plan.txt")
	default:
		return ""
	}
}
