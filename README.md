# Pet Manager ·  SaaS 底座脚手架(后台管理系统)

多租户 SaaS 的全栈脚手架(后台管理系统)：Go + Gin + PostgreSQL（schema 级多租户）+ Redis 后端，React + Vite + Ant Design Pro 前端。当前聚焦**底座能力**：多租户隔离、统一鉴权与 RBAC、审计/登录日志、参数配置、字典、菜单可视化治理、内置定时任务（租户两段式删除与到期销毁）。

新增服务器 SSH 管理、租户网站分配与 chroot/SFTP 文件维护，以及独立 Directus Headless CMS 部署和文章、页面、导航、站点设置初始模型。启用方法、依赖选型及验收范围见 [服务器与网站接入说明](doc/hosting.md)。SSH 凭证需配置 `PM_HOSTING_ENCRYPTION_KEY`；CMS 域名的 DNS、反向代理与 HTTPS 由平台部署配置。

- 后端脚手架文档集中在 [server\_doc/](server_doc/cmd/api/main.md)，**镜像** **`server/`** **的目录结构**（如源码 `server/cmd/api/main.go` 的文档是 `server_doc/cmd/api/main.md`），从 [cmd/api/main.md](server_doc/cmd/api/main.md) 开始
- 设计资料见 [doc/](doc/architecture.md)：`architecture.md` / `prd.md` / `plan.md`

***

## 1. 技术栈

| 端    | 技术                                                                                                 |
| ---- | -------------------------------------------------------------------------------------------------- |
| 后端   | Go 1.26 · Gin · GORM · PostgreSQL 14+ · Redis · viper · zap · golang-jwt/v5 · bcrypt · swaggo/swag |
| 前端   | React 18 · Vite · TypeScript · Ant Design 5 / ProComponents · zustand · axios                      |
| 依赖部署 | Docker Compose（PG / Redis）                                                                         |

## 2. 目录结构

```
pet-manager/
├── server/                 # 后端（Go module: pet-manager/server）
│   ├── cmd/api/            # 唯一可执行入口
│   ├── configs/            # 默认配置 config.yaml
│   ├── docs/               # swag 生成物（已提交，勿手改）
│   ├── internal/
│   │   ├── bootstrap/      # 应用装配中心
│   │   ├── config/         # 配置树与加载
│   │   ├── infra/          # db / redis / logger / session
│   │   ├── middleware/     # 全局与鉴权链中间件
│   │   ├── migrate/        # AutoMigrate + 索引 + 租户 schema 管理
│   │   ├── model/          # 一表一文件的数据模型
│   │   ├── modules/        # 业务模块（auth/tenant/rbac/...）
│   │   ├── pkg/            # ctxkey/errcode/jwtx/response/... 工具包
│   │   ├── router/         # 路由命名空间与模块挂载
│   │   ├── seed/           # 权限点字典与种子数据
│   │   └── tests/          # 集成测试（需 PG+Redis）
│   └── Makefile            # 常用命令入口
├── server_doc/             # 后端脚手架文档（镜像 server/ 结构，与代码分离）
│                           #   如 server/cmd/api/main.go ↔ server_doc/cmd/api/main.md
├── admin/                  # 前端（Vite 工程）
├── deploy/                 # docker-compose 本地依赖
└── doc/                    # 架构/需求/计划文档
```

