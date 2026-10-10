package hosting

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
)

// directusVersion 是固定部署的 Directus 镜像版本。
const directusVersion = "12.5.0"

//go:embed directus-bootstrap.js
var directusBootstrap string

// siteUser 返回站点受限系统账号名（按网站 ID 唯一生成，非租户可控）。
func siteUser(id uint64) string { return "pm_site_" + strconv.FormatUint(id, 10) }

// siteRoot 返回站点 chroot 根目录（租户文件可见范围）。
func siteRoot(id uint64) string { return "/srv/pet-manager/sites/" + strconv.FormatUint(id, 10) }

// controlRoot 返回平台私有的站点控制目录（compose 等，租户不可见）。
func controlRoot(id uint64) string { return "/etc/pet-manager/sites/" + strconv.FormatUint(id, 10) }

// normalizeFingerprint 规范化主机指纹：接受裸 SHA256 摘要或 ssh-keygen -lf 单行输出，
// 统一为 "SHA256:<base64>"，并拒绝多行/歧义输入，确保后续能用固定值做 pin 比对。
func normalizeFingerprint(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("expected a single host fingerprint")
	}
	fields := strings.Fields(value)
	if len(fields) > 1 {
		// ssh-keygen -lf output: key size, fingerprint, optional comment, key type.
		bits, err := strconv.Atoi(fields[0])
		if len(fields) < 3 || err != nil || bits <= 0 || !strings.HasPrefix(fields[1], "SHA256:") || !strings.HasPrefix(fields[len(fields)-1], "(") || !strings.HasSuffix(fields[len(fields)-1], ")") {
			return "", fmt.Errorf("invalid ssh-keygen fingerprint output")
		}
		for _, field := range fields[2:] {
			if strings.HasPrefix(field, "SHA256:") {
				return "", fmt.Errorf("ambiguous host fingerprint")
			}
		}
		value = fields[1]
	}
	digest := strings.TrimPrefix(value, "SHA256:")
	encoding := base64.RawStdEncoding.Strict()
	if strings.HasSuffix(digest, "=") {
		encoding = base64.StdEncoding.Strict()
	}
	b, err := encoding.DecodeString(digest)
	if err != nil || len(b) != 32 {
		return "", fmt.Errorf("invalid SHA256 host fingerprint")
	}
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(b), nil
}

