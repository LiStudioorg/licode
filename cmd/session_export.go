// 单会话导出：把某个会话的对话记录以 Markdown 下载。
package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"licode/internal/session"
	"licode/internal/settings"
)

// handleSessionExport 导出一个会话为 Markdown 文件。
func handleSessionExport(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("session_id")
	if sid == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "缺少 session_id"})
		return
	}
	s, err := session.LoadSessionFile(filepath.Join(settings.SessionsDir(), sid+".json"))
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

// shellCandidates 常见 shell 路径，尽量先绝对路径。
var shellCandidates = []string{
	"/bin/sh", "/bin/bash", "/usr/bin/bash", "/bin/zsh", "/usr/bin/zsh",
	"/bin/fish", "/usr/bin/fish", "/usr/bin/dash", "/bin/dash",
	"/usr/bin/ksh", "/bin/ksh", "/usr/bin/pwsh", "/usr/bin/powershell",
}

// handleShells 探测本机可用 shell（去重，分别对绝对路径与 PATH 查一次）。
func handleShells(w http.ResponseWriter, r *http.Request) {
	seen := map[string]bool{}
	check := func(c string) {
		if seen[c] {
			return
		}
		if st, err := os.Stat(c); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			seen[c] = true
			return
		}
		if p, err := exec.LookPath(c); err == nil && !seen[p] {
			seen[p] = true
		}
	}
	for _, c := range shellCandidates {
		check(c)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	writeJSON(w, http.StatusOK, out)
}