# 进程控制（Windows）

> [开发者入口](../../../../README.md) · [执行器](executor.md) · [进程控制（类 Unix）](process_unix.md)

## 职责

提供 `configureProcess(cmd *exec.Cmd)` 的 **Windows** 实现，使脚本任务在超时/取消时能终止子进程树。

## 构建约束

由文件名 `_windows` 后缀隐含的构建约束生效（对应 `process_unix.go` 的 `!windows`）。

## 实现要点

- Windows 没有进程组与 `SIGKILL` 语义，改用 `taskkill /F /T /PID <pid>` 连同**子进程树**一并终止。
- `taskkill` 不可用（如未在 PATH 中）时，回退为仅杀主进程 `cmd.Process.Kill()`。