// provisionScript 生成站点初始化脚本：建受限账号、chroot 目录与 sshd Match 配置并热加载，
// 全程使用生成的路径与 ID，不含任何租户可控内容。
// provisionScript uses generated paths and IDs, never tenant-controlled code.
func provisionScript(id uint64, publicKey string) string {
	user, root := siteUser(id), siteRoot(id)
	lines := []string{
		"set -eu",
		"export PATH=\"$PATH:/usr/sbin:/sbin\"",
		"printf '%s\\n' PM_HOSTING_STEP:root",
		"test \"$(id -u)\" = 0",
		"printf '%s\\n' PM_HOSTING_STEP:sshd",
		"command -v sshd >/dev/null",
		"printf '%s\\n' PM_HOSTING_STEP:useradd",
		"command -v useradd >/dev/null",
		"printf '%s\\n' PM_HOSTING_STEP:systemctl",
		"command -v systemctl >/dev/null",
		"printf '%s\\n' PM_HOSTING_STEP:flock",
		"command -v flock >/dev/null",
		"printf '%s\\n' PM_HOSTING_STEP:lock",
		"test ! -L /run/pet-manager-ssh.lock",
		"exec 9>/run/pet-manager-ssh.lock",
		"flock -w 5 -x 9",
		"printf '%s\\n' PM_HOSTING_STEP:include_directory",
		"test -d /etc/ssh/sshd_config.d",
		"printf '%s\\n' PM_HOSTING_STEP:existing_config",
		"sshd -t",
		"printf '%s\\n' PM_HOSTING_STEP:directories",
		"umask 077",
		"for p in /srv/pet-manager /srv/pet-manager/sites " + root + " /etc/ssh/pet-manager; do test ! -L \"$p\"; done",
		"mkdir -p " + root + " /etc/ssh/pet-manager",
		"chown root:root /srv/pet-manager /srv/pet-manager/sites " + root + " /etc/ssh/pet-manager",
		"chmod 0711 /srv/pet-manager /srv/pet-manager/sites",
		// The jailed account must list its own root during isolation verification.
		// Parent directories remain non-listable; the chroot remains root-owned and non-writable.
		"chmod 0755 " + root,
		"chmod 0755 /etc/ssh/pet-manager",
		"printf '%s\\n' PM_HOSTING_STEP:account",
		"if id " + user + " >/dev/null 2>&1; then",
		"  test \"$(getent passwd " + user + " | cut -d: -f6)\" = /www",
		"  test \"$(id -u " + user + ")\" -ge 1000",
		"  test \"$(id -Gn " + user + ")\" = " + user,
		"else",
		"  useradd --home-dir /www --no-create-home --shell /usr/sbin/nologin --user-group " + user,
		"  usermod -p '*' " + user,
		"fi",
		"printf '%s\\n' PM_HOSTING_STEP:website_directory",
		"test ! -L " + root + "/www",
		"mkdir -p " + root + "/www",
		"chown " + user + ":" + user + " " + root + "/www",
		"chmod 0750 " + root + "/www",
		"test ! -L /etc/ssh/pet-manager/" + user,
		"printf '%s\\n' " + quote(strings.TrimSpace(publicKey)) + " > /etc/ssh/pet-manager/" + user,
		"chown root:root /etc/ssh/pet-manager/" + user,
		"chmod 0644 /etc/ssh/pet-manager/" + user,
		"printf '%s\\n' PM_HOSTING_STEP:ssh_config",
		"test ! -L /etc/ssh/sshd_config.d/00-pet-manager-" + user + ".conf",
		"conf=/etc/ssh/sshd_config.d/00-pet-manager-" + user + ".conf",
		"backup=$(mktemp /etc/ssh/.pm-backup-XXXXXX)",
		"had_conf=0",
		"if test -f \"$conf\"; then cp \"$conf\" \"$backup\"; had_conf=1; fi",
		"rollback() { if test \"$had_conf\" = 1; then cp \"$backup\" \"$conf\"; else rm -f \"$conf\"; fi; rm -f \"$backup\"; systemctl reload sshd >/dev/null 2>&1 || systemctl reload ssh >/dev/null 2>&1 || true; }",
		"trap rollback EXIT",
		"cat > /etc/ssh/sshd_config.d/00-pet-manager-" + user + ".conf <<'PM_SSH_CONFIG'",
		"Match User " + user,
		"  ChrootDirectory " + root,
		"  ForceCommand internal-sftp -d /www -u 0027",
		"  AuthorizedKeysFile /etc/ssh/pet-manager/%u",
		"  AuthenticationMethods publickey",
		"  PubkeyAuthentication yes",
		"  PasswordAuthentication no",
		"  KbdInteractiveAuthentication no",
		"  DisableForwarding yes",
		"  PermitTTY no",
		"Match all",
		"PM_SSH_CONFIG",
		"printf '%s\\n' PM_HOSTING_STEP:ssh_validate",
		"sshd -t",
		"printf '%s\\n' PM_HOSTING_STEP:ssh_include",
		"effective=$(sshd -T -C user=" + user + ",host=localhost,addr=127.0.0.1)",
		"printf '%s\\n' \"$effective\" | grep -Fqx 'chrootdirectory " + root + "'",
		"printf '%s\\n' \"$effective\" | grep -Fqx 'forcecommand internal-sftp -d /www -u 0027'",
		"printf '%s\\n' \"$effective\" | grep -Fqx 'disableforwarding yes'",
		"printf '%s\\n' \"$effective\" | grep -Fqx 'authenticationmethods publickey'",
		"printf '%s\\n' PM_HOSTING_STEP:ssh_reload",
		"systemctl reload sshd || systemctl reload ssh",
		"trap - EXIT",
		"rm -f \"$backup\"",
	}
	return strings.Join(lines, "\n") + "\n"
}

