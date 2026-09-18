// Package web embeds the Nuxt frontend served by cmd/serve.
// 前端产物全部打包进二进制，运行时不依赖外网/CDN。
package web

import (
	"crypto/x509"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// NuxtFS 嵌入的 Nuxt 静态前端产物（由 nuxtweb/dist 生成，见 scripts/sync-nuxt.sh）。
// cmd/serve.go 用它提供主页/登录页/设置页/工具页与 /_nuxt/ 资源。
// 注意：必须用 all: 前缀，否则 _nuxt/ 目录会被 embed 默认规则排除。
//
//go:embed all:nuxt
var nuxtFS embed.FS

// CACertPEM 嵌入的权威 CA 根证书束（https://curl.se/ca/cacert.pem，Mozilla CA 列表）。
// internal/ai 与 internal/agent 构造 HTTP 客户端时用它作为唯一信任来源，
// 不依赖系统证书池，保证在系统证书缺失/被篡改的环境下 TLS 校验行为一致。
//
//go:embed certs/cacert.pem
var CACertPEM []byte

// CACertPool 返回只含内置 cacert.pem 的证书池（进程内单例）。
// 第二个返回值为 false 表示 embed 内容损坏，调用方应回退系统证书池。
func CACertPool() (*x509.CertPool, bool) {
	caOnce.Do(func() {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(CACertPEM) {
			caErr = true
			return
		}
		caPool = pool
	})
	return caPool, !caErr
}

// userCADir 用户自签/私有 CA 证书目录（与内置 cacert.pem 分开存放）。
func userCADir() string {
	if v := os.Getenv("LICODE_HOME"); v != "" {
		return filepath.Join(v, "certs")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".licode", "certs")
	}
	return filepath.Join(".licode", "certs")
}

// MergedCACertPool 返回内置权威 CA + 用户自定义 CA（~/.licode/certs/*.pem|crt|cer）的合并池。
// 用户证书只是追加，不覆盖内置束；内置束损坏时返回 false（调用方回退系统池）。
// 带目录指纹缓存：内容未变化时复用上次构建的池，避免每次请求读盘。
func MergedCACertPool() (*x509.CertPool, bool) {
	base, ok := CACertPool()
	if !ok {
		return nil, false
	}
	dir := userCADir()
	fingerprint := dirFingerprint(dir)

	userCAMu.Lock()
	defer userCAMu.Unlock()
	if mergedCache != nil && fingerprint == mergedFp {
		return mergedCache, true
	}

	merged := x509.NewCertPool()
	merged.AppendCertsFromPEM(CACertPEM)
	// 追加用户证书（目录不存在则跳过，仅用内置束）。
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || (!strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, ".crt") && !strings.HasSuffix(name, ".cer")) {
				continue
			}
			if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
				merged.AppendCertsFromPEM(data)
			}
		}
	}
	_ = base // base 与 merged 内容一致；merged 为统一出口
	mergedCache = merged
	mergedFp = fingerprint
	return merged, true
}

// dirFingerprint 计算目录内证书文件的指纹（名字+mtime+大小拼接）。
func dirFingerprint(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "" // 目录不存在 → 稳定指纹
	}
	var sb strings.Builder
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || (!strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, ".crt") && !strings.HasSuffix(name, ".cer")) {
			continue
		}
		if info, err := e.Info(); err == nil {
			fmt.Fprintf(&sb, "%s|%d|%d;", name, info.ModTime().UnixNano(), info.Size())
		}
	}
	return sb.String()
}

var (
	caOnce sync.Once
	caPool *x509.CertPool
	caErr  bool

	userCAMu    sync.Mutex
	mergedCache *x509.CertPool
	mergedFp    string
)

// NuxtFS 返回挂载在 / 与 /_nuxt/ 下的 Nuxt 静态前端文件系统。
// 目录结构：index.html（主页）、login/、settings/、tools/、_nuxt/（JS/CSS）。
func NuxtFS() fs.FS {
	sub, err := fs.Sub(nuxtFS, "nuxt")
	if err != nil {
		panic(err)
	}
	return sub
}

// ReadNuxt 读取 Nuxt 静态前端中的一个文件（相对路径，如 "login/index.html"）。
func ReadNuxt(name string) ([]byte, error) {
	return fs.ReadFile(nuxtFS, "nuxt/"+name)
}
