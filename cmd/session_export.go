// 单会话导出：把某个会话的对话记录以 Markdown 下载。
package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"licode/internal/session"
	"licode/internal/settings"
)

// sessionIDPattern 限定会话 ID 为 32 位十六进制（session.genID 的格式），
// 防止通过 ../ 进行路径穿越读取任意文件。
var sessionIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// handleSessionExport 导出一个会话为 Markdown 文件。
func handleSessionExport(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("session_id")
	if sid == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "缺少 session_id"})
		return
	}
	if !sessionIDPattern.MatchString(sid) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "session_id 非法"})
		return
	}
	path := filepath.Join(settings.SessionsDir(), sid+".json")
	if rel, err := filepath.Rel(settings.SessionsDir(), path); err != nil || strings.HasPrefix(rel, "..") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "session_id 非法"})
		return
	}
	s, err := session.LoadSessionFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.Title())
	for _, m := range s.Messages() {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", m.Role, m.Content)
		for _, tc := range m.ToolCalls {
			fmt.Fprintf(&b, "```\n%s(%s)\n```\n\n", tc.Function.Name, tc.Function.Arguments)
		}
	}
	body := b.String()
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.md", sid))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

// shellCandidates 常见 shell 绝对路径，尽量先绝对路径。
var shellCandidates = []string{
	"/bin/sh", "/bin/bash", "/usr/bin/bash", "/bin/zsh", "/usr/bin/zsh",
	"/bin/fish", "/usr/bin/fish", "/usr/bin/dash", "/bin/dash",
	"/usr/bin/ksh", "/bin/ksh", "/usr/bin/pwsh", "/usr/bin/powershell",
}

// shellNames 常见 shell 名，用于 PATH 与环境目录查找（Termux 等系统的
// shell 不在 /bin，必须按名称查 PATH）。
var shellNames = []string{"sh", "bash", "dash", "zsh", "fish", "ksh", "pwsh", "powershell"}

// handleShells 探测本机可用 shell：绝对路径 + PATH 扫描 + $SHELL + Termux $PREFIX/bin。
// 注意：不能用 exec.LookPath —— Android seccomp 会拦截 faccessat2，触发 SIGSYS
// 直接杀掉进程（Termux 上打开设置页即崩溃），因此这里只用 os.Stat 检查。
func handleShells(w http.ResponseWriter, r *http.Request) {
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		clean := filepath.Clean(p)
		if seen[clean] {
			return
		}
		if st, err := os.Stat(clean); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			seen[clean] = true
		}
	}
	for _, c := range shellCandidates {
		add(c)
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		for _, n := range shellNames {
			add(filepath.Join(dir, n))
		}
	}
	add(os.Getenv("SHELL"))
	if prefix := os.Getenv("PREFIX"); prefix != "" {
		for _, n := range shellNames {
			add(filepath.Join(prefix, "bin", n))
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	writeJSON(w, http.StatusOK, out)
}