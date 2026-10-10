package objectstore

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"os"
	"pet-manager/server/internal/config"
	"strings"
	"testing"
	"time"
)

// 覆盖配置门禁：停用可空跑、启用必须配置完整，presign_ttl 上限为 7 天。
func TestConfigValidation(t *testing.T) {
	if s, err := New(config.TOSConfig{}); err != nil || s != nil {
		t.Fatal("停用配置应允许启动")
	}
	if _, err := New(config.TOSConfig{Enabled: true}); err == nil {
		t.Fatal("启用时必须完整配置")
	}
	cfg := config.TOSConfig{Enabled: true, Endpoint: "tos-cn-shanghai.volces.com", Region: "cn-shanghai", Bucket: "test", AccessKey: "test", SecretKey: "test", PresignTTL: 8 * 24 * time.Hour, MaxUploadBytes: 100}
	if _, err := New(cfg); err == nil {
		t.Fatal("不得超过七天")
	}
	cfg.PresignTTL = time.Minute
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.Sign(context.Background(), "test", "private/file", "合同.txt")
	if err != nil || !strings.HasPrefix(u, "https://") || !strings.Contains(u, "response-content-disposition") {
		t.Fatal("签名参数错误")
	}
}

// 仅对随机且由本测试新建的对象操作，不列举、删除桶内现有文件。
func TestLiveTOSRoundTrip(t *testing.T) {
	if os.Getenv("PM_TEST_TOS") != "1" {
		t.Skip("设置 PM_TEST_TOS=1 启用配置桶内独立对象测试")
	}
	cfg, err := config.Load(os.Getenv("PM_TEST_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cfg.TOS)
	if err != nil || s == nil {
		t.Fatal("TOS 未启用或配置无效")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	key := strings.Trim(cfg.TOS.KeyPrefix, "/") + "/integration-tests/" + rand.Text()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Delete(cleanupCtx, cfg.TOS.Bucket, key); err != nil {
			t.Error("清理测试对象失败")
		}
	})
	const content = "pet-manager isolated TOS integration test"
	if err := s.Put(ctx, cfg.TOS.Bucket, key, "测试附件.txt", "text/plain", int64(len(content)), strings.NewReader(content)); err != nil {
		t.Fatal("TOS 上传失败:", err)
	}
	u, err := s.Sign(ctx, cfg.TOS.Bucket, key, "测试附件.txt")
	if err != nil {
		t.Fatal("签名失败")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("TOS 下载失败")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != 200 || string(body) != content {
		t.Fatalf("TOS 内容校验失败，HTTP %d", res.StatusCode)
	}
}
