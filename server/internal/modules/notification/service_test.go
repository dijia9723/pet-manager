package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
)

// 此测试仅在独立、随机 schema 中建表，不迁移或修改现有业务表。
func testService(t *testing.T) (*Service, *db.Manager) {
	t.Helper()
	if os.Getenv("PM_TEST_INTEGRATION") != "1" {
		t.Skip("设置 PM_TEST_INTEGRATION=1 和可选 PM_TEST_CONFIG 启用数据库集成测试")
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
	schema := fmt.Sprintf("notification_test_%d", time.Now().UnixNano())
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
	if err := m.Platform().AutoMigrate(&model.Notification{}, &model.NotificationReceipt{}, &model.Tenant{}, &model.TenantUser{}, &model.PlatformUser{}); err != nil {
		t.Fatal(err)
	}
	if err := m.Platform().Exec("CREATE UNIQUE INDEX uk_test_receipt ON sys_notification_receipt (notification_id,user_type,tenant_id,user_id) WHERE deleted_at IS NULL").Error; err != nil {
		t.Fatal(err)
	}
	return NewService(m), m
}

// TestNotificationLifecycleAndIsolation 覆盖草稿→发布→回执→撤回全流程，
// 以及平台/租户、跨租户账号之间的消息隔离与越权拦截。
func TestNotificationLifecycleAndIsolation(t *testing.T) {
	s, m := testService(t)
	tx := m.Platform()
	tenants := []model.Tenant{{Name: "甲", Code: "a", SchemaName: "tenant_1", Status: "enabled"}, {Name: "乙", Code: "b", SchemaName: "tenant_2", Status: "enabled"}, {Name: "停用", Code: "c", SchemaName: "tenant_3", Status: "disabled"}}
	past := time.Now().Add(-time.Hour)
	tenants = append(tenants, model.Tenant{Name: "到期", Code: "d", SchemaName: "tenant_4", Status: "enabled", ExpireAt: &past})
	if err := tx.Create(&tenants).Error; err != nil {
		t.Fatal(err)
	}
	users := []model.TenantUser{{TenantID: tenants[0].ID, Username: "owner", Status: "enabled"}, {TenantID: tenants[0].ID, Username: "staff", Status: "enabled"}, {TenantID: tenants[1].ID, Username: "other", Status: "enabled"}, {TenantID: tenants[0].ID, Username: "disabled", Status: "disabled"}, {TenantID: tenants[2].ID, Username: "disabled_tenant", Status: "enabled"}, {TenantID: tenants[3].ID, Username: "expired", Status: "enabled"}}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	p := model.PlatformUser{Username: "platform", Status: "enabled"}
	if err := tx.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	platform := ctxkey.WithUser(context.Background(), p.ID, p.Username, model.UserTypePlatform)
	input := Input{Title: " 测试通知 ", Content: "请完成检查", Category: "business", Priority: "urgent", RequireProcess: true, TargetType: "tenants", TargetIDs: []uint64{tenants[0].ID, tenants[0].ID}}
	n, err := s.Save(platform, 0, input)
	if err != nil {
		t.Fatal(err)
	}
	if n.Title != "测试通知" || len(n.TargetIDs) != 1 {
		t.Fatal("未规范化输入")
	}
	if err := s.Publish(platform, n.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(platform, n.ID); errcode.From(err).Code != errcode.ErrConflict {
		t.Fatalf("重复发布应该冲突: %v", err)
	}
	if _, err := s.Save(platform, n.ID, input); errcode.From(err).Code != errcode.ErrConflict {
		t.Fatalf("已发布通知不应允许编辑: %v", err)
	}
	receipts, err := s.Receipts(platform, n.ID, ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if receipts.Total != 2 {
		t.Fatalf("只应下发给指定租户的两名启用账号: %d", receipts.Total)
	}
	withTenant := func(u model.TenantUser, fn func(context.Context)) {
		err := tx.Transaction(func(requestTx *gorm.DB) error {
			// 临时同名表优先于普通 search_path；收件箱仍必须读取显式限定的平台表。
			for _, table := range []string{"sys_notification", "sys_notification_receipt"} {
				if err := requestTx.Exec("CREATE TEMP TABLE " + table + " (LIKE " + s.table(table) + ") ON COMMIT DROP").Error; err != nil {
					return err
				}
			}
			ctx := ctxkey.WithUser(context.Background(), u.ID, u.Username, model.UserTypeTenant)
			ctx = ctxkey.WithTenant(ctx, u.TenantID, "test", "tenant_1")
			ctx = ctxkey.WithTenantDB(ctx, requestTx)
			fn(ctx)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	feedback := "已完成"
	withTenant(users[2], func(ctx context.Context) {
		page, err := s.Inbox(ctx, ListQuery{})
		if err != nil || page.Total != 0 {
			t.Fatalf("跨租户收件箱泄漏: %v", err)
		}
		if err := s.Acknowledge(ctx, n.ID, &feedback); errcode.From(err).Code != errcode.ErrNotFound {
			t.Fatalf("不应能处理他人通知: %v", err)
		}
	})
	if count, err := s.Unread(platform); err != nil || count != 0 {
		t.Fatalf("平台与租户相同数字 ID 不应共享消息: %d %v", count, err)
	}
	withTenant(users[0], func(ctx context.Context) {
		count, err := s.Unread(ctx)
		if err != nil || count != 1 {
			t.Fatalf("未读数: %d %v", count, err)
		}
		if err := s.Acknowledge(ctx, n.ID, nil); err != nil {
			t.Fatal(err)
		}
		if err := s.Acknowledge(ctx, n.ID, nil); err != nil {
			t.Fatal(err)
		}
		if err := s.Acknowledge(ctx, n.ID, &feedback); err != nil {
			t.Fatal(err)
		}
		if err := s.Acknowledge(ctx, n.ID, &feedback); errcode.From(err).Code != errcode.ErrConflict {
			t.Fatalf("重复处理应冲突: %v", err)
		}
		page, err := s.Inbox(ctx, ListQuery{State: "processed"})
		if err != nil || page.Total != 1 || page.List[0].Feedback != feedback {
			t.Fatalf("处理结果未保存: %v", err)
		}
	})
	withTenant(users[1], func(ctx context.Context) {
		count, err := s.Unread(ctx)
		if err != nil || count != 1 {
			t.Fatalf("同租户账号的已读状态应独立: %d %v", count, err)
		}
	})
	list, err := s.List(platform, ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if list.List[0].ReadCount != 1 || list.List[0].ProcessedCount != 1 {
		t.Fatalf("统计错误: %+v", list.List[0])
	}
	if err := s.Revoke(platform, n.ID); err != nil {
		t.Fatal(err)
	}
	withTenant(users[1], func(ctx context.Context) {
		count, err := s.Unread(ctx)
		if err != nil || count != 0 {
			t.Fatalf("撤回通知不应计入未读: %d %v", count, err)
		}
		page, err := s.Inbox(ctx, ListQuery{})
		if err != nil || page.Total != 0 {
			t.Fatalf("撤回通知不应可见: %v", err)
		}
		if err := s.Acknowledge(ctx, n.ID, nil); errcode.From(err).Code != errcode.ErrNotFound {
			t.Fatalf("不应能读取已撤回通知: %v", err)
		}
	})
	receipts, err = s.Receipts(platform, n.ID, ListQuery{})
	if err != nil || receipts.Total != 2 {
		t.Fatalf("撤回后应保留回执: %v", err)
	}

	// 无效目标整体回滚；全部租户排除停用和到期租户及停用账号。
	input.TargetType, input.TargetIDs = "tenant_users", []uint64{users[0].ID, users[3].ID}
	n, err = s.Save(platform, 0, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(platform, n.ID); errcode.From(err).Code != errcode.ErrInvalidParam {
		t.Fatalf("无效目标应阻止发布: %v", err)
	}
	receipts, err = s.Receipts(platform, n.ID, ListQuery{})
	if err != nil || receipts.Total != 0 {
		t.Fatalf("失败发布应无残留回执: %v", err)
	}
	input.TargetType, input.TargetIDs = "all_tenants", nil
	n, err = s.Save(platform, 0, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(platform, n.ID); err != nil {
		t.Fatal(err)
	}
	receipts, err = s.Receipts(platform, n.ID, ListQuery{})
	if err != nil || receipts.Total != 3 {
		t.Fatalf("全部租户应有三名有效接收人: %d %v", receipts.Total, err)
	}
	input.TargetType, input.TargetIDs = "platform_users", []uint64{p.ID}
	input.RequireProcess = false
	n, err = s.Save(platform, 0, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(platform, n.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Acknowledge(platform, n.ID, &feedback); errcode.From(err).Code != errcode.ErrConflict {
		t.Fatalf("普通通知不应能提交处理结果: %v", err)
	}
	if err := s.Acknowledge(platform, n.ID, nil); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"tenants", "tenant_users", "platform_users"} {
		options, err := s.Targets(platform, TargetQuery{Type: typ})
		if err != nil || options.Total == 0 {
			t.Fatalf("接收对象列表 %s: %v", typ, err)
		}
	}
	options, err := s.Targets(platform, TargetQuery{Type: "tenant_users", Keyword: "staff"})
	if err != nil || options.Total != 1 {
		t.Fatalf("目标搜索失败: %v", err)
	}
}

// TestConcurrentPublish 验证并发发布只有一次成功，回执不重复。
func TestConcurrentPublish(t *testing.T) {
	s, m := testService(t)
	p := model.PlatformUser{Username: "sender", Status: "enabled"}
	if err := m.Platform().Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	ctx := ctxkey.WithUser(context.Background(), p.ID, p.Username, model.UserTypePlatform)
	n, err := s.Save(ctx, 0, Input{Title: "并发发布", Content: "内容", TargetType: "platform_users", TargetIDs: []uint64{p.ID}, Category: "announcement", Priority: "normal"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.Publish(ctx, n.ID) }()
	}
	wg.Wait()
	close(results)
	ok, conflict := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errcode.From(err).Code == errcode.ErrConflict {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("成功/冲突=%d/%d", ok, conflict)
	}
	page, err := s.Receipts(ctx, n.ID, ListQuery{})
	if err != nil || page.Total != 1 {
		t.Fatalf("并发发布产生重复回执: %v", err)
	}
}

// TestInputValidation 覆盖标题/内容为空、缺少接收对象与非法目标 ID 的入参校验。
func TestInputValidation(t *testing.T) {
	for _, input := range []Input{{Title: " ", Content: "内容", TargetType: "all_tenants"}, {Title: "标题", Content: " ", TargetType: "all_tenants"}, {Title: "标题", Content: "内容", TargetType: "tenants"}, {Title: "标题", Content: "内容", TargetType: "tenant_users", TargetIDs: []uint64{0}}} {
		if err := validateInput(&input); err == nil {
			t.Errorf("应拒绝无效输入: %+v", input)
		}
	}
	input := Input{Title: "标题", Content: strings.Repeat("文", 10), TargetType: "all_tenants", TargetIDs: []uint64{7}}
	if err := validateInput(&input); err != nil || len(input.TargetIDs) != 0 {
		t.Fatal("全部租户应清除显式 ID")
	}
}

// TestTenantPublishingAndAuthorization 覆盖租户店内发布的接收范围限制、
// 伪造发送方字段、坏草稿重校验与他租户越权拒绝。
func TestTenantPublishingAndAuthorization(t *testing.T) {
	s, m := testService(t)
	tx := m.Platform()
	tenants := []model.Tenant{{Name: "甲店", Code: "a", SchemaName: "tenant_1", Status: "enabled"}, {Name: "乙店", Code: "b", SchemaName: "tenant_2", Status: "enabled"}}
	if err := tx.Create(&tenants).Error; err != nil {
		t.Fatal(err)
	}
	users := []model.TenantUser{{TenantID: tenants[0].ID, Username: "owner-a", Status: "enabled"}, {TenantID: tenants[0].ID, Username: "staff-a", Status: "enabled"}, {TenantID: tenants[1].ID, Username: "owner-b", Status: "enabled"}, {TenantID: tenants[0].ID, Username: "disabled-a", Status: "disabled"}}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	p := model.PlatformUser{Username: "platform", Status: "enabled"}
	if err := tx.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	platform := ctxkey.WithUser(context.Background(), p.ID, p.Username, model.UserTypePlatform)
	platformNotice, err := s.Save(platform, 0, Input{Title: "平台通知", Content: "内容", Category: "announcement", Priority: "normal", TargetType: "all_tenants"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(platform, platformNotice.ID); err != nil {
		t.Fatal(err)
	}
	withTenant := func(u model.TenantUser, fn func(context.Context)) {
		t.Helper()
		err := tx.Transaction(func(requestTx *gorm.DB) error {
			for _, table := range []string{"sys_notification", "sys_notification_receipt", "sys_tenant_user", "sys_tenant"} {
				if err := requestTx.Exec("CREATE TEMP TABLE " + table + " (LIKE " + s.table(table) + ") ON COMMIT DROP").Error; err != nil {
					return err
				}
			}
			ctx := ctxkey.WithUser(context.Background(), u.ID, u.Username, model.UserTypeTenant)
			ctx = ctxkey.WithTenant(ctx, u.TenantID, "test", "tenant_1")
			ctx = ctxkey.WithTenantDB(ctx, requestTx)
			fn(ctx)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	input := Input{Title: "店内检查", Content: "请完成检查", Category: "business", Priority: "normal", RequireProcess: true, TargetType: "own_tenant"}
	var ownID, selectedID, corruptedID uint64
	withTenant(users[0], func(ctx context.Context) {
		options, err := s.Targets(ctx, TargetQuery{Type: "tenant_users"})
		if err != nil || options.Total != 2 {
			t.Fatalf("只能列出本租户两个启用账号: %+v %v", options, err)
		}
		for _, option := range options.List {
			if option.ID == users[2].ID || option.ID == users[3].ID {
				t.Fatal("目标列表泄漏其他租户或停用账号")
			}
		}
		options, err = s.Targets(ctx, TargetQuery{Type: "tenant_users", Keyword: "owner-b"})
		if err != nil || options.Total != 0 {
			t.Fatal("搜索泄漏其他租户账号")
		}
		for _, typ := range []string{"tenants", "platform_users"} {
			if _, err := s.Targets(ctx, TargetQuery{Type: typ}); errcode.From(err).Code != errcode.ErrForbiddenAction {
				t.Fatalf("租户不应能搜索 %s: %v", typ, err)
			}
		}
		for _, typ := range []string{"all_tenants", "tenants", "platform_users"} {
			bad := input
			bad.TargetType, bad.TargetIDs = typ, []uint64{tenants[0].ID}
			if _, err := s.Save(ctx, 0, bad); errcode.From(err).Code != errcode.ErrForbiddenAction {
				t.Fatalf("应拒绝租户跨范围发送 %s: %v", typ, err)
			}
		}
		for _, ids := range [][]uint64{{users[2].ID}, {users[0].ID, users[2].ID}, {users[3].ID}, {999999}} {
			bad := input
			bad.TargetType, bad.TargetIDs = "tenant_users", ids
			if _, err := s.Save(ctx, 0, bad); errcode.From(err).Code != errcode.ErrForbiddenAction {
				t.Fatalf("应拒绝外部或不可用接收账号 %v: %v", ids, err)
			}
		}
		n, err := s.Save(ctx, 0, input)
		if err != nil {
			t.Fatal(err)
		}
		ownID = n.ID
		if n.SenderType != model.UserTypeTenant || n.SenderTenantID != tenants[0].ID {
			t.Fatal("发送主体未绑定登录租户")
		}
		if err := s.Publish(ctx, n.ID); err != nil {
			t.Fatal(err)
		}
		receipts, err := s.Receipts(ctx, n.ID, ListQuery{})
		if err != nil || receipts.Total != 2 {
			t.Fatalf("本租户全部账号下发错误: %+v %v", receipts, err)
		}
		for _, r := range receipts.List {
			if r.TenantID != tenants[0].ID {
				t.Fatal("通知跨租户投递")
			}
		}
		selected := input
		selected.TargetType, selected.TargetIDs = "tenant_users", []uint64{users[1].ID}
		n, err = s.Save(ctx, 0, selected)
		if err != nil {
			t.Fatal(err)
		}
		selectedID = n.ID
		if err := s.Publish(ctx, n.ID); err != nil {
			t.Fatal(err)
		}
		receipts, err = s.Receipts(ctx, n.ID, ListQuery{})
		if err != nil || receipts.Total != 1 || receipts.List[0].UserID != users[1].ID {
			t.Fatal("指定本租户账号投递失败")
		}
		n, err = s.Save(ctx, 0, selected)
		if err != nil {
			t.Fatal(err)
		}
		corruptedID = n.ID
		// 模拟存量坏草稿，发布时仍需重新校验，不能生成部分回执。
		if err := s.notifications(ctxkey.TenantDB(ctx)).Where("id = ?", n.ID).Update("target_ids", fmt.Sprintf("[%d,%d]", users[1].ID, users[2].ID)).Error; err != nil {
			t.Fatal(err)
		}
		if err := s.Publish(ctx, n.ID); errcode.From(err).Code != errcode.ErrInvalidParam {
			t.Fatalf("坏草稿发布未被拒绝: %v", err)
		}
		receipts, err = s.Receipts(ctx, n.ID, ListQuery{})
		if err != nil || receipts.Total != 0 {
			t.Fatal("发布失败不应残留回执")
		}
		list, err := s.List(ctx, ListQuery{})
		if err != nil || list.Total != 3 {
			t.Fatalf("租户发布列表混入平台通知: %+v %v", list, err)
		}
		// HTTP 入参中的伪造发送方字段不能覆盖鉴权上下文。
		gin.SetMode(gin.TestMode)
		engine := gin.New()
		engine.POST("/tenant/notifications", func(c *gin.Context) { c.Request = c.Request.WithContext(ctx); s.handleCreate(c) })
		payload := map[string]any{"title": "伪造字段检查", "content": "内容", "category": "business", "priority": "normal", "target_type": "own_tenant", "sender_type": "platform", "sender_tenant_id": tenants[1].ID, "sender_id": p.ID}
		body, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/tenant/notifications", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)
		var response struct {
			Data model.Notification `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || response.Data.SenderType != "tenant" || response.Data.SenderTenantID != tenants[0].ID {
			t.Fatalf("发送方字段被伪造: %s", w.Body.String())
		}
	})
	withTenant(users[2], func(ctx context.Context) {
		list, err := s.List(ctx, ListQuery{})
		if err != nil || list.Total != 0 {
			t.Fatal("乙租户不应能查看甲租户发布列表")
		}
		inbox, err := s.Inbox(ctx, ListQuery{})
		if err != nil || inbox.Total != 1 || inbox.List[0].ID != platformNotice.ID {
			t.Fatal("乙租户不应收到甲租户消息")
		}
		for _, id := range []uint64{ownID, selectedID, corruptedID, platformNotice.ID} {
			if _, err := s.Save(ctx, id, input); errcode.From(err).Code != errcode.ErrNotFound {
				t.Fatalf("应拒绝编辑他方通知: %v", err)
			}
			if err := s.Publish(ctx, id); errcode.From(err).Code != errcode.ErrNotFound {
				t.Fatalf("应拒绝发布他方通知: %v", err)
			}
			if err := s.Revoke(ctx, id); errcode.From(err).Code != errcode.ErrNotFound {
				t.Fatalf("应拒绝撤回他方通知: %v", err)
			}
			if _, err := s.Receipts(ctx, id, ListQuery{}); errcode.From(err).Code != errcode.ErrNotFound {
				t.Fatalf("应拒绝读取他方回执: %v", err)
			}
		}
	})
	withTenant(users[1], func(ctx context.Context) {
		feedback := "已完成店内检查"
		if err := s.Acknowledge(ctx, ownID, &feedback); err != nil {
			t.Fatal(err)
		}
		list, err := s.List(ctx, ListQuery{})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, n := range list.List {
			if n.ID == ownID && n.ProcessedCount == 1 {
				found = true
			}
		}
		if !found {
			t.Fatal("店内发布消息处理统计未更新")
		}
		if err := s.Revoke(ctx, selectedID); err != nil {
			t.Fatal(err)
		}
	})
	list, err := s.List(platform, ListQuery{})
	if err != nil || list.Total != 1 {
		t.Fatal("平台发布列表不应混入租户发布消息")
	}
	if err := s.Revoke(platform, ownID); errcode.From(err).Code != errcode.ErrNotFound {
		t.Fatal("平台发布治理接口不应管理租户消息")
	}
}

// TestLegacySenderMigration 验证旧结构升级后历史通知仍归属平台。
func TestLegacySenderMigration(t *testing.T) {
	s, m := testService(t)
	ctx := ctxkey.WithUser(context.Background(), 1, "platform", model.UserTypePlatform)
	n, err := s.Save(ctx, 0, Input{Title: "历史平台通知", Content: "内容", Category: "announcement", Priority: "normal", TargetType: "all_tenants"})
	if err != nil {
		t.Fatal(err)
	}
	// 将独立测试表恢复到旧结构，再迁移，验证历史通知仍归属于平台。
	if err := m.Platform().Exec("ALTER TABLE sys_notification DROP COLUMN sender_type, DROP COLUMN sender_tenant_id").Error; err != nil {
		t.Fatal(err)
	}
	if err := m.Platform().AutoMigrate(&model.Notification{}); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx, ListQuery{})
	if err != nil || list.Total != 1 || list.List[0].ID != n.ID || list.List[0].SenderType != "platform" || list.List[0].SenderTenantID != 0 {
		t.Fatalf("历史消息发送方迁移失败: %+v %v", list, err)
	}
}
