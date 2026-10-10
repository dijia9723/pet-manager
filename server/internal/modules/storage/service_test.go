package storage_test

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"io"
	"os"
	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/modules/notification"
	"pet-manager/server/internal/modules/storage"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeStore 是内存对象存储桩，支持注入 Put/Delete 失败以覆盖异常路径。
type fakeStore struct {
	mu                  sync.Mutex
	objects             map[string][]byte
	failPut, failDelete bool
}

func (s *fakeStore) Put(_ context.Context, bucket, key, _, _ string, size int64, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.objects[bucket+"/"+key] = body
	if s.failPut {
		return errors.New("injected upload failure")
	}
	if int64(len(body)) != size {
		return errors.New("size mismatch")
	}
	return nil
}
func (s *fakeStore) Delete(_ context.Context, bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failDelete {
		return errors.New("injected delete failure")
	}
	delete(s.objects, bucket+"/"+key)
	return nil
}
func (s *fakeStore) Sign(_ context.Context, bucket, key, _ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[bucket+"/"+key]; !ok {
		return "", errors.New("missing object")
	}
	return "https://storage.example.test/file", nil
}

// fixture 汇总用例所需的独立 schema、服务实例与预置的租户/套餐数据。
type fixture struct {
	s        *storage.Service
	m        *db.Manager
	objects  *fakeStore
	platform context.Context
	tenants  []model.Tenant
	users    []model.TenantUser
	plans    []model.DictItem
}

