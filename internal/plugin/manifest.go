// Package plugin 实现进程插件系统：
// 插件是独立进程（任意语言），通过 stdio JSON-RPC 与宿主通信，
// 可贡献工具、斜杠命令、提示词、设置界面与只读面板。
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// APIVersion 是当前宿主支持的插件 API 版本。
const APIVersion = 1

// IDPattern 限制插件 ID 格式（用于目录名与工具前缀）。
var IDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,40}$`)

// Permission 声明插件需要的权限（进程插件无法被宿主强制沙箱，仅作启用确认）。
type Permission struct {
	FS    []string `json:"fs,omitempty"`    // 需要访问的路径/目录
	Net   []string `json:"net,omitempty"`   // 需要访问的域名
	Shell bool     `json:"shell,omitempty"` // 需要执行其他命令
	Env   bool     `json:"env,omitempty"`   // 需要继承宿主环境变量
}

// Empty 报告权限声明是否为空（为空时仅需基本确认）。
func (p Permission) Empty() bool {
	return len(p.FS) == 0 && len(p.Net) == 0 && !p.Shell && !p.Env
}

// Summary 返回权限的中文摘要（前端展示/确认用）。
func (p Permission) Summary() []string {
	var out []string
	if len(p.FS) > 0 {
		out = append(out, "文件访问: "+strings.Join(p.FS, "、"))
	}
	if len(p.Net) > 0 {
		out = append(out, "网络访问: "+strings.Join(p.Net, "、"))
	}
	if p.Shell {
		out = append(out, "执行系统命令")
	}
	if p.Env {
		out = append(out, "读取环境变量（可能包含密钥）")
	}
	return out
}

// ToolDef 是插件贡献的工具定义。
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

// CommandDef 是插件贡献的斜杠命令。
type CommandDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PanelDef 是插件贡献的只读面板（声明式 UI）。
type PanelDef struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// Manifest 是插件清单（plugins/<id>/plugin.json）。
type Manifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	APIVersion  int               `json:"apiVersion"`
	Description string            `json:"description,omitempty"`
	Author      string            `json:"author,omitempty"`
	Entry       string            `json:"entry"` // 可执行命令（在插件目录下运行）
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Capabilities []string         `json:"capabilities,omitempty"`
	Permissions Permission        `json:"permissions"`
	Prompt      string            `json:"prompt,omitempty"`
	Contributes struct {
		Tools    []ToolDef       `json:"tools,omitempty"`
		Commands []CommandDef    `json:"commands,omitempty"`
		Panels   []PanelDef      `json:"panels,omitempty"`
		Settings json.RawMessage `json:"settings,omitempty"` // JSON Schema
	} `json:"contributes"`
	// Dir 是插件所在目录（加载时填充，不写入清单）。
	Dir string `json:"-"`
}

// LoadManifest 读取并校验插件清单。
func LoadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("plugin.json 解析失败: %w", err)
	}
	m.Dir = dir
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) validate() error {
	if !IDPattern.MatchString(m.ID) {
		return fmt.Errorf("插件 id %q 非法（小写字母/数字/_-，2-41 字符）", m.ID)
	}
	if strings.TrimSpace(m.Entry) == "" {
		return fmt.Errorf("缺少 entry（可执行命令）")
	}
	if m.APIVersion != APIVersion {
		return fmt.Errorf("apiVersion=%d 与宿主 %d 不兼容", m.APIVersion, APIVersion)
	}
	seen := map[string]bool{}
	for i, t := range m.Contributes.Tools {
		if strings.TrimSpace(t.Name) == "" {
			return fmt.Errorf("contributes.tools[%d] 缺少 name", i)
		}
		if seen[t.Name] {
			return fmt.Errorf("工具名重复: %s", t.Name)
		}
		seen[t.Name] = true
		if t.Schema == nil {
			m.Contributes.Tools[i].Schema = map[string]any{"type": "object"}
		}
	}
	for i, c := range m.Contributes.Commands {
		if strings.TrimSpace(c.Name) == "" {
			return fmt.Errorf("contributes.commands[%d] 缺少 name", i)
		}
	}
	return nil
}

// ToolName 返回插件工具在宿主中的全名（plugin__<id>__<tool>）。
func (m *Manifest) ToolName(tool string) string {
	return "plugin__" + m.ID + "__" + tool
}

// HasCapability 报告是否声明了某能力。
func (m *Manifest) HasCapability(cap string) bool {
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}
