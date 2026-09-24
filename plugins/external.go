package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"licode/cordis"
	"licode/internal/agent"
)

// Manifest 是外部进程插件目录下 plugin.json 的清单。
//
// 采用宽松结构：未知字段忽略，permissions 可为字符串数组或对象（旧格式），
// 兼容带 args/contributes/capabilities 的富清单；工具发现以 MCP tools/list 为准，
// contributes 中的静态工具声明仅作降级提示（不在 manifest 内强制）。
type Manifest struct {
	Name        string          `json:"name"`
	ID          string          `json:"id"`
	Version     string          `json:"version"`
	Entry       string          `json:"entry"`  // 相对插件目录的可执行文件或解释器
	Args        []string        `json:"args"`   // 传给 entry 的参数（相对路径以插件目录解析）
	Description string          `json:"description"`
	Inject      []string        `json:"inject"`    // 依赖的核心服务名
	Provide     []string        `json:"provide"`   // 对外提供的服务名（占位注册）
	Permissions json.RawMessage `json:"permissions"`
	AutoStart   *bool           `json:"auto_start"` // 缺省 true
}

// Enabled 报告是否随启动自动加载（缺省启用，显式 false 关闭）。
func (m Manifest) Enabled() bool { return m.AutoStart == nil || *m.AutoStart }

// Discover 扫描 pluginsDir 下含 plugin.json 的子目录，按名称排序返回清单。
// 目录不存在返回空；清单损坏的单个插件跳过并记录到返回的告警串。
func Discover(pluginsDir string) ([]Manifest, []string) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, nil
	}
	var out []Manifest
	var warns []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		mpath := filepath.Join(pluginsDir, e.Name(), "plugin.json")
		data, err := os.ReadFile(mpath)
		if err != nil {
			continue // 没有 manifest 的目录不是插件
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			warns = append(warns, fmt.Sprintf("插件 %s 清单解析失败: %v", e.Name(), err))
			continue
		}
		if strings.TrimSpace(m.Name) == "" {
			m.Name = e.Name()
		}
		// 插件标识一律以目录名为准（清单 name 视为显示名，不参与加载/命名）。
		m.Name = e.Name()
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, warns
}

// ExternalPlugin 把外部进程插件包装为标准 Cordis 插件（MCPPlugin 适配器）：
// Apply 时拉起子进程并按 MCP 协议握手、tools/list 发现工具注册进插件树；
// 卸载时框架自动关闭连接并杀死进程。子进程崩溃只让工具调用报错，核心无恙。
func ExternalPlugin(pluginsDir string, m Manifest) cordis.Plugin {
	return cordis.Plugin{
		Name:   "ext-" + m.Name,
		Inject: m.Inject,
		Apply: func(ctx cordis.Context) error {
			base := filepath.Join(pluginsDir, m.Name)
			entry := strings.TrimSpace(m.Entry)
			if entry == "" {
				return fmt.Errorf("插件 %s 缺少 entry", m.Name)
			}
			// 含路径分隔符：视为插件目录内的文件，补全相对路径并确保可执行；
			// 裸命令名（如 python3）留给启动时按 PATH 解析（MCP 适配器内部处理）。
			if strings.ContainsAny(entry, `/\`) {
				if !filepath.IsAbs(entry) {
					entry = filepath.Join(base, entry)
				}
				st, err := os.Stat(entry)
				if err != nil || st.IsDir() {
					return fmt.Errorf("插件 %s entry 不存在: %s", m.Name, entry)
				}
				_ = os.Chmod(entry, st.Mode()|0o111)
			}

			mgr := agent.NewMCPManager()
			reg := agent.NewRegistry()
			hctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			server := agent.MCPServer{Name: m.Name, Command: entry, Args: m.Args, Cwd: base}
			if err := mgr.RegisterContext(hctx, reg, []agent.MCPServer{server}); err != nil {
				return fmt.Errorf("插件 %s 启动失败: %w", m.Name, err)
			}
			if err := ctx.Effect(func() (func(), error) {
				return mgr.Close, nil
			}); err != nil {
				mgr.Close()
				return err
			}
			// 清单声明的服务占位注册（其他插件可 Inject 依赖它）。
			for _, svc := range m.Provide {
				if err := ctx.Provide(svc, m); err != nil {
					mgr.Close()
					return err
				}
			}
			for _, name := range reg.Names() {
				t, ok := reg.Get(name)
				if !ok {
					continue
				}
				if err := ctx.RegisterTool(cordis.Tool{
					Name:        t.Name,
					Description: t.Description,
					Schema:      t.Schema,
					Execute:     t.Run,
				}); err != nil {
					mgr.Close()
					return err
				}
			}
			n := len(reg.Names())
			if n == 0 && len(m.Provide) == 0 {
				mgr.Close()
				return fmt.Errorf("插件 %s 未通过 tools/list 暴露任何工具", m.Name)
			}
			ctx.Logger().Printf("外部插件 %s v%s 已加载：%d 个工具", m.Name, m.Version, n)
			return nil
		},
	}
}

// RegisterExternalAll 发现并加载全部启用的外部插件，返回加载清单与告警。
// 单个插件失败只记录（崩溃隔离/坏插件不拖垮启动）。
func RegisterExternalAll(r *cordis.Runtime, pluginsDir string) ([]string, []string) {
	manifests, warns := Discover(pluginsDir)
	var loaded []string
	for _, m := range manifests {
		if !m.Enabled() {
			continue
		}
		if err := r.Load(ExternalPlugin(pluginsDir, m)); err != nil {
			warns = append(warns, err.Error())
			continue
		}
		loaded = append(loaded, m.Name)
	}
	return loaded, warns
}
