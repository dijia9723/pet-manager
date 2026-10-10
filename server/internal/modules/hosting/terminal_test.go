package hosting

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"golang.org/x/crypto/ssh"
	"pet-manager/server/internal/model"
)

// testPTYSize 记录测试 SSH 服务端收到的 PTY 尺寸。
type testPTYSize struct {
	Cols, Rows uint32
}

// terminalSSHServer 启动一个可协商 PTY/Shell 的测试 SSH 服务端，
// 回传收到的尺寸序列与连接关闭信号；allowPTY 控制是否接受 PTY 请求。
func terminalSSHServer(t *testing.T, allowPTY bool) (model.ManagedServer, <-chan testPTYSize, <-chan struct{}) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	sizes := make(chan testPTYSize, 8)
	closed := make(chan struct{})
	var closedOnce sync.Once
	serveConn := func(conn net.Conn) {
		defer closedOnce.Do(func() { close(closed) })
		defer conn.Close()
		client, channels, requests, err := ssh.NewServerConn(conn, cfg)
		if err != nil {
			return
		}
		defer client.Close()
		go ssh.DiscardRequests(requests)
		for channel := range channels {
			if channel.ChannelType() != "session" {
				_ = channel.Reject(ssh.UnknownChannelType, "session required")
				continue
			}
			stream, requests, err := channel.Accept()
			if err != nil {
				return
			}
			go func() {
				defer stream.Close()
				for request := range requests {
					switch request.Type {
					case "pty-req":
						var pty struct {
							Term                      string
							Cols, Rows, Width, Height uint32
							Modes                     string
						}
						if ssh.Unmarshal(request.Payload, &pty) == nil {
							sizes <- testPTYSize{Cols: pty.Cols, Rows: pty.Rows}
							_ = request.Reply(allowPTY, nil)
						}
					case "window-change":
						var size struct{ Cols, Rows, Width, Height uint32 }
						if ssh.Unmarshal(request.Payload, &size) == nil {
							sizes <- testPTYSize{Cols: size.Cols, Rows: size.Rows}
						}
					case "shell":
						_ = request.Reply(true, nil)
						go func() {
							defer stream.Close()
							_, _ = stream.Write([]byte("\x1b[32mpm@test$ \x1b[0m"))
							buffer := make([]byte, 4096)
							for {
								n, err := stream.Read(buffer)
								if err != nil {
									return
								}
								if string(buffer[:n]) == "exit\r" {
									_, _ = stream.SendRequest("exit-status", false, ssh.Marshal(struct{ Code uint32 }{7}))
									return
								}
								_, _ = stream.Write(buffer[:n])
							}
						}()
					}
				}
			}()
		}
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveConn(conn)
		}
	}()
	address := listener.Addr().(*net.TCPAddr)
	return model.ManagedServer{Host: "127.0.0.1", Port: address.Port, Username: "admin", AuthType: "password", HostFingerprint: ssh.FingerprintSHA256(signer.PublicKey())}, sizes, closed
}

// terminalWSServer 以本模块的 serveTerminal 搭建返回 ws:// 地址的测试 WebSocket 服务端。
func terminalWSServer(t *testing.T, row model.ManagedServer) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{terminalProtocol}, OriginPatterns: []string{"http://localhost:5173"}})
		if err != nil {
			return
		}
		defer ws.CloseNow()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		serveTerminal(ctx, ws, row, "test-only-secret", 80, 24)
	}))
	t.Cleanup(server.Close)
	return strings.Replace(server.URL, "http:", "ws:", 1)
}

// readTerminalEvent 读到指定类型的控制事件后返回（忽略输出与其它事件）。
func readTerminalEvent(t *testing.T, ctx context.Context, ws *websocket.Conn, typ string) terminalEvent {
	t.Helper()
	for {
		kind, data, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if kind == websocket.MessageText {
			var event terminalEvent
			if err := json.Unmarshal(data, &event); err != nil {
				t.Fatal(err)
			}
			if event.Type == typ {
				return event
			}
		}
	}
}

// readTerminalOutput 持续读取二进制输出直到包含期望片段。
func readTerminalOutput(t *testing.T, ctx context.Context, ws *websocket.Conn, expected []byte) {
	t.Helper()
	var output []byte
	for !bytes.Contains(output, expected) {
		kind, data, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if kind == websocket.MessageBinary {
			output = append(output, data...)
		}
	}
}

// waitSSHClosed 等待测试 SSH 连接关闭，超时即失败（验证断连能释放 SSH）。
func waitSSHClosed(t *testing.T, ctx context.Context, closed <-chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	case <-ctx.Done():
		t.Fatal("browser disconnect did not release SSH connection")
	}
}