// setup 在随机 schema 中建表并预置字典、租户与账号，需 PM_TEST_INTEGRATION=1。
func setup(t *testing.T) *fixture {
	t.Helper()
	if os.Getenv("PM_TEST_INTEGRATION") != "1" {
		t.Skip("设置 PM_TEST_INTEGRATION=1 启用独立 schema 数据库测试")
	}
	cfg, err := config.Load(os.Getenv("PM_TEST_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Database.LogLevel = "silent"
	admin, err := db.New(cfg.Database, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("storage_test_%d", time.Now().UnixNano())
	if err := admin.Platform().Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Platform().Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	cfg.Database.PlatformSchema = schema
	m, err := db.New(cfg.Database, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Platform().AutoMigrate(&model.StorageAccount{}, &model.StorageResource{}, &model.StorageChange{}, &model.Dict{}, &model.DictItem{}, &model.Tenant{}, &model.TenantUser{}, &model.Notification{}, &model.NotificationReceipt{}); err != nil {
		t.Fatal(err)
	}
	if err := m.Platform().Exec("CREATE UNIQUE INDEX uk_test_storage_account ON sys_storage_account (tenant_id) WHERE deleted_at IS NULL").Error; err != nil {
		t.Fatal(err)
	}
	f := &fixture{m: m, objects: &fakeStore{objects: map[string][]byte{}}, platform: ctxkey.WithUser(context.Background(), 1, "admin", model.UserTypePlatform)}
	f.s = storage.NewService(m, f.objects, config.TOSConfig{Enabled: true, Bucket: "test-bucket", KeyPrefix: "isolated-tests", MaxUploadBytes: 100})
	d := model.Dict{Code: "storage.plan", Name: "套餐", Status: "enabled"}
	must(t, m.Platform().Create(&d).Error)
	f.plans = []model.DictItem{{DictID: d.ID, Label: "基础", Value: "basic", Sort: 10, Status: "enabled", StorageBytes: 10}, {DictID: d.ID, Label: "标准", Value: "standard", Sort: 20, Status: "enabled", StorageBytes: 30}, {DictID: d.ID, Label: "停用", Value: "disabled", Sort: 30, Status: "disabled", StorageBytes: 50}}
	must(t, m.Platform().Create(&f.plans).Error)
	f.tenants = []model.Tenant{{Name: "甲", Code: "a", SchemaName: "tenant_1", Status: "enabled"}, {Name: "乙", Code: "b", SchemaName: "tenant_2", Status: "enabled"}}
	must(t, m.Platform().Create(&f.tenants).Error)
	f.users = []model.TenantUser{{TenantID: f.tenants[0].ID, Username: "a", Status: "enabled"}, {TenantID: f.tenants[1].ID, Username: "b", Status: "enabled"}}
	must(t, m.Platform().Create(&f.users).Error)
	return f
}

// must 在非 nil 错误时终止用例。
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// tenant 在真实租户事务中构造租户上下文执行 fn，模拟带 search_path 的请求。
func (f *fixture) tenant(t *testing.T, index int, fn func(context.Context)) {
	t.Helper()
	u := f.users[index]
	must(t, f.m.TenantTx(context.Background(), fmt.Sprintf("tenant_%d", u.TenantID), func(tx *gorm.DB) error {
		ctx := ctxkey.WithUser(context.Background(), u.ID, u.Username, model.UserTypeTenant)
		ctx = ctxkey.WithTenant(ctx, u.TenantID, "test", fmt.Sprintf("tenant_%d", u.TenantID))
		ctx = ctxkey.WithTenantDB(ctx, tx)
		fn(ctx)
		return nil
	}))
}

// TestStorageQuotaIsolationAndRecovery 覆盖配额预占/释放、跨租户越权拦截、
// 套餐分配与扩容、删除失败恢复、清理重试以及失败上传的容量回收。
func TestStorageQuotaIsolationAndRecovery(t *testing.T) {
	f := setup(t)
	var r *model.StorageResource
	f.tenant(t, 0, func(ctx context.Context) {
		summary, err := f.s.Summary(ctx)
		must(t, err)
		if summary.QuotaBytes != 10 || summary.UsedBytes != 0 {
			t.Fatal("默认套餐错误")
		}
		r, err = f.s.Upload(ctx, "../合同.txt", "text/plain", 6, strings.NewReader("abcdef"))
		must(t, err)
		if r.Name != "合同.txt" {
			t.Fatal("文件名未规范化")
		}
		_, err = f.s.Upload(ctx, "large", "text/plain", 6, strings.NewReader("abcdef"))
		if errcode.From(err).Code != errcode.ErrConflict {
			t.Fatalf("应拒绝超额: %v", err)
		}
		_, err = f.s.Upload(ctx, "too-large", "text/plain", 101, strings.NewReader("abcdef"))
		if errcode.From(err).Code != errcode.ErrRequestTooLarge {
			t.Fatalf("应拒绝超限文件: %v", err)
		}
	})
	f.tenant(t, 1, func(ctx context.Context) {
		target := f.tenants[0].ID
		rows, err := f.s.List(ctx, storage.ListQuery{TenantID: &target})
		must(t, err)
		if rows.Total != 0 {
			t.Fatal("伪造 tenant_id 泄漏其他租户资源")
		}
		_, err = f.s.Download(ctx, r.ID)
		if errcode.From(err).Code != errcode.ErrNotFound {
			t.Fatalf("跨租户下载未拒绝: %v", err)
		}
		if err := f.s.Delete(ctx, r.ID); errcode.From(err).Code != errcode.ErrNotFound {
			t.Fatalf("跨租户删除未拒绝: %v", err)
		}
		_, err = f.s.Allocate(ctx, f.tenants[0].ID, storage.AllocationInput{Action: "expand", AddBytes: 10})
		if errcode.From(err).Code != errcode.ErrForbiddenAction {
			t.Fatal("租户不得自扩容")
		}
	})
	_, err := f.s.Allocate(f.platform, f.tenants[0].ID, storage.AllocationInput{Action: "assign", PlanID: f.plans[2].ID})
	if errcode.From(err).Code != errcode.ErrInvalidParam {
		t.Fatal("不得分配停用套餐")
	}
	summary, err := f.s.Allocate(f.platform, f.tenants[0].ID, storage.AllocationInput{Action: "expand", AddBytes: 5})
	must(t, err)
	if summary.QuotaBytes != 15 {
		t.Fatal("扩容错误")
	}
	summary, err = f.s.Allocate(f.platform, f.tenants[0].ID, storage.AllocationInput{Action: "assign", PlanID: f.plans[1].ID})
	must(t, err)
	if summary.QuotaBytes != 35 || summary.ExtraBytes != 5 {
		t.Fatal("升级未保留扩容")
	}
	f.objects.failDelete = true
	f.tenant(t, 0, func(ctx context.Context) {
		if err := f.s.Delete(ctx, r.ID); errcode.From(err).Code != errcode.ErrServiceUnavailable {
			t.Fatal("故障应保留待清理记录")
		}
		summary, err := f.s.Summary(ctx)
		must(t, err)
		if summary.UsedBytes != 6 {
			t.Fatal("删除失败不得释放容量")
		}
		page, err := f.s.List(ctx, storage.ListQuery{})
		must(t, err)
		if page.Total != 1 || page.List[0].Status != "deleting" {
			t.Fatal("待清理记录未持久化")
		}
	})
	f.objects.failDelete = false
	_, err = f.s.Cleanup(context.Background())
	must(t, err)
	f.tenant(t, 0, func(ctx context.Context) {
		summary, err := f.s.Summary(ctx)
		must(t, err)
		if summary.UsedBytes != 0 {
			t.Fatal("清理后未释放容量")
		}
		f.objects.failPut = true
		_, err = f.s.Upload(ctx, "fail.txt", "text/plain", 4, strings.NewReader("fail"))
		if errcode.From(err).Code != errcode.ErrServiceUnavailable {
			t.Fatal("应返回上传失败")
		}
		f.objects.failPut = false
		summary, err = f.s.Summary(ctx)
		must(t, err)
		if summary.UsedBytes != 0 {
			t.Fatal("失败上传容量未回收")
		}
	})
	changes, err := f.s.Changes(f.platform, f.tenants[0].ID, storage.ListQuery{})
	must(t, err)
	if changes.Total != 2 {
		t.Fatal("配额变更未记录")
	}
	if len(f.objects.objects) != 0 {
		t.Fatal("失败和删除文件残留")
	}
}

// TestConcurrentUploadsCannotExceedQuota 验证连接池仅 1 个连接时并发上传也不会超额。
func TestConcurrentUploadsCannotExceedQuota(t *testing.T) {
	f := setup(t)
	pool, err := f.m.Platform().DB()
	must(t, err)
	pool.SetMaxOpenConns(1) // 短事务模式即使仅一个连接，也不能等第二个连接而耗尽池。
	ctxBase, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u := f.users[0]
			ctx := ctxkey.WithUser(ctxBase, u.ID, u.Username, model.UserTypeTenant)
			ctx = ctxkey.WithTenant(ctx, u.TenantID, "test", "tenant_1")
			_, err := f.s.Upload(ctx, "concurrent.txt", "text/plain", 6, strings.NewReader("abcdef"))
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errcode.From(err).Code == errcode.ErrConflict {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("并发超额: success=%d conflict=%d", success, conflict)
	}
	ctx := ctxkey.WithUser(ctxBase, f.users[0].ID, "owner", model.UserTypeTenant)
	ctx = ctxkey.WithTenant(ctx, f.tenants[0].ID, "test", "tenant_1")
	summary, err := f.s.Summary(ctx)
	must(t, err)
	if summary.UsedBytes != 6 {
		t.Fatalf("占用错误 %d", summary.UsedBytes)
	}
}

// TestNotificationAttachmentsAuthorization 覆盖附件去重、发布/撤回与跨账号的下载鉴权边界。
func TestNotificationAttachmentsAuthorization(t *testing.T) {
	f := setup(t)
	nsvc := notification.NewService(f.m, f.s)
	file, err := f.s.Upload(f.platform, "平台通知.txt", "text/plain", 5, strings.NewReader("hello"))
	must(t, err)
	input := notification.Input{Title: "附件测试", Content: "正文", Category: "announcement", Priority: "normal", TargetType: "tenants", TargetIDs: []uint64{f.tenants[0].ID}, AttachmentIDs: []uint64{file.ID, file.ID}}
	n, err := nsvc.Save(f.platform, 0, input)
	must(t, err)
	if len(n.Attachments) != 1 {
		t.Fatal("附件未去重")
	}
	must(t, nsvc.Publish(f.platform, n.ID))
	f.tenant(t, 0, func(ctx context.Context) {
		page, err := nsvc.Inbox(ctx, notification.ListQuery{})
		must(t, err)
		if page.Total != 1 || len(page.List[0].Attachments) != 1 {
			t.Fatal("收件箱附件元数据丢失")
		}
		_, err = nsvc.DownloadAttachment(ctx, n.ID, file.ID, false)
		must(t, err)
		_, err = f.s.Download(ctx, file.ID)
		if errcode.From(err).Code != errcode.ErrNotFound {
			t.Fatal("消息授权不得开放直接资源访问")
		}
		_, err = nsvc.DownloadAttachment(ctx, n.ID, file.ID+1, false)
		if errcode.From(err).Code != errcode.ErrNotFound {
			t.Fatal("不得下载非本消息附件")
		}
		input.TargetType = "own_tenant"
		input.TargetIDs = nil
		_, err = nsvc.Save(ctx, 0, input)
		if errcode.From(err).Code != errcode.ErrInvalidParam {
			t.Fatal("租户不得引用平台资源作为自己附件")
		}
	})
	f.tenant(t, 1, func(ctx context.Context) {
		_, err = nsvc.DownloadAttachment(ctx, n.ID, file.ID, false)
		if errcode.From(err).Code != errcode.ErrNotFound {
			t.Fatal("非接收账号不得下载附件")
		}
	})
	must(t, nsvc.Revoke(f.platform, n.ID))
	f.tenant(t, 0, func(ctx context.Context) {
		_, err = nsvc.DownloadAttachment(ctx, n.ID, file.ID, false)
		if errcode.From(err).Code != errcode.ErrNotFound {
			t.Fatal("撤回后不得再签发附件 URL")
		}
	})
	input.TargetType = "tenants"
	input.TargetIDs = []uint64{f.tenants[0].ID}
	draft, err := nsvc.Save(f.platform, 0, input)
	must(t, err)
	must(t, f.s.Delete(f.platform, file.ID))
	if err := nsvc.Publish(f.platform, draft.ID); errcode.From(err).Code != errcode.ErrInvalidParam {
		t.Fatal("删除附件后不得发布旧草稿")
	}
	_, err = nsvc.DownloadAttachment(f.platform, n.ID, file.ID, true)
	if errcode.From(err).Code != errcode.ErrNotFound {
		t.Fatal("文件删除后不可下载")
	}
	var tenantN *model.Notification
	f.tenant(t, 0, func(ctx context.Context) {
		r, err := f.s.Upload(ctx, "店内附件.txt", "text/plain", 4, strings.NewReader("test"))
		must(t, err)
		input.TargetType = "own_tenant"
		input.TargetIDs = nil
		input.AttachmentIDs = []uint64{r.ID}
		tenantN, err = nsvc.Save(ctx, 0, input)
		must(t, err)
		must(t, nsvc.Publish(ctx, tenantN.ID))
		_, err = nsvc.DownloadAttachment(ctx, tenantN.ID, r.ID, true)
		must(t, err)
	})
	f.tenant(t, 0, func(ctx context.Context) {
		_, err := nsvc.DownloadAttachment(ctx, tenantN.ID, tenantN.Attachments[0].ResourceID, false)
		must(t, err)
	})
}

// TestStorageUploadValidationWithoutDatabase 验证无数据库时文件名与大小校验仍生效。
func TestStorageUploadValidationWithoutDatabase(t *testing.T) {
	s := storage.NewService(nil, &fakeStore{}, config.TOSConfig{MaxUploadBytes: 5})
	for _, name := range []string{"", "bad\rname", "/"} {
		_, err := s.Upload(context.Background(), name, "text/plain", 1, strings.NewReader("x"))
		if errcode.From(err).Code != errcode.ErrInvalidParam {
			t.Fatalf("无效文件名 %q 未拦截", name)
		}
	}
	_, err := s.Upload(context.Background(), "large", "text/plain", 6, strings.NewReader("x"))
	if errcode.From(err).Code != errcode.ErrRequestTooLarge {
		t.Fatal("未拦截超限")
	}
}
