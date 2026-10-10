# internal/modules/auth/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [handler.md](handler.md) · [jwtx](../../pkg/jwtx/jwt.md)

## 职责

登录态全部业务：登录、刷新、登出、个人信息（查询/改昵称）、修改自己的密码、菜单。

**登录入口只有一个**：`tenant_code` 非空走租户账号（loginTenant），空走平台管理员（loginPlatform），前端按返回的 `user_type` 决定落地路由。

## 核心类型

| 类型 | 说明 |
| --- | --- |
| `LoginInput` | username/password/tenant_code |
| `RefreshInput` | refresh_token（定义在 [handler.md](handler.md)） |
| `UpdateProfileInput` | 个人资料入参，目前仅 nickname（required,max=64） |
| `ChangePasswordInput` | old_password + new_password（6~64 位）；原密码不做长度约束，兼容历史短口令 |
| `UserInfo` | 前台识别身份/渲染权限的最小集合：含 roles、permissions |
| `LoginResult` | 双令牌 + expires_in + User |
| `MenuNode` | 侧边栏节点，key 即权限点编码 |
| `LoginRecorder` | 由 loginlog 模块实现的消费方接口（依赖反转） |

## 关键流程

- `Login`：bcrypt 校验 → 组装 UserInfo（租户用户需 JOIN 角色与权限点）→ `issue` 签双令牌 + 写 Redis 会话；
- `Refresh`：校验 refresh 类型 + 会话存活 → `revalidate` 回源确认账号/租户仍可用，再签新令牌；
- `Logout`：按 sid 删除会话，令牌立即失效；
- `UpdateProfile`：按 ctx 中的 user_type 分支更新 sys_platform_user / sys_tenant_user 的 nickname（账号表都在平台库，不走租户事务），返回最新 UserInfo；RowsAffected=0 视为未登录 401；
- `ChangePassword`：新旧相同/原口令不符返回 400（刻意不用 401，避免触发前端静默刷新令牌）；成功后 `RevokeUserSessions` 踢掉该账号全部会话（含当前会话），前端引导重新登录；踢下线 best-effort 只告警；
- `Menus`：按账号权限点从权限点字典组装菜单树（平台/租户各一套 scope）；
- 失败/成功都通过 `LoginRecorder` 记登录日志。

## 依赖

db.Manager、session.Store、jwtx.Manager、`middleware.PermissionResolver`（rbac Service）、LoginRecorder（loginlog Service）。
