# Pet Manager · 多租户 SaaS 底座脚手架

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14%2B-4169E1?logo=postgresql&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-yellow)

多租户 SaaS 全栈脚手架（后台管理系统），开箱即用一整套**底座能力**：多租户隔离、统一鉴权与 RBAC、审计/登录日志、参数配置、数据字典、菜单可视化治理、内置定时任务与统一存储。业务模块按统一约定增量接入即可，无需重复造轮子。

- **后端**：Go 1.26 · Gin · GORM · PostgreSQL（schema 级多租户）· Redis
- **前端**：React 18 · Vite · TypeScript · Ant Design 5 / ProComponents
- **脚手架文档**：[server_doc/](server_doc/cmd/api/main.md)，**镜像** `server/` 的目录结构（源码 `server/cmd/api/main.go` ↔ 文档 `server_doc/cmd/api/main.md`），建议从 [cmd/api/main.md](server_doc/cmd/api/main.md) 开始

## ✨ 特性

- **多租户隔离**：单连接池 + schema 级隔离。平台数据走 `Platform()`；租户数据必须在事务内 `SET LOCAL search_path = tenant_x, public`，事务结束自动复位，连接池复用不串租户。schema 名由系统生成并正则校验，无注入面。
- **统一鉴权与 RBAC**：四组路由命名空间（Public / Authed / Platform / Tenant），鉴权链（凭证解析 → 租户门禁 → 租户事务 → 权限点）统一施加；JWT 带 `sid`，每请求校验 Redis 会话，登出/改密/停用立即生效；权限点声明式治理，名称/路径/图标可在「菜单配置」页在线维护。
- **审计与登录日志**：审计只记录登录后的写操作、不记请求体；缓冲队列 + 单 worker，队列满即丢弃，绝不拖慢主请求；采集策略（开关/方法白名单/排除路由）可配置。
- **参数配置与数据字典**：平台、租户双层，代码种子只补不覆盖，在线可改。
- **定时任务**：系统方法 / HTTP(curl) / sh 脚本三类任务，5 段 cron、启停、手动执行、超时与日志；进程守卫 + Redis 锁防多实例重入，预置任务只补不覆盖。
- **租户生命周期**：两段式删除 —— 先进入待销毁宽限期（默认 7 天，可撤销），到期由定时任务 `DROP SCHEMA CASCADE` 物理回收。
- **统一存储**：TOS 资源台账、套餐配额预占与释放、消息附件；后台每 5 分钟重试清理。
- **站内通知 / 消息中心**：平台与租户双层发布，接收范围在发布时冻结，支持处理回执与撤回。
- **可选：服务器与网站托管**：SSH 管理与 chroot/SFTP 文件维护、租户网站分配，以及独立 Directus Headless CMS 部署。

## 🧱 技术栈

| 端 | 技术 |
| --- | --- |
| 后端 | Go 1.26 · Gin · GORM · PostgreSQL 14+ · Redis · viper · zap · golang-jwt/v5 · bcrypt · swaggo/swag |
| 前端 | React 18 · Vite · TypeScript · Ant Design 5 / ProComponents · zustand · axios |
| 本地依赖 | Docker Compose（PostgreSQL / Redis） |

## 📁 目录结构

```
pet-manager/
├── server/                 # 后端（Go module: pet-manager/server）
│   ├── cmd/api/            # 唯一可执行入口
│   ├── configs/            # 配置文件（默认 config.yaml）
│   ├── docs/               # swag 生成物（已提交，勿手改）
│   ├── internal/
│   │   ├── bootstrap/      # 应用装配中心
│   │   ├── config/         # 配置树与加载
│   │   ├── infra/          # db / cache / logger / session / objectstore
│   │   ├── middleware/     # 全局与鉴权链中间件
│   │   ├── migrate/        # AutoMigrate + 索引 + 租户 schema 管理
│   │   ├── model/          # 一表一文件的数据模型
│   │   ├── modules/        # 业务模块（auth / tenant / rbac / ...）
│   │   ├── pkg/            # ctxkey / errcode / jwtx / response / ... 工具包
│   │   ├── router/         # 路由命名空间与模块挂载
│   │   ├── seed/           # 权限点字典与种子数据
│   │   └── tests/          # 集成测试（需 PG + Redis）
│   └── Makefile            # 常用命令入口
├── server_doc/             # 后端脚手架文档（镜像 server/ 结构，与代码分离）
├── admin/                  # 前端（Vite + React）
└── deploy/                 # docker-compose 本地依赖
```

> 脚手架文档与代码分离：源码在 `server/`，说明文档在 `server_doc/` 的同名镜像路径（如 `server/internal/model/tenant.go` ↔ [server_doc/internal/model/tenant.md](server_doc/internal/model/tenant.md)），说明职责、类型、约定与坑。

