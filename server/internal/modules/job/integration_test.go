package job

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/cache"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
)

// 显式配置才执行；只在随机独立 schema 和 Redis 前缀中创建测试数据，
// 不启动应用、不迁移公共表、不注册任何真实业务方法。
func TestJobIntegration(t *testing.T) {
	configPath := os.Getenv("PM_TEST_JOB_CONFIG")
	if configPath == "" {
		t.Skip("设置 PM_TEST_JOB_CONFIG 后运行隔离的 PostgreSQL/Redis 集成测试")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal("无法加载测试配置")
	}
	schema := "job_test_" + strings.ToLower(rand.Text())
	if !regexp.MustCompile(`^job_test_[a-z2-7]+$`).MatchString(schema) {
		t.Fatal("测试命名空间不合法")
	}
	cfg.Database.PlatformSchema, cfg.Database.LogLevel = schema, "silent"
	cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns = 4, 2
	manager, err := db.New(cfg.Database, zap.NewNop())
	if err != nil {
		t.Fatal("无法连接测试数据库")
	}
	t.Cleanup(func() { _ = manager.Close() })
	p := manager.Platform()
	if err := p.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal("创建隔离测试 schema 失败")
	}
	t.Cleanup(func() {
		if err := p.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error; err != nil {
			t.Error("清理隔离测试 schema 失败")
		}
	})
	var actualSchema string
	if err := p.Raw("SELECT current_schema()").Scan(&actualSchema).Error; err != nil || actualSchema != schema {
		t.Fatal("测试数据库未隔离，停止写入")
	}
	if err := p.AutoMigrate(&model.Job{}, &model.JobLog{}); err != nil {
		t.Fatal(err)
	}
	if err := p.Exec(`CREATE UNIQUE INDEX uk_sys_job_code ON sys_job (code) WHERE deleted_at IS NULL`).Error; err != nil {
		t.Fatal(err)
	}
	cfg.Redis.KeyPrefix = schema + ":"
	client, err := cache.New(cfg.Redis)
	if err != nil {
		t.Fatal("无法连接测试 Redis")
	}
	t.Cleanup(func() {
		_ = client.DelByPrefix(context.Background(), schema+":")
		_ = client.Close()
	})
	s := NewService(manager, client, zap.NewNop(), ExecutorOptions{ShellPath: cfg.Jobs.ShellPath})
	gate := make(chan struct{})
	s.RegisterBuiltin(Builtin{Code: "safe_builtin", Name: "安全测试预置", Spec: "* * * * *", Fn: func(ctx context.Context) (string, error) {
		select {
		case <-gate:
			return "系统方法执行成功", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}})
	ctx := context.Background()
	if err := s.syncBuiltins(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := s.List(ctx)
	if err != nil || len(items) != 1 || !items[0].Builtin || items[0].Config.Handler != "safe_builtin" {
		t.Fatalf("预置同步异常: %+v %v", items, err)
	}
	if err := s.Delete(ctx, items[0].ID); err == nil {
		t.Fatal("预置任务应禁止删除")
	}
	create := func(in CreateInput) *Item {
		t.Helper()
		item, err := s.Create(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	in := CreateInput{Code: "custom_system", Name: "新建系统任务", Kind: model.JobKindSystem, Spec: "*/15 * * * *", Config: ExecutionConfig{Handler: "safe_builtin"}}
	first := create(in)
	if first.Builtin || first.Status != model.StatusDisabled || first.TimeoutSeconds != 60 {
		t.Fatalf("自定义任务默认值异常: %+v", first)
	}
	if _, err := s.Create(ctx, in); err == nil {
		t.Fatal("重复编码应失败")
	}
	in.Code = "second_system"
	second := create(in)
	run, err := s.prepareRun(ctx, first.ID, model.JobTriggerManual)
	if err != nil || run == nil {
		t.Fatal(err)
	}
	if err := s.Trigger(ctx, first.ID); err == nil {
		t.Fatal("同一任务重复触发应失败")
	}
	other := NewService(manager, client, zap.NewNop())
	other.RegisterMethod(Method{Code: "safe_builtin", Name: "测试方法", Fn: func(context.Context) (string, error) { return "", nil }})
	if err := other.Trigger(ctx, second.ID); err == nil {
		t.Fatal("多实例相同方法并发应失败")
	}
	if err := s.Delete(ctx, first.ID); err == nil {
		t.Fatal("执行中的任务应禁止删除")
	}
	changedKind := model.JobKindHTTP
	changedConfig := ExecutionConfig{URL: "http://127.0.0.1:1", Method: "GET"}
	if err := s.Update(ctx, first.ID, UpdateInput{Spec: first.Spec, Kind: &changedKind, Config: &changedConfig}); err != nil {
		t.Fatal(err)
	}
	if err := other.Trigger(ctx, first.ID); err == nil {
		t.Fatal("运行中修改类型后仍应阻止跨实例重复执行")
	}
	if err := other.Delete(ctx, first.ID); err == nil {
		t.Fatal("运行中修改类型后仍应禁止跨实例删除")
	}
	close(gate)
	run()
	logs, err := s.ListLogs(ctx, first.ID, LogListQuery{})
	if err != nil || logs.Total != 1 || logs.List[0].Status != model.JobRunSuccess {
		t.Fatalf("系统任务日志异常: %+v %v", logs, err)
	}
	if err := s.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	logs, err = s.ListLogs(ctx, first.ID, LogListQuery{})
	if err != nil || logs.Total != 1 {
		t.Fatal("软删除后应保留可查询日志", err)
	}
	in.Code = "custom_system"
	_ = create(in) // 删除后可复用编码。

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "method=%s", r.Method)
	}))
	defer httpServer.Close()
	httpJob := create(CreateInput{Code: "http_test", Name: "HTTP 集成任务", Kind: model.JobKindHTTP, Spec: "*/15 * * * *", Status: model.StatusEnabled, Config: ExecutionConfig{Curl: "curl -X POST '" + httpServer.URL + "' --json '{\"ok\":true}'"}})
	if !strings.Contains(httpJob.Config.Curl, "curl") || httpJob.Config.Method != "POST" {
		t.Fatal("curl 配置未解析")
	}
	run, err = s.prepareRun(ctx, httpJob.ID, model.JobTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	run()
	var current model.Job
	if err := p.First(&current, httpJob.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.NextRunAt == nil || !current.NextRunAt.Equal(*httpJob.NextRunAt) || current.LastStatus != model.JobRunSuccess {
		t.Fatal("手动执行改变了原调度或执行失败")
	}
	if err := p.Model(&model.Job{}).Where("id = ?", httpJob.ID).Update("next_run_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	run, err = s.prepareRun(ctx, httpJob.ID, model.JobTriggerSchedule)
	if err != nil || run == nil {
		t.Fatal("到期调度未触发", err)
	}
	if err := s.SetEnabled(ctx, httpJob.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(ctx, httpJob.ID, UpdateInput{Spec: "0 3 * * *", Remark: "执行中修改调度"}); err != nil {
		t.Fatal(err)
	}
	run()
	current = model.Job{}
	if err := p.First(&current, httpJob.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != model.StatusDisabled || current.NextRunAt != nil || current.Spec != "0 3 * * *" {
		t.Fatalf("执行完成覆盖了停用/编辑状态: status=%s next=%v spec=%s", current.Status, current.NextRunAt, current.Spec)
	}
	if err := s.SetEnabled(ctx, httpJob.ID, true); err != nil {
		t.Fatal(err)
	}
	run, err = s.prepareRun(ctx, httpJob.ID, model.JobTriggerSchedule)
	if err != nil || run != nil {
		t.Fatal("未到期任务不应执行", err)
	}

	if s.Methods().ShellAvailable {
		shellJob := create(CreateInput{Code: "shell_test", Name: "Shell 集成任务", Kind: model.JobKindShell, Spec: "* * * * *", Config: ExecutionConfig{Script: `printf 'hello %s' "$1"`, Args: []string{"中文参数"}}})
		if err := s.Trigger(ctx, shellJob.ID); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			logs, err := s.ListLogs(ctx, shellJob.ID, LogListQuery{})
			if err != nil {
				t.Fatal(err)
			}
			if logs.Total == 1 && logs.List[0].Status != model.JobRunRunning {
				if logs.List[0].Status != model.JobRunSuccess || logs.List[0].Result != "hello 中文参数" {
					t.Fatalf("脚本异步执行失败: %+v", logs)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("脚本异步执行未完成")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}
