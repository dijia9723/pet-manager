# internal/modules/audit/policy.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [采集中间件](../../middleware/audit.md) · [audit.Service](service.md) · [Config 模型](../../model/config.md)

## 职责

审计**采集策略**：决定"哪些请求要落审计日志"。策略不落配置文件，而是落在平台参数表 `sys_config`（参数配置页直接治理、无需发版），Service 以 TTL 缓存原子替换，请求路径零 DB 开销。

## 参数键（group_key = `audit`）

| 参数键 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `audit.enabled` | bool | `true` | 审计总开关。停用后登录后的请求不再落审计库 |
| `audit.methods` | string | `POST,PUT,PATCH,DELETE` | 纳入审计的 HTTP 方法，逗号分隔，自动 trim/大写/去重；需要记录读请求可追加 `GET`；空或非法回退默认写方法 |
| `audit.exclude_routes` | json | `[]` | 排除的路由模板 JSON 数组，支持 `*` 通配（跨任意字符，含 `/`），如 `["/api/v1/platform/dicts/*"]`；非法 JSON 时忽略排除并告警 |

- 三个键由 [seed](../../seed/seed.md) 以"只补不覆盖"预置；管理员可在参数配置页改值、停用（status=disabled）或软删；
- 键缺失 / 停用 / 取值非法均回退默认值并 warn，**绝不因配置问题静默关掉审计**（methods 解析为空同样回退）；
- TTL `policyTTL=60s`：各实例最迟 60 秒感知改动，不做主动失效；加载有 3s 超时，配置库抖动时保留当前快照。

## 关键类型与函数

- `policy`：不可变快照（enabled + methods 集合 + 编译后的排除正则），整体 `atomic.Pointer[policy]` 替换；
- `defaultPolicy()`：内置默认，等价于中间件早期硬编码行为（登录后写操作全记），`NewService` 时先兜底；
- `loadPolicy(ctx, manager, log)`：查 `group_key='audit' AND status='enabled'` 的参数行构建快照；查库失败返回 nil，调用方保留旧策略；
- `mustBuildPolicy(...)`：`*` 经 `regexp.QuoteMeta` 后还原为 `.*`，整体锚定 `^...$`；
- `policy.allows(method, route)`：总开关 → 方法白名单 → 任一排除规则命中即不记。

## 约定与坑

- 排除规则匹配的是 gin 路由模板 `c.FullPath()`（稳定、不含具体 id），404 等未命中路由才回退实际路径；
- 修改审计参数本身（POST `/api/v1/platform/configs`）默认也会被审计——不要把参数配置接口加入排除清单，否则管理员可借此隐藏操作痕迹；
- 审计不记请求体的硬约定不受策略影响。