## 🚀 快速开始

### 环境要求

- Go ≥ 1.26、Node.js ≥ 18、Docker Desktop（用于起 PostgreSQL / Redis）
- 可选：`make`（Windows 推荐 Git Bash；不使用 make 可直接执行下文等价命令）
- 可选：Swagger CLI（仅改动接口注解时需要）

  ```bash
  go install github.com/swaggo/swag/cmd/swag@latest
  ```

### 1. 启动依赖

```bash
cd server
make infra-up        # 等价 docker compose -f ../deploy/docker-compose.yml up -d
```

如已有现成的 PostgreSQL / Redis，跳过本步，改 `server/configs/config.yaml`（配置项说明见 [configs/config.md](server_doc/configs/config.md)），或用 `PM_` 环境变量指向你的实例。

### 2. 启动后端

```bash
cd server
make run             # 等价 go run ./cmd/api -c ./configs/config.yaml
```

首次启动会自动：建平台库表与索引 → 补齐权限点字典 → 建平台管理员 → 为存量租户补表。

- 默认监听 `:8088`，健康检查 <http://localhost:8088/healthz>
- 默认管理员 `admin`，口令以配置/环境变量为准；**生产必须用 `PM_SEED_PLATFORM_ADMIN_PASSWORD` 覆盖**
- Swagger UI（仅 `app.env != prod` 注册）：<http://localhost:8088/swagger/index.html>

### 3. 启动前端

```bash
cd admin
npm install
npm run dev          # 默认 http://localhost:5173
```

登录页不填「租户编码」即平台管理员入口；填写租户编码走租户账号入口。

### 4. 配置覆盖

所有配置项支持 `PM_` 前缀环境变量（`.` → `_`），例如：

```bash
PM_DATABASE_HOST=10.0.0.1 PM_REDIS_PASSWORD=xxx PM_APP_ADDR=:9090 make run
```

配置项含义见 [configs/config.md](server_doc/configs/config.md) 与 [internal/config/config.md](server_doc/internal/config/config.md)。

> 服务器托管（SSH / SFTP / Directus CMS）为可选能力，需配置 `PM_HOSTING_ENCRYPTION_KEY`；启用方式与验收范围见 server_doc 中的 hosting 模块文档。

## 🛠 常用命令（`server/` 目录）

| 命令 | 作用 |
| --- | --- |
| `make run` | 本地启动 API |
| `make build` | 编译到 `bin/pet-manager-api` |
| `make fmt` / `make vet` | gofmt / go vet |
| `make test` | 全部无依赖单测 |
| `make test-int` | 集成测试（需 PG + Redis，`PM_TEST_INTEGRATION=1`） |
| `make swag` | 重新生成 Swagger 文档到 `docs/` |
| `make swag-fmt` | 格式化 Swagger 注解 |
| `make infra-up/down/logs` | 启停 / 查看本地依赖 |
| `make tidy` | go mod tidy |

前端门禁：`npm run tsc`、`npm run lint`、`npm run build`。

## 🏗 架构要点

1. **单连接池 + schema 隔离**：只有一个 PG 连接池。平台数据用 `Platform()`；租户数据必须在事务内 `SET LOCAL search_path = tenant_x, public`（见 [db/manager.md](server_doc/internal/infra/db/manager.md)），事务结束自动复位，连接池复用不串租户。
2. **四组路由命名空间**：Public / Authed / Platform / Tenant，鉴权链由 [router](server_doc/internal/router/router.md) 统一施加，模块只声明权限点。
3. **统一响应与错误码**：全站一个 `response.Body`，`code=0` 成功；错误码号段与 HTTP 映射见 [errcode.md](server_doc/internal/pkg/errcode/errcode.md)。越权固定 **403 + 20005**。
4. **会话可强制下线**：JWT 带 `sid`，每请求查 Redis 会话；登出/改密/停用立即生效。权限点按「租户 + 账号」缓存 10min，变更需主动失效。
5. **权限点声明式治理**：代码 [seed](server_doc/internal/seed/seed.md) 只补不覆盖；字典写操作后全量失效权限缓存。
6. **审计非阻塞**：只记登录后写操作、不记请求体；缓冲队列 + 单 worker，满则丢弃。
7. **模型约定**：所有表继承 `BaseModel`（`id` / `created_at` / `updated_at` / `deleted_at`）；自然键唯一一律用 `WHERE deleted_at IS NULL` 部分唯一索引。
8. **任务管理与租户销毁**：见上文「特性」；配置示例与方法注册见 [job/service.md](server_doc/internal/modules/job/service.md)。

### 新增一个业务模块

