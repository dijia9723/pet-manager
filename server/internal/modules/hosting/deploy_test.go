package hosting

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/sftp"
	"pet-manager/server/internal/pkg/errcode"
)

// rootPermissionLister 在内存 SFTP 上模拟"根目录拒绝读取"以复现权限不足场景。
type rootPermissionLister struct {
	sftp.FileLister
}

func (l rootPermissionLister) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	if r.Method == "List" && r.Filepath == "/" {
		return nil, sftp.ErrSSHFxPermissionDenied
	}
	return l.FileLister.Filelist(r)
}

// TestSFTPIsolationVerification 覆盖 SFTP 隔离校验：正常站点通过，越权/根目录异常被拒绝。
func TestSFTPIsolationVerification(t *testing.T) {
	for _, tc := range []struct {
		name, extraPath string
		rootDenied      bool
		wantCode        errcode.Code
		wantMessage     string
	}{
		{name: "isolated root with www"},
		{name: "root read permission denied", rootDenied: true, wantCode: errcode.ErrServiceUnavailable, wantMessage: "无目录读取权限"},
		{name: "host filesystem visible", extraPath: "/etc/passwd", wantCode: errcode.ErrForbiddenAction, wantMessage: "隔离验证失败"},
		{name: "unexpected root entry", extraPath: "/other-site", wantCode: errcode.ErrForbiddenAction, wantMessage: "根目录不符合隔离约定"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handlers := sftp.InMemHandler()
			if tc.rootDenied {
				handlers.FileList = rootPermissionLister{handlers.FileList}
			}
			serverConn, clientConn := net.Pipe()
			server := sftp.NewRequestServer(serverConn, handlers)
			go func() { _ = server.Serve(); _ = server.Close() }()
			t.Cleanup(func() { _ = clientConn.Close(); _ = server.Close() })
			client, err := sftp.NewClientPipe(clientConn, clientConn)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if err := client.Mkdir("/www"); err != nil {
				t.Fatal(err)
			}
			if tc.extraPath != "" {
				if strings.HasPrefix(tc.extraPath, "/etc/") {
					if err := client.Mkdir("/etc"); err != nil {
						t.Fatal(err)
					}
				}
				file, err := client.Create(tc.extraPath)
				if err != nil {
					t.Fatal(err)
				}
				_ = file.Close()
			}
			// Confirm a real SFTP permission status reproduces the old generic 20005.
			if tc.rootDenied {
				_, err := client.ReadDir("/")
				if !os.IsPermission(err) || errcode.From(fileError(err)).Code != errcode.ErrForbiddenAction {
					t.Fatalf("expected SFTP permission denial, got %v", err)
				}
			}
			err = verifySFTPFiles(client)
			if tc.wantCode == errcode.OK {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || errcode.From(err).Code != tc.wantCode || !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("verification = %v; want code %d / %q", err, tc.wantCode, tc.wantMessage)
			}
		})
	}
}

// TestProvisionRootPermissionsPermitIsolationVerification 断言初始化脚本设置了
// 能让隔离校验通过的目录属主与权限规则。
func TestProvisionRootPermissionsPermitIsolationVerification(t *testing.T) {
	script := provisionScript(7, "ssh-ed25519 public")
	for _, line := range []string{
		"chown root:root /srv/pet-manager /srv/pet-manager/sites /srv/pet-manager/sites/7 /etc/ssh/pet-manager",
		"chmod 0711 /srv/pet-manager /srv/pet-manager/sites",
		"chmod 0755 /srv/pet-manager/sites/7",
		"chown pm_site_7:pm_site_7 /srv/pet-manager/sites/7/www",
		"chmod 0750 /srv/pet-manager/sites/7/www",
	} {
		if !strings.Contains(script, "\n"+line+"\n") {
			t.Fatalf("missing directory ownership/permission rule %q", line)
		}
	}
}

// TestAdminScriptUsesRootOrNoninteractiveSudoAndPreservesStdin 覆盖提权命令：
// root 直跑、免密 sudo 提权、无法提权时报 root_required，且脚本经 stdin 而非 argv。
func TestAdminScriptUsesRootOrNoninteractiveSudoAndPreservesStdin(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX shell unavailable")
	}
	bin := t.TempDir()
	for name, script := range map[string]string{
		"id":   "#!/bin/sh\nprintf '%s\\n' \"$PM_TEST_UID\"\n",
		"sudo": "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\"$PM_TEST_CALLS\"\ntest \"$PM_TEST_SUDO\" = allowed || exit 1\nif test \"$4\" = true; then exit 0; fi\nPM_TEST_UID=0 exec /bin/sh -s\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Git's POSIX shell on Windows needs its own /c/... path representation.
	pathCommand := exec.Command(shell, "-c", "cd \"$1\" && pwd", "sh", filepath.ToSlash(bin))
	posixBin, err := pathCommand.Output()
	if err != nil {
		t.Fatal("resolve shell path", err)
	}
	for _, tc := range []struct {
		name, uid, sudo string
		code            int
		want            string
	}{
		{"root", "0", "denied", 0, "PAYLOAD:0"},
		{"ubuntu with sudo", "1000", "allowed", 0, "PAYLOAD:0"},
		{"sudo requires password", "1000", "denied", 77, "PM_HOSTING_ERROR:root_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := filepath.ToSlash(filepath.Join(bin, strings.ReplaceAll(tc.name, " ", "_")+".log"))
			command := exec.Command(shell, "-c", "PATH=\"$PM_TEST_BIN:$PATH\"; export PATH; "+adminScriptCommand)
			command.Env = append(os.Environ(), "PM_TEST_BIN="+strings.TrimSpace(string(posixBin)), "PM_TEST_UID="+tc.uid, "PM_TEST_SUDO="+tc.sudo, "PM_TEST_CALLS="+calls)
			command.Stdin = strings.NewReader("printf 'PAYLOAD:%s\\n' \"$(id -u)\"\n")
			output, err := command.CombinedOutput()
			code := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tc.code || !strings.Contains(string(output), tc.want) {
				t.Fatalf("exit %d, output %q; want %d / %q", code, output, tc.code, tc.want)
			}
			if tc.code != 0 && strings.Contains(string(output), "PAYLOAD") {
				t.Fatal("ran script without privilege")
			}
			if data, err := os.ReadFile(calls); err == nil && strings.Contains(string(data), "PAYLOAD") {
				t.Fatal("script placed in argv")
			}
		})
	}
}

// TestProvisionFailureReportsOnlyKnownStage 断言失败诊断只回显已知步骤文案，不泄露脚本原文。
func TestProvisionFailureReportsOnlyKnownStage(t *testing.T) {
	for _, tc := range []struct{ output, want string }{
		{"PM_HOSTING_STEP:root\nPM_HOSTING_STEP:ssh_include\nSECRET_NOT_FOR_RESPONSE", "Include"},
		{"PM_HOSTING_STEP:existing_config\n", "现有 sshd 配置"},
		{"PM_HOSTING_STEP:ssh_reload\n", "SSH 服务重载失败"},
		{"PM_HOSTING_STEP:unknown\nSECRET_NOT_FOR_RESPONSE", "OpenSSH 与 systemd"},
	} {
		message := provisionFailure(&CommandResult{ExitCode: 1, Output: tc.output}).Error()
		if !strings.Contains(message, tc.want) || strings.Contains(message, "SECRET_NOT_FOR_RESPONSE") {
			t.Fatalf("incorrect or unsafe diagnostic: %s", message)
		}
	}
}
