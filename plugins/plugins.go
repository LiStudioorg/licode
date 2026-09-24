// Package plugins 收录编译进二进制的内置 Cordis 插件。
//
// 插件本身不实现工具逻辑（实现在 internal/agent 中，保持与旧路径兼容），
// 而是把它们注册进 Cordis 工具树，使权限/审计/脱敏策略以 waterfall 监听器
// 的形式存在——这是"无特权核心"的接线层。
package plugins

import (
	"licode/cordis"
	"licode/internal/agent"
)

// fsToolNames 是文件类工具（builtin-tools-fs 插件注册的部分）。
var fsToolNames = []string{"Read", "Write", "Edit", "ListDirectory", "Grep", "Glob", "Delete", "Move"}

// shellToolNames 是命令执行类工具（builtin-tools-shell 插件注册的部分）。
var shellToolNames = []string{"Shell"}

// RegisterAll 加载全部内置插件（不含 llm.config，由宿主 Provide）。
func RegisterAll(r *cordis.Runtime, shell agent.ShellConfig) error {
	plugins := []cordis.Plugin{
		BuiltinToolsFS(shell),
		BuiltinToolsShell(shell),
		BuiltinPermissions(),
		BuiltinLLM(),
	}
	plugins = append(plugins, BuiltinLLMProviders()...)
	for _, p := range plugins {
		if err := r.Load(p); err != nil {
			return err
		}
	}
	return nil
}

// BuiltinToolsFS 把文件类内置工具（含 ~/.licode/tools 热加载的外部命令工具）
// 注册进 Cordis 工具树。
func BuiltinToolsFS(shell agent.ShellConfig) cordis.Plugin {
	return cordis.Plugin{
		Name: "builtin-tools-fs",
		Apply: func(ctx cordis.Context) error {
			return registerFromBridge(ctx, shell, fsToolNames, true)
		},
	}
}

// BuiltinToolsShell 把 Shell 工具注册进 Cordis 工具树。
func BuiltinToolsShell(shell agent.ShellConfig) cordis.Plugin {
	return cordis.Plugin{
		Name: "builtin-tools-shell",
		Apply: func(ctx cordis.Context) error {
			return registerFromBridge(ctx, shell, shellToolNames, false)
		},
	}
}

// registerFromBridge 通过临时 agent.Registry 复用现有工具实现。
func registerFromBridge(ctx cordis.Context, shell agent.ShellConfig, names []string, withExternal bool) error {
	reg := agent.NewRegistry()
	agent.RegisterDefaultTools(reg, shell)
	if withExternal {
		reg.MergeFrom(agent.ExternalTools)
	}
	for _, name := range names {
		if err := registerOne(ctx, reg, name); err != nil {
			return err
		}
	}
	if withExternal {
		for _, name := range agent.ExternalTools.Names() {
			if contains(fsToolNames, name) || contains(shellToolNames, name) {
				continue
			}
			if err := registerOne(ctx, reg, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func registerOne(ctx cordis.Context, reg *agent.Registry, name string) error {
	t, ok := reg.Get(name)
	if !ok {
		return nil
	}
	return ctx.RegisterTool(cordis.Tool{
		Name:        t.Name,
		Description: t.Description,
		Schema:      t.Schema,
		Execute:     t.Run,
	})
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
