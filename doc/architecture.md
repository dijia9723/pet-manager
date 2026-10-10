# 架构说明（W1 底座 + 租户 RBAC 脚手架）

本文是 W1 交付物的落地说明：目录怎么分、分层怎么切、多租户的硬约束在哪里执行、
后续加表 / 加拦截器 / 加接口该怎么改。配套文档：[prd.md](./prd.md)（需求）、[plan.md](./plan.md)（排期）。

---

## 一、目录结构

```
pet-manager/
├── server/                      # Go 后端（Gin + GORM + PostgreSQL + Redis）
│   ├── cmd/api/main.go          # 入口：解析 -c 配置 → 装配 → 优雅退出
│   ├── configs/config.yaml      # 默认配置，所有项可用 PM_ 前缀环境变量覆盖
│   ├── Makefile                 # run / build / test / test-int / infra-up
│   └── internal/
│       ├── bootstrap/           # 装配根：基础设施 → 中间件 → 路由 → 模块
│       ├── config/              # viper 配置树与默认值
│       ├── infra/               # 基础设施：logger / db / cache / session
│       ├── middleware/          # 全站拦截器 + 可插拔注册表（registry.go）
│       ├── model/               # 数据模型：一表一文件（base.go + models.go 聚合清单 + consts.go）
│       ├── migrate/             # 平台库与租户 schema 的迁移
│       ├── seed/                # 权限点字典 / 平台管理员
│       ├── router/              # 路由分层与模块挂载（router.Module）
│       ├── modules/             # 业务模块（每个模块自带 Register）
│       │   ├── auth/            # 登录 / 刷新 / 登出 / 个人信息 / 菜单
│       │   ├── rbac/            # 角色 CRUD、权限点树、权限解析
│       │   ├── tenant/          # 平台端租户 CRUD + 开通编排 + TenantResolver
│       │   ├── tenantuser/      # 租户端子账号 CRUD
│       │   ├── tenantprofile/   # 租户端店铺信息
│       │   ├── audit/           # 审计日志：非阻塞采集 + 平台端只读查询
│       │   ├── loginlog/        # 登录日志：登录成功/失败采集 + 平台端只读查询
│       │   ├── config/          # 参数配置 CRUD：平台端 sys_config + 租户端 t_config
│       │   ├── dict/            # 字典管理 CRUD：平台端 sys_dict* + 租户端 t_dict*（类型 + 字典项）
│       │   └── permission/      # 平台端权限点（菜单/按钮）可视化配置
│       ├── pkg/                 # 通用件：errcode / response / ctxkey / jwtx / security / pagination
│       └── tests/               # 需真实依赖的验收测试（默认跳过）
├── admin/                       # React 后台（Vite6 + React19 + TS + AntD5 + ProComponents）
│   ├── .env[.development|.production]  # 多环境变量（唯一出口 src/config）
│   ├── eslint.config.js         # ESLint9 flat config；.prettierrc.json
│   └── src/
│       ├── config/              # appConfig：env 的唯一出口
│       ├── constants/           # storageKey() / STORE_KEYS（持久化键命名空间）
│       ├── api/                 # axios 封装：统一解包、401 静默 refresh 单飞
│       ├── router/
│       │   ├── routes/modules/  # 路由即菜单：AppRouteModule meta 声明（按 order 排序）
│       │   ├── guard.tsx        # AuthGuard：perm 单权限点 / anyOf
│       │   └── helpers.ts       # findRouteByPath（精确 + 前缀匹配）
│       ├── store/               # zustand：auth / preferences / tabs（均 persist）
│       ├── layouts/             # MainLayout + TabsBar / HeaderActions / PreferencesDrawer
│       ├── components/          # KeepAliveOutlet / ChunkErrorBoundary / RouteTracker / PageLoading
│       ├── hooks/               # usePermission / useFullscreen / useMessage
│       ├── styles/global.css    # 基线 / nprogress / 页签条 / 分屏登录页
│       └── pages/{login,dashboard,platform,tenant,errors}
├── miniapp/  print-helper/      # 尚未创建，分别在 W6 / W4 开工时建
└── deploy/docker-compose.yml    # 本地 PG15 / Redis7 / MinIO
```

### 后端依赖方向（只允许向下）

```
cmd → bootstrap → modules → router → middleware → infra → pkg → model
```