// TestTerminalStreamsInputResizeAndShellExit 覆盖终端端到端：初始 PTY、resize 转发、
// 文本/二进制输入回显、退出码与正常关闭并释放 SSH。
func TestTerminalStreamsInputResizeAndShellExit(t *testing.T) {
	row, sizes, closed := terminalSSHServer(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, terminalWSServer(t, row), &websocket.DialOptions{Subprotocols: []string{terminalProtocol}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	readTerminalEvent(t, ctx, ws, "ready")
	select {
	case size := <-sizes:
		if size.Cols != 80 || size.Rows != 24 {
			t.Fatal("incorrect initial PTY", size)
		}
	case <-ctx.Done():
		t.Fatal("no PTY request")
	}
	if err := wsjson.Write(ctx, ws, terminalMessage{Type: "resize", Cols: 132, Rows: 42}); err != nil {
		t.Fatal(err)
	}
	select {
	case size := <-sizes:
		if size.Cols != 132 || size.Rows != 42 {
			t.Fatal("incorrect resize", size)
		}
	case <-ctx.Done():
		t.Fatal("resize not forwarded")
	}
	for _, input := range []string{"cd /tmp\r", "echo 你好 😀\r", "\x03", "\x1b[A", "\t"} {
		if err := wsjson.Write(ctx, ws, terminalMessage{Type: "input", Data: input}); err != nil {
			t.Fatal(err)
		}
		readTerminalOutput(t, ctx, ws, []byte(input))
	}
	if err := ws.Write(ctx, websocket.MessageBinary, []byte{0x80, 0xff}); err != nil {
		t.Fatal(err)
	}
	readTerminalOutput(t, ctx, ws, []byte{0x80, 0xff})
	if err := wsjson.Write(ctx, ws, terminalMessage{Type: "input", Data: "exit\r"}); err != nil {
		t.Fatal(err)
	}
	if event := readTerminalEvent(t, ctx, ws, "exit"); event.ExitCode != 7 {
		t.Fatal("lost exit status", event)
	}
	if _, _, err := ws.Read(ctx); websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatal("unclean shell exit", err)
	}
	waitSSHClosed(t, ctx, closed)
}

// TestTerminalDisconnectAndInvalidInputReleaseSSH 断言浏览器断连或非法输入都会以策略违规
// 关闭并释放底层 SSH 连接。
func TestTerminalDisconnectAndInvalidInputReleaseSSH(t *testing.T) {
	for _, invalid := range []string{"", "{", `{"type":"resize","cols":99999,"rows":24}`, `{"type":"exec","data":"whoami"}`} {
		t.Run(invalid, func(t *testing.T) {
			row, _, closed := terminalSSHServer(t, true)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ws, _, err := websocket.Dial(ctx, terminalWSServer(t, row), &websocket.DialOptions{Subprotocols: []string{terminalProtocol}})
			if err != nil {
				t.Fatal(err)
			}
			defer ws.CloseNow()
			readTerminalEvent(t, ctx, ws, "ready")
			if invalid == "" {
				_ = ws.Close(websocket.StatusNormalClosure, "test disconnect")
			} else {
				if err := ws.Write(ctx, websocket.MessageText, []byte(invalid)); err != nil {
					t.Fatal(err)
				}
				for {
					_, _, err := ws.Read(ctx)
					if err != nil {
						if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
							t.Fatal("invalid input not rejected", err)
						}
						break
					}
				}
			}
			waitSSHClosed(t, ctx, closed)
		})
	}
}

// TestTerminalRejectsOriginAndReportsSSHFailures 覆盖来源校验（不可信 Origin 拒绝）
// 以及 PTY 被拒/指纹不符时向浏览器回报错误并释放连接。
func TestTerminalRejectsOriginAndReportsSSHFailures(t *testing.T) {
	t.Run("origin rejected before SSH", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ws, response, err := websocket.Dial(ctx, terminalWSServer(t, model.ManagedServer{}), &websocket.DialOptions{Subprotocols: []string{terminalProtocol}, HTTPHeader: http.Header{"Origin": {"https://untrusted.example"}}})
		if err == nil {
			ws.CloseNow()
			t.Fatal("accepted untrusted origin")
		}
		if response == nil || response.StatusCode != http.StatusForbidden {
			t.Fatal("origin not rejected with 403")
		}
	})
	for _, tc := range []struct {
		name     string
		allowPTY bool
		badPin   bool
		want     string
	}{
		{"PTY refused", false, false, "服务器拒绝分配 SSH 终端"},
		{"pin mismatch", true, true, "SSH 主机指纹不匹配"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, _, closed := terminalSSHServer(t, tc.allowPTY)
			if tc.badPin {
				row.HostFingerprint = "SHA256:" + base64.RawStdEncoding.EncodeToString(make([]byte, 32))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ws, _, err := websocket.Dial(ctx, terminalWSServer(t, row), &websocket.DialOptions{Subprotocols: []string{terminalProtocol}})
			if err != nil {
				t.Fatal(err)
			}
			defer ws.CloseNow()
			event := readTerminalEvent(t, ctx, ws, "error")
			if !strings.Contains(event.Message, tc.want) {
				t.Fatal("missing failure detail", event)
			}
			// Reading the close frame completes the failure close handshake.
			for {
				if _, _, err := ws.Read(ctx); err != nil {
					break
				}
			}
			waitSSHClosed(t, ctx, closed)
		})
	}
}
