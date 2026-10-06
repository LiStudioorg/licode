// Package backup 提供会话与配置的 zip 导出/导入，方便迁移与备份。
package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"licode/internal/settings"
)

// Export 把配置、会话、Skills、附加提示词打包成 zip，返回 zip 字节。
func Export() ([]byte, error) {
	base := settings.BaseDir()
	var files []string
	addFile := func(p string) {
		abs := filepath.Join(base, p)
		if _, err := os.Stat(abs); err == nil {
			files = append(files, p)
		}
	}
	// 配置、技能、附加提示词、会话
	addFile("config.json")
	addFile("system-prompt.md")
	walkDir(settings.SkillsDir(), &files)
	walkDir(settings.MDPromptDir(), &files)
	walkDir(settings.SessionsDir(), &files)

	// 写出 zip
	tmp, err := os.CreateTemp("", "licode-export-*.zip")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	w := zip.NewWriter(tmp)
	for _, p := range files {
		if err := addToZip(w, p); err != nil {
			w.Close()
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	tmp.Close()
	return os.ReadFile(tmp.Name())
}

func walkDir(dir string, files *[]string) {
	base := settings.BaseDir()
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}
		*files = append(*files, rel)
		return nil
	})
}

func addToZip(w *zip.Writer, rel string) error {
	abs := filepath.Join(settings.BaseDir(), rel)
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	f, err := w.Create(rel)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// allowedImportPath 限定导入只覆盖已知子目录，防止 zip 内夹带 session.key 等敏感文件。
func allowedImportPath(rel string) bool {
	if rel == "config.json" || rel == "system-prompt.md" {
		return true
	}
	// 附加提示词目录实为 md/（settings.MDPromptDir），旧值 md-prompt/ 与导出
	// 相对路径对不上，导致备份包里的 md/*.md 导入时被判为非法路径而断裂。
	for _, p := range []string{"skills/", "md/", "sessions/"} {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}

// Import 从 zip 字节恢复配置/会话/Skills。dest 为目标根目录（默认 ~/.licode）。
// ImportReport 汇总导入过程中的安全降级动作，供 UI 提示用户。
type ImportReport struct {
	// MCPServersDropped 是从导入的 config.json 中剥离的 MCP 服务器条目数。
	// MCP stdio 条目会在下次构建 Agent 时 spawn 任意命令：导入的备份包
	// 属于外部输入，静默落盘等于"导入即获得开机自启任意命令"的持久化，
	// 因此默认剥离；用户确认来源可信后可在设置里手动重新添加。
	MCPServersDropped int `json:"mcp_servers_dropped"`
}

func sanitizeConfigJSON(data []byte) ([]byte, ImportReport, error) {
	var rep ImportReport
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		// config.json 不合法会在重启后静默回退默认配置，导入必须 fail fast。
		return nil, rep, fmt.Errorf("config.json 不是合法 JSON: %w", err)
	}
	if servers, ok := m["mcp_servers"].([]any); ok && len(servers) > 0 {
		rep.MCPServersDropped = len(servers)
		delete(m, "mcp_servers")
		out, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return nil, rep, err
		}
		return out, rep, nil
	}
	return data, rep, nil
}

func Import(data []byte, dest string) (ImportReport, error) {
	var rep ImportReport
	if dest == "" {
		dest = settings.BaseDir()
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return rep, err
	}
	for _, f := range zr.File {
		clean := filepath.Clean(f.Name)
		if strings.Contains(clean, "..") || filepath.IsAbs(clean) {
			return rep, fmt.Errorf("非法路径 %q", f.Name)
		}
		// 兼容旧版备份包：附加提示词目录曾叫 md-prompt/，现统一为 md/。
		if rel := strings.TrimPrefix(clean, "md-prompt"+string(filepath.Separator)); rel != clean {
			clean = filepath.Join("md", rel)
		}
		if !allowedImportPath(clean) {
			return rep, fmt.Errorf("不允许导入的路径 %q", f.Name)
		}
		target := filepath.Join(dest, clean)
		rc, err := f.Open()
		if err != nil {
			return rep, err
		}
		content, rerr := io.ReadAll(rc)
		rc.Close()
		if rerr != nil {
			return rep, rerr
		}
		if clean == "config.json" {
			var serr error
			content, rep, serr = sanitizeConfigJSON(content)
			if serr != nil {
				return rep, serr
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return rep, err
		}
		// 配置文件含明文 API Key，落盘权限收紧为仅属主可读写。
		mode := os.FileMode(0o644)
		if clean == "config.json" {
			mode = 0o600
		}
		if err := os.WriteFile(target, content, mode); err != nil {
			return rep, err
		}
		if mode == 0o600 {
			// WriteFile 对已存在的文件不改权限，这里显式收紧。
			_ = os.Chmod(target, mode)
		}
	}
	return rep, nil
}
