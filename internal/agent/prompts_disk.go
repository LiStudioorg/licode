package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// modePrompt 读取 ~/.licode/prompts 下的模式提示词（mode-<mode>.md），
// 作为 C 区易变上下文注入；磁盘缺失时回退内嵌默认（见 prompt.go），都没有则空串。
func modePrompt(mode, promptDir string) string {
	var out string
	if promptDir != "" && mode != "" {
		out = readPromptFile(filepath.Join(promptDir, "mode-"+sanitizePromptName(mode)+".md"))
	}
	if out == "" {
		out = embeddedModeDefault(mode)
	}
	return out
}

// selectModelPrompt 按 modelID 读取模型层提示词覆盖（model-<modelID>.md），
// 拼进 A 区冻结前缀；同一 (modelID, promptDir) 结果恒定，不破坏前缀缓存。
// 磁盘缺失时回退按模型家族匹配的内嵌默认（开箱即用），都没有则空串。
func selectModelPrompt(modelID, promptDir string) string {
	var out string
	if promptDir != "" && modelID != "" {
		out = readPromptFile(filepath.Join(promptDir, "model-"+sanitizePromptName(modelID)+".md"))
	}
	if out == "" {
		out = embeddedModelDefault(modelID)
	}
	return out
}

func readPromptFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// sanitizePromptName 把模型 ID/模式名中的路径分隔与特殊字符替换为 '-'，
// 保证拼接出的文件名安全。
func sanitizePromptName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