1. 在 `internal/modules/<mod>/` 建 `service.go` + `handler.go`（复杂模块加 `module.go`），Service 实现 `router.Module`（`Name()` + `Register(r)`）；
2. 在 [bootstrap/app.go](server/internal/bootstrap/app.go) 构造并加入 `modules` 列表；
3. 新表：一表一文件 + 在 [models.go](server_doc/internal/model/models.md) 登记 + 必要时在 [migrate.go](server_doc/internal/migrate/migrate.md) 加部分唯一索引；
4. 新权限点：在 [seed.go](server_doc/internal/seed/seed.md) 的 `permDefs` 声明；写操作后按范围失效权限缓存；
5. 给每个 handler 补 swag 注解并执行 `make swag`；
6. 在 `server_doc/` 的镜像位置编写/更新同名 `.md`；
7. 自检：`make fmt && make vet && make test`。

## 📚 API 文档

- 地址：<http://localhost:8088/swagger/index.html>（仅 `app.env != prod` 注册，prod 自动关闭）
- 通用信息注解在 [main.go](server/cmd/api/main.go)（标题、`@host`、`@BasePath /api/v1`、`BearerAuth` 安全定义）
- 每个接口在对应 handler 的注释中以 swag 注解声明（`@Summary` / `@Tags` / `@Param` / `@Success` / `@Security` / `@Router`）
- 统一响应体 `{code,message,data}`（见 [response.md](server_doc/internal/pkg/response/response.md)），`data` 的实际类型写在 `@Success` 描述里
- 鉴权：Swagger UI 右上角「Authorize」填 `Bearer {access_token}`
- 改动注解后**必须重新生成**：`make swag`（生成物已提交，未装 CLI 不影响编译）
- 生成物目录说明见 [docs/docs.md](server_doc/docs/docs.md)

## 🗂 文档导航

**入口 / 配置 / 装配**

- [cmd/api/main.md](server_doc/cmd/api/main.md) — 进程入口、优雅退出、Swagger 通用注解
- [configs/config.md](server_doc/configs/config.md) — 配置文件段说明与环境变量映射
- [internal/config/config.md](server_doc/internal/config/config.md) — 配置树与 viper 加载
- [internal/bootstrap/app.md](server_doc/internal/bootstrap/app.md) — 装配顺序与 App 生命周期
- [docs/docs.md](server_doc/docs/docs.md) — Swagger 生成物说明

**基础设施 infra**

- [infra/db/manager.md](server_doc/internal/infra/db/manager.md) · [manager_test.md](server_doc/internal/infra/db/manager_test.md)
- [infra/cache/redis.md](server_doc/internal/infra/cache/redis.md) · [infra/session/store.md](server_doc/internal/infra/session/store.md) · [infra/logger/logger.md](server_doc/internal/infra/logger/logger.md)
- TOS 适配：[infra/objectstore/tos.md](server_doc/internal/infra/objectstore/tos.md) · [tos_test.md](server_doc/internal/infra/objectstore/tos_test.md)

**中间件 middleware**

- [registry.md](server_doc/internal/middleware/registry.md) · [common.md](server_doc/internal/middleware/common.md)
- [auth.md](server_doc/internal/middleware/auth.md) · [tenant.md](server_doc/internal/middleware/tenant.md) · [rbac.md](server_doc/internal/middleware/rbac.md) · [audit.md](server_doc/internal/middleware/audit.md) · [rbac_test.md](server_doc/internal/middleware/rbac_test.md)

**路由 / 迁移 / 种子 / 测试**

- [router/router.md](server_doc/internal/router/router.md)
- [migrate/migrate.md](server_doc/internal/migrate/migrate.md) · [seed/seed.md](server_doc/internal/seed/seed.md) · [seed/storage.md](server_doc/internal/seed/storage.md)
- [tests/integration_test.md](server_doc/internal/tests/integration_test.md)

**数据模型 model**

- 总册 [models.md](server_doc/internal/model/models.md) · 基类 [base.md](server_doc/internal/model/base.md) · 枚举 [consts.md](server_doc/internal/model/consts.md)
- 平台库：[tenant](server_doc/internal/model/tenant.md) · [platform_user](server_doc/internal/model/platform_user.md) · [tenant_user](server_doc/internal/model/tenant_user.md) · [permission](server_doc/internal/model/permission.md) · [audit_log](server_doc/internal/model/audit_log.md) · [login_log](server_doc/internal/model/login_log.md) · [config](server_doc/internal/model/config.md) · [dict](server_doc/internal/model/dict.md) · [dict_item](server_doc/internal/model/dict_item.md) · [job](server_doc/internal/model/job.md) · [job_log](server_doc/internal/model/job_log.md)
- 租户库：[role](server_doc/internal/model/role.md) · [role_permission](server_doc/internal/model/role_permission.md) · [user_role](server_doc/internal/model/user_role.md) · [tenant_config](server_doc/internal/model/tenant_config.md) · [tenant_dict](server_doc/internal/model/tenant_dict.md) · [tenant_dict_item](server_doc/internal/model/tenant_dict_item.md) · [schema_version](server_doc/internal/model/schema_version.md)
- 通知：[notification](server_doc/internal/model/notification.md) · [notification_receipt](server_doc/internal/model/notification_receipt.md)
- 存储台账：[storage_account](server_doc/internal/model/storage_account.md) · [storage_resource](server_doc/internal/model/storage_resource.md) · [storage_change](server_doc/internal/model/storage_change.md)

