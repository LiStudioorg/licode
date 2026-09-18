package cmd

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// userCADir 与 web.MergedCACertPool 保持同一目录约定。
func userCADir() string {
	if v := os.Getenv("LICODE_HOME"); v != "" {
		return filepath.Join(v, "certs")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".licode", "certs")
	}
	return filepath.Join(".licode", "certs")
}

// 用户自定义 CA 管理：~/.licode/certs/ 目录下的 .pem/.crt/.cer 文件。
// 与内置 cacert.pem 分开；上传后即时生效（TLS 池按目录指纹缓存，文件变化自动重建）。

// handleCACerts GET /api/ca → 列出用户 CA 文件及其证书摘要。
func handleCACerts(w http.ResponseWriter, r *http.Request) {
	dir := userCADir()
	type caInfo struct {
		Name      string   `json:"name"`
		Size      int64    `json:"size"`
		ModTime   string   `json:"mod_time"`
		Subjects  []string `json:"subjects,omitempty"`
		ValidCert bool     `json:"valid"`
	}
	var out []caInfo
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !isCertFile(name) {
				continue
			}
			info := caInfo{Name: name, ValidCert: false}
			if fi, err := e.Info(); err == nil {
				info.Size = fi.Size()
				info.ModTime = fi.ModTime().Format(time.RFC3339)
			}
			if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
				info.Subjects = pemSubjects(data)
				info.ValidCert = len(info.Subjects) > 0
			}
			out = append(out, info)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"dir": dir, "certs": out})
}

// handleCAUpload POST /api/ca/upload multipart 字段 file → 保存到 ~/.licode/certs/。
func handleCAUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(4 << 20); err != nil { // 4MB 上限足够 CA 文件
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "表单解析失败"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "缺少 file 字段"})
		return
	}
	defer file.Close()

	name := filepath.Base(header.Filename)
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if name == "" || name == "." || name == ".." {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "文件名非法"})
		return
	}
	if !isCertFile(name) {
		name += ".pem"
	}

	// io.ReadAll 而非单次 Read：multipart 文件的单次 Read 可能只返回部分内容。
	content, err := io.ReadAll(io.LimitReader(file, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "读取上传内容失败"})
		return
	}
	if len(pemSubjects(content)) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "不是有效的 PEM 证书文件"})
		return
	}

	dir := userCADir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "创建目录失败"})
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "写入失败"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "subjects": pemSubjects(content)})
}

// handleCADelete POST /api/ca/delete {name} → 删除指定用户 CA 文件。
func handleCADelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "参数错误"})
		return
	}
	name := filepath.Base(req.Name) // 防路径穿越
	if !isCertFile(name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "仅支持 pem/crt/cer"})
		return
	}
	p := filepath.Join(userCADir(), name)
	if _, err := os.Stat(p); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "文件不存在"})
		return
	}
	if err := os.Remove(p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "删除失败"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func isCertFile(name string) bool {
	return strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".crt") || strings.HasSuffix(name, ".cer")
}

// pemSubjects 提取 PEM 中所有证书的 Subject CN（用于前端展示与有效性判断）。
func pemSubjects(pemData []byte) []string {
	var out []string
	rest := pemData
	for {
		block, rest2 := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = rest2
		if block.Type != "CERTIFICATE" {
			continue
		}
		if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
			sub := cert.Subject.String()
			if cert.Subject.CommonName != "" {
				sub = cert.Subject.CommonName
			}
			out = append(out, fmt.Sprintf("%s（至 %s）", sub, cert.NotAfter.Format("2006-01-02")))
		}
	}
	return out
}
