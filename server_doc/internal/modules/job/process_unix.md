# 进程控制（类 Unix）

> [开发者入口](../../../../README.md) · [执行器](executor.md) · [进程控制（Windows）](process_windows.md)

## 职责

提供 `configureProcess(cmd *exec.Cmd)` 的 **类 Unix** 实现，使脚本任务在超时/取消时能终止整个进程组。

## 构建约束

文件带 `//go:build !windows`，即 Linux/macOS 等类 Unix 平台生效。

## 实现要点

- `SysProcAttr{Setpgid: true}`：让子进程自成一个进程组。
- `cmd.Cancel` 用 `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)` 向**整个进程组**发送信号，脚本派生的子进程也随任务一起结束，避免孤儿进程。
