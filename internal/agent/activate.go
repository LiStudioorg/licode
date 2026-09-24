package agent

import (
	"sort"

	"licode/internal/ai"
)

// 运行模式。build=完整工具；plan=只读（不可改工作区）。
const (
	ModeBuild = "build"
	ModePlan  = "plan"
)

// readOnlyToolSet 是 plan 模式下仍可用的只读/无副作用工具集合（按需工具激活的“常驻只读集”）。
// 其余工具（Write/Edit/Delete/Move/Shell/MCP/外部命令等）在 plan 模式下不进入目录，
// 模型看不到也就无法调用，天然实现“只读规划”。
var readOnlyToolSet = map[string]bool{
	"Read":          true,
	"ListDirectory": true,
	"Glob":          true,
	"Grep":          true,
}

// IsReadOnlyTool 报告工具在 plan 模式下是否可用。
func IsReadOnlyTool(name string) bool { return readOnlyToolSet[name] }

// VisibleTools 是 visibleTools 的导出包装，供上层（keepalive 预热）复用与真实请求
// 完全一致的“稳定工具前缀”（同序、同集合），从而命中同一个 Provider 缓存块。
func (a *Agent) VisibleTools() []ai.Tool { return a.visibleTools() }

// visibleTools 返回本次请求要发给模型的工具目录：
//   - 按工具名确定性升序排序（关键：Map 遍历顺序随机会让每次请求的工具前缀都不同，
//     直接击穿 Provider 的前缀缓存，这里一次性消除该非确定性）。
//   - 过滤掉权限为 deny 的工具。
//   - plan 模式下只保留只读工具（按需激活的只读子集）。
func (a *Agent) visibleTools() []ai.Tool {
	names := a.Tools.Names()
	plan := a.Mode == ModePlan
	allowed := make([]string, 0, len(names))
	for _, n := range names {
		if a.permissionMode(n) == "deny" {
			continue
		}
		if plan && !readOnlyToolSet[n] {
			continue
		}
		allowed = append(allowed, n)
	}
	sort.Strings(allowed)
	return a.Tools.Subset(allowed)
}
