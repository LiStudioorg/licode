//go:build unix || linux

package procutil

import (
	"os/exec"
	"syscall"
)

// SetupProcessGroup 让子进程运行在独立进程组，并接管取消时的清理逻辑：
// 只杀 shell 本身会留下它派生的孙进程（构建、测试守护进程等）成为孤儿，
// 长期积累耗尽 CPU/内存/文件描述符。取消时对整个进程组发 SIGKILL。
// 返回的 cleanup 可安全多次调用。
func SetupProcessGroup(cmd *exec.Cmd) func() {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return func() {
		if cmd.Process == nil {
			return
		}
		// 负 PID = 进程组；EPERM/ESRCH（已退出）忽略。
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