// adminScriptCommand 以 root 或免密 sudo 执行经 stdin 传入的脚本；两者都不可用时
// 输出 root_required 并以退出码 77 结束。
// Only platform-owned scripts use privilege escalation. Tenant file operations
// always dial their restricted SFTP account and never run this command.
const adminScriptCommand = "if test \"$(id -u)\" = 0; then exec sh -s; " +
	"elif command -v sudo >/dev/null 2>&1 && sudo -n -u root true </dev/null >/dev/null 2>&1; then exec sudo -n -u root sh -s; " +
	"else printf '%s\\n' PM_HOSTING_ERROR:root_required >&2; exit 77; fi"

// provisionFailure 由脚本输出的 PM_HOSTING_STEP 标记映射为面向用户的失败原因，
// 只回显已知步骤文案，绝不透传脚本原文（避免泄露敏感信息）。
func provisionFailure(result *CommandResult) error {
	steps := map[string]string{
		"root":              "脚本未获得 root 权限，请检查 SSH 账号或免密 sudo 配置",
		"sshd":              "未找到 sshd，请安装 OpenSSH Server",
		"useradd":           "未找到 useradd，请检查系统账号管理工具",
		"systemctl":         "未找到 systemctl，当前初始化流程需要 systemd",
		"flock":             "未找到 flock，请安装 util-linux",
		"lock":              "无法获取 SSH 配置锁，请稍后重试或检查锁文件权限",
		"include_directory": "缺少 /etc/ssh/sshd_config.d 目录，请检查 OpenSSH 配置",
		"existing_config":   "现有 sshd 配置未通过 sshd -t，请先修复 SSH 配置",
		"directories":       "无法创建安全的站点根目录，请检查权限及符号链接",
		"account":           "无法创建受限账号或已有同名账号不符合隔离要求",
		"website_directory": "无法创建或授权网站目录，请检查目录权限及符号链接",
		"ssh_config":        "无法写入 SSH 隔离配置或公钥文件",
		"ssh_validate":      "新增 SSH 隔离配置未通过 sshd -t，已尝试恢复原配置",
		"ssh_include":       "隔离配置未生效，请确认 sshd_config 的 Include 加载 sshd_config.d，且没有冲突的 Match 配置",
		"ssh_reload":        "SSH 服务重载失败，请检查 systemd 的 ssh/sshd 服务，已尝试恢复原配置",
	}
	detail := "请检查服务器的 OpenSSH 与 systemd 配置"
	for _, line := range strings.Split(result.Output, "\n") {
		if step, ok := strings.CutPrefix(strings.TrimSpace(line), "PM_HOSTING_STEP:"); ok {
			if message, known := steps[step]; known {
				detail = message
			}
		}
	}
	return errcode.Newf(errcode.ErrServiceUnavailable, "SFTP 初始化失败：%s（退出码 %d）", detail, result.ExitCode)
}

// verifySFTP 用受限账号建立新的 SFTP 会话并校验隔离是否生效。
func verifySFTP(client *ssh.Client) error {
	files, err := sftp.NewClient(client)
	if err != nil {
		return errcode.Newf(errcode.ErrServiceUnavailable, "受限 SFTP 会话验证失败")
	}
	defer files.Close()
	return verifySFTPFiles(files)
}

// verifySFTPFiles 校验隔离约定：站点根仅含 www、看不到宿主 /etc/passwd、可列目录。
func verifySFTPFiles(files *sftp.Client) error {
	if _, err := files.Stat("/www"); err != nil {
		return errcode.Newf(errcode.ErrServiceUnavailable, "网站目录验证失败")
	}
	if _, err := files.Stat("/etc/passwd"); !os.IsNotExist(err) {
		return errcode.Newf(errcode.ErrForbiddenAction, "SFTP 隔离验证失败")
	}
	entries, err := files.ReadDir("/")
	if err != nil {
		if os.IsPermission(err) {
			return errcode.Newf(errcode.ErrServiceUnavailable, "SFTP 根目录验证失败：受限账号无目录读取权限，请重新初始化目录以修复站点根目录权限")
		}
		return errcode.Newf(errcode.ErrServiceUnavailable, "SFTP 根目录验证失败：无法列出站点根目录，请检查受限 SFTP 会话")
	}
	for _, entry := range entries {
		if entry.Name() != "www" {
			return errcode.Newf(errcode.ErrForbiddenAction, "SFTP 根目录不符合隔离约定")
		}
	}
	return nil
}

