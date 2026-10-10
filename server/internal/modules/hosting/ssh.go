package hosting

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
)

// newCipher 从 Base64 编码的 32 字节密钥构造 AES-GCM 加解密器。
func newCipher(key string) (cipher.AEAD, error) {
	b, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(b) != 32 {
		return nil, errors.New("hosting.encryption_key 必须为 32 字节密钥的 Base64 编码")
	}
	c, err := aes.NewCipher(b)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(c)
}

// encrypt 用随机 nonce 加密凭证/密钥，附加固定的用途标签作为 AAD 防止跨用途重放；
// 未配置加密密钥时返回不可用错误。
func (s *Service) encrypt(value string) (string, error) {
	if s.cipher == nil {
		return "", errcode.Newf(errcode.ErrServiceUnavailable, "请配置 PM_HOSTING_ENCRYPTION_KEY 后使用服务器管理")
	}
	n := make([]byte, s.cipher.NonceSize())
	if _, err := rand.Read(n); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(s.cipher.Seal(n, n, []byte(value), []byte("pet-manager-hosting-v1"))), nil
}

// decrypt 解密 encrypt 产生的密文，校验失败（被篡改或密钥不符）时返回不可用错误。
func (s *Service) decrypt(value string) (string, error) {
	if s.cipher == nil {
		return "", errcode.Newf(errcode.ErrServiceUnavailable, "服务器凭证加密密钥未配置")
	}
	b, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(b) < s.cipher.NonceSize() {
		return "", errcode.New(errcode.ErrInternal)
	}
	n := s.cipher.NonceSize()
	plain, err := s.cipher.Open(nil, b[:n], b[n:], []byte("pet-manager-hosting-v1"))
	if err != nil {
		return "", errcode.Newf(errcode.ErrServiceUnavailable, "无法解密服务器凭证，请检查加密密钥")
	}
	return string(plain), nil
}

// hostKeyMismatchError 表示主机公钥指纹与配置的 pin 不一致，携带实际/期望值用于诊断。
type hostKeyMismatchError struct {
	keyType  string
	expected string
	actual   string
}

func (e *hostKeyMismatchError) Error() string {
	return fmt.Sprintf("SSH 主机指纹不匹配（%s）：配置 %s，服务器 %s", e.keyType, e.expected, e.actual)
}

// fingerprintCallback 返回严格的 HostKeyCallback：仅接受与 pin 完全一致的 SHA256 指纹。
func fingerprintCallback(expected string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if actual := ssh.FingerprintSHA256(key); actual != expected {
			return &hostKeyMismatchError{keyType: key.Type(), expected: expected, actual: actual}
		}
		return nil
	}
}

// algorithmsForHostKey 返回同一主机公钥对应的全部签名算法，
// 用于指纹匹配后据此收敛候选算法（RSA 的多种签名算法共享同一公钥与指纹）。
func algorithmsForHostKey(keyType string) []string {
	switch keyType {
	case ssh.KeyAlgoRSA:
		return []string{ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512}
	case ssh.CertAlgoRSAv01:
		return []string{ssh.CertAlgoRSASHA256v01, ssh.CertAlgoRSASHA512v01}
	default:
		return []string{keyType}
	}
}

// sshConnectionError 把底层 SSH/网络错误翻译为面向用户的可诊断提示，
// 覆盖取消、超时、指纹不符、算法协商失败、认证失败与连接被拒等情形。
func sshConnectionError(ctx context.Context, err error, authType string) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return errcode.Newf(errcode.ErrServiceUnavailable, "SSH 连接已取消")
	}
	var timeout net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
		return errcode.Newf(errcode.ErrServiceUnavailable, "SSH 连接或握手超时，请检查网络及服务器 SSH 服务")
	}
	var mismatch *hostKeyMismatchError
	if errors.As(err, &mismatch) {
		return errcode.Newf(errcode.ErrServiceUnavailable, "%s", mismatch.Error())
	}
	var negotiation *ssh.AlgorithmNegotiationError
	if errors.As(err, &negotiation) {
		return errcode.Newf(errcode.ErrServiceUnavailable, "SSH 算法协商失败（%s），请检查服务器支持的安全算法", negotiation.What)
	}
	// x/crypto/ssh has no exported client authentication error type.
	if strings.Contains(err.Error(), "ssh: unable to authenticate") {
		if authType == "key" {
			return errcode.Newf(errcode.ErrServiceUnavailable, "SSH 私钥认证失败（主机指纹已验证），请检查账号及 authorized_keys")
		}
		return errcode.Newf(errcode.ErrServiceUnavailable, "SSH 密码认证失败（主机指纹已验证），请检查账号、密码及服务器是否允许密码登录")
	}
	var network *net.OpError
	if errors.As(err, &network) && network.Op == "dial" {
		return errcode.Newf(errcode.ErrServiceUnavailable, "无法连接 SSH 服务器，请检查地址、端口、防火墙及 SSH 服务")
	}
	return errcode.Newf(errcode.ErrServiceUnavailable, "SSH 握手失败或连接被服务器关闭，请检查 SSH 服务日志")
}

