// Package tests 放的是"必须连真实依赖才有意义"的验收测试。
//
// 这里验证两条硬约束，它们一旦被破坏就是数据事故，不适合只靠代码评审兜住：
//  1. 租户数据隔离：两个租户互不可见，且连接池复用不会串 search_path；
//  2. 越权拦截：缺少权限点时接口必须返回 403。
//
// 默认跳过，避免 CI 没有 PG/Redis 时误报失败。开启方式：
//
//	docker compose -f deploy/docker-compose.yml up -d
//	cd server && PM_TEST_INTEGRATION=1 go test ./internal/tests/... -v
package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/cache"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/middleware"
	"pet-manager/server/internal/migrate"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// 固定的测试 schema 名，必须落在 tenant_<数字> 形态内才能通过 ValidateSchema。
const (
	schemaA = "tenant_9001"
	schemaB = "tenant_9002"

	rbacTenantID = uint64(9007)
)

func integrationConfig(t *testing.T) *config.Config {
	t.Helper()
	if os.Getenv("PM_TEST_INTEGRATION") != "1" {
		t.Skip("集成测试默认跳过；开启方式: PM_TEST_INTEGRATION=1 go test ./internal/tests/...")
	}
	// 传空路径：找不到配置文件时自动退回默认值，默认值与 deploy/docker-compose.yml 一致，
	// 同时仍支持用 PM_ 环境变量覆盖（例如 PM_DATABASE_HOST）。
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("加载测试配置失败: %v", err)
	}
	return cfg
}

