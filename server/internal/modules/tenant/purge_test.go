package tenant

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/cache"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/modules/storage"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// purgeFixture 是销毁相关集成测试的共享装置：隔离的 PG schema + Redis 前缀 + 两个租户。
type purgeFixture struct {
	s        *Service
	m        *db.Manager
	sessions *session.Store
	a, b     model.Tenant
	ctx      context.Context
}

// purgeSetup 搭建隔离运行环境（需 PM_TEST_INTEGRATION=1），创建两个带独立 schema 的租户。
func purgeSetup(t *testing.T) *purgeFixture {
	t.Helper()
	if os.Getenv("PM_TEST_INTEGRATION") != "1" {
		t.Skip("设置 PM_TEST_INTEGRATION=1 启用隔离 PG/Redis 集成测试")
	}
	cfg, err := config.Load(os.Getenv("PM_TEST_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Database.LogLevel = "silent"
	admin, err := db.New(cfg.Database, zap.NewNop())
	purgeMust(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	stamp := uint64(time.Now().UnixNano())
	platformSchema := fmt.Sprintf("purge_test_%d", stamp)
	purgeMust(t, admin.Platform().Exec("CREATE SCHEMA "+platformSchema).Error)
	t.Cleanup(func() { purgeMust(t, admin.Platform().Exec("DROP SCHEMA "+platformSchema+" CASCADE").Error) })
	cfg.Database.PlatformSchema = platformSchema
	m, err := db.New(cfg.Database, zap.NewNop())
	purgeMust(t, err)
	t.Cleanup(func() { _ = m.Close() })
	purgeMust(t, m.Platform().AutoMigrate(&model.Tenant{}, &model.TenantUser{}, &model.Notification{}, &model.NotificationReceipt{}, &model.StorageAccount{}, &model.StorageResource{}, &model.StorageChange{}, &model.AuditLog{}, &model.LoginLog{}))
	cfg.Redis.KeyPrefix = fmt.Sprintf("purge_test:%d:", stamp)
	c, err := cache.New(cfg.Redis)
	purgeMust(t, err)
	t.Cleanup(func() { _ = c.Close() })
	t.Cleanup(func() { purgeMust(t, c.DelByPrefix(context.Background(), cfg.Redis.KeyPrefix)) })
	sessions := session.New(c)
	f := &purgeFixture{m: m, sessions: sessions, ctx: ctxkey.WithUser(context.Background(), 1, "test-admin", model.UserTypePlatform)}
	f.s = NewService(m, sessions, c, nil, zap.NewNop())
	f.a = model.Tenant{BaseModel: model.BaseModel{ID: stamp}, Name: "测试甲", Code: "test-a", SchemaName: db.SchemaForTenant(stamp), Status: model.StatusEnabled}
	f.b = model.Tenant{BaseModel: model.BaseModel{ID: stamp + 1}, Name: "测试乙", Code: "test-b", SchemaName: db.SchemaForTenant(stamp + 1), Status: model.StatusEnabled}
	purgeMust(t, m.Platform().Create(&f.a).Error)
	purgeMust(t, m.Platform().Create(&f.b).Error)
	for _, tenant := range []model.Tenant{f.a, f.b} {
		schema := tenant.SchemaName
		purgeMust(t, m.CreateTenantSchema(context.Background(), schema))
		t.Cleanup(func() { purgeMust(t, m.DropTenantSchema(context.Background(), schema)) })
		purgeMust(t, m.Platform().Exec("CREATE TABLE "+schema+".business_data (id bigint PRIMARY KEY, value text)").Error)
		purgeMust(t, m.Platform().Exec("INSERT INTO "+schema+".business_data VALUES (1,'isolated test data')").Error)
	}
	users := []model.TenantUser{{BaseModel: model.BaseModel{ID: 1}, TenantID: f.a.ID, Username: "owner-a", Status: model.StatusEnabled}, {BaseModel: model.BaseModel{ID: 10}, TenantID: f.b.ID, Username: "owner-b", Status: model.StatusEnabled}}
	purgeMust(t, m.Platform().Create(&users).Error)
	for _, u := range users {
		purgeMust(t, sessions.SaveSession(context.Background(), model.UserTypeTenant, u.ID, "test-session", time.Minute))
		purgeMust(t, sessions.CachePermissions(context.Background(), u.TenantID, u.ID, []string{"tenant.storage.list"}))
	}
	return f
}

// purgeMust 断言错误为空，否则终止当前测试。
func purgeMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// schemaExists 查询 PostgreSQL 是否仍存在指定 schema。
func schemaExists(t *testing.T, f *purgeFixture, schema string) bool {
	t.Helper()
	var exists bool
	purgeMust(t, f.m.Platform().Raw("SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname=?)", schema).Scan(&exists).Error)
	return exists
}

// TestImmediateDestroyAndTenantIsolation 验证立即销毁仅清理目标租户且不误伤其他租户/审计留痕。
func TestImmediateDestroyAndTenantIsolation(t *testing.T) {
	f := purgeSetup(t)
	notice := model.Notification{Title: "店内消息", Content: "正文", SenderType: model.UserTypeTenant, SenderTenantID: f.a.ID, Status: "published"}
	purgeMust(t, f.m.Platform().Create(&notice).Error)
	platformNotice := model.Notification{Title: "平台通知", Content: "正文", SenderType: model.UserTypePlatform, Status: "published"}
	purgeMust(t, f.m.Platform().Create(&platformNotice).Error)
	receipts := []model.NotificationReceipt{{NotificationID: notice.ID, UserType: model.UserTypeTenant, TenantID: f.a.ID, UserID: 1}, {NotificationID: platformNotice.ID, UserType: model.UserTypeTenant, TenantID: f.b.ID, UserID: 10}}
	purgeMust(t, f.m.Platform().Create(&receipts).Error)
	purgeMust(t, f.m.Platform().Create(&model.StorageChange{TenantID: f.a.ID, Action: "expand"}).Error)
	purgeMust(t, f.m.Platform().Create(&model.AuditLog{TenantID: f.a.ID}).Error)
	result, err := f.s.Destroy(f.ctx, f.a.ID, DestroyInput{ConfirmCode: f.a.Code})
	purgeMust(t, err)
	if result.PendingResources != 0 {
		t.Fatal("无文件租户不应有待清理资源")
	}
	if schemaExists(t, f, f.a.SchemaName) || !schemaExists(t, f, f.b.SchemaName) {
		t.Fatal("销毁 schema 范围不正确")
	}
	var count int64
	for _, entry := range []struct {
		value any
		query string
		arg   any
	}{{&model.Tenant{}, "id=?", f.a.ID}, {&model.TenantUser{}, "tenant_id=?", f.a.ID}, {&model.Notification{}, "sender_tenant_id=?", f.a.ID}, {&model.NotificationReceipt{}, "tenant_id=?", f.a.ID}, {&model.StorageChange{}, "tenant_id=?", f.a.ID}} {
		purgeMust(t, f.m.Platform().Unscoped().Model(entry.value).Where(entry.query, entry.arg).Count(&count).Error)
		if count != 0 {
			t.Fatalf("残留租户记录 %T", entry.value)
		}
	}
	alive, err := f.sessions.SessionAlive(context.Background(), model.UserTypeTenant, 1, "test-session")
	purgeMust(t, err)
	if alive {
		t.Fatal("被销毁租户会话未注销")
	}
	alive, err = f.sessions.SessionAlive(context.Background(), model.UserTypeTenant, 10, "test-session")
	purgeMust(t, err)
	if !alive {
		t.Fatal("用户 ID 前缀重叠误注销其他租户")
	}
	if _, ok := f.sessions.CachedPermissions(context.Background(), f.b.ID, 10); !ok {
		t.Fatal("误清理其他租户权限")
	}
	purgeMust(t, f.m.Platform().Model(&model.AuditLog{}).Where("tenant_id=?", f.a.ID).Count(&count).Error)
	if count != 1 {
		t.Fatal("审计留痕应保留")
	}
	_, err = f.s.Destroy(f.ctx, f.a.ID, DestroyInput{ConfirmCode: f.a.Code})
	if errcode.From(err).Code != errcode.ErrTenantNotFound {
		t.Fatal("重复销毁应返回不存在")
	}
}

// TestDestroyConfirmationAndSchemaValidation 验证编码确认、平台身份校验与 schema 归属校验。
func TestDestroyConfirmationAndSchemaValidation(t *testing.T) {
	f := purgeSetup(t)
	for _, code := range []string{"", "wrong", f.a.Code + " "} {
		_, err := f.s.Destroy(f.ctx, f.a.ID, DestroyInput{ConfirmCode: code})
		if errcode.From(err).Code != errcode.ErrInvalidParam {
			t.Fatalf("错误确认不应销毁: %v", err)
		}
	}
	ctx := ctxkey.WithUser(context.Background(), 1, "owner", model.UserTypeTenant)
	_, err := f.s.Destroy(ctx, f.a.ID, DestroyInput{ConfirmCode: f.a.Code})
	if errcode.From(err).Code != errcode.ErrForbiddenAction {
		t.Fatal("租户不得销毁")
	}
	purgeMust(t, f.m.Platform().Model(&model.Tenant{}).Where("id=?", f.a.ID).Update("schema_name", f.b.SchemaName).Error)
	_, err = f.s.Destroy(f.ctx, f.a.ID, DestroyInput{ConfirmCode: f.a.Code})
	if errcode.From(err).Code != errcode.ErrTenantSchemaBad {
		t.Fatal("拒绝租户 schema 指向另一租户")
	}
	if !schemaExists(t, f, f.a.SchemaName) || !schemaExists(t, f, f.b.SchemaName) {
		t.Fatal("验证失败时不得删除 schema")
	}
}

// TestDestroyRollsBackSchemaAndRecordsOnFailure 验证事务内任一步失败时 DROP SCHEMA 与业务记录整体回滚。
func TestDestroyRollsBackSchemaAndRecordsOnFailure(t *testing.T) {
	f := purgeSetup(t)
	purgeMust(t, f.m.Platform().Exec("DROP TABLE sys_storage_change").Error)
	_, err := f.s.Destroy(f.ctx, f.a.ID, DestroyInput{ConfirmCode: f.a.Code})
	if err == nil {
		t.Fatal("注入数据库故障应返回失败")
	}
	if !schemaExists(t, f, f.a.SchemaName) {
		t.Fatal("失败销毁未回滚 DROP SCHEMA")
	}
	var count int64
	purgeMust(t, f.m.Platform().Model(&model.Tenant{}).Where("id=?", f.a.ID).Count(&count).Error)
	if count != 1 {
		t.Fatal("失败不应删除租户")
	}
	purgeMust(t, f.m.Platform().Model(&model.TenantUser{}).Where("tenant_id=?", f.a.ID).Count(&count).Error)
	if count != 1 {
		t.Fatal("失败不应删除账号")
	}
	if _, ok := f.sessions.CachedTenant(context.Background(), f.a.ID); ok {
		t.Fatal("回滚应清理临时 deleted 快照")
	}
}

// TestScheduledPurgeRechecksDeadline 验证定时销毁在行锁内复核到期时间，跳过未到期与已恢复租户。
func TestScheduledPurgeRechecksDeadline(t *testing.T) {
	f := purgeSetup(t)
	future := time.Now().Add(time.Hour)
	purgeMust(t, f.m.Platform().Model(&model.Tenant{}).Where("id=?", f.a.ID).Updates(map[string]any{"status": model.StatusDeleted, "purge_at": future}).Error)
	_, err := f.s.purgeOne(f.ctx, f.a.ID, nil)
	if errcode.From(err).Code != errcode.ErrConflict {
		t.Fatal("未到期不得定时销毁")
	}
	result, err := f.s.Destroy(f.ctx, f.a.ID, DestroyInput{ConfirmCode: f.a.Code})
	purgeMust(t, err)
	if result.PendingResources != 0 || schemaExists(t, f, f.a.SchemaName) {
		t.Fatal("立即销毁未跳过宽限期")
	}
	purgeMust(t, f.m.Platform().Model(&model.Tenant{}).Where("id=?", f.b.ID).Updates(map[string]any{"status": model.StatusDeleted, "purge_at": time.Now().Add(-time.Minute)}).Error)
	_, err = f.s.RunPurge(f.ctx)
	purgeMust(t, err)
	if schemaExists(t, f, f.b.SchemaName) {
		t.Fatal("到期任务未复用销毁")
	}
}

// purgeObjects 是内存版对象存储桩，用于模拟上传/删除与注入删除故障。
//
// 场景：销毁发生在容量预占已提交、对象 PUT 尚未返回时，上传完成必须转入清理队列。
type purgeObjects struct {
	objects    map[string]bool
	onPut      func()
	failDelete bool
}

// Put 写入对象，并在首次写入时触发 onPut 回调（用于在途 PUT 期间触发销毁）。
func (o *purgeObjects) Put(_ context.Context, bucket, key, _, _ string, _ int64, r io.Reader) error {
	if _, err := io.ReadAll(r); err != nil {
		return err
	}
	if o.onPut != nil {
		o.onPut()
		o.onPut = nil
	}
	o.objects[bucket+"/"+key] = true
	return nil
}

// Delete 删除对象，failDelete 为真时注入删除失败以验证重试。
func (o *purgeObjects) Delete(_ context.Context, bucket, key string) error {
	if o.failDelete {
		return errors.New("injected TOS failure")
	}
	delete(o.objects, bucket+"/"+key)
	return nil
}

// Sign 是对象存储接口的空实现，测试不校验签名。
func (o *purgeObjects) Sign(context.Context, string, string, string) (string, error) { return "", nil }

// TestDestroyDuringUploadAndStorageCleanupRetry 验证在途上传期间销毁与存储清理重试的行为。
func TestDestroyDuringUploadAndStorageCleanupRetry(t *testing.T) {
	f := purgeSetup(t)
	accounts := []model.StorageAccount{{TenantID: f.a.ID, BaseBytes: 100, UsedBytes: 4}, {TenantID: f.b.ID, BaseBytes: 100, UsedBytes: 4}}
	purgeMust(t, f.m.Platform().Create(&accounts).Error)
	rows := []model.StorageResource{{TenantID: f.a.ID, Name: "ready-a.txt", SizeBytes: 4, Bucket: "test", ObjectKey: "a", Status: "ready"}, {TenantID: f.b.ID, Name: "ready-b.txt", SizeBytes: 4, Bucket: "test", ObjectKey: "b", Status: "ready"}}
	purgeMust(t, f.m.Platform().Create(&rows).Error)
	objects := &purgeObjects{objects: map[string]bool{"test/a": true, "test/b": true}}
	s := storage.NewService(f.m, objects, config.TOSConfig{Enabled: true, Bucket: "test", KeyPrefix: "isolated", MaxUploadBytes: 100})
	objects.onPut = func() {
		result, err := f.s.Destroy(f.ctx, f.a.ID, DestroyInput{ConfirmCode: f.a.Code})
		purgeMust(t, err)
		if result.PendingResources != 2 {
			t.Fatalf("应保留就绪和在途文件待清理: %+v", result)
		}
		var uploading int64
		purgeMust(t, f.m.Platform().Model(&model.StorageResource{}).Where("tenant_id=? AND status=?", f.a.ID, "uploading").Count(&uploading).Error)
		if uploading != 1 {
			t.Fatal("在途 PUT 未完成前不能清理对象")
		}
	}
	ctx := ctxkey.WithUser(context.Background(), 1, "owner-a", model.UserTypeTenant)
	ctx = ctxkey.WithTenant(ctx, f.a.ID, f.a.Code, f.a.SchemaName)
	_, err := s.Upload(ctx, "inflight.txt", "text/plain", 4, strings.NewReader("data"))
	if errcode.From(err).Code != errcode.ErrConflict {
		t.Fatalf("销毁后的在途上传不得就绪: %v", err)
	}
	var pending int64
	purgeMust(t, f.m.Platform().Model(&model.StorageResource{}).Where("tenant_id=? AND status=?", f.a.ID, "deleting").Count(&pending).Error)
	if pending != 2 {
		t.Fatal("销毁期间上传完成未进入清理队列")
	}
	objects.failDelete = true
	_, err = s.Cleanup(f.ctx)
	if err == nil {
		t.Fatal("注入对象删除故障应返回待重试")
	}
	var account model.StorageAccount
	purgeMust(t, f.m.Platform().Where("tenant_id=?", f.a.ID).First(&account).Error)
	if account.UsedBytes != 8 {
		t.Fatal("删除失败应保留容量和台账")
	}
	objects.failDelete = false
	_, err = s.Cleanup(f.ctx)
	purgeMust(t, err)
	purgeMust(t, f.m.Platform().Model(&model.StorageAccount{}).Where("tenant_id=?", f.a.ID).Count(&pending).Error)
	if pending != 0 {
		t.Fatal("最后一个对象清理成功后应删除已销毁租户存储账户")
	}
	purgeMust(t, f.m.Platform().Model(&model.StorageResource{}).Where("tenant_id=?", f.a.ID).Count(&pending).Error)
	if pending != 0 || len(objects.objects) != 1 || !objects.objects["test/b"] {
		t.Fatal("清理不应残留对象或影响其他租户")
	}
	_, err = s.Upload(ctx, "late.txt", "text/plain", 4, strings.NewReader("data"))
	if errcode.From(err).Code != errcode.ErrNotFound {
		t.Fatalf("销毁后旧上下文不得预占或上传: %v", err)
	}
}
