package hosting

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkg/sftp"
	"pet-manager/server/internal/pkg/errcode"
)

// 文件大小上限：普通上传/下载 10 MiB，文本编辑 1 MiB。
const maxFileBytes = 10 << 20
const maxTextBytes = 1 << 20

// FileItem 是目录列表返回项。
type FileItem struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	Directory  bool      `json:"directory"`
	Symlink    bool      `json:"symlink"`
	ModifiedAt time.Time `json:"modified_at"`
}

// filePath 把用户输入的应用路径规范化为 chroot 内 /www 下的绝对路径：
// 拒绝反斜杠/控制字符与 ".." 穿越，allowRoot 控制是否允许指向网站根目录。
// All application paths are relative to /www inside the remote OS chroot.
func filePath(value string, allowRoot bool) (string, error) {
	if len(value) > 1024 || strings.ContainsAny(value, "\\\x00\r\n") {
		return "", errcode.New(errcode.ErrInvalidParam)
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return "", errcode.New(errcode.ErrForbiddenAction)
		}
	}
	clean := path.Clean("/" + value)
	if clean == "/" && !allowRoot {
		return "", errcode.Newf(errcode.ErrForbiddenAction, "不允许修改网站根目录")
	}
	return path.Join("/www", clean), nil
}

// checkedPath 在 filePath 基础上逐级 Lstat，拒绝路径中任何符号链接组件（纵深防御，
// 真正的安全边界是远端 OS chroot）；allowMissing 仅在末级不存在时放行（用于新建）。
// Reject symlink components as defense in depth; the OS chroot is the security boundary.
func checkedPath(client *sftp.Client, value string, allowMissing, allowRoot bool) (string, error) {
	target, err := filePath(value, allowRoot)
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.TrimPrefix(target, "/"), "/")
	current := ""
	for i, part := range parts {
		current += "/" + part
		info, err := client.Lstat(current)
		if err != nil {
			if allowMissing && i == len(parts)-1 && os.IsNotExist(err) {
				return target, nil
			}
			return "", fileError(err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errcode.Newf(errcode.ErrForbiddenAction, "不允许操作符号链接")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", errcode.New(errcode.ErrInvalidParam)
		}
	}
	return target, nil
}

// fileError 把 SFTP 系统错误映射为业务错误码（不存在/无权限/服务不可用）。
func fileError(err error) error {
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return errcode.New(errcode.ErrNotFound)
	}
	if os.IsPermission(err) {
		return errcode.New(errcode.ErrForbiddenAction)
	}
	return errcode.Newf(errcode.ErrServiceUnavailable, "远程文件操作失败")
}

// files 用站点的受限 SFTP 账号建立连接，返回客户端与统一的关闭函数；
// 要求网站已启用且初始化就绪、服务器启用。
func (s *Service) files(ctx context.Context, id uint64) (*sftp.Client, func(), error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	website, err := s.website(ctx, id)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if website.Status != "enabled" || website.ProvisionStatus != "ready" {
		cancel()
		return nil, nil, errcode.Newf(errcode.ErrConflict, "网站尚未初始化或已停用")
	}
	server, err := s.server(ctx, website.ServerID)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if server.Status != "enabled" {
		cancel()
		return nil, nil, errcode.Newf(errcode.ErrConflict, "服务器已停用")
	}
	key, err := s.decrypt(website.SFTPKey)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	sshClient, closeConn, err := dial(ctx, *server, siteUser(id), "key", key)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		closeConn()
		cancel()
		return nil, nil, errcode.Newf(errcode.ErrServiceUnavailable, "无法连接受限 SFTP 账号")
	}
	closeAll := func() { _ = client.Close(); closeConn(); cancel() }
	return client, closeAll, nil
}

// ListFiles 列出指定目录内容；路径允许根目录，但逐级拒绝符号链接。
func (s *Service) ListFiles(ctx context.Context, id uint64, value string) ([]FileItem, error) {
	if _, err := filePath(value, true); err != nil {
		return nil, err
	}
	client, closeAll, err := s.files(ctx, id)
	if err != nil {
		return nil, err
	}
	defer closeAll()
	target, err := checkedPath(client, value, false, true)
	if err != nil {
		return nil, err
	}
	rows, err := client.ReadDir(target)
	if err != nil {
		return nil, fileError(err)
	}
	items := make([]FileItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, FileItem{Name: row.Name(), Size: row.Size(), Directory: row.IsDir(), Symlink: row.Mode()&os.ModeSymlink != 0, ModifiedAt: row.ModTime()})
	}
	return items, nil
}