// runAdminScript 以平台账号连接服务器并执行特权脚本；脚本经 stdin 送入，
// 脚本内容不会出现在命令历史或 argv 中。非 root 且无法免密 sudo 时给出明确提示。
// Send scripts through stdin so secrets do not appear in command history or argv.
func (s *Service) runAdminScript(ctx context.Context, serverID uint64, script string) (*CommandResult, error) {
	server, err := s.server(ctx, serverID)
	if err != nil {
		return nil, err
	}
	if server.Status != "enabled" {
		return nil, errcode.Newf(errcode.ErrConflict, "服务器已停用")
	}
	secret, err := s.decrypt(server.Credential)
	if err != nil {
		return nil, err
	}
	client, closeConn, err := dial(ctx, *server, server.Username, server.AuthType, secret)
	if err != nil {
		return nil, err
	}
	defer closeConn()
	result, err := run(client, adminScriptCommand, strings.NewReader(script))
	if err == nil && result.ExitCode == 77 && strings.Contains(result.Output, "PM_HOSTING_ERROR:root_required") {
		return nil, errcode.Newf(errcode.ErrServiceUnavailable, "当前 SSH 账号不是 root，且无法通过 sudo -n 提权；请使用 root 或配置该账号的免密 sudo 权限")
	}
	return result, err
}

// finishStatus 用独立超时更新网站某个状态列，避免主流程取消影响状态落地。
func (s *Service) finishStatus(id uint64, column, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return dbError(s.manager.Platform().WithContext(ctx).Model(&model.Website{}).Where("id=?", id).Update(column, status).Error)
}

// Provision 初始化站点：以 CAS 抢占"待初始化/失败"状态后执行初始化脚本，
// 再用受限账号验证 SFTP 隔离，成功置 ready、失败置 failed（带 60s 总超时）。
func (s *Service) Provision(ctx context.Context, id uint64) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	website, err := s.website(ctx, id)
	if err != nil {
		return err
	}
	if website.Status != "enabled" {
		return errcode.Newf(errcode.ErrConflict, "请先启用网站")
	}
	key, err := s.decrypt(website.SFTPKey)
	if err != nil {
		return err
	}
	signer, err := ssh.ParsePrivateKey([]byte(key))
	if err != nil {
		return errcode.New(errcode.ErrInternal)
	}
	claim := s.manager.Platform().WithContext(ctx).Model(&model.Website{}).Where("id=? AND updated_at=? AND provision_status IN ?", id, website.UpdatedAt, []string{"pending", "failed"}).Update("provision_status", "provisioning")
	if claim.Error != nil {
		return dbError(claim.Error)
	}
	if claim.RowsAffected != 1 {
		return errcode.Newf(errcode.ErrConflict, "网站已初始化或正在初始化")
	}
	status := "failed"
	defer func() {
		if status == "failed" {
			_ = s.finishStatus(id, "provision_status", status)
		}
	}()
	result, err := s.runAdminScript(ctx, website.ServerID, provisionScript(id, string(ssh.MarshalAuthorizedKey(signer.PublicKey()))))
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return provisionFailure(result)
	}
	server, err := s.server(ctx, website.ServerID)
	if err != nil {
		return err
	}
	client, closeConn, err := dial(ctx, *server, siteUser(id), "key", key)
	if err != nil {
		return err
	}
	defer closeConn()
	if err := verifySFTP(client); err != nil {
		return err
	}
	if err := s.finishStatus(id, "provision_status", "ready"); err != nil {
		return err
	}
	status = "ready"
	return nil
}

// DeployInput 是 Directus 部署入参，端口需落在 1024-65535 的非特权区间。
type DeployInput struct {
	Port          int    `json:"port" binding:"required,min=1024,max=65535"`
	AdminEmail    string `json:"admin_email" binding:"required,email,max=255"`
	AdminPassword string `json:"admin_password" binding:"required,min=12,max=128"`
	LicenseKey    string `json:"license_key" binding:"max=2048"`
}

// cmsSecrets 是每次部署生成并加密保存的 CMS 机密（库密码与 SECRET）。
type cmsSecrets struct {
	DBPassword string `json:"db_password"`
	Secret     string `json:"secret"`
}

