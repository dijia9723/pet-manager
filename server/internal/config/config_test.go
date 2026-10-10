package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTOSConfigEnvironmentOverrides 校验 TOS 相关配置项可由 PM_TOS_* 环境变量覆盖，且未覆盖项取默认值。
func TestTOSConfigEnvironmentOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  env: test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PM_TOS_ENABLED", "true")
	t.Setenv("PM_TOS_ACCESS_KEY", "test-access")
	t.Setenv("PM_TOS_SECRET_KEY", "test-secret")
	t.Setenv("PM_TOS_BUCKET", "test-bucket")
	t.Setenv("PM_TOS_PRESIGN_TTL", "20m")
	t.Setenv("PM_TOS_MAX_UPLOAD_BYTES", "1234")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TOS.Enabled || cfg.TOS.AccessKey != "test-access" || cfg.TOS.SecretKey != "test-secret" || cfg.TOS.Bucket != "test-bucket" || cfg.TOS.PresignTTL != 20*time.Minute || cfg.TOS.MaxUploadBytes != 1234 || cfg.TOS.KeyPrefix != "apps/" {
		t.Fatal("TOS 环境变量或默认值未注册")
	}
}
