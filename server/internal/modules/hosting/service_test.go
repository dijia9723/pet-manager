package hosting

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"pet-manager/server/internal/config"
	"pet-manager/server/internal/middleware"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/jwtx"
	"pet-manager/server/internal/router"
)

// TestNormalizeHostFingerprint 覆盖主机指纹规范化：多种合法写法归一，非法/歧义写法拒绝。
func TestNormalizeHostFingerprint(t *testing.T) {
	const digest = "0JhMGudxQCCEnwP89A1rkk/wtiHbzDM2BDuDNqxk/H4"
	const canonical = "SHA256:" + digest
	for _, input := range []string{
		digest, canonical, " \t" + digest + "\r\n", digest + "=", canonical + "=",
		"256 " + canonical + " root@VM-0-4-ubuntu (ED25519)",
		"256 " + canonical + " (ED25519)",
		"256 " + canonical + " comment with spaces (ED25519)\n",
	} {
		t.Run(input, func(t *testing.T) {
			got, err := normalizeFingerprint(input)
			if err != nil || got != canonical {
				t.Fatalf("normalizeFingerprint() = %q, %v; want %q", got, err, canonical)
			}
			// Valid formats must also pass the save boundary before credentials/database access.
			_, err = (&Service{}).SaveServer(context.Background(), 0, ServerInput{Name: "test", Host: "127.0.0.1", HostFingerprint: input})
			if err == nil || !strings.Contains(err.Error(), "SSH 凭证不能为空") {
				t.Fatalf("save rejected a valid fingerprint: %v", err)
			}
		})
	}
	for _, input := range []string{
		"", "SHA256:", "SHA256:abc", "MD5:00:11:22:33", "sha256:" + digest,
		strings.Repeat("A", 42), strings.Repeat("A", 44),
		digest[:len(digest)-1] + "5", digest + "==", canonical + " trailing text",
		"256 " + canonical, "invalid " + canonical + " (ED25519)",
		"256 " + canonical + " " + canonical + " (ED25519)",
		canonical + "\n" + canonical,
	} {
		t.Run("invalid/"+input, func(t *testing.T) {
			if _, err := normalizeFingerprint(input); err == nil {
				t.Fatal("accepted invalid or ambiguous fingerprint")
			}
			_, err := (&Service{}).SaveServer(context.Background(), 0, ServerInput{Name: "test", Host: "127.0.0.1", HostFingerprint: input})
			if err == nil || !strings.Contains(err.Error(), "主机指纹无效") {
				t.Fatalf("save did not reject fingerprint before IO: %v", err)
			}
		})
	}
}

// TestCredentialsAreEncryptedAuthenticatedAndNeverSerialized 覆盖凭证加密：
// 随机化、可解密、防篡改，且序列化输出绝不暴露凭证字段。
func TestCredentialsAreEncryptedAuthenticatedAndNeverSerialized(t *testing.T) {
	c, err := newCipher(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{cipher: c}
	ciphertext, err := service.encrypt("PRIVATE SSH SECRET")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.encrypt("PRIVATE SSH SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == second || strings.Contains(ciphertext, "PRIVATE") {
		t.Fatal("encryption is not randomized")
	}
	plain, err := service.decrypt(ciphertext)
	if err != nil || plain != "PRIVATE SSH SECRET" {
		t.Fatal("decrypt failed", err)
	}
	raw, _ := base64.StdEncoding.DecodeString(ciphertext)
	raw[len(raw)-1] ^= 1
	if _, err := service.decrypt(base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("accepted tampered ciphertext")
	}
	for _, row := range []any{model.ManagedServer{Credential: "LEAK"}, model.Website{SFTPKey: "LEAK", CMSSecret: "LEAK"}} {
		data, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "LEAK") || strings.Contains(string(data), "credential") || strings.Contains(string(data), "sftp_key") || strings.Contains(string(data), "cms_secret") {
			t.Fatal("credential exposed", string(data))
		}
	}
	if _, err := (&Service{}).encrypt("secret"); err == nil {
		t.Fatal("accepted credential without encryption key")
	}
	if _, err := newCipher("weak-key"); err == nil {
		t.Fatal("accepted weak key")
	}
}