func openManager(t *testing.T, cfg *config.Config) *db.Manager {
	t.Helper()
	m, err := db.New(cfg.Database, zap.NewNop())
	if err != nil {
		t.Fatalf("连接测试数据库失败（请先启动 deploy/docker-compose.yml）: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// TestTenantIsolation 是本次脚手架最关键的一条验收：租户之间必须彻底隔离。
func TestTenantIsolation(t *testing.T) {
	cfg := integrationConfig(t)
	m := openManager(t, cfg)
	ctx := context.Background()

	schemas := []string{schemaA, schemaB}
	for _, s := range schemas {
		if err := m.DropTenantSchema(ctx, s); err != nil {
			t.Fatalf("清理历史测试 schema %s 失败: %v", s, err)
		}
	}
	t.Cleanup(func() {
		for _, s := range schemas {
			_ = m.DropTenantSchema(context.Background(), s)
		}
	})

	for _, s := range schemas {
		if err := migrate.TenantSchema(ctx, m, s); err != nil {
			t.Fatalf("初始化租户 schema %s 失败: %v", s, err)
		}
	}

	// 两个租户使用完全相同的角色编码：这在"一租户一 schema"下必须成立，
	// 也顺带说明唯一约束是 schema 级的，不会被其他租户占用。
	createRole := func(schema, name string) {
		t.Helper()
		err := m.TenantTx(ctx, schema, func(tx *gorm.DB) error {
			return tx.Create(&model.Role{Name: name, Code: model.RoleCodeOwner, IsSystem: true}).Error
		})
		if err != nil {
			t.Fatalf("向 %s 写入角色失败: %v", schema, err)
		}
	}
	createRole(schemaA, "甲店-主账号")
	createRole(schemaB, "乙店-主账号")

	readNames := func(schema string) []string {
		t.Helper()
		names := []string{}
		err := m.TenantTx(ctx, schema, func(tx *gorm.DB) error {
			return tx.Model(&model.Role{}).Order("id asc").Pluck("name", &names).Error
		})
		if err != nil {
			t.Fatalf("读取 %s 角色失败: %v", schema, err)
		}
		return names
	}

	assertOnly := func(round string, schema, want string) {
		t.Helper()
		got := readNames(schema)
		if len(got) != 1 || got[0] != want {
			t.Fatalf("%s：%s 期望只看到 [%s]，实际 %v", round, schema, want, got)
		}
	}

	assertOnly("首轮", schemaA, "甲店-主账号")
	assertOnly("首轮", schemaB, "乙店-主账号")

	// 交替读取多轮：不同租户的事务会复用同一个连接池，
	// 若 search_path 的复位没做好，这里必然读到上一个租户的数据。
	for i := 1; i <= 5; i++ {
		round := "第 " + strconv.Itoa(i) + " 轮交替读取"
		assertOnly(round, schemaA, "甲店-主账号")
		assertOnly(round, schemaB, "乙店-主账号")
	}

	// 事务结束后连接必须复位回平台 schema
	var current string
	if err := m.Platform().WithContext(ctx).Raw("SELECT current_schema()").Scan(&current).Error; err != nil {
		t.Fatalf("查询 current_schema 失败: %v", err)
	}
	if current != cfg.Database.PlatformSchema {
		t.Fatalf("租户事务结束后 search_path 未复位：current_schema=%q，期望 %q", current, cfg.Database.PlatformSchema)
	}

	// 平台连接不得直接读到租户表，否则说明租户数据没有真正隔离
	var n int64
	if err := m.Platform().WithContext(ctx).Raw("SELECT count(*) FROM t_role").Scan(&n).Error; err == nil {
		t.Fatalf("平台连接竟能直接查询 t_role，租户表必须只能通过 TenantTx 访问")
	}
}

// TestRBACForbidden 验证缺少权限点时统一返回 403 与业务码 20005。
func TestRBACForbidden(t *testing.T) {
	cfg := integrationConfig(t)

	c, err := cache.New(cfg.Redis)
	if err != nil {
		t.Fatalf("连接测试 Redis 失败（请先启动 deploy/docker-compose.yml）: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	// 权限点缓存按"租户+账号"落 Redis，测试前后各清一次，避免脏缓存影响结果
	prefix := c.Key("perm", "t", strconv.FormatUint(rbacTenantID, 10))
	_ = c.DelByPrefix(context.Background(), prefix)
	t.Cleanup(func() { _ = c.DelByPrefix(context.Background(), prefix) })

	resolver := stubResolver{perms: map[uint64][]string{
		11: {"tenant.user.list"},
		12: {"tenant.user.list", "tenant.user.create"},
	}}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	g := engine.Group("/x")
	g.Use(injectTenant(rbacTenantID), middleware.NewRBAC(resolver, session.New(c)).Require("tenant.user.create"))
	g.GET("", func(c *gin.Context) { response.OK(c, "ok") })

	status, body := doRequest(engine, "/x", 11)
	if status != http.StatusForbidden {
		t.Fatalf("仅持 tenant.user.list 的账号应被拒绝：期望 403，实际 %d", status)
	}
	if body.Code != int(errcode.ErrForbiddenAction) {
		t.Fatalf("越权响应业务码期望 %d，实际 %d", errcode.ErrForbiddenAction, body.Code)
	}

	status, _ = doRequest(engine, "/x", 12)
	if status != http.StatusOK {
		t.Fatalf("持 tenant.user.create 的账号应放行：期望 200，实际 %d", status)
	}
}

// stubResolver 是权限解析桩，按账号 ID 返回预置权限点。
type stubResolver struct {
	perms map[uint64][]string
}

// ResolvePermissions 返回桩数据，避免测试依赖真实权限库。
func (s stubResolver) ResolvePermissions(_ context.Context, _, userID uint64, _ string) ([]string, error) {
	return s.perms[userID], nil
}

// injectTenant 模拟 Auth + TenantGuard 之后的上下文，让 RBAC 中间件可以直接被测试。
func injectTenant(tenantID uint64) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := strconv.ParseUint(c.GetHeader("X-Test-User"), 10, 64)
		ctx := ctxkey.WithUser(c.Request.Context(), uid, "tester", ctxkey.UserTypeTenant)
		ctx = ctxkey.WithTenant(ctx, tenantID, "t9007", schemaA)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// doRequest 以指定用户 ID 发起请求并解析统一响应体。
func doRequest(engine *gin.Engine, path string, userID uint64) (int, response.Body) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Test-User", strconv.FormatUint(userID, 10))
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var body response.Body
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}
