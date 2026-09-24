package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// modePrompt 读取 ~/.licode/prompts 下的模式提示词（mode-<mode>.md），
// 作为 C 区易变上下文注入；目录为空、文件缺失一律返回空串（不影响其余提示词）。
func modePrompt(mode, promptDir string) string {
	if promptDir == "" || mode == "" {
		return ""
	}
	return readPromptFile(filepath.Join(promptDir, "mode-"+sanitizePromptName(mode)+".md"))
}

// selectModelPrompt 按 modelID 读取模型层提示词覆盖（model-<modelID>.md），
// 拼进 A 区冻结前缀；同一 (modelID, promptDir) 结果恒定，不破坏前缀缓存。
func selectModelPrompt(modelID, promptDir string) string {
	if promptDir == "" || modelID == "" {
		return ""
	}
	return readPromptFile(filepath.Join(promptDir, "model-"+sanitizePromptName(modelID)+".md"))
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
