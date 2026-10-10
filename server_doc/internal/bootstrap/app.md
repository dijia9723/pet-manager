# internal/bootstrap/app.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [入口 main.go](../../cmd/api/main.md) · [router/router.md](../router/router.md)

## 职责

应用**装配中心**：把配置、基础设施、中间件、业务模块按"依赖在前"的线性顺序组装成可运行的 `App`。任何一环失败直接返回错误，不做降级。

hosting.NewService 接收 cfg.CORS.AllowOrigins，供 SSH WebSocket 终端验证浏览器来源。

## 装配顺序（New）

```
logger -> db.Manager
  -> migrate.Platform（平台库结构 + 索引 + 遗留清理）
  -> seed.Run（权限点字典只补不覆盖 + 平台管理员）
  -> migrate.AllTenants（存量租户批量补表）
  -> redis cache -> session.Store -> jwt.Manager
  -> 业务 Service（rbac、tenant、loginlog、auth、tenantuser、tenantprofile、audit、permission、config、dict）
  -> job.Service + RegisterBuiltin(tenant_purge -> tenant.RunPurge)（在 dictSvc 之后，便于后续模块继续注册内置任务）
  -> gin.Engine + 中间件注册表
  -> /healthz + /swagger/*any（非 prod）
  -> router.Mount(全部模块)
```

## 关键设计

- **中间件注册表**：通过 `middleware.NewRegistry().Add(name, order, handler)` 按序位装配，避免"书写顺序即执行顺序"；
- **模块自注册**：每个业务 Service 实现 `router.Module`（`Name()` + `Register(r)`），这里只负责 `Mount`；
- **审计 worker 生命周期**：`auditSvc.Run(ctx)` 与 HTTP 服务同生，`Shutdown` 时先停 HTTP 再 `Close()` flush 队列；
- **任务调度器生命周期**：`auditSvc.Run` 之后 `jobSvc.Run(ctx)` 启动 30s tick 的调度循环（内置任务在 [job 模块](../modules/job/service.md) 管理），`Shutdown` 时 `jobSvc.Close()` 停止派发（执行中的任务由 10 分钟超时兜底）；
- **Swagger 开关**：`!cfg.App.IsProd()` 时注册 `/swagger/*any`，并匿名导入 `pet-manager/server/docs` 触发 swag 生成物注册（缺该导入会导致 doc.json 404/500）；
- **探活**：`GET /healthz` 在鉴权链路之外，会真 Ping 数据库，供容器/K8s 使用。

## App 方法

| 方法 | 说明 |
| --- | --- |
| `New(cfg)` | 全量装配，返回即可 Run |
| `Run()` | 阻塞启动 HTTP 服务 |
| `Shutdown(ctx)` | 优雅退出：停 HTTP → 停任务调度 → flush 审计 → 关 DB 连接池 → 关 Redis |
| `Engine()` | 暴露 gin 引擎，供 httptest 注入 |

## 新增模块时

1. 写好模块（实现 `router.Module`）；
2. 在本文件 import 并加入 `modules` 列表；
3. 如需中间件依赖（Resolver 等），在构造 router.Deps 处注入。

## 信息通知模块

构造 `notification.NewService(manager)` 并加入 router.Mount，挂载平台下发治理与平台/租户个人收件箱，不启动额外后台 worker。

信息通知模块也挂载租户店内发布治理。权限种子运行后，创建 session.Store 时 InvalidateAllPermissions，清理上一版本的角色权限缓存，使新增发布权限在新进程中立即按最新字典校验。权限缓存失效失败则中止启动，避免旧缓存导致新功能不可用；会话保持有效。

## 统一存储与附件

objectstore.New(cfg.TOS) 校验配置，storage.NewService 注入统一资源服务并挂载路由。停用时使用 nil Store，避免 typed nil 接口。notification.NewService(manager, storageSvc) 注入附件能力。jobSvc 注册 storage_cleanup，每 5 分钟重试对象清理与半小时未完成上传。

## 服务器与网站模块

新增 hosting 服务，读取 cfg.Hosting 加密配置，恢复中断的旧部署并加入路由模块。没有配置密钥时原应用可启动，但拒绝写入 SSH 凭证。
