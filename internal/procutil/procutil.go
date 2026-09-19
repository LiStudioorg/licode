// Package procutil 提供不依赖 exec.LookPath 的可执行文件解析。
//
// Android/Termux 的 seccomp 会拦截 faccessat2，而 exec.LookPath 内部会触发
// 该系统调用，导致 SIGSYS 直接杀进程；因此这里用 os.Stat 手动扫描 PATH。
package procutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveEntry 把命令解析为可执行文件的绝对路径：
//   - 含路径分隔符：原样返回（exec.Command 不会再走 LookPath）
//   - 裸命令名：按 PATH 顺序用 os.Stat 查找
func ResolveEntry(entry string) (string, error) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", fmt.Errorf("命令为空")
	}
	if strings.ContainsAny(entry, `/\`) {
		return entry, nil
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, entry)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("找不到可执行文件 %q（需在 PATH 中或使用绝对路径）", entry)
}