- `model` 与 `pkg` 不依赖任何上层；
- `middleware` 只通过接口依赖业务能力（`PermissionResolver` / `TenantResolver`），
  避免 middleware ↔ modules 循环引用；
- 业务模块之间不互相 import（`tenant` 依赖 `rbac` 的 `SeedTenantRoles` 是唯一例外，
  因为它要复用"预置角色"这一份定义）。

---

## 二、多租户：一租户一 Schema

### 2.1 表命名与归属

| 归属 | 命名 | 位置 | 内容 |
| --- | --- | --- | --- |
| 平台库 | `sys_*` | `public` | `sys_platform_user`、`sys_tenant`、`sys_tenant_user`、`sys_permission`、`sys_audit_log`（审计）、`sys_login_log`（登录）、`sys_config`（参数）、`sys_dict` + `sys_dict_item`（字典） |
| 租户库 | `t_*` | `tenant_<id>` | `t_role`、`t_role_permission`、`t_user_role`、`t_config`（租户参数）、`t_dict` + `t_dict_item`（租户字典）、`t_schema_version`（W2 起追加宠物档案 / 工单 / 打印模板…） |

账号（含租户主账号与子账号）统一放平台库 `sys_tenant_user`：登录入口唯一、跨租户查账号不必切 schema。
角色与授权放租户 schema：租户可自由增删角色。
跨 schema 只存 ID、**不建外键**，保证将来租户 schema 可以整体搬到独立库。

### 2.2 唯一入口与硬约束

```go
// 请求链路：middleware.TenantScope 已开启租户事务
tx, err := ctxkey.MustTenantDB(ctx.Request.Context())

// 后台任务 / 开通流程：显式开事务
err := manager.TenantTx(ctx, schema, func(tx *gorm.DB) error { ... })
```

- `TenantTx` / `BeginTenant` 内部执行
  `set_config('search_path', '<tenant_x>, public', true)`，
  第三参数 `true` 即事务级（等价 `SET LOCAL`），**提交或回滚后连接自动复位**，
  连接池复用不会串租户。
- search\_path 带上 `public` 是刻意设计：租户事务内可以直接 JOIN `sys_permission`，
  也能在同一个事务里写 `sys_tenant_user`（例如"建子账号 + 绑角色"是一个原子操作）。
- schema 名由 `db.SchemaForTenant(id)` 生成，`db.ValidateSchema` 用
  `^tenant_[0-9]+$` 校验后才允许拼进 DDL / search\_path，不接受用户输入。
- **评审红线**：租户域代码里出现 `manager.Platform()` 一律驳回。
  平台库查询（租户列表、权限点字典）才允许用 `Platform()`。

### 2.3 BaseModel 与软删除的唯一性

**组织约定：一个表一个文件。** 文件名 = 表名去掉前缀（`TenantUser` → `tenant_user.go`），
避免多个表挤在一个文件里随业务膨胀。目录内另有三个非模型文件：
[consts.go](../server/internal/model/consts.go)（跨表通用常量）、
[models.go](../server/internal/model/models.go)（`PlatformModels()` / `TenantModels()` 聚合清单）、
[base.go](../server/internal/model/base.go)（`BaseModel` 与包级文档）。

> 新增一张表要动两处：先在 `model/` 建新文件定义模型，再到 `models.go` 对应清单里登记，
> 否则 `AutoMigrate` 不会建表。

所有 `sys_*` / `t_*` 模型统一嵌入 [base.go](../server/internal/model/base.go) 的 `BaseModel`：

```go
type BaseModel struct {
    ID        uint64         // 主键
    CreatedAt time.Time      // 创建时间
    UpdatedAt time.Time      // 更新时间
    DeletedAt gorm.DeletedAt // 软删除标记
}
```

纯关联表（`t_role_permission`、`t_user_role`）与内部版本表（`t_schema_version`）例外，它们不参与软删除。

**软删除 + 唯一索引的坑**：普通唯一索引会把已删除的历史行也算进去，
导致"删掉的用户名 / 租户编码再也建不回来"。因此自然键唯一性统一改用
**部分唯一索引**（`WHERE deleted_at IS NULL`），在 `AutoMigrate` 之后由
[migrate.go](../server/internal/migrate/migrate.go) 的 `platformPartialUniqueIndexes` /
`tenantPartialUniqueIndexes` 补齐。

