package hosting

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"

	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

// terminalProtocol 是终端 WebSocket 必须协商的子协议名。
const terminalProtocol = "pet-manager-terminal.v1"

// terminalMessage 是浏览器发往服务端的控制消息：输入或窗口 resize。
type terminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// terminalEvent 是服务端发往浏览器的控制事件：ready/error/exit。
type terminalEvent struct {
	Type     string `json:"type"`
	Message  string `json:"message,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// validTerminalSize 校验终端尺寸边界，避免异常行列数。
func validTerminalSize(cols, rows int) bool {
	return cols >= 2 && cols <= 500 && rows >= 1 && rows <= 200
}

// terminalSize 从查询参数解析终端行列数，缺省 80x24。
func terminalSize(c *gin.Context) (int, int, error) {
	cols, errCols := strconv.Atoi(c.DefaultQuery("cols", "80"))
	rows, errRows := strconv.Atoi(c.DefaultQuery("rows", "24"))
	if errCols != nil || errRows != nil || !validTerminalSize(cols, rows) {
		return 0, 0, errcode.Newf(errcode.ErrInvalidParam, "终端尺寸不合法")
	}
	return cols, rows, nil
}

// handleTerminal 建立终端 WebSocket：需协商固定子协议、校验来源与终端尺寸，
// 升级后交由 serveTerminal 完成 SSH PTY 会话。
// @Summary 打开平台 SSH 交互终端（WebSocket / SSH PTY）
// @Description 使用 pet-manager-terminal.v1 子协议；浏览器通过 bearer.<access_token> 子协议携带登录凭证。输入和 resize 为 JSON 文本消息，输出为二进制 UTF-8 流，控制事件为 JSON。
// @Tags 平台-服务器管理
// @Security BearerAuth
// @Param id path int true "服务器 ID"
// @Param cols query int false "终端列数，默认 80"
// @Param rows query int false "终端行数，默认 24"
// @Success 101 {string} string "WebSocket 升级"
// @Router /platform/servers/{id}/terminal [get]
func (s *Service) handleTerminal(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	cols, rows, err := terminalSize(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	server, err := s.server(c.Request.Context(), id)
	if err != nil {
		respond(c, nil, err)
		return
	}
	ws, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		Subprotocols: []string{terminalProtocol}, OriginPatterns: s.terminalOrigins,
	})
	if err != nil {
		return
	}
	defer ws.CloseNow()
	if ws.Subprotocol() != terminalProtocol {
		_ = ws.Close(websocket.StatusPolicyViolation, "terminal protocol required")
		return
	}
	// Hijacked requests have a different lifecycle; the reader cancels this
	// context as soon as the browser disconnects, including during SSH setup.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 2*time.Hour)
	defer cancel()
	if server.Status != "enabled" {
		terminalFailure(ctx, ws, "服务器已停用")
		return
	}
	secret, err := s.decrypt(server.Credential)
	if err != nil {
		terminalFailure(ctx, ws, err.Error())
		return
	}
	serveTerminal(ctx, ws, *server, secret, cols, rows)
}

// writeTerminalEvent 以 15s 超时向浏览器发送一个 JSON 控制事件。
func writeTerminalEvent(ctx context.Context, ws *websocket.Conn, event terminalEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return ws.Write(writeCtx, websocket.MessageText, data)
}

// terminalFailure 向浏览器发送 error 事件并以内部错误码关闭连接。
func terminalFailure(ctx context.Context, ws *websocket.Conn, message string) {
	_ = writeTerminalEvent(ctx, ws, terminalEvent{Type: "error", Message: message})
	_ = ws.Close(websocket.StatusInternalError, "terminal failed")
}

// terminalOutput 把 SSH 会话输出转发为 WebSocket 二进制消息，逐条带写超时。
type terminalOutput struct {
	ctx context.Context
	ws  *websocket.Conn
}

// Write 以 15s 超时把 SSH 输出写入 WebSocket。
func (w terminalOutput) Write(data []byte) (int, error) {
	ctx, cancel := context.WithTimeout(w.ctx, 15*time.Second)
	defer cancel()
	if err := w.ws.Write(ctx, websocket.MessageBinary, data); err != nil {
		return 0, err
	}
	return len(data), nil
}

// readTerminal 读取浏览器消息并送入 channel：二进制视为原始输入，文本按 JSON 解析；
// 非法的输入类型/尺寸或 JSON 以策略违规关闭连接，读失败即 cancel 释放整个会话。
func readTerminal(ctx context.Context, ws *websocket.Conn, messages chan<- terminalMessage, cancel context.CancelFunc) {
	defer cancel()
	ws.SetReadLimit(128 << 10)
	for {
		typ, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var message terminalMessage
		if typ == websocket.MessageBinary {
			message = terminalMessage{Type: "input", Data: string(data)}
		} else if err := json.Unmarshal(data, &message); err != nil {
			_ = ws.Close(websocket.StatusPolicyViolation, "invalid terminal message")
			return
		}
		if (message.Type == "input" && len(message.Data) <= 32<<10) || (message.Type == "resize" && validTerminalSize(message.Cols, message.Rows)) {
			select {
			case messages <- message:
			case <-ctx.Done():
				return
			}
		} else {
			_ = ws.Close(websocket.StatusPolicyViolation, "invalid terminal input or size")
			return
		}
	}
}

// terminalHeartbeat 每 25s 发一次 WebSocket ping 探活，失败即取消会话，
// 用于及时发现浏览器断连并回收 SSH 连接。
func terminalHeartbeat(ctx context.Context, ws *websocket.Conn, cancel context.CancelFunc) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, stop := context.WithTimeout(ctx, 10*time.Second)
			err := ws.Ping(pingCtx)
			stop()
			if err != nil {
				cancel()
				return
			}
		}
	}
}

// serveTerminal 编排一次终端会话的生命周期：并发起读消息与心跳 goroutine，
// 建立 SSH 连接与 PTY（PTY/Shell 建立阶段另有 30s 超时），随后在输入/resize/退出/断连
// 之间转发，任一环节结束都会取消上下文并关闭连接。
func serveTerminal(parent context.Context, ws *websocket.Conn, server model.ManagedServer, secret string, cols, rows int) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	messages := make(chan terminalMessage, 64)
	go readTerminal(ctx, ws, messages, cancel)
	go terminalHeartbeat(ctx, ws, cancel)
	client, closeConn, err := dial(ctx, server, server.Username, server.AuthType, secret)
	if err != nil {
		terminalFailure(ctx, ws, err.Error())
		return
	}
	defer closeConn()
	// Bound PTY/shell setup too; interactive I/O keeps the longer session deadline.
	setupTimer := time.AfterFunc(30*time.Second, closeConn)
	defer setupTimer.Stop()
	session, err := client.NewSession()
	if err != nil {
		terminalFailure(ctx, ws, "无法创建 SSH 终端会话")
		return
	}
	defer session.Close()
	stdin, err := session.StdinPipe()
	if err != nil {
		terminalFailure(ctx, ws, "无法打开 SSH 终端输入")
		return
	}
	output := terminalOutput{ctx: ctx, ws: ws}
	session.Stdout, session.Stderr = output, output
	if err := session.RequestPty("xterm-256color", rows, cols, ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 38400, ssh.TTY_OP_OSPEED: 38400}); err != nil {
		terminalFailure(ctx, ws, "服务器拒绝分配 SSH 终端，请检查账号的 PTY 权限")
		return
	}
	if err := session.Shell(); err != nil {
		terminalFailure(ctx, ws, "服务器拒绝启动交互 Shell，请检查账号是否允许登录 Shell")
		return
	}
	setupTimer.Stop()
	if err := writeTerminalEvent(ctx, ws, terminalEvent{Type: "ready"}); err != nil {
		return
	}
	done := make(chan error, 1)
	go func() { done <- session.Wait() }()
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-done:
			exitCode := 0
			var exit *ssh.ExitError
			if errors.As(err, &exit) {
				exitCode = exit.ExitStatus()
			} else if err != nil {
				terminalFailure(ctx, ws, "SSH 终端连接已中断")
				return
			}
			_ = writeTerminalEvent(ctx, ws, terminalEvent{Type: "exit", ExitCode: exitCode})
			_ = ws.Close(websocket.StatusNormalClosure, "shell exited")
			return
		case message := <-messages:
			if message.Type == "resize" {
				err = session.WindowChange(message.Rows, message.Cols)
			} else {
				_, err = io.WriteString(stdin, message.Data)
			}
			if err != nil {
				terminalFailure(ctx, ws, "SSH 终端连接已中断")
				return
			}
		}
	}
}
