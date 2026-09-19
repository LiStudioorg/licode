package plugin

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxInstallSize  = 50 << 20 // 解压总量上限
	maxInstallFiles = 500
)

// Install 从 zip 字节安装/升级插件到第一个插件目录，返回插件 ID（不自动启用）。
func (m *Manager) Install(data []byte) (string, error) {
	if len(m.dirs) == 0 {
		return "", fmt.Errorf("未配置插件目录")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("不是有效的 zip: %w", err)
	}
	if len(zr.File) > maxInstallFiles {
		return "", fmt.Errorf("文件数量超过上限（%d）", maxInstallFiles)
	}

	// 找到 plugin.json 所在的顶层前缀（支持 zip 根目录或单层子目录）
	prefix := ""
	found := false
	for _, f := range zr.File {
		clean := path.Clean(f.Name)
		if clean == "plugin.json" {
			found = true
			break
		}
		if strings.HasSuffix(clean, "/plugin.json") && strings.Count(clean, "/") == 1 {
			prefix = strings.TrimSuffix(clean, "plugin.json")
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("zip 中未找到 plugin.json（需在根目录或单层子目录）")
	}

	tmp, err := os.MkdirTemp("", "licode-plugin-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	var total int64
	for _, f := range zr.File {
		name := path.Clean(f.Name)
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(name, prefix)
		if rel == "" || strings.Contains(rel, "..") || path.IsAbs(rel) {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > maxInstallSize {
			return "", fmt.Errorf("解压总量超过上限")
		}
		dst := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return "", err
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out, err := os.Create(dst)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, werr := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
		out.Close()
		rc.Close()
		if werr != nil {
			return "", werr
		}
	}

	man, err := LoadManifest(tmp)
	if err != nil {
		return "", fmt.Errorf("插件清单无效: %w", err)
	}
	target := filepath.Join(m.dirs[0], man.ID)
	if err := os.MkdirAll(m.dirs[0], 0o700); err != nil {
		return "", err
	}

	m.mu.Lock()
	if _, exists := m.plugins[man.ID]; exists {
		delete(m.plugins, man.ID)
	}
	m.mu.Unlock()

	// 已存在则先移入 .trash（视为升级）
	if _, err := os.Stat(target); err == nil {
		trash := filepath.Join(m.dirs[0], ".trash")
		_ = os.MkdirAll(trash, 0o700)
		_ = os.Rename(target, filepath.Join(trash, man.ID+"-"+time.Now().Format("20060102150405")))
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", fmt.Errorf("安装失败: %w", err)
	}

	// 注册到内存（已启用过则按已启用处理并启动）
	m.mu.Lock()
	m.plugins[man.ID] = &Plugin{Manifest: man}
	enabled := m.state.Enabled[man.ID]
	m.mu.Unlock()
	if enabled {
		if err := m.startPlugin(man.ID); err != nil {
			return man.ID, fmt.Errorf("已安装但启动失败: %w", err)
		}
	}
	return man.ID, nil
}