> 新增带唯一业务键的表时，记得同步往这两个列表里加一条 `CREATE UNIQUE INDEX IF NOT EXISTS ... WHERE deleted_at IS NULL`。

### 2.4 租户开通编排

`modules/tenant.Create` 的顺序与补偿：

```
1. 平台库插入租户记录 → 拿到自增 ID → schema = tenant_<id>
2. CREATE SCHEMA + AutoMigrate 全部租户表
3. 在租户 schema 内预置角色：owner（全量）/ staff（基础业务）
4. 创建主账号（sys_tenant_user）并绑定 owner 角色
```

第 2 步之后任一步失败 → 补偿删除 schema 与租户记录，不留半成品。
同一租户编码的开通动作走 Redis 锁串行化，避免并发建出两个同编码租户。

存量租户的后续新表由 `migrate.AllTenants` 在启动时批量补齐。

### 2.5 租户两段式删除与任务调度

删除租户不做即时物理删除，采用「待销毁 → 到期回收」两段式，避免误删即时不可逆：

```
平台「删除」                  宽限期（默认 7 天）                tenant_purge 定时任务（每日 03:30）
status=deleted        ──▶   行/schema 全部保留            ──▶   DROP SCHEMA tenant_<id> CASCADE
purge_at = now + N 天         租户门禁拦截（非 enabled 即拦）      Unscoped 硬删 sys_tenant_user / sys_tenant
踢全部会话 + 失效快照          可「撤销删除」恢复为 disabled        审计/登录日志保留（合规留痕）
```

- 宽限天数取平台参数 `tenant.purge_after_days`（种子默认 7，非法/停用回退 7）；
- 到期判定走 `(status, purge_at)` 复合索引，任务幂等、可在「任务管理」页手动执行（运维场景把宽限设 0 即立即到期）。

任务调度是平台通用能力（`modules/job`，表 `sys_job` / `sys_job_log`）：

- **内置任务声明式注册**：装配期 `RegisterBuiltin{code, spec(5段cron), Fn}`，启动/每轮 tick 向 sys_job「只补不覆盖」，页面改的 cron/启停重启不回滚；
- **调度**：自实现 5 段 cron（零三方依赖，分钟级、本地时区、日/周 OR 语义），30s 扫描 `next_run_at <= now`；
- **防重入**：进程内 running 守卫 + Redis 锁 `pm:lock:job:run:<code>`（TTL 30min > 执行超时 10min），多实例只有一处执行；
- **可治理**：平台「任务管理」页支持查看/启停/编辑 cron（非法表达式 400）/立即执行/执行日志；每次执行先落 running 日志，结束回写状态、耗时与摘要。

---

## 三、可插拔拦截器（中间件）

[registry.go](../server/internal/middleware/registry.go) 提供"按名字注册 + 按序位排序"的注册表，
替代"按代码书写顺序 Use"的做法。

| 序位 | 常量 | 作用 |
| --- | --- | --- |
| 10 | `OrderRecovery` | panic 兜底 |
| 20 | `OrderRequestID` | 请求 ID，贯穿日志与响应头 |
| 30 | `OrderSecurityHeaders` | 基础安全响应头 |
| 40 | `OrderCORS` | 白名单跨域 |
| 50 | `OrderRateLimit` | Redis 固定窗口限流（fail-open） |
| 60 | `OrderAccessLog` | 结构化访问日志 |
| 100 | `OrderAuth` | 解析 JWT + 校验 Redis 会话 |
| 110 | `OrderTenantContext` | 租户存在性 / 状态 / 到期门禁 |
| 120 | `OrderTenantScope` | 开启租户事务（search\_path） |
| 130 | `OrderRBAC` | 权限点校验 |
| 900 | `OrderAudit` | 写操作审计（白名单方法 + 已登录才记，非阻塞投递） |

审计落在 `sys_audit_log`（平台库，`tenant_id=0` 表示平台事件），
刻意**不记录请求体**：重置密码等接口的明文口令绝不能进审计表，
追溯依赖 `route + status + 操作人 + request_id` 回查访问日志。
投递为带缓冲队列 + 单 worker 落库，队列满即丢弃，审计失败绝不影响主请求。

新增拦截器（审计、埋点、幂等、指标…）只要：

```go
registry.Add("idempotency", 65, myMiddleware)   // 取相邻两档之间的空档
```

不必改动任何已有文件，启动日志会打印实际执行顺序（`HTTP 拦截器链已装配`）。

