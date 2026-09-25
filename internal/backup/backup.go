// Package backup 提供会话与配置的 zip 导出/导入，方便迁移与备份。
package backup

import (
	"archive/zip"
	"bytes"
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
func Import(data []byte, dest string) error {
	if dest == "" {
		dest = settings.BaseDir()
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		clean := filepath.Clean(f.Name)
		if strings.Contains(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("非法路径 %q", f.Name)
		}
		// 兼容旧版备份包：附加提示词目录曾叫 md-prompt/，现统一为 md/。
		if rel := strings.TrimPrefix(clean, "md-prompt"+string(filepath.Separator)); rel != clean {
			clean = filepath.Join("md", rel)
		}
		if !allowedImportPath(clean) {
			return fmt.Errorf("不允许导入的路径 %q", f.Name)
		}
		target := filepath.Join(dest, clean)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			rc.Close()
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, werr := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if werr != nil {
			return werr
		}
	}
	return nil
}