// TestTenantWebsiteScopeCannotBeOverridden 断言租户作用域不可被请求参数覆盖，
// 无租户身份的上下文被拒绝，平台身份放行。
func TestTenantWebsiteScopeCannotBeOverridden(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=test dbname=test sslmode=disable"}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint64{17, 28} {
		ctx := ctxkey.WithTenant(ctxkey.WithUser(context.Background(), 1, "tenant", "tenant"), id, "code", "tenant_1")
		scoped, err := scopeWebsites(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		query := scoped.Where("tenant_id=?", 99).First(&model.Website{}, 5)
		sql := query.Statement.SQL.String()
		if !strings.Contains(sql, "tenant_id=$1") || !strings.Contains(sql, "tenant_id=$2") || query.Statement.Vars[0] != id {
			t.Fatal("tenant guard lost", sql, query.Statement.Vars)
		}
	}
	for _, ctx := range []context.Context{context.Background(), ctxkey.WithUser(context.Background(), 1, "tenant", "tenant")} {
		if _, err := scopeWebsites(ctx, db); err == nil {
			t.Fatal("accepted absent tenant identity")
		}
	}
	ctx := ctxkey.WithUser(context.Background(), 1, "admin", "platform")
	if _, err := scopeWebsites(ctx, db); err != nil {
		t.Fatal(err)
	}
}

// TestPathsRejectEscapeAndRootMutationBeforeAnyIO 断言路径在触达 IO 前即拒绝穿越与根目录修改，
// 合法路径被规范化到 /www 下，超大写入被拒绝。
func TestPathsRejectEscapeAndRootMutationBeforeAnyIO(t *testing.T) {
	s := &Service{}
	for _, value := range []string{"../other", "/../etc/passwd", "a/../../other", "a/../b", "..", "\\etc\\passwd", "x\x00y", "x\ny", "/"} {
		if err := s.WriteFile(context.Background(), 1, value, []byte("x")); err == nil {
			t.Fatalf("accepted unsafe path %q", value)
		}
	}
	for _, value := range []string{"index.html", "/assets/main.js", "/a//b", "/文章.txt"} {
		target, err := filePath(value, false)
		if err != nil || !strings.HasPrefix(target, "/www/") {
			t.Fatal(value, target, err)
		}
	}
	if err := s.WriteFile(context.Background(), 1, "index.html", make([]byte, maxFileBytes+1)); err == nil {
		t.Fatal("accepted oversized upload")
	}
}

// TestSFTPRejectsSymlinkComponents 断言路径中任一级符号链接都被拒绝，缺失父目录也不放行。
func TestSFTPRejectsSymlinkComponents(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	server := sftp.NewRequestServer(serverConn, sftp.InMemHandler())
	go func() { _ = server.Serve(); _ = server.Close() }()
	client, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Mkdir("/www"); err != nil {
		t.Fatal(err)
	}
	file, err := client.OpenFile("/www/index.html", os.O_WRONLY|os.O_CREATE)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if err := client.Symlink("/www/index.html", "/www/link"); err != nil {
		t.Fatal(err)
	}
	if err := client.Symlink("/www", "/www/linked-dir"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"/link", "/linked-dir/index.html", "/linked-dir/new.html"} {
		if _, err := checkedPath(client, value, true, false); err == nil || errcode.From(err).Code != errcode.ErrForbiddenAction {
			t.Fatal("symlink not rejected", value, err)
		}
	}
	if _, err := checkedPath(client, "/new.html", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := checkedPath(client, "/absent/new.html", true, false); err == nil {
		t.Fatal("accepted missing parent")
	}
}

// TestPinnedSSHAndRequestCancellation 断言指纹 pin 生效（换 key 即失败）且请求取消能关闭连接。
func TestPinnedSSHAndRequestCancellation(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(signer)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				client, channels, requests, err := ssh.NewServerConn(conn, serverConfig)
				if err != nil {
					conn.Close()
					return
				}
				defer client.Close()
				go ssh.DiscardRequests(requests)
				for channel := range channels {
					_ = channel.Reject(ssh.Prohibited, "no sessions")
				}
			}()
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	var parsedPort int
	for _, digit := range port {
		parsedPort = parsedPort*10 + int(digit-'0')
	}
	row := model.ManagedServer{Host: host, Port: parsedPort, HostFingerprint: ssh.FingerprintSHA256(signer.PublicKey())}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	client, closeConn, err := dial(ctx, row, "admin", "password", "secret")
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	_ = client.Close()
	closeConn()
	row.HostFingerprint = "SHA256:" + base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if client, closeConn, err := dial(ctx, row, "admin", "password", "secret"); err == nil {
		client.Close()
		closeConn()
		t.Fatal("accepted changed host key")
	}
}