---

## 四、鉴权与会话

- **JWT**（`internal/pkg/jwtx`）：access / refresh 两类 token 共用一个 `sid`（会话 ID）。
  载荷含 `uid / uname / utype / tid / tcode / tschema`。
- **Redis 会话**（`internal/infra/session`）：三类数据统一失效，
  - `pm:session:<utype>:<uid>:<sid>`：会话存活标记，登出即删 → **可强制下线**；
  - `pm:perm:t:<tenantID>:<userID>`：权限点缓存，角色/授权变更时按租户整体失效；
  - `pm:tenant:<id>`：租户快照（状态、到期时间），避免每请求回查平台库。
- **权限点判定**：`middleware.HasAny`，平台管理员为通配 `*`；
  租户 `owner` 角色直接返回全部 `scope=tenant` 权限点（平台后续新增权限点无需回填历史租户）。

### 登录入口

同一个登录页：填了 `tenant_code` 走租户账号，没填走平台管理员；
返回体里的 `user.user_type` 决定前端落地路由。

登录成功与失败都会写入 `sys_login_log`（`modules/loginlog`）。采集通过 `auth` 定义的
`LoginRecorder` 接口解耦（消费方定义、`loginlog` 实现），`auth` 不反向 import `loginlog`，
避免循环依赖；落库在派生 context 内异步进行，失败仅告警、不影响登录响应。

---

## 五、路由分层与模块挂载

[router.go](../server/internal/router/router.go) 把"鉴权链路"收敛到分组上，业务模块只声明权限点：

| 方法 | 路径前缀 | 施加的链路 |
| --- | --- | --- |
| `Public(path)` | `/api/v1` + path | 无 |
| `Authed(path)` | `/api/v1` + path | 登录 |
| `Platform(path)` | `/api/v1/platform` + path | 登录 + `user_type=platform` |
| `Tenant(path)` | `/api/v1/tenant` + path | 登录 + `user_type=tenant` + 租户门禁 + 租户事务 |

模块实现 `router.Module`（`Name()` + `Register(*Router)`），在 `bootstrap` 里统一 `r.Mount(...)`。

### 新增一个业务模块的步骤

1. `internal/model/` 里定义模型（嵌入 `BaseModel`，`t_*` 表追加到 `TenantModels()`）；
2. 有唯一业务键 → 在 `migrate` 的 `tenantPartialUniqueIndexes` 加部分唯一索引；
3. `internal/modules/<name>/` 写 `service.go`（业务 + `Name()`/`Register()`）与 `handler.go`；
4. 若是租户表，`migrate.TenantSchemaVersion` 递增一个版本号；
5. 在 `bootstrap` 的 `modules` 切片里追加；
6. 需要新权限点 → 在 `internal/seed/seed.go` 的 `permDefs` 里加一条，
   启动只创建**缺失的 code**（seed 只补不覆盖）；已有的名称/路径/图标/排序改动
   由平台侧"菜单配置"页治理（`internal/modules/permission`），代码改动不再覆盖线上配置。

> **权限点字典治理约定**：`sys_permission` 是菜单与权限点的唯一来源，
> 平台侧"菜单配置"页可增删改；已软删的 code 视为管理员有意删除，
> seed 启动时不会复活它。权限点写操作后调用 `session.InvalidateAllPermissions`
> 全量失效权限缓存（跨租户），下一次请求自动重建。

**菜单树与容器节点**：菜单由 `auth.Menus()` 从 `sys_permission` 中 `scope + type=menu` 的节点
经 `buildMenuTree` 组装。像「日志管理」`platform.log`、「配置管理」`platform.config`
这类**只有子项、自身无路由**的容器菜单（`path` 为空）会被 `pruneEmptyMenus` 剪枝约束：
若无子项则整节点剔除，避免渲染成空白项。前端 `layouts/menu.tsx` 的 `toMenuData` 做同样过滤。

> seed 采用"只补不覆盖"（已存在的 code 含软删一律跳过），菜单的父子挂靠只对
> `parent_id = 0`（尚未挂靠）的节点做一次幂等对齐（`alignMenuStructure`），
> 不会覆盖管理员在"菜单配置"页的调整。历史遗留的 `platform.audit` 会被自动挂到
> `platform.log` 之下，与新增的 `platform.loginlog` 同属「日志管理」。

---