**业务模块 modules**

| 模块 | 文档 |
| --- | --- |
| auth 认证 | [handler](server_doc/internal/modules/auth/handler.md) · [service](server_doc/internal/modules/auth/service.md) |
| tenant 租户管理 | [handler](server_doc/internal/modules/tenant/handler.md) · [service](server_doc/internal/modules/tenant/service.md) · [purge](server_doc/internal/modules/tenant/purge.md) · [purge_test](server_doc/internal/modules/tenant/purge_test.md) |
| rbac 角色授权 | [module](server_doc/internal/modules/rbac/module.md) · [service](server_doc/internal/modules/rbac/service.md) |
| tenantuser 子账号 | [handler](server_doc/internal/modules/tenantuser/handler.md) · [service](server_doc/internal/modules/tenantuser/service.md) |
| tenantprofile 店铺资料 | [service](server_doc/internal/modules/tenantprofile/service.md) |
| audit 审计日志 | [handler](server_doc/internal/modules/audit/handler.md) · [module](server_doc/internal/modules/audit/module.md) · [service](server_doc/internal/modules/audit/service.md) · [policy](server_doc/internal/modules/audit/policy.md) · [policy_test](server_doc/internal/modules/audit/policy_test.md) |
| loginlog 登录日志 | [handler](server_doc/internal/modules/loginlog/handler.md) · [module](server_doc/internal/modules/loginlog/module.md) · [service](server_doc/internal/modules/loginlog/service.md) |
| permission 菜单配置 | [handler](server_doc/internal/modules/permission/handler.md) · [module](server_doc/internal/modules/permission/module.md) · [service](server_doc/internal/modules/permission/service.md) |
| config 参数配置（平台 + 租户） | [handler](server_doc/internal/modules/config/handler.md) · [tenant_handler](server_doc/internal/modules/config/tenant_handler.md) · [module](server_doc/internal/modules/config/module.md) · [service](server_doc/internal/modules/config/service.md) · [tenant_service](server_doc/internal/modules/config/tenant_service.md) |
| dict 字典管理（平台 + 租户） | [handler](server_doc/internal/modules/dict/handler.md) · [tenant_handler](server_doc/internal/modules/dict/tenant_handler.md) · [module](server_doc/internal/modules/dict/module.md) · [service](server_doc/internal/modules/dict/service.md) · [tenant_service](server_doc/internal/modules/dict/tenant_service.md) |
| job 任务管理 | [module](server_doc/internal/modules/job/module.md) · [handler](server_doc/internal/modules/job/handler.md) · [service](server_doc/internal/modules/job/service.md) · [executor](server_doc/internal/modules/job/executor.md) · [curl](server_doc/internal/modules/job/curl.md) · [lock](server_doc/internal/modules/job/lock.md) · [manage](server_doc/internal/modules/job/manage.md) · [cron](server_doc/internal/modules/job/cron.md) |
| notification 信息通知 | [module](server_doc/internal/modules/notification/module.md) · [handler](server_doc/internal/modules/notification/handler.md) · [service](server_doc/internal/modules/notification/service.md) · [attachment](server_doc/internal/modules/notification/attachment.md) |
| storage 统一存储 | [module](server_doc/internal/modules/storage/module.md) · [handler](server_doc/internal/modules/storage/handler.md) · [service](server_doc/internal/modules/storage/service.md) |

**工具包 pkg**

[ctxkey](server_doc/internal/pkg/ctxkey/ctxkey.md) · [errcode](server_doc/internal/pkg/errcode/errcode.md) · [jwtx](server_doc/internal/pkg/jwtx/jwt.md) · [response](server_doc/internal/pkg/response/response.md) · [pagination](server_doc/internal/pkg/pagination/pagination.md) · [security](server_doc/internal/pkg/security/password.md)

## 🤝 贡献

欢迎提交 Issue 与 Pull Request。提交前请确保通过门禁：

```bash
cd server && make fmt && make vet && make test
cd ../admin && npm run tsc && npm run lint && npm run build
```

后端改动请同步更新 `server_doc/` 中对应镜像路径的 `.md` 文档，并在改动 swag 注解后执行 `make swag`。

## 📄 许可证

本项目基于 [MIT License](LICENSE) 开源。
