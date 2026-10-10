# terminal.go

平台 GET /servers/:id/terminal 在现有 JWT、会话状态与 platform.server.exec 权限门禁后升级 WebSocket。协商 pet-manager-terminal.v1，凭证 bearer.<access_token> 子协议仅用于鉴权，不回显；来源检查使用同源或 CORS 配置中的允许来源。

通过已存储、加密的凭证与严格主机指纹校验建立 SSH 会话，分配 xterm-256color PTY，启动持续 Shell。浏览器 JSON input/resize 分别转发 stdin 与 WindowChange；二进制输入用于原始终端字符，SSH stdout/stderr 以二进制流输出，保留 ANSI 和跨帧 UTF-8。ready/error/exit 控制事件使用 JSON，退出包含 SSH 退出码。

每帧最多 128 KiB，单次输入最多 32 KiB，行列范围为 1–200 / 2–500；输入队列有界，输出同步写入并设置 15 秒上限。握手/PTY/Shell 初始化有 30 秒上限，交互会话最长 2 小时，25 秒心跳检测掉线。浏览器关闭或连接中断取消上下文并关闭远端会话；重新连接建立新 Shell。终端输入与凭证不写入日志。