## 六、统一响应与错误码

```json
{ "code": 0, "message": "ok", "data": { } }
```

- `code = 0` 为成功；非 0 时 HTTP 状态码由 `errcode.Code.HTTPStatus()` 映射
  （401 未登录、403 越权、409 冲突…），前端既能按 HTTP 语义处理也能按 `code` 精细提示。
- 分页统一 `{ list, total }`，入参 `page` / `page_size`（上限 200）。
- 错误码分段：`1xxxx` 框架级、`2xxxx` 认证与租户域、`3xxxx` 业务域（后续模块自扩展）。

---

## 七、接口清单（与后台契约，冻结）

| 方法 | 路径 | 权限点 |
| --- | --- | --- |
| POST | `/api/v1/auth/login` | 公开 |
| POST | `/api/v1/auth/refresh` | 公开 |
| POST | `/api/v1/auth/logout` | 登录 |
| GET | `/api/v1/auth/profile` | 登录 |
| GET | `/api/v1/auth/menus` | 登录（按权限过滤） |
| GET / POST | `/api/v1/platform/tenants` | `platform.tenant.list` / `.create` |
| GET / PUT | `/api/v1/platform/tenants/:id` | `platform.tenant.list` / `.update` |
| POST | `/api/v1/platform/tenants/:id/enable\|disable` | `platform.tenant.update` |
| GET / POST | `/api/v1/tenant/users` | `tenant.user.list` / `.create` |
| PUT / DELETE | `/api/v1/tenant/users/:id` | `tenant.user.update` / `.delete` |
| POST | `/api/v1/tenant/users/:id/reset-password` | `tenant.user.reset` |
| GET / POST | `/api/v1/tenant/roles` | `tenant.role.list` / `.create` |
| PUT / DELETE | `/api/v1/tenant/roles/:id` | `tenant.role.update` / `.delete` |
| GET | `/api/v1/tenant/permissions` | `tenant.role.list` |
| GET / PUT | `/api/v1/tenant/info` | `tenant.profile.view` / `.update` |
| GET | `/api/v1/platform/audit-logs` | `platform.audit.list` |
| GET | `/api/v1/platform/login-logs` | `platform.loginlog.list` |
| GET / POST | `/api/v1/platform/configs` | `platform.param.list` / `.create` |
| PUT / DELETE | `/api/v1/platform/configs/:id` | `platform.param.update` / `.delete` |
| GET / POST | `/api/v1/platform/dicts` | `platform.dict.list` / `.create` |
| PUT / DELETE | `/api/v1/platform/dicts/:id` | `platform.dict.update` / `.delete` |
| GET / POST | `/api/v1/platform/dicts/:id/items` | `platform.dict.list` / `.create` |
| PUT / DELETE | `/api/v1/platform/dicts/:id/items/:itemId` | `platform.dict.update` / `.delete` |
| GET | `/api/v1/platform/permissions/tree` | `platform.permission.list` |
| POST | `/api/v1/platform/permissions` | `platform.permission.create` |
| PUT / DELETE | `/api/v1/platform/permissions/:id` | `platform.permission.update` / `.delete` |
| GET | `/healthz` | 公开（探活） |

---

## 八、前端脚手架（对标 Vben Admin 的能力映射）

技术栈保持 React19 + Vite6 + AntD5 + ProComponents + zustand + react-query，
以 React 方式实现 Vben Admin 的高频底座能力（不引入 monorepo / i18n / 动态后端路由）。

### 8.1 环境变量（src/config 为唯一出口）

| 变量 | 说明 |
| --- | --- |
| `VITE_APP_TITLE` | 站点标题（index.html 模板 + 路由动态标题） |
| `VITE_APP_NAMESPACE` | localStorage 持久化键命名空间，默认 `pet-manager-admin` |
| `VITE_PORT` | dev 端口（`.env.development`） |
| `VITE_BASE_API` | API 前缀，默认 `/api/v1` |
| `VITE_PROXY_TARGET` | dev 代理目标，默认 `http://127.0.0.1:8088` |

所有持久化键经 `storageKey(name)` 收敛为 `<namespace>:<name>`（auth / preferences / tabs / token）。

### 8.2 路由即菜单 + 懒加载分包

- `AppRouteModule`（`router/types.ts`）：`key/path/title/icon/order/perm/anyOf/hideInMenu/affixTab/keepAlive/component`，
  模块文件放 `router/routes/modules/`，聚合时按 `order` 排序。**新增页面 = 一个 module 项 + 一个页面文件**。
