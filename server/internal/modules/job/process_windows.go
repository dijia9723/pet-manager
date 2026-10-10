package job

import (
	"os/exec"
	"strconv"
)

// 适用平台：Windows（由文件名 _windows 后缀隐含的构建约束）。
// Windows 无进程组与 SIGKILL，改用 taskkill /F /T 连同子进程树一并终止；
// taskkill 不可用时回退为仅杀主进程。
func configureProcess(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		err := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		if err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
