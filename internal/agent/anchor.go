package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// PromptAnchor 是会话内“字节级冻结”的系统前缀（A 区模型层 + B 区环境快照）。
// 关键不变量：同一会话/同一配置/同一模型下，System 的字节序列恒定，
// 从而最大化各家 Provider 的前缀缓存命中；所有易变内容（日期/用量/模式提示词/
// RAG 记忆/用户消息/工具结果）一律不进这里，改由 C 区尾部追加。
type PromptAnchor struct {
	System string // 冻结的系统提示词（A+B 区）
	Hash   string // System 的 sha256（用于日志与稳定性校验）
}

// AnchorOpts 组装冻结前缀所需的全部确定性输入。
type AnchorOpts struct {
	ModelName      string // 模型可读名（B 区）
	ModelID        string // 模型完整 ID（B 区，也用于 selectModelPrompt）
	Cwd            string // 工作目录（B 区）
	OS             string // 操作系统（B 区）
	IsGit          bool   // 是否 git 仓库（B 区）
	SystemOverride string // ~/.licode/system-prompt.md：覆盖基础模板
	AppendPrompt   string // ~/.licode/md 附加提示词（A 区）
	ProjectSummary string // 项目摘要，如 AGENTS.md（A 区）
	PromptDir      string // ~/.licode/prompts 磁盘覆盖目录（可空）
}

// BuildAnchor 按“A 区在上、B 区在下”的固定顺序拼接系统前缀并计算哈希。
// 顺序与分隔符都是固定的，任何字段缺失都以跳过该段处理（不影响其余字节）。
func BuildAnchor(o AnchorOpts) PromptAnchor {
	var parts []string

	// A 区：基础模板（可被 system-prompt.md 覆盖）。
	base := strings.TrimSpace(o.SystemOverride)
	if base == "" {
		base = fillMainTemplate(o.Cwd, o.OS, o.IsGit, o.ModelName, o.ModelID)
	}
	parts = append(parts, base)

	// A 区：模型层覆盖（按 modelID 选择）。
	if mp := selectModelPrompt(o.ModelID, o.PromptDir); mp != "" {
		parts = append(parts, mp)
	}

	// A 区：项目摘要（AGENTS.md 等）。
	if ps := strings.TrimSpace(o.ProjectSummary); ps != "" {
		parts = append(parts, ps)
	}

	// A 区：用户自定义附加提示词（~/.licode/md）。
	if ap := strings.TrimSpace(o.AppendPrompt); ap != "" {
		parts = append(parts, ap)
	}

	sys := strings.Join(parts, "\n\n")
	sum := sha256.Sum256([]byte(sys))
	return PromptAnchor{System: sys, Hash: hex.EncodeToString(sum[:])}
}
