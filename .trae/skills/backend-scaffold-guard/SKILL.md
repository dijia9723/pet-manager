---
name: backend-scaffold-guard
description: 维护 pet-manager 后端脚手架一致性。新增或修改后端接口、业务模块、数据表、权限点、配置项时使用，确保同步更新同名 md 脚手架文档、Swagger 注解并重新生成 docs，最后过门禁。纯前端或文档查阅任务不要使用。
---

# Backend Scaffold Guard

pet-manager 后端（`server/`，module `pet-manager/server`）的脚手架一致性守护。任何后端代码改动都必须同时满足"代码、文档、Swagger、门禁"四同步。入口文档是仓库根 `README.md`。

## 文档位置（重要）

- 代码在 `server/`，脚手架文档**统一在仓库根 `server_doc/`**，镜像 `server/` 的相对目录结构，同名对应：`server/internal/modules/foo/handler.go` ↔ `server_doc/internal/modules/foo/handler.md`；
- **禁止在 `server/` 代码目录内新建任何 `.md`**（`server/docs/` 是 swag 生成物 Go 包，不是文档目录，其中不得放 md）；
- 文档之间的相对链接按 `server_doc/` 内深度书写；文档引用真实代码文件时链接到 `../../server/...`（按实际深度）。

## 触发后的工作方式

先读对应源码文件，再读 `server_doc/` 镜像位置的同名 `.md`（没有则新建），改动完成后按下列清单补齐，不要遗漏。

## 清单

### 1. 新增 HTTP 接口

- handler 必须有 swag 注解：`@Summary` `@Tags`（用已有中文分组名）`@Accept/@Produce`、受保护接口加 `@Security BearerAuth`、`@Param`（body 引用具名 DTO，禁止匿名 struct）、`@Success 200 {object} response.Body`、`@Router`（路径不含 `/api/v1` 前缀，路径参数写 `{id}`）；
- 路由只在模块的 `Register` 中声明；平台接口挂 `r.Platform`，租户接口挂 `r.Tenant` 并紧跟 `r.Require("权限点")`；
- 重新生成文档（在 `server/` 目录）：
  - 有 make：`make swag`
  - Windows PowerShell 无 make：`& "$(go env GOPATH)\bin\swag.exe" init -d cmd/api,internal -g main.go -o docs --parseInternal`
  - CLI 缺失先 `go install github.com/swaggo/swag/cmd/swag@latest`；
- swag 库版本必须与 CLI 同大版本，否则生成的 docs.go 会编译失败（unknown field LeftDelim），用 `go get github.com/swaggo/swag@latest` 对齐；
- 生成物 `server/docs/` 已提交，禁止手改、禁止删除后不生成。

### 2. 新增业务模块

- 目录 `internal/modules/<mod>/`，Service 实现 `router.Module`（`Name()` 与 `Register(r)`），参考已有模块的 handler/service（或 module）分文件方式；
- 在 `internal/bootstrap/app.go` import 并加入 `modules` 切片；需要中间件回源能力时定义小接口（Resolver 模式），不要反向依赖；
- 在 `server_doc/internal/modules/<mod>/` 同步每个 `.go` 文件的同名 `.md`（文档目录镜像源码，不放在代码旁）。

### 3. 新增数据表

四处必做，缺一不可：

1. `internal/model/` 一表一文件，嵌入 `BaseModel`（纯关联表/内部版本表除外），写 `TableName()`，`sys_*` 平台表或 `t_*` 租户表命名；
2. 在 `internal/model/models.go` 的 `PlatformModels()` 或 `TenantModels()` 登记；
3. 有自然键唯一要求，在 `internal/migrate/migrate.go` 加 `CREATE UNIQUE INDEX ... WHERE deleted_at IS NULL` 部分唯一索引；租户表递增 `TenantSchemaVersion`；
4. 在 `server_doc/internal/model/` 新建同名 model `.md`；涉及菜单/按钮在 `seed.go` 的 `permDefs` 声明权限点。

### 4. 新增权限点 / 写操作缓存约定

- 权限点在 `internal/seed/seed.go` 的 `permDefs` 声明（只补不覆盖）；
- 角色/授权变更后调 `InvalidateTenantPermissions`；权限点字典写操作后必须 `InvalidateAllPermissions`；
- 权限点裸 SQL 查询必须显式 `deleted_at IS NULL`；越权返回 403 + 业务码 20005。

### 5. 文档同步规则

- 每个新增/改动的 `server/**/*.go`（含 `cmd/api/main.go`、`configs/`）都要在 `server_doc/` 的镜像路径有同名 `.md`（如 `server/cmd/api/main.go` → `server_doc/cmd/api/main.md`），内容含：职责、关键类型/函数、上下游协作、约定与坑；
- 根 `README.md` 第 7 节文档地图新增条目；
- 测试文件 `*_test.go` 可写简短同名 md 说明覆盖范围；
- 文档链接用相对路径；不要新建与现有文档重复的 md。

### 6. 硬约束（写代码时不可违反）

- 租户数据只走 `TenantTx` / `ctxkey.MustTenantDB(ctx)`，事务内 `set_config(..., true)`，禁止租户域直连 `Platform()`；
- 所有表继承 BaseModel（id/created_at/updated_at/deleted_at）；
- 响应只走 `response.OK/Fail/Abort`，错误走 `errcode` 体系；新增错误码同时补 messages 与 HTTPStatus 映射；
- viper 新增配置项必须在 `defaults()` SetDefault；
- 审计不记请求体；表单偏好抽屉等前端约定见 `doc/architecture.md`。

### 7. 交付前门禁（在 `server/` 执行，全部通过才算完）

```bash
gofmt -l -w ./internal ./cmd
go vet ./...
go build ./...
go test ./internal/... -count=1
```

改了 swag 注解还要验证服务起来后 `GET /swagger/doc.json` 返回 200 且路径数与接口数一致（Windows 用 `Invoke-WebRequest`/`Invoke-RestMethod`，不要用 curl 别名判断）。前端无改动时不跑前端门禁。