// ReadFile 读取文件内容：text=true 时按 UTF-8 文本校验并限制到 1 MiB，
// 否则按二进制下载限制到 10 MiB；仅接受普通文件。
func (s *Service) ReadFile(ctx context.Context, id uint64, value string, text bool) ([]byte, error) {
	if _, err := filePath(value, false); err != nil {
		return nil, err
	}
	client, closeAll, err := s.files(ctx, id)
	if err != nil {
		return nil, err
	}
	defer closeAll()
	target, err := checkedPath(client, value, false, false)
	if err != nil {
		return nil, err
	}
	file, err := client.Open(target)
	if err != nil {
		return nil, fileError(err)
	}
	defer file.Close()
	limit := int64(maxFileBytes)
	if text {
		limit = maxTextBytes
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fileError(err)
	}
	if !info.Mode().IsRegular() {
		return nil, errcode.New(errcode.ErrInvalidParam)
	}
	if info.Size() > limit {
		return nil, errcode.Newf(errcode.ErrRequestTooLarge, "文件超过操作大小限制")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fileError(err)
	}
	if int64(len(data)) > limit {
		return nil, errcode.New(errcode.ErrRequestTooLarge)
	}
	if text && (!utf8.Valid(data) || strings.ContainsRune(string(data), 0)) {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "此文件不是 UTF-8 文本，请下载后编辑")
	}
	return data, nil
}

// WriteFile 写入文件：先写隐藏临时文件再原子改名，避免并发下出现半截文件；
// 落盘前二次校验目标路径，chroot 同时约束并发的链接替换。
// Write via a private temporary file followed by atomic rename; avoid partial overwrites.
func (s *Service) WriteFile(ctx context.Context, id uint64, value string, data []byte) error {
	if _, err := filePath(value, false); err != nil {
		return err
	}
	if len(data) > maxFileBytes {
		return errcode.New(errcode.ErrRequestTooLarge)
	}
	client, closeAll, err := s.files(ctx, id)
	if err != nil {
		return err
	}
	defer closeAll()
	target, err := checkedPath(client, value, true, false)
	if err != nil {
		return err
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	temp, err := client.OpenFile(path.Join(path.Dir(target), ".pm-upload-"+hex.EncodeToString(random)), os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return fileError(err)
	}
	tempName := temp.Name()
	defer client.Remove(tempName)
	if err := temp.Chmod(0640); err != nil {
		temp.Close()
		return fileError(err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fileError(err)
	}
	if err := temp.Close(); err != nil {
		return fileError(err)
	}
	// Recheck the destination. The chroot also confines any concurrent link replacement.
	if _, err := checkedPath(client, value, true, false); err != nil {
		return err
	}
	return fileError(client.PosixRename(tempName, target))
}

// MutateFile 执行目录/文件操作：mkdir 建目录并设 0750，rename 目标须不存在，delete 区分文件与目录。
func (s *Service) MutateFile(ctx context.Context, id uint64, operation, value, destination string) error {
	if _, err := filePath(value, false); err != nil {
		return err
	}
	if operation == "rename" {
		if _, err := filePath(destination, false); err != nil {
			return err
		}
	}
	client, closeAll, err := s.files(ctx, id)
	if err != nil {
		return err
	}
	defer closeAll()
	target, err := checkedPath(client, value, operation == "mkdir", false)
	if err != nil {
		return err
	}
	switch operation {
	case "mkdir":
		if err := client.Mkdir(target); err != nil {
			return fileError(err)
		}
		return fileError(client.Chmod(target, 0750))
	case "rename":
		to, err := checkedPath(client, destination, true, false)
		if err != nil {
			return err
		}
		if _, err := client.Lstat(to); err == nil {
			return errcode.Newf(errcode.ErrConflict, "目标文件已存在")
		} else if !os.IsNotExist(err) {
			return fileError(err)
		}
		return fileError(client.Rename(target, to))
	case "delete":
		info, err := client.Lstat(target)
		if err != nil {
			return fileError(err)
		}
		if info.IsDir() {
			return fileError(client.RemoveDirectory(target))
		}
		return fileError(client.Remove(target))
	default:
		return errcode.New(errcode.ErrInvalidParam)
	}
}