> 后端脚手架文档全部集中在 `server_doc/`，**目录层级与** **`server/`** **源码一一对应**（如 `server/internal/model/tenant.go` ↔ [server\_doc/internal/model/tenant.md](server_doc/internal/model/tenant.md)），说明职责、类型、约定与坑。完整索引见 [第 7 节](#7-脚手架文档地图)。

## 3. 快速上手

### 3.1 准备环境

- Go ≥ 1.26、Node.js ≥ 18、Docker Desktop（用于起 PG/Redis）
- 可选：`make`（Windows 推荐 Git Bash；不用 make 可直接执行下文等价命令）
- Swagger CLI（仅改接口注解时需要）：
  ```bash
  go install github.com/swaggo/swag/cmd/swag@latest
  ```

### 3.2 启动依赖

```bash
cd server
make infra-up        # docker compose -f ../deploy/docker-compose.yml up -d
```

如使用现成的 PG/Redis，跳过本步，改 `server/configs/config.yaml`（配置项说明见 [configs/config.md](server_doc/configs/config.md)）或用 `PM_` 环境变量指向你的实例。

### 3.3 启动后端

```bash
cd server
make run             # 等价 go run ./cmd/api -c ./configs/config.yaml
```

首次启动会自动：建平台库表与索引 → 补齐权限点字典 → 建平台管理员 → 为存量租户补表。

- 默认监听 `:8088`；健康检查：<http://localhost:8088/healthz>
- 默认管理员：`admin`（口令以配置/环境变量为准，生产必须用 `PM_SEED_PLATFORM_ADMIN_PASSWORD` 覆盖）
- Swagger UI（dev/test）：<http://localhost:8088/swagger/index.html>

### 3.4 启动前端

```bash
cd admin
npm install
npm run dev          # 默认 http://localhost:5173
```

登录页不填「租户编码」即平台管理员入口；填写租户编码走租户账号入口。

### 3.5 配置覆盖

所有配置项支持 `PM_` 前缀环境变量（`.` → `_`），例如：

```bash
PM_DATABASE_HOST=10.0.0.1 PM_REDIS_PASSWORD=xxx PM_APP_ADDR=:9090 make run
```

配置项含义见 [configs/config.md](server_doc/configs/config.md) 与 [config/config.md](server_doc/internal/config/config.md)。

## 4. 常用命令（server/ 目录）

| 命令                        | 作用                                       |
| ------------------------- | ---------------------------------------- |
| `make run`                | 本地启动 API                                 |
| `make build`              | 编译到 `bin/pet-manager-api`                |
| `make fmt` / `make vet`   | gofmt / go vet                           |
| `make test`               | 全部无依赖单测                                  |
| `make test-int`           | 集成测试（需 PG+Redis，`PM_TEST_INTEGRATION=1`） |
| `make swag`               | 重新生成 Swagger 文档到 `docs/`                 |
| `make swag-fmt`           | 格式化 Swagger 注解                           |
| `make infra-up/down/logs` | 启停/查看本地依赖                                |
| `make tidy`               | go mod tidy                              |

前端门禁：`npm run tsc`（或 `tsc --noEmit`）、`npm run lint`、`npm run build`。

## 5. Swagger 接口文档

- 地址：<http://localhost:8088/swagger/index.html> （仅 `app.env != prod` 注册；prod 自动关闭）
- 通用信息注解在 [main.go](server/cmd/api/main.go)（标题、`@host`、`@BasePath /api/v1`、`BearerAuth` 安全定义）；
- 每个接口在对应 handler 的注释里用 swag 注解声明（`@Summary/@Tags/@Param/@Success/@Router/@Security`）；
- 统一响应体 `{code,message,data}`（见 [response.md](server_doc/internal/pkg/response/response.md)），Swagger 中 data 的实际类型写在 `@Success` 描述里；
- 鉴权：Swagger UI 右上角「Authorize」填 `Bearer {access_token}`；
- 改了注解**必须重新生成**：`make swag`。生成物已提交，没装 CLI 不影响编译。生成物目录说明见 [docs/docs.md](server_doc/docs/docs.md)。

## 6. 架构要点（新人必读）

1. **单连接池 + schema 隔离**：只有一个 PG 连接池。平台数据用 `Platform()`；租户数据必须在事务内 `SET LOCAL search_path = tenant_x, public`（[db/manager.md](server_doc/internal/infra/db/manager.md)），事务结束自动复位，连接池复用不串租户。schema 名系统生成并正则校验，无注入面。
2. **四组路由命名空间**：Public / Authed / Platform / Tenant，鉴权链（凭证解析 → 租户门禁 → 租户事务 → 权限点）由 [router](server_doc/internal/router/router.md) 统一施加，模块只声明权限点。
3. **统一响应与错误码**：全站一个 `response.Body`，`code=0` 成功；错误码号段与 HTTP 映射见 [errcode.md](server_doc/internal/pkg/errcode/errcode.md)。越权固定 **403 + 20005**。
4. **会话可强制下线**：JWT 带 `sid`，每请求查 Redis 会话；登出/改密/停用立即生效。权限点按「租户+账号」缓存 10min，变更需主动失效。
5. **权限点声明式治理**：代码 [seed](server_doc/internal/seed/seed.md) 只补不覆盖，名称/路径/图标等由「菜单配置」页在线维护；字典写操作后全量失效权限缓存。
6. **审计非阻塞**：只记登录后写操作，不记请求体；缓冲队列 + 单 worker，满则丢弃，绝不拖慢主请求。
7. **模型约定**：所有表继承 BaseModel（id/created\_at/updated\_at/deleted\_at）；自然键唯一一律用 `WHERE deleted_at IS NULL` 部分唯一索引。
8. **任务管理与租户销毁**：支持页面新建系统方法、HTTP/curl 请求、sh 脚本任务，统一提供 5 段 cron、启停、手动执行、超时和日志；进程守卫 + Redis 锁防多实例重入。代码预置任务只补不覆盖。租户删除为两段式：先 `deleted` 待销毁（默认 7 天宽限，可撤销），到期任务 `DROP SCHEMA CASCADE` 物理回收。配置示例及方法注册见 [job/service](server_doc/internal/modules/job/service.md)。

### 新增一个业务模块（后端）

1. 在 `internal/modules/<mod>/` 建 `service.go` + `handler.go`（复杂模块加 `module.go`），Service 实现 `router.Module`（`Name()` + `Register(r)`）；
2. 在 [bootstrap/app.go](server/internal/bootstrap/app.go) 构造并加入 `modules` 列表；
3. 新表：一表一文件 + 在 [models.go](server_doc/internal/model/models.md) 登记 + 必要时在 [migrate.go](server_doc/internal/migrate/migrate.md) 加部分唯一索引；
4. 新权限点：在 [seed.go](server_doc/internal/seed/seed.md) 的 `permDefs` 声明；写操作后按范围失效权限缓存；
5. 给每个 handler 补 swag 注解并执行 `make swag`；
6. **在** **`server_doc/`** **镜像位置编写/更新同名** **`.md`**（源码在 `server/`，文档在 `server_doc/`，本约定由项目 skill 守护）；
7. `make fmt && make vet && make test` 自检。

前端约定（抽屉表单、DataTable/DataDrawer 二次封装、表格密度、空态插画、双主题等）见 [doc/architecture.md](doc/architecture.md) 的「前端脚手架」章节。

## 7. 脚手架文档地图

### 入口 / 配置 / 装配

- [cmd/api/main.md](server_doc/cmd/api/main.md) — 进程入口、优雅退出、Swagger 通用注解
- [configs/config.md](server_doc/configs/config.md) — 配置文件段说明与环境变量映射
- [internal/config/config.md](server_doc/internal/config/config.md) — 配置树与 viper 加载
- [internal/bootstrap/app.md](server_doc/internal/bootstrap/app.md) — 装配顺序与 App 生命周期
- [docs/docs.md](server_doc/docs/docs.md) — Swagger 生成物说明

### 基础设施 infra

- [infra/db/manager.md](server_doc/internal/infra/db/manager.md) · [manager\_test.md](server_doc/internal/infra/db/manager_test.md)
- [infra/cache/redis.md](server_doc/internal/infra/cache/redis.md)
- [infra/session/store.md](server_doc/internal/infra/session/store.md)
- [infra/logger/logger.md](server_doc/internal/infra/logger/logger.md)

### 中间件 middleware

- [registry.md](server_doc/internal/middleware/registry.md) · [common.md](server_doc/internal/middleware/common.md)
- [auth.md](server_doc/internal/middleware/auth.md) · [tenant.md](server_doc/internal/middleware/tenant.md) · [rbac.md](server_doc/internal/middleware/rbac.md) · [audit.md](server_doc/internal/middleware/audit.md)
- [rbac\_test.md](server_doc/internal/middleware/rbac_test.md)

### 数据 / 迁移 / 路由

- [router/router.md](server_doc/internal/router/router.md)
- [migrate/migrate.md](server_doc/internal/migrate/migrate.md) · [seed/seed.md](server_doc/internal/seed/seed.md)
- [tests/integration\_test.md](server_doc/internal/tests/integration_test.md)

### 数据模型 model

- 总册：[models.md](server_doc/internal/model/models.md) · 基类：[base.md](server_doc/internal/model/base.md) · 枚举：[consts.md](server_doc/internal/model/consts.md)
- 平台库：[tenant](server_doc/internal/model/tenant.md) · [platform\_user](server_doc/internal/model/platform_user.md) · [tenant\_user](server_doc/internal/model/tenant_user.md) · [permission](server_doc/internal/model/permission.md) · [audit\_log](server_doc/internal/model/audit_log.md) · [login\_log](server_doc/internal/model/login_log.md) · [config](server_doc/internal/model/config.md) · [dict](server_doc/internal/model/dict.md) · [dict\_item](server_doc/internal/model/dict_item.md) · [job](server_doc/internal/model/job.md) · [job\_log](server_doc/internal/model/job_log.md)
- 租户库：[role](server_doc/internal/model/role.md) · [role\_permission](server_doc/internal/model/role_permission.md) · [user\_role](server_doc/internal/model/user_role.md) · [tenant\_config](server_doc/internal/model/tenant_config.md) · [tenant\_dict](server_doc/internal/model/tenant_dict.md) · [tenant\_dict\_item](server_doc/internal/model/tenant_dict_item.md) · [schema\_version](server_doc/internal/model/schema_version.md)
- 信息通知：[notification](server_doc/internal/model/notification.md) · [notification\_receipt](server_doc/internal/model/notification_receipt.md)
- 存储台账：[storage\_account](server_doc/internal/model/storage_account.md) · [storage\_resource](server_doc/internal/model/storage_resource.md) · [storage\_change](server_doc/internal/model/storage_change.md)

### 业务模块 modules

| 模块                   | 文档                                                                                                                                                                                                                                                                                                                               |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| auth 认证              | [handler](server_doc/internal/modules/auth/handler.md) · [service](server_doc/internal/modules/auth/service.md)                                                                                                                                                                                                                  |
| tenant 租户管理          | [handler](server_doc/internal/modules/tenant/handler.md) · [service](server_doc/internal/modules/tenant/service.md) · [purge](server_doc/internal/modules/tenant/purge.md) · [purge\_test](server_doc/internal/modules/tenant/purge_test.md)                                                                                     |
| rbac 角色授权            | [module](server_doc/internal/modules/rbac/module.md) · [service](server_doc/internal/modules/rbac/service.md)                                                                                                                                                                                                                    |
| tenantuser 子账号       | [handler](server_doc/internal/modules/tenantuser/handler.md) · [service](server_doc/internal/modules/tenantuser/service.md)                                                                                                                                                                                                      |
| tenantprofile 店铺资料   | [service](server_doc/internal/modules/tenantprofile/service.md)                                                                                                                                                                                                                                                                  |
| audit 审计日志           | [handler](server_doc/internal/modules/audit/handler.md) · [module](server_doc/internal/modules/audit/module.md) · [service](server_doc/internal/modules/audit/service.md) · [policy](server_doc/internal/modules/audit/policy.md) · [policy\_test](server_doc/internal/modules/audit/policy_test.md)                             |
| loginlog 登录日志        | [handler](server_doc/internal/modules/loginlog/handler.md) · [module](server_doc/internal/modules/loginlog/module.md) · [service](server_doc/internal/modules/loginlog/service.md)                                                                                                                                               |
| permission 菜单配置      | [handler](server_doc/internal/modules/permission/handler.md) · [module](server_doc/internal/modules/permission/module.md) · [service](server_doc/internal/modules/permission/service.md)                                                                                                                                         |
| config 参数配置（平台 + 租户） | [handler](server_doc/internal/modules/config/handler.md) · [tenant\_handler](server_doc/internal/modules/config/tenant_handler.md) · [module](server_doc/internal/modules/config/module.md) · [service](server_doc/internal/modules/config/service.md) · [tenant\_service](server_doc/internal/modules/config/tenant_service.md) |
| dict 字典管理（平台 + 租户）   | [handler](server_doc/internal/modules/dict/handler.md) · [tenant\_handler](server_doc/internal/modules/dict/tenant_handler.md) · [module](server_doc/internal/modules/dict/module.md) · [service](server_doc/internal/modules/dict/service.md) · [tenant\_service](server_doc/internal/modules/dict/tenant_service.md)           |
| job 任务管理（定时任务/租户销毁）  | [module](server_doc/internal/modules/job/module.md) · [handler](server_doc/internal/modules/job/handler.md) · [service](server_doc/internal/modules/job/service.md) · [cron](server_doc/internal/modules/job/cron.md) · [cron\_test](server_doc/internal/modules/job/cron_test.md)                                               |
| notification 信息通知    | [module](server_doc/internal/modules/notification/module.md) · [handler](server_doc/internal/modules/notification/handler.md) · [service](server_doc/internal/modules/notification/service.md) · [service\_test](server_doc/internal/modules/notification/service_test.md)                                                       |

\| storage 统一存储 | [module](server_doc/internal/modules/storage/module.md) · [handler](server_doc/internal/modules/storage/handler.md) · [service](server_doc/internal/modules/storage/service.md) · [service\_test](server_doc/internal/modules/storage/service_test.md) |

- TOS 适配：[tos](server_doc/internal/infra/objectstore/tos.md) · [tos\_test](server_doc/internal/infra/objectstore/tos_test.md)
- 存储字典种子：[storage](server_doc/internal/seed/storage.md)
- 消息附件鉴权：[attachment](server_doc/internal/modules/notification/attachment.md)
- TOS 环境配置测试：[config\_test](server_doc/internal/config/config_test.md)

### 工具包 pkg

[ctxkey](server_doc/internal/pkg/ctxkey/ctxkey.md) · [errcode](server_doc/internal/pkg/errcode/errcode.md) · [jwtx](server_doc/internal/pkg/jwtx/jwt.md) · [response](server_doc/internal/pkg/response/response.md) · [pagination](server_doc/internal/pkg/pagination/pagination.md) · [security](server_doc/internal/pkg/security/password.md)

## 8. 信息通知与消息处理

平台通过「信息通知」创建草稿，选择全部租户、指定租户、指定租户账号或平台账号，保存后发布。指定租户会下发给其全部启用账号；接收范围在发布时冻结。通知可设置普通/紧急和是否需要处理，发布后可查看各账号回执或撤回。

平台与租户账号通过顶栏铃铛或「消息中心」查看未读消息，打开消息自动已读；需要处理的消息可提交处理反馈。顶栏未读数每 30 秒更新，已读/处理后立即刷新。当前为站内通知。后端重启时自动迁移通知表、唯一索引和补齐菜单权限；已登录账号重新登录可刷新菜单。

租户消息中心新增「发布消息」页签，可向本租户全部启用账号或指定启用账号发布店内消息，支持草稿、发布、撤回和处理回执。后端在保存和发布时校验接收账号归属，并隔离各租户的发布列表与回执。租户主账号自动具备权限，子账号需在「角色授权」中授予消息查看、编辑、发布或撤回权限。

## 9. 统一存储、套餐与消息附件

平台「租户管理」支持单租户「立即销毁」：启用、停用、待销毁租户均可操作，必须具备 platform.tenant.destroy 权限并输入精确租户编码。跳过宽限期，业务数据、账号、租户消息与回执、套餐变更和租户记录立即永久删除，不可撤销；会话注销，审计和登录历史保留。TOS 文件由后台清理并重试，在途上传等待完成或超时回收，清完文件再释放存储账户。普通「删除」仍保留宽限期和撤销入口。

TOS 配置按服务端 tos 段读取，默认签名 10 分钟、单文件 100 MiB。资源全部写平台 TOS，租户资源按归属空间隔离。平台「存储资源」管理全平台文件；租户页面仅管理本租户文件，支持上传、下载、删除与清理重试。删除会使引用该文件的消息附件无法下载。

平台「字典管理」中的 storage.plan 定义套餐编码、等级名称、GiB 容量与排序。默认基础 1 GiB、标准 10 GiB、高级 50 GiB，首次使用取排序最小的启用套餐。平台在「存储资源 → 租户存储套餐」分配/升级、追加扩容、查看变更记录。升级保留扩容；字典变更需重新分配套餐才影响已分配容量。占用按实际字节计算，包含上传/待清理预占，超额上传拒绝。

平台通知与租户店内消息可上传新附件或复用已有文件，每条最多 10 个。保存与发布均校验文件归属；接收人凭消息接收快照获取临时下载地址。撤回停止签发新下载地址，已签发链接在有效期内仍可能使用。删除故障保留台账与占用，由每 5 分钟存储清理任务重试。