// dial bounds both handshake and I/O by the request deadline and closes on cancellation.
func dial(ctx context.Context, server model.ManagedServer, username, authType, secret string) (*ssh.Client, func(), error) {
	expected, err := normalizeFingerprint(server.HostFingerprint)
	if err != nil {
		return nil, nil, errcode.Newf(errcode.ErrInvalidParam, "已保存的 SSH 主机指纹无效，请编辑服务器后重新保存")
	}
	var auth ssh.AuthMethod
	if authType == "key" {
		key, err := ssh.ParsePrivateKey([]byte(secret))
		if err != nil {
			return nil, nil, errcode.Newf(errcode.ErrInvalidParam, "SSH 私钥格式不合法（请使用无口令私钥）")
		}
		auth = ssh.PublicKeys(key)
	} else {
		auth = ssh.Password(secret)
	}
	addr := net.JoinHostPort(server.Host, strconv.Itoa(server.Port))
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}
	handshakeDeadline := minTime(deadline, time.Now().Add(30*time.Second))
	// A server may expose several different host keys. Try another key type only
	// after a pin mismatch; credentials are sent only once the exact pin matches.
	algorithms := ssh.SupportedAlgorithms().HostKeys
	algorithms = append([]string{ssh.KeyAlgoED25519}, slices.DeleteFunc(algorithms, func(a string) bool { return a == ssh.KeyAlgoED25519 })...)
	var lastMismatch *hostKeyMismatchError
	for len(algorithms) > 0 {
		conn, err := (&net.Dialer{Timeout: 10 * time.Second, Deadline: handshakeDeadline}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, nil, sshConnectionError(ctx, err, authType)
		}
		_ = conn.SetDeadline(handshakeDeadline)
		stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
		closeConn := func() { stop(); _ = conn.Close() }
		c, channels, requests, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
			User: username, Auth: []ssh.AuthMethod{auth},
			HostKeyCallback: fingerprintCallback(expected), HostKeyAlgorithms: algorithms,
		})
		if err == nil {
			_ = conn.SetDeadline(deadline)
			return ssh.NewClient(c, channels, requests), closeConn, nil
		}
		closeConn()
		var mismatch *hostKeyMismatchError
		if errors.As(err, &mismatch) {
			lastMismatch = mismatch
			checked := algorithmsForHostKey(mismatch.keyType)
			remaining := slices.DeleteFunc(algorithms, func(a string) bool { return slices.Contains(checked, a) })
			if len(remaining) == len(algorithms) {
				return nil, nil, sshConnectionError(ctx, mismatch, authType)
			}
			algorithms = remaining
			continue
		}
		var negotiation *ssh.AlgorithmNegotiationError
		if lastMismatch != nil && errors.As(err, &negotiation) && negotiation.What == "host key" && ctx.Err() == nil {
			return nil, nil, sshConnectionError(ctx, lastMismatch, authType)
		}
		return nil, nil, sshConnectionError(ctx, err, authType)
	}
	return nil, nil, sshConnectionError(ctx, lastMismatch, authType)
}

// minTime 返回较早的时刻。
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// limitedOutput 是并发安全的限长缓冲（1 MiB），用于 SSH 命令输出，超出即丢弃。
type limitedOutput struct {
	mu   sync.Mutex
	text strings.Builder
}

// Write 始终返回已写入长度，超出上限的部分直接丢弃，避免 SSH 写端报错中断。
func (b *limitedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := (1 << 20) - b.text.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.text.Write(p)
	}
	return n, nil
}

// String 返回缓冲的命令输出。
func (b *limitedOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.text.String() }

// CommandResult 是 SSH 命令执行结果：合并的 stdout/stderr 输出与退出码。
type CommandResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}

// run 在既有 SSH 连接上执行一条命令：输出合并、限长，可选 stdin（用于推送脚本），
// 非零退出码作为正常结果返回，会话级错误才返回 error。
func run(client *ssh.Client, command string, input io.Reader) (*CommandResult, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, errcode.Newf(errcode.ErrServiceUnavailable, "无法创建 SSH 会话")
	}
	defer session.Close()
	output := &limitedOutput{}
	session.Stdout = output
	session.Stderr = output
	session.Stdin = input
	err = session.Run(command)
	result := &CommandResult{Output: output.String()}
	if err != nil {
		var exit *ssh.ExitError
		if errors.As(err, &exit) {
			result.ExitCode = exit.ExitStatus()
		} else {
			return nil, errcode.Newf(errcode.ErrServiceUnavailable, "SSH 执行中断或超时")
		}
	}
	return result, nil
}

// quote 用单引号安全包裹值供 shell 使用，内部单引号按 ' -> '"'"' 转义。
func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