- 菜单是 mixed 模式：优先后端 `/auth/menus`（按权限过滤），本地 modules 兜底；
  按钮级/页面级权限由 `AuthGuard` 按 `perm` / `anyOf` 拦截，落空渲染 403。
- 全部页面 `React.lazy` 懒加载；`vite.config.ts` 的 manualChunks 固定四条 vendor
  （pro-vendor / antd-vendor / react-vendor / utils-vendor，顺序敏感：pro 前缀必须先于 antd 匹配），
  每个业务页是独立小 chunk。
- `ChunkErrorBoundary` 捕获"发布后旧 chunk 失效"（Failed to fetch dynamically imported module），
  模块级开关保证只自动刷新一次。

### 8.3 多标签页与实例缓存（KeepAlive）

- `store/tabs.ts`：`TabItem { path,title,icon,affix }`，tabs 持久化（刷新后页签还在），
  支持关闭 / 关闭其他 / 左右关闭 / 全部关闭 / 刷新；身份首页与工作台为 affix 固定页签；
  页签集合会按当前可访问路由定期 `pruneByPaths`。
- `KeepAliveOutlet`：把 `useOutlet()` 元素镜像进 `useRef(Map)`，打开过的可缓存页用
  `display:none` 保活（组件状态、查询条件不丢）；切走再切回不重挂载。
  - 逐页开关：路由 meta `keepAlive: false` 即不缓存。
  - **已知约束**：缓存实例读取的 router params 是当前 URL，带 `:id` 的详情页必须设 `keepAlive: false`。
  - 页签"刷新"通过 `refreshTicks` 强制重挂载该页。
- 渲染期镜像必须用 ref 且幂等；zustand v5 选择器禁止返回新引用（如 `s.tabs.map()`），
  否则 useSyncExternalStore 快照每轮变化会无限重渲染。

### 8.4 偏好设置中心

`store/preferences.ts` 持久化字段：`themeMode / colorPrimary / layout / tabbarEnable /
keepAliveEnable / breadcrumbEnable / watermarkEnable / collapsed`。

- `main.tsx` 的 `ThemedRoot` 订阅后驱动 antd `ConfigProvider`（darkAlgorithm / 主题色）与
  `html.app-dark`；`PreferencesDrawer` 提供布局（侧边/顶栏/混合）、6 预设色 + 自定义色、
  暗色、水印（内容为昵称 + 租户名）、页签/面包屑/缓存开关与恢复默认。
- 顶栏 widgets（`HeaderActions`）：页签刷新、浏览器全屏（`useFullscreen`）、明暗切换、偏好抽屉。

### 8.5 其它底座约定

- antd 静态方法不消费主题上下文：axios 层的 message 由 `main.tsx` 内 `MessageBridge`
  注入 `App.useApp()` 实例（`bindMessageInstance`）；业务页统一用 `hooks/useMessage`。
- 切路由 NProgress 进度条（KeepAliveOutlet 启动、RouteTracker 在 chunk 加载完成后收尾，
  命中缓存走双 rAF，另设 4s 兜底）；`RouteTracker` 同步 `document.title = 页面名 - 站点名`。
- 登录页为左右分屏（≤900px 隐藏品牌区）；切换账号登录会 `resetTabs()` 清掉上个会话页签。
- 代码规范：`npm run lint`（ESLint9 flat config）/ `npm run format`（Prettier：无分号、单引号、100 列）。

---

## 九、本地启动

```bash
# 1. 起依赖
docker compose -f deploy/docker-compose.yml up -d

# 2. 起后端（首次启动自动建表 + 灌权限点字典 + 建平台管理员）
cd server && go run ./cmd/api -c ./configs/config.yaml
#    默认平台管理员口令取自 configs/config.yaml 的 seed.platform_admin_password
#    （生产务必用 PM_SEED_PLATFORM_ADMIN_PASSWORD 覆盖）

# 3. 起后台
cd admin && npm install && npm run dev   # http://localhost:5173，已代理 /api/v1 → 127.0.0.1:8088
```

首次体验路径：用平台管理员登录 → 租户管理新建租户（同步创建主账号）→
退出后用「租户账号」+ 租户编码登录 → 子账号管理建店员、角色授权分配权限点。