// startSSHTestServer 启动一个仅用于握手的测试 SSH 服务端，返回其地址信息。
func startSSHTestServer(t *testing.T, cfg *ssh.ServerConfig) model.ManagedServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				client, channels, requests, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer client.Close()
				go ssh.DiscardRequests(requests)
				for channel := range channels {
					_ = channel.Reject(ssh.Prohibited, "no sessions")
				}
			}()
		}
	}()
	address := listener.Addr().(*net.TCPAddr)
	return model.ManagedServer{Host: "127.0.0.1", Port: address.Port}
}

// TestSSHSelectsPinnedKeyBeforeSendingCredentials 断言在选择到 pin 对应的主机公钥之前
// 不发送任何认证凭证，各类主机密钥（ed25519/ecdsa/rsa）都能正确选中。
func TestSSHSelectsPinnedKeyBeforeSendingCredentials(t *testing.T) {
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signers := make(map[string]ssh.Signer)
	for name, key := range map[string]any{"ed25519": edKey, "ecdsa": ecKey, "rsa": rsaKey} {
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			t.Fatal(err)
		}
		signers[name] = signer
	}
	for _, tc := range []struct {
		name      string
		pin       string
		password  string
		wantError string
		wantAuth  int64
	}{
		{"ed25519", ssh.FingerprintSHA256(signers["ed25519"].PublicKey()), "test-password", "", 1},
		{"ecdsa", ssh.FingerprintSHA256(signers["ecdsa"].PublicKey()), "test-password", "", 1},
		{"rsa", ssh.FingerprintSHA256(signers["rsa"].PublicKey()), "test-password", "", 1},
		{"legacy digest", strings.TrimPrefix(ssh.FingerprintSHA256(signers["ed25519"].PublicKey()), "SHA256:"), "test-password", "", 1},
		{"wrong password after key selection", ssh.FingerprintSHA256(signers["rsa"].PublicKey()), "wrong-password", "SSH 密码认证失败（主机指纹已验证）", 1},
		{"untrusted key", "SHA256:" + base64.RawStdEncoding.EncodeToString(make([]byte, 32)), "test-password", "SSH 主机指纹不匹配", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var authCalls atomic.Int64
			cfg := &ssh.ServerConfig{PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
				authCalls.Add(1)
				if meta.User() == "admin" && string(password) == "test-password" {
					return nil, nil
				}
				return nil, errcode.New(errcode.ErrUnauthorized)
			}}
			for _, signer := range signers {
				cfg.AddHostKey(signer)
			}
			row := startSSHTestServer(t, cfg)
			row.HostFingerprint = tc.pin
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			client, closeConn, err := dial(ctx, row, "admin", "password", tc.password)
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				client.Close()
				closeConn()
			} else if err == nil {
				client.Close()
				closeConn()
				t.Fatal("unexpected SSH success")
			} else if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("got %v; want %q", err, tc.wantError)
			}
			if calls := authCalls.Load(); calls != tc.wantAuth {
				t.Fatalf("password attempts = %d; want %d", calls, tc.wantAuth)
			}
		})
	}

	t.Run("default library chooses a different key", func(t *testing.T) {
		cfg := &ssh.ServerConfig{NoClientAuth: true}
		for _, signer := range signers {
			cfg.AddHostKey(signer)
		}
		row := startSSHTestServer(t, cfg)
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(row.Host, strconv.Itoa(row.Port)), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		_, _, _, err = ssh.NewClientConn(conn, "test", &ssh.ClientConfig{User: "admin", HostKeyCallback: fingerprintCallback(ssh.FingerprintSHA256(signers["ed25519"].PublicKey()))})
		var mismatch *hostKeyMismatchError
		if !errors.As(err, &mismatch) || mismatch.keyType != ssh.KeyAlgoECDSA256 {
			t.Fatalf("did not reproduce default host key mismatch: %v", err)
		}
	})
}

// TestSSHConnectionFailureMessages 覆盖各类 SSH/网络错误映射出的用户提示。
func TestSSHConnectionFailureMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		ctx  context.Context
		err  error
		want string
	}{
		{ctx, context.Canceled, "SSH 连接已取消"},
		{context.Background(), context.DeadlineExceeded, "SSH 连接或握手超时"},
		{context.Background(), &net.OpError{Op: "dial", Err: errors.New("connection refused")}, "无法连接 SSH 服务器"},
		{context.Background(), &ssh.AlgorithmNegotiationError{What: "host key"}, "SSH 算法协商失败"},
		{context.Background(), errors.New("ssh: unable to authenticate"), "SSH 私钥认证失败（主机指纹已验证）"},
		{context.Background(), io.EOF, "SSH 握手失败或连接被服务器关闭"},
	} {
		if got := sshConnectionError(tc.ctx, tc.err, "key"); !strings.Contains(got.Error(), tc.want) {
			t.Fatalf("got %v; want %q", got, tc.want)
		}
	}
}

