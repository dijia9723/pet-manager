// Package objectstore 封装平台 TOS，避免业务模块接触访问凭证。
package objectstore

import (
	"context"
	"fmt"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"
	"io"
	"mime"
	"net/url"
	"pet-manager/server/internal/config"
	"strings"
	"time"
)

// Store 是对象存储的最小能力面：上传、删除、生成下载签名。
// 参数依次为 (ctx, bucket, key, 文件名, contentType, 大小, 内容)。
type Store interface {
	Put(context.Context, string, string, string, string, int64, io.Reader) error
	Delete(context.Context, string, string) error
	Sign(context.Context, string, string, string) (string, error)
}

// TOS 是基于火山引擎 TOS 的 Store 实现。
type TOS struct {
	client *tos.ClientV2 // TOS 客户端
	ttl    time.Duration // 下载签名 URL 有效期
}

// New 校验配置并初始化 TOS 客户端；未启用时返回 (nil, nil)，
// 调用方据此得知对象存储不可用，但不应视为启动失败。
func New(cfg config.TOSConfig) (*TOS, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.Endpoint == "" || cfg.Region == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("TOS 已启用，但 endpoint/region/access_key/secret_key/bucket 未完整配置")
	}
	if cfg.PresignTTL < time.Second || cfg.PresignTTL > 7*24*time.Hour {
		return nil, fmt.Errorf("TOS presign_ttl 必须为 1 秒到 7 天")
	}
	if cfg.MaxUploadBytes <= 0 || cfg.MaxUploadBytes > 1<<30 {
		return nil, fmt.Errorf("TOS max_upload_bytes 必须为 1 字节到 1 GiB")
	}
	endpoint := cfg.Endpoint
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("TOS endpoint 无效")
	}
	client, err := tos.NewClientV2(endpoint, tos.WithRegion(cfg.Region), tos.WithCredentials(tos.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey)), tos.WithRequestTimeout(2*time.Minute))
	if err != nil {
		return nil, fmt.Errorf("初始化 TOS 客户端失败")
	}
	return &TOS{client: client, ttl: cfg.PresignTTL}, nil
}

// Put 上传对象。对象默认私有（ACLPrivate），下载只能凭签名 URL。
func (s *TOS) Put(ctx context.Context, bucket, key, name, contentType string, size int64, body io.Reader) error {
	_, err := s.client.PutObjectV2(ctx, &tos.PutObjectV2Input{PutObjectBasicInput: tos.PutObjectBasicInput{Bucket: bucket, Key: key, ContentLength: size, ContentType: contentType, ContentDisposition: disposition(name), ACL: enum.ACLPrivate}, Content: body})
	return err
}

// Delete 删除对象。
func (s *TOS) Delete(ctx context.Context, bucket, key string) error {
	_, err := s.client.DeleteObjectV2(ctx, &tos.DeleteObjectV2Input{Bucket: bucket, Key: key})
	return err
}

// disposition 把文件名编码为 attachment 的 Content-Disposition，触发浏览器下载。
func disposition(name string) string {
	return mime.FormatMediaType("attachment", map[string]string{"filename": name})
}

// Sign 生成带时效的 GET 下载签名 URL，并强制以附件方式下载、指定原始文件名。
func (s *TOS) Sign(ctx context.Context, bucket, key, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r, err := s.client.PreSignedURL(&tos.PreSignedURLInput{HTTPMethod: enum.HttpMethodGet, Bucket: bucket, Key: key, Expires: int64(s.ttl.Seconds()), Query: map[string]string{"response-content-disposition": disposition(name), "response-content-type": "application/octet-stream"}})
	if err != nil {
		return "", err
	}
	// 下载始终使用有时效的签名 URL；public_base_url 不用于消息私有附件。
	if !strings.HasPrefix(r.SignedUrl, "http") {
		return "", fmt.Errorf("无效的 TOS 下载 URL")
	}
	return r.SignedUrl, nil
}
