# internal/model/consts.go — 枚举常量

> 脚手架导航：[回到开发者入口](../../../README.md)

| 常量组 | 取值 | 用途 |
| --- | --- | --- |
| 通用状态 | `enabled` / `disabled` | sys_* 与 t_* 的 status 列共用 |
| 租户状态 | `deleted` | 仅 sys_tenant 使用的待销毁态（两段式删除），见 [tenant.md](tenant.md)；任务自身的执行状态常量在 [job.md](job.md) |
| 用户类型 | `platform` / `tenant` | 区分平台管理员与租户账号，同时是权限点 scope 取值 |
| 权限点类型 | `menu` / `button` | menu 进菜单树，button 仅按钮级控制 |
| 参数值类型 | `string` / `number` / `bool` / `json` | 参数配置前端控件选择 |
| 登录结果 | `success` / `fail` | 登录日志业务语义值 |

注意登录日志的 `status` 是语义字符串，与审计日志用 HTTP 码 int 的口径刻意不同，不要强行统一。
