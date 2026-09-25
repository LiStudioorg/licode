//go:build !unix && !linux

package procutil

import "os/exec"

// SetupProcessGroup 非 Unix 平台回退：仅杀直接子进程（Go 默认行为）。
func SetupProcessGroup(cmd *exec.Cmd) func() {
	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}
