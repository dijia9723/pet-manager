//go:build !windows

package job

import (
	"os/exec"
	"syscall"
)

// 适用平台：!windows（Linux/macOS 等类 Unix）。
// 任务超时终止整个进程组，脚本派生的子进程也随任务结束。
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