// randomSecret 生成 URL 安全的随机机密（32 字节熵）。
func randomSecret() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), err
}

// composeFor 生成每个站点独立的 compose：自带数据库与命名卷、端口仅绑定本机回环、
// 不挂载宿主机或 Docker socket；因 compose 会对 $ 做插值，需把机密中的 $ 转义为 $$。
// Each Directus instance has its own network, database and volumes; no host/Docker socket mounts.
func composeFor(site model.Website, in DeployInput, secrets cmsSecrets) (string, error) {
	origin, err := url.Parse(site.URL)
	if err != nil {
		return "", err
	}
	environment := map[string]string{
		"SECRET": secrets.Secret, "DB_CLIENT": "pg", "DB_HOST": "database", "DB_PORT": "5432", "DB_DATABASE": "directus", "DB_USER": "directus", "DB_PASSWORD": secrets.DBPassword,
		"ADMIN_EMAIL": in.AdminEmail, "ADMIN_PASSWORD": in.AdminPassword, "PUBLIC_URL": site.CMSURL, "CORS_ENABLED": "true", "CORS_ORIGIN": origin.Scheme + "://" + origin.Host, "WEBSOCKETS_ENABLED": "true", "RATE_LIMITER_ENABLED": "true",
	}
	if in.LicenseKey != "" {
		environment["LICENSE_KEY"] = in.LicenseKey
	}
	// Compose interpolates dollar signs in JSON strings; preserve literal credential values.
	for key, value := range environment {
		environment[key] = strings.ReplaceAll(value, "$", "$$")
	}
	compose := map[string]any{
		"services": map[string]any{
			"database": map[string]any{"image": "postgres:17.11-alpine", "restart": "unless-stopped", "environment": map[string]string{"POSTGRES_DB": "directus", "POSTGRES_USER": "directus", "POSTGRES_PASSWORD": secrets.DBPassword}, "volumes": []string{"database:/var/lib/postgresql/data"}, "healthcheck": map[string]any{"test": []string{"CMD-SHELL", "pg_isready -U directus"}, "interval": "5s", "timeout": "5s", "retries": 20}, "security_opt": []string{"no-new-privileges:true"}},
			"directus": map[string]any{"image": "directus/directus:" + directusVersion, "restart": "unless-stopped", "environment": environment, "ports": []string{fmt.Sprintf("127.0.0.1:%d:8055", in.Port)}, "volumes": []string{"uploads:/directus/uploads", "extensions:/directus/extensions"}, "depends_on": map[string]any{"database": map[string]string{"condition": "service_healthy"}}, "healthcheck": map[string]any{"test": []string{"CMD", "node", "-e", "fetch('http://127.0.0.1:8055/server/ping').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"}, "interval": "5s", "timeout": "5s", "retries": 24, "start_period": "30s"}, "security_opt": []string{"no-new-privileges:true"}},
		}, "volumes": map[string]any{"database": map[string]any{}, "uploads": map[string]any{}, "extensions": map[string]any{}},
	}
	data, err := json.MarshalIndent(compose, "", "  ")
	return string(data), err
}

// deployScript 生成部署脚本：在平台控制目录写入 base64 编码的 compose，
// 以独立 compose 项目启动容器等待健康检查，再注入 bootstrap 脚本，全程带文件锁。
func deployScript(id uint64, compose string) string {
	root := controlRoot(id)
	encoded := base64.StdEncoding.EncodeToString([]byte(compose))
	return strings.Join([]string{
		"set -eu",
		"test \"$(id -u)\" = 0",
		"docker compose version >/dev/null",
		fmt.Sprintf("test ! -L /run/pet-manager-cms-%d.lock", id),
		fmt.Sprintf("exec 9>/run/pet-manager-cms-%d.lock", id),
		"flock -w 5 -x 9",
		"umask 077",
		"for p in /etc/pet-manager /etc/pet-manager/sites " + root + "; do test ! -L \"$p\"; done",
		"mkdir -p " + root,
		"chown root:root /etc/pet-manager /etc/pet-manager/sites " + root,
		"chmod 0700 /etc/pet-manager /etc/pet-manager/sites " + root,
		"test ! -L " + root + "/compose.json",
		"printf '%s' " + quote(encoded) + " | base64 -d > " + root + "/compose.json",
		"chmod 0600 " + root + "/compose.json",
		fmt.Sprintf("docker compose -p pm-site-%d -f %s/compose.json up -d --wait --wait-timeout 150 >/dev/null 2>&1", id, root),
		fmt.Sprintf("docker compose -p pm-site-%d -f %s/compose.json exec -T directus node <<'PM_CMS_BOOTSTRAP'", id, root),
		directusBootstrap,
		"PM_CMS_BOOTSTRAP",
	}, "\n") + "\n"
}