// TestDirectusIsolationAndLiteralPassword 断言部署产物隔离安全：
// 端口仅绑回环、不暴露数据库、不挂载宿主路径，且机密中的 $ 被转义避免 compose 插值。
func TestDirectusIsolationAndLiteralPassword(t *testing.T) {
	site := model.Website{CMSURL: "https://cms.example.com", URL: "https://example.com"}
	compose, err := composeFor(site, DeployInput{Port: 8056, AdminEmail: "admin@example.com", AdminPassword: "abc$HOME${SECRET}123", LicenseKey: "license$literal"}, cmsSecrets{DBPassword: "random-db", Secret: "random-secret"})
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Services map[string]struct {
			Image       string            `json:"image"`
			Ports       []string          `json:"ports"`
			Volumes     []string          `json:"volumes"`
			Environment map[string]string `json:"environment"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(compose), &config); err != nil {
		t.Fatal(err)
	}
	cms := config.Services["directus"]
	if cms.Image != "directus/directus:"+directusVersion || cms.Ports[0] != "127.0.0.1:8056:8055" {
		t.Fatal("unsafe image or public listener", cms)
	}
	if cms.Environment["ADMIN_PASSWORD"] != "abc$$HOME$${SECRET}123" {
		t.Fatal("compose will interpolate credentials")
	}
	if len(config.Services["database"].Ports) != 0 {
		t.Fatal("database exposed")
	}
	for _, service := range config.Services {
		for _, volume := range service.Volumes {
			if strings.HasPrefix(volume, "/") || strings.Contains(volume, "docker.sock") {
				t.Fatal("unsafe host mount", volume)
			}
		}
	}
	script := deployScript(7, compose)
	if strings.Contains(script, siteRoot(7)) || !strings.Contains(script, controlRoot(7)) || !strings.Contains(script, "-p pm-site-7") {
		t.Fatal("tenant can edit privileged compose")
	}
	provision := provisionScript(7, "ssh-ed25519 public")
	for _, required := range []string{"ChrootDirectory " + siteRoot(7), "ForceCommand internal-sftp", "DisableForwarding yes", "PermitTTY no", "sshd -t", "sshd -T"} {
		if !strings.Contains(provision, required) {
			t.Fatal("missing isolation", required)
		}
	}
}

// TestTenantHasNoSSHOrDeploymentRoutesAndCannotUsePlatformTokenScope 断言租户侧不存在
// 特权路由，且租户令牌无法访问平台 SSH/部署接口。
func TestTenantHasNoSSHOrDeploymentRoutesAndCannotUsePlatformTokenScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	jwt := jwtx.New(config.JWTConfig{Secret: "test-only-secret", Issuer: "test", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	route := router.New(engine, router.Deps{Auth: middleware.NewAuthenticator(jwt, nil)})
	(&Service{}).Register(route)
	for _, request := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/tenant/servers/1/exec"},
		{http.MethodGet, "/api/v1/tenant/servers/1/terminal"},
		{http.MethodPost, "/api/v1/tenant/websites/1/provision"},
		{http.MethodPost, "/api/v1/tenant/websites/1/cms/deploy"},
		{http.MethodPost, "/api/v1/tenant/websites"},
		{http.MethodPut, "/api/v1/tenant/websites/1"},
	} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatal("privileged tenant route exists", request, response.Code)
		}
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/platform/servers/1/exec", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatal("SSH accessible without authentication")
	}
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/platform/servers/1/terminal", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatal("terminal accessible without authentication")
	}
	token, _, err := jwt.Sign(jwtx.Claims{UserID: 2, Username: "tenant", UserType: "tenant", TenantID: 17}, jwtx.TokenTypeAccess)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/platform/servers/1/exec", "/api/v1/platform/websites/1/provision", "/api/v1/platform/websites/1/cms/deploy"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatal("tenant token accessed platform action", path, response.Code)
		}
		var body struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Code != int(errcode.ErrForbiddenAction) {
			t.Fatal("unexpected denial", response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/platform/servers/1/terminal", nil)
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Protocol", terminalProtocol+", bearer."+token)
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "20005") {
		t.Fatal("tenant token accessed platform terminal", response.Code, response.Body.String())
	}
}
