# internal/seed/seed.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [权限点模型](../model/permission.md) · [migrate](../migrate/migrate.md)

## 职责

首次启动的初始化数据，由 bootstrap 在迁移之后执行。两条原则：

platform.server.exec 的默认名称为「进入服务器 SSH 终端」，权限点同时保护交互终端与原命令接口；现有权限点名称按只补不覆盖规则保留。

1. **幂等**：可重复执行，已存在的数据不重建、不覆盖用户改动；
2. **权限点字典只补不覆盖**：启动只创建缺失的 code（含被软删的也跳过），名称/路径/图标/排序等由平台"菜单配置"页在线治理。

## Run 六步

1. `seedPermissions`：按声明式 `permDefs` 补齐权限点；
2. `alignMenuStructure`：对齐菜单父子结构；
3. `seedPlatformAdmin`：按 `SeedConfig` 建平台管理员（口令 bcrypt），生产用 `PM_SEED_PLATFORM_ADMIN_PASSWORD` 注入；
4. `seedAuditConfigs`：补齐审计采集策略参数（`audit.enabled` / `audit.methods` / `audit.exclude_routes`，group_key=`audit`），读取与热更新见 [audit/policy.md](../modules/audit/policy.md)；
5. `seedTenantConfigs`：补齐租户运营参数（当前仅 `tenant.purge_after_days`=7，type=number，控制删除租户后几天物理销毁），读取见 [tenant.Service](../modules/tenant/service.md)。

## permDef 声明格式

```go
{Code: "tenant.user.create", Name: "新建子账号", Type: model.PermTypeButton,
 Scope: model.UserTypeTenant, Sort: 12, Parent: "tenant.user"}
```

- `Type`：`menu` 进菜单树，`button` 只做按钮级控制；
- `Scope`：`platform` / `tenant`，决定菜单下发对象；
- 平台管理员虽按 `*` 通配授权，平台权限点仍要登记——平台菜单也从同一张字典表下发，保持菜单与权限同源。

### 平台侧新增权限点

- `platform.tenant.delete`（Sort 14）：租户删除/撤销删除共用一个权限点（两段式删除，见 [tenant handler](../modules/tenant/handler.md)）；
- `platform.tenant.destroy`（Sort 15）：单个租户立即物理销毁的独立按钮权限，跳过宽限期，不复用可撤销删除权限；
- 「任务管理」菜单组 Sort 20：`platform.job`（菜单，clock 图标，`/platform/jobs`）→ `.list/.update/.trigger/.log`（Sort 21–24），启停/编辑走 `.update`，手动执行走 `.trigger`。

### 租户侧配置管理权限点（Sort 40–50）

`tenant.config`（分组菜单）→ `tenant.param`（参数配置，`/tenant/configs`）与 `tenant.dict`（字典管理，`/tenant/dicts`），
按钮点为各自的 `.list/.create/.update/.delete`。这些新增权限点无需回填存量租户：
owner 角色在 [rbac.ResolvePermissions](../modules/rbac/service.md) 中动态拿到全部租户级权限点；
staff 角色需在"角色授权"页显式勾选（新租户的默认授权白名单见 rbac 的 `defaultStaffCodes`）。

## 参数种子约定

`auditConfigDefs`、`tenantConfigDefs` 与权限点同样遵循"只补不覆盖"：用 Unscoped 连软删行一起读，已存在（含被管理员在参数配置页软删）的键一律跳过，重启不复活、不覆盖线上值。

## 新增权限点检查单

1. 在 `permDefs` 追加定义（选好 Sort 空档与 Parent）；
2. 前端按钮用 `usePermission` 对 code 做可见性控制；
3. 写操作后由业务侧调 `InvalidateAllPermissions` 全量失效缓存；
4. 同步更新本文档与相关 [模块文档](../modules/rbac/module.md)。

## 信息通知权限

新增 platform.notification 菜单（/platform/notifications）和 list/create/publish/revoke 按钮；新增 platform.message、tenant.message 菜单（/messages）。tenant.message 下登记 tenant.notification.list/create/publish/revoke，分别控制本租户消息发布管理、草稿编辑、发布和撤回。种子仍只补不覆盖；主账号自动获得全部租户权限，子账号需角色授权。个人收件箱和本人的已读/处理操作无需额外业务角色权限。

## 统一存储与附件

Run 第六步 seedStoragePlans，首次创建 storage.plan 字典。新增 platform.storage、tenant.storage 菜单与 list/upload/delete，平台另有 allocate。主账号动态取得租户权限，子账号需角色授权，通知编辑权限可上传/选择附件。

## 托管权限点

新增平台服务器菜单/查看/创建/编辑/删除/SSH 执行，以及平台网站菜单/查看/分配/编辑/初始化/部署/文件查看/文件维护。租户仅有网站查看、文件查看、文件维护，不提供服务器或部署权限。