// DeployCMS 部署站点 CMS：要求网站已启用、已初始化且选择 Directus；
// 复用或生成机密并加密保存，CAS 抢占"待部署/失败"状态（唯一索引串行化端口占用），
// 执行部署脚本，成功置 ready、失败置 failed（带 180s 总超时，重试保留数据卷）。
func (s *Service) DeployCMS(ctx context.Context, id uint64, in DeployInput) error {
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	website, err := s.website(ctx, id)
	if err != nil {
		return err
	}
	if website.CMS != "directus" || website.CMSURL == "" || website.URL == "" || website.Status != "enabled" {
		return errcode.Newf(errcode.ErrInvalidParam, "请启用网站，选择 Directus 并填写网站/CMS 地址")
	}
	if website.ProvisionStatus != "ready" {
		return errcode.Newf(errcode.ErrConflict, "请先初始化网站目录")
	}
	var secrets cmsSecrets
	if website.CMSSecret != "" {
		value, err := s.decrypt(website.CMSSecret)
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(value), &secrets); err != nil {
			return errcode.New(errcode.ErrInternal)
		}
	} else {
		secrets.DBPassword, err = randomSecret()
		if err != nil {
			return err
		}
		secrets.Secret, err = randomSecret()
		if err != nil {
			return err
		}
	}
	data, err := json.Marshal(secrets)
	if err != nil {
		return err
	}
	encrypted, err := s.encrypt(string(data))
	if err != nil {
		return err
	}
	// Unique index serializes port reservations across deployments on the same server.
	claim := s.manager.Platform().WithContext(ctx).Model(&model.Website{}).Where("id=? AND updated_at=? AND cms_status IN ?", id, website.UpdatedAt, []string{"pending", "failed"}).Updates(map[string]any{"cms_status": "deploying", "cms_port": in.Port, "cms_version": directusVersion, "cms_secret": encrypted})
	if claim.Error != nil {
		return errcode.Newf(errcode.ErrConflict, "CMS 端口已被其他网站占用或部署状态保存失败")
	}
	if claim.RowsAffected != 1 {
		return errcode.Newf(errcode.ErrConflict, "CMS 已部署或正在部署")
	}
	status := "failed"
	defer func() {
		if status == "failed" {
			_ = s.finishStatus(id, "cms_status", status)
		}
	}()
	compose, err := composeFor(*website, in, secrets)
	if err != nil {
		return err
	}
	result, err := s.runAdminScript(ctx, website.ServerID, deployScript(id, compose))
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return errcode.Newf(errcode.ErrServiceUnavailable, "Directus 部署失败，请通过服务器管理检查 Docker、端口和容器日志；重试会保留数据卷")
	}
	if err := s.finishStatus(id, "cms_status", "ready"); err != nil {
		return err
	}
	status = "ready"
	return nil
}

// RecoverInterrupted 回收超时（>10 分钟）仍停留在中间态的部署状态为 failed，
// 只处理明显的陈旧记录，避免误改其他实例正在进行的请求。
// Recover only stale claims, never another instance's live request.
func (s *Service) RecoverInterrupted(ctx context.Context) error {
	tx := s.manager.Platform().WithContext(ctx).Model(&model.Website{}).Where("updated_at < ?", time.Now().Add(-10*time.Minute))
	return dbError(tx.Where("provision_status = ? OR cms_status = ?", "provisioning", "deploying").Updates(map[string]any{"provision_status": gorm.Expr("CASE WHEN provision_status = 'provisioning' THEN 'failed' ELSE provision_status END"), "cms_status": gorm.Expr("CASE WHEN cms_status = 'deploying' THEN 'failed' ELSE cms_status END")}).Error)
}